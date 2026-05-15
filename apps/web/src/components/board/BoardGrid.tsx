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
      <h2 className="text-lg font-bold tracking-tight mb-4 font-display">
        보드
      </h2>
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-4">
        {boards.map((board) => (
          <Link
            key={board.id}
            href={`/boards/${board.id}`}
            className="group block transition-transform duration-200 hover:-translate-y-0.5 focus-visible:-translate-y-0.5 focus-visible:outline-none"
          >
            <BoardCover images={board.cover_images} />
            <div className="mt-2">
              <div className="text-sm font-semibold text-text-primary group-hover:text-accent transition-colors truncate">
                {board.name}
              </div>
              <div className="text-xs text-text-dim font-mono">
                {board.pin_count} pins
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
