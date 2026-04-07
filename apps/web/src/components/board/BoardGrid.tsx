"use client";

import Link from "next/link";
import type { Board } from "@/lib/api";
import BoardCover from "./BoardCover";

export default function BoardGrid({ boards }: { boards: Board[] }) {
  if (boards.length === 0) {
    return null;
  }

  return (
    <div>
      <h2
        className="text-lg font-bold tracking-tight mb-4"
        style={{ fontFamily: "'General Sans', sans-serif" }}
      >
        보드
      </h2>
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-4">
        {boards.map((board) => (
          <Link
            key={board.id}
            href={`/boards/${board.id}`}
            className="group"
          >
            <BoardCover images={board.cover_images} />
            <div className="mt-2">
              <div className="text-sm font-semibold text-text-primary group-hover:text-accent transition-colors truncate">
                {board.name}
              </div>
              <div
                className="text-xs text-text-dim"
                style={{ fontFamily: "'Geist Mono', monospace" }}
              >
                {board.pin_count} pins
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
