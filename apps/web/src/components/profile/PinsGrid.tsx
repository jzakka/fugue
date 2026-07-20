"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { Pin } from "@/lib/api";
import { fetchPins } from "@/lib/api";
import PinCard from "@/components/feed/PinCard";
import CardSkeleton from "@/components/feed/CardSkeleton";
import EmptyState from "@/components/feed/EmptyState";

const PAGE_SIZE = 20;
const MEDIA_TYPES = [
  { value: "", label: "전체" },
  { value: "image", label: "이미지" },
  { value: "audio", label: "음악" },
  { value: "video", label: "영상" },
];

export default function PinsGrid({
  creatorId,
  initialPins,
  initialHasMore,
  initialOffset = 0,
}: {
  creatorId: string;
  initialPins: Pin[];
  initialHasMore: boolean;
  initialOffset?: number;
}) {
  const [pins, setPins] = useState(initialPins);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeType, setActiveType] = useState("");
  const offsetRef = useRef(initialOffset + initialPins.length);
  const sentinelRef = useRef<HTMLDivElement>(null);

  const reload = useCallback(
    async (mediaType: string) => {
      setLoading(true);
      setError(null);
      offsetRef.current = 0;
      try {
        const data = await fetchPins({
          creator_id: creatorId,
          media_type: mediaType || undefined,
          limit: PAGE_SIZE,
        });
        setPins(data.pins);
        setHasMore(data.has_more);
        offsetRef.current = data.pins.length;
      } catch {
        setError("작품을 불러올 수 없습니다");
        setPins([]);
        setHasMore(false);
      } finally {
        setLoading(false);
      }
    },
    [creatorId]
  );

  function handleTypeChange(mediaType: string) {
    setActiveType(mediaType);
    reload(mediaType);
  }

  const loadMore = useCallback(async () => {
    if (loading || !hasMore) return;
    setLoading(true);
    setError(null);
    try {
      const data = await fetchPins({
        creator_id: creatorId,
        media_type: activeType || undefined,
        limit: PAGE_SIZE,
        offset: offsetRef.current,
      });
      setPins((prev) => [...prev, ...data.pins]);
      setHasMore(data.has_more);
      offsetRef.current += data.pins.length;
    } catch {
      setError("추가 작품을 불러올 수 없습니다");
      setHasMore(false);
    } finally {
      setLoading(false);
    }
  }, [creatorId, activeType, loading, hasMore]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || error || loading) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) loadMore();
      },
      { rootMargin: "200px" }
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [loadMore, loading, error]);

  const noscriptParams = new URLSearchParams();
  noscriptParams.set("offset", String(offsetRef.current));

  return (
    <div>
      {/* Media type filter tabs */}
      <div className="flex gap-2 mb-6 overflow-x-auto pb-2 scrollbar-hide">
        {MEDIA_TYPES.map((mt) => (
          <button
            key={mt.value}
            onClick={() => handleTypeChange(mt.value)}
            aria-pressed={activeType === mt.value}
            className={`px-4 py-1.5 rounded-full text-sm font-medium whitespace-nowrap transition-colors cursor-pointer ${
              activeType === mt.value
                ? "bg-text-primary text-bg"
                : "bg-surface border border-border text-text-muted hover:text-text-primary hover:border-text-muted focus-visible:text-text-primary focus-visible:border-text-muted"
            }`}
          >
            {mt.label}
          </button>
        ))}
      </div>

      {/* Error */}
      {error && (
        <div
          role="alert"
          aria-live="polite"
          className="mb-4 p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error"
        >
          {error}
          <button
            onClick={() => reload(activeType)}
            className="ml-3 text-accent hover:underline focus-visible:underline cursor-pointer"
          >
            다시 시도
          </button>
        </div>
      )}

      {/* Grid */}
      {loading && pins.length === 0 ? (
        <div role="status" aria-label="작품 로딩 중" className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </div>
      ) : pins.length === 0 && !error ? (
        <EmptyState message="아직 등록된 작품이 없습니다" />
      ) : pins.length > 0 ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {pins.map((pin) => (
            <PinCard key={pin.id} pin={pin} />
          ))}
        </div>
      ) : null}

      {hasMore && <div ref={sentinelRef} className="h-4" />}
      {loading && pins.length > 0 && (
        <div
          role="status"
          aria-label="추가 핀 로딩 중"
          className="flex justify-center py-8"
        >
          <div className="w-6 h-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
        </div>
      )}
      <noscript>
        {hasMore && (
          <div className="flex justify-center py-8">
            <a
              href={`?${noscriptParams.toString()}`}
              className="px-6 py-3 bg-surface border border-border rounded-full text-sm text-text-muted hover:text-text-primary focus-visible:text-text-primary transition-colors"
            >
              다음 페이지
            </a>
          </div>
        )}
      </noscript>
    </div>
  );
}
