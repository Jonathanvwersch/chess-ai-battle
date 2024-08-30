"use client";

import { Chess } from "chess.ts";
import React from "react";
import { Chessboard } from "react-chessboard";

type Props = Readonly<{ gameState: GameState | null }>;

const ChessBoard = ({ gameState }: Props) => {
  return (
    <Chessboard
      position={gameState?.fen || new Chess().fen()}
      customBoardStyle={{
        borderRadius: "0.5rem",
        boxShadow:
          "0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)",
      }}
      customDarkSquareStyle={{ backgroundColor: "#4b5563" }}
      customLightSquareStyle={{ backgroundColor: "#9ca3af" }}
      isDraggablePiece={() => false}
    />
  );
};

export default ChessBoard;
