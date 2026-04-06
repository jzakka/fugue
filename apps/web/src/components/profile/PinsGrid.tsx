"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { Pin } from "@/lib/api";
import { fetchPins } from "@/lib/api";
import PinCard from "@/components/feed/PinCard";
import CardSkeleton from "@/components/feed/CardSkeleton";

const PAGE_SIZE = 20;
const FIELDS = ["전체", "음악", "일러스트", "영상", "글", "사진", "기타"];

export default function PinsGrid({
  creatorId,
  initialPins,
  initialHasMore,
}: {
  creatorId: string;
  initialPins: Pin[];
  initialHasMore: boolean;
}) {
  const [pins, setPins] = useState(initialPins);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [loading, setLoading] = useState(false);
  const [activeField, setActiveField] = useState("전체");
  const offsetRef = useRef(initialPins.length);
  const sentinelRef = useRef<HTMLDivElement>(null);

  const reload = useCallback(
    async (field: string) => {
      setLoading(true);
      offsetRef.current = 0;
      try {
        const data = await fetchPins({
          creator_id: creatorId,
          field: field === "전체" ? undefined : field,
          limit: PAGE_SIZE,
        });
        setPins(data.pins);
        setHasMore(data.has_more);
        offsetRef.current = data.pins.length;
      } catch {
        setPins([]);
        setHasMore(false);
      } finally {
        setLoading(false);
      }
    },
    [creatorId]
  );

  function handleFieldChange(field: string) {
    setActiveField(field);
    reload(field);
  }

  const loadMore = useCallback(async () => {
    if (loading || !hasMore) return;
    setLoading(true);
    try {
      const data = await fetchPins({
        creator_id: creatorId,
        field: activeField === "전체" ? undefined : activeField,
        limit: PAGE_SIZE,
        offset: offsetRef.current,
      });
      setPins((prev) => [...prev, ...data.pins]);
      setHasMore(data.has_more);
      offsetRef.current += data.pins.length;
    } catch {
      setHasMore(false);
    } finally {
      setLoading(false);
    }
  }, [creatorId, activeField, loading, hasMore]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || loading) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) loadMore();
      },
      { rootMargin: "200px" }
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [loadMore, loading]);

  return (
    <div>
      {/* Field filter tabs */}
      <div className="flex gap-2 mb-6 overflow-x-auto pb-2">
        {FIELDS.map((field) => (
          <button
            key={field}
            onClick={() => handleFieldChange(field)}
            className={`px-4 py-2 rounded-full text-sm whitespace-nowrap transition-colors cursor-pointer ${
              activeField === field
                ? "bg-accent text-white"
                : "bg-surface border border-border text-text-muted hover:text-text-primary hover:border-accent"
            }`}
          >
            {field}
          </button>
        ))}
      </div>

      {/* Grid */}
      {loading && pins.length === 0 ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </div>
      ) : pins.length === 0 ? (
        <div className="text-center py-16 text-text-dim">
          <p className="text-lg">아직 등록된 작품이 없습니다</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {pins.map((pin) => (
            <PinCard key={pin.id} pin={pin} />
          ))}
        </div>
      )}

      {hasMore && <div ref={sentinelRef} className="h-4" />}
      {loading && pins.length > 0 && (
        <div className="flex justify-center py-8">
          <div className="w-6 h-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
        </div>
      )}
    </div>
  );
}
