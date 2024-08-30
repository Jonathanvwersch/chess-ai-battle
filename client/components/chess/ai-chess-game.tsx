"use client";
import React from "react";
import { Card, CardContent, CardFooter, CardHeader } from "../common/card";
import ChessBoard from "./chess-board";
import OpenAI from "@/assets/icons/open-ai";
import ClaudeAI from "@/assets/icons/claude";
import { useSSE } from "@/hooks/use-sse";

interface GameState {
  id: number;
  fen: string;
  lastMove: string;
  lastPlayer: string;
  createdAt: string;
  moveHistory: string[];
  gameOutcome: string | null;
}

const AIChessGame = () => {
  const { data: gameState } = useSSE<GameState>({
    url: "http://localhost:8080/api/chess-game-state",
    initialState: null,
  });
  console.log(gameState);

  return (
    <Card className="w-full max-w-[500px] bg-background">
      <CardContent className="pt-6">
        <CardHeader className="pt-0 flex justify-center items-center">
          <div
            className={gameState?.lastPlayer === "openai" ? "animate-spin" : ""}
          >
            <ClaudeAI />
          </div>
        </CardHeader>
        <div className="aspect-square w-full">
          <ChessBoard gameState={gameState} />
        </div>
        <CardFooter className="pt-6 pb-0 flex justify-center items-center">
          <div
            className={
              gameState?.lastPlayer === "anthropic" ? "animate-spin" : ""
            }
          >
            <OpenAI />
          </div>
        </CardFooter>
      </CardContent>
    </Card>
  );
};

export default AIChessGame;
