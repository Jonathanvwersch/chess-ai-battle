export async function getGameState(): Promise<GameState> {
  const response = await fetch(`http://localhost:8080/api/chess-game-state`);

  if (!response.ok) {
    throw new Error("Failed to fetch chess state");
  }

  return await response.json();
}
