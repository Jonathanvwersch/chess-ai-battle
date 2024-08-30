package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/notnil/chess"
	"github.com/sashabaranov/go-openai"
)

type GameState struct {
	ID           int       `json:"id"`
	FEN          string    `json:"fen"`
	LastMove     string    `json:"lastMove"`
	LastPlayer   string    `json:"lastPlayer"`
	CreatedAt    time.Time `json:"createdAt"`
	MoveHistory  []string  `json:"moveHistory"`
	GameOutcome  string    `json:"gameOutcome"`
}

var (
	db               *sql.DB
	game             *chess.Game
	openAIClient     *openai.Client
	moveHistory      []string
	gameStateChannel chan *GameState
	lastMove         string
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	initDB()
	initGame()
	initOpenAI()
	checkAnthropicKey()

	gameStateChannel = make(chan *GameState, 100)

	app := fiber.New()
	setupCORS(app)
	setupRoutes(app)

	go playGame()

	startServer(app)
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./chess.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS game_states (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fen TEXT NOT NULL,
		last_move TEXT NOT NULL,
		last_player TEXT NOT NULL,
		move_history TEXT NOT NULL,
		game_outcome TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatal(err)
	}
}

func initGame() {
	game = chess.NewGame()
	moveHistory = []string{}
	lastMove = ""
}

func initOpenAI() {
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		log.Fatal("OPENAI_API_KEY is not set in the environment")
	}
	openAIClient = openai.NewClient(openAIKey)
}

func checkAnthropicKey() {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set in the environment")
	}
}

func setupCORS(app *fiber.App) {
	app.Use(cors.New(cors.Config{
		AllowOrigins:     os.Getenv("ALLOWED_ORIGINS"),
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH",
		AllowHeaders:     "Origin, Content-Type, Accept",
		AllowCredentials: true,
	}))
}

func setupRoutes(app *fiber.App) {
	app.Get("/api/chess-game-state", handleSSEGameState)
}

func startServer(app *fiber.App) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server is running on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

func handleSSEGameState(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	log.Println("New SSE connection established")

	ctx := c.Context()
	if ctx == nil {
		return fiber.ErrInternalServerError
	}

	done := make(chan struct{})
	closeOnce := sync.Once{}

	go func() {
		<-ctx.Done()
		closeOnce.Do(func() {
			close(done)
		})
	}()

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		for {
			select {
			case gameState, ok := <-gameStateChannel:
				if !ok {
					log.Println("Game state channel closed")
					return
				}
				data, err := json.Marshal(gameState)
				if err != nil {
					log.Printf("Error marshaling game state: %v", err)
					continue
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					log.Printf("Error writing SSE data: %v", err)
					return
				}
				if err := w.Flush(); err != nil {
					log.Printf("Error flushing SSE writer: %v", err)
					return
				}
				log.Println("Sent game state update to SSE client")
			case <-done:
				log.Println("SSE connection closed")
				return
			}
		}
	})

	return nil
}

func playGame() {
	players := []string{"openai", "anthropic"}
	playerIndex := 0
	moveCount := 0

	for game.Outcome() == chess.NoOutcome {
		currentPlayer := players[playerIndex]
		log.Printf("Starting move %d for player %s", moveCount+1, currentPlayer)

		if err := makeMove(currentPlayer); err != nil {
			log.Printf("Error making move for %s: %v", currentPlayer, err)
			time.Sleep(5 * time.Second)
			continue
		}

		playerIndex = (playerIndex + 1) % 2
		moveCount++

		log.Printf("Completed move %d. Current FEN: %s", moveCount, game.FEN())

		currentState := &GameState{
			FEN:          game.FEN(),
			LastMove:     lastMove,
			LastPlayer:   currentPlayer,
			MoveHistory:  moveHistory,
			GameOutcome:  game.Outcome().String(),
			CreatedAt:    time.Now(),
		}
		select {
		case gameStateChannel <- currentState:
			log.Println("Sent game state update to SSE channel")
		default:
			log.Println("No SSE clients connected, skipped sending update")
		}

		time.Sleep(3 * time.Second)
	}

	log.Println("Game over. Outcome:", game.Outcome())

	if err := saveGameState(players[playerIndex], lastMove, game.Outcome().String()); err != nil {
		log.Printf("Error saving final game state: %v", err)
	}

	log.Println("Starting a new game...")
	initGame()
	playGame()
}

func makeMove(player string) error {
	fen := game.FEN()
	log.Printf("Getting move for %s. Current FEN: %s", player, fen)

	var lastInvalidMove string
	for attempts := 0; attempts < 3; attempts++ {
		move, err := getMove(player, fen, attempts > 0, lastInvalidMove)
		if err != nil {
			log.Printf("Error getting move from %s (attempt %d): %v", player, attempts+1, err)
			continue
		}
		log.Printf("Received move from %s: %s", player, move)

		if err := validateAndApplyMove(move); err != nil {
			log.Printf("Invalid move by %s: %s - %v", player, move, err)
			lastInvalidMove = move
			continue
		}

		log.Printf("Applied move %s for %s", move, player)
		lastMove = move
		moveHistory = append(moveHistory, move)
		return saveGameState(player, move, "")
	}

	return fmt.Errorf("failed to get a valid move after 3 attempts")
}

func getMove(player, fen string, isRetry bool, lastInvalidMove string) (string, error) {
	switch player {
	case "openai":
		return getOpenAIMove(fen, isRetry, lastInvalidMove)
	case "anthropic":
		return getAnthropicMove(fen, isRetry, lastInvalidMove)
	default:
		return "", fmt.Errorf("unknown player: %s", player)
	}
}

func validateAndApplyMove(move string) error {
	err := game.MoveStr(move)
	if err != nil {
		return err
	}
	return nil
}

func saveGameState(player, move, outcome string) error {
	if db == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	gameState := &GameState{
		FEN:          game.FEN(),
		LastMove:     move,
		LastPlayer:   player,
		MoveHistory:  moveHistory,
		GameOutcome:  outcome,
		CreatedAt:    time.Now(),
	}

	_, err := db.Exec(`INSERT INTO game_states (fen, last_move, last_player, move_history, game_outcome) VALUES (?, ?, ?, ?, ?)`,
		gameState.FEN, gameState.LastMove, gameState.LastPlayer, strings.Join(gameState.MoveHistory, ","), gameState.GameOutcome)
	if err != nil {
		return fmt.Errorf("error saving game state: %v", err)
	}

	log.Printf("%s played: %s", player, move)

	select {
	case gameStateChannel <- gameState:
		log.Println("Sent game state update to SSE channel after save")
	default:
		log.Println("No SSE clients connected, skipped sending update after save")
	}

	return nil
}

func getOpenAIMove(fen string, isRetry bool, lastInvalidMove string) (string, error) {
	ctx := context.Background()
	retryMessage := ""
	if isRetry {
		retryMessage = fmt.Sprintf("Your previous move '%s' was invalid. Please try again with a valid move. ", lastInvalidMove)
	}
	prompt := fmt.Sprintf(`%sYou are playing a game of chess. The current board state in FEN notation is:
%s

The move history (in algebraic notation) is:
%s

Please provide your next move in standard algebraic notation (e.g., "e4", "Nf3", "O-O").
Your move must be legal according to the current board state and chess rules.
Respond with only the move, nothing else.`, retryMessage, fen, strings.Join(moveHistory, ", "))

	resp, err := openAIClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: "You are an expert chess player. Provide only valid chess moves in standard algebraic notation."},
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
			MaxTokens: 10,
		},
	)

	if err != nil {
		return "", err
	}

	move := strings.TrimSpace(resp.Choices[0].Message.Content)
	return move, nil
}

func getAnthropicMove(fen string, isRetry bool, lastInvalidMove string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"
	retryMessage := ""
	if isRetry {
		retryMessage = fmt.Sprintf("Your previous move '%s' was invalid. Please try again with a valid move. ", lastInvalidMove)
	}
	prompt := fmt.Sprintf(`%sYou are playing a game of chess. The current board state in FEN notation is:
%s

The move history (in algebraic notation) is:
%s

Please provide your next move in standard algebraic notation (e.g., "e4", "Nf3", "O-O").
Your move must be legal according to the current board state and chess rules.
Respond with only the move, nothing else.`, retryMessage, fen, strings.Join(moveHistory, ", "))

	requestBody, err := json.Marshal(map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 10,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", fmt.Errorf("error marshaling request body: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", os.Getenv("ANTHROPIC_API_KEY"))
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	err = json.Unmarshal(body, &anthropicResp)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling response: %v", err)
	}

	if len(anthropicResp.Content) == 0 || anthropicResp.Content[0].Text == "" {
		return "", fmt.Errorf("empty completion from Anthropic")
	}

	move := strings.TrimSpace(anthropicResp.Content[0].Text)
	return move, nil
}