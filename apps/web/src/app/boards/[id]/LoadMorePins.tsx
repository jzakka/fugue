"use client";

import { useState } from "react";
import { fetchBoard } from "@/lib/api";
import type { Pin } from "@/lib/api";
import PinCard from "@/components/feed/PinCard";

export default function LoadMorePins({
  boardId,
  initialCount,
}: {
  boardId: string;
  initialCount: number;
}) {
  const [pins, setPins] = useState<Pin[]>([]);
  const [loading, setLoading] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [offset, setOffset] = useState(initialCount);

  async function loadMore() {
    setLoading(true);
    try {
      const data = await fetchBoard(boardId, { limit: 20, offset });
      setPins((prev) => [...prev, ...data.pins]);
      setOffset((prev) => prev + data.pins.length);
      setHasMore(data.has_more);
    } catch {
      // Silently fail
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      {pins.map((pin) => (
        <PinCard key={pin.id} pin={pin} />
      ))}
      {hasMore && (
        <div className="col-span-full flex justify-center py-8">
          <button
            onClick={loadMore}
            disabled={loading}
            aria-busy={loading}
            className="px-6 py-3 bg-surface border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent focus-visible:text-text-primary focus-visible:border-accent transition-colors disabled:opacity-50 cursor-pointer"
          >
            {loading ? (
              <div
                role="status"
                aria-label="추가 핀 로딩 중"
                className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mx-auto"
              />
            ) : (
              "더보기"
            )}
          </button>
        </div>
      )}
    </>
  );
}
