"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import type { Pin } from "@/lib/api";
import { fetchPins } from "@/lib/api";
import MasonryGrid from "./MasonryGrid";
import PinCard from "./PinCard";
import CardSkeleton from "./CardSkeleton";
import EmptyState from "./EmptyState";

const PAGE_SIZE = 20;

export default function FeedContainer({
  initialPins,
  initialHasMore,
  initialMediaType,
  initialOffset = 0,
  initialError = false,
}: {
  initialPins: Pin[];
  initialHasMore: boolean;
  initialMediaType: string;
  initialOffset?: number;
  initialError?: boolean;
}) {
  const searchParams = useSearchParams();
  const mediaType = searchParams.get("media_type") || "";

  const [pins, setPins] = useState<Pin[]>(initialPins);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(
    initialError ? "작품을 불러올 수 없습니다" : null
  );

  const sentinelRef = useRef<HTMLDivElement>(null);
  const offsetRef = useRef(initialOffset + initialPins.length);
  const mediaTypeRef = useRef(initialMediaType);
  const abortRef = useRef<AbortController | null>(null);

  const reloadMediaType = useCallback(
    async (targetType: string) => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;

      setLoading(true);
      setError(null);
      offsetRef.current = 0;

      try {
        const data = await fetchPins({ media_type: targetType || undefined, limit: PAGE_SIZE, offset: 0 });
        if (controller.signal.aborted) return;
        setPins(data.pins);
        setHasMore(data.has_more);
        offsetRef.current = data.pins.length;
      } catch {
        if (controller.signal.aborted) return;
        setError("작품을 불러올 수 없습니다");
        setPins([]);
        setHasMore(false);
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    },
    []
  );

  useEffect(() => {
    if (mediaType === mediaTypeRef.current) return;
    mediaTypeRef.current = mediaType;
    reloadMediaType(mediaType);
  }, [mediaType, reloadMediaType]);

  const loadMore = useCallback(async () => {
    if (loading || !hasMore) return;

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setError(null);

    try {
      const data = await fetchPins({
        media_type: mediaType || undefined,
        limit: PAGE_SIZE,
        offset: offsetRef.current,
      });
      if (controller.signal.aborted) return;
      setPins((prev) => [...prev, ...data.pins]);
      setHasMore(data.has_more);
      offsetRef.current += data.pins.length;
    } catch {
      if (controller.signal.aborted) return;
      setError("추가 작품을 불러올 수 없습니다");
      setHasMore(false);
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, [mediaType, loading, hasMore]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || error || loading) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          loadMore();
        }
      },
      { rootMargin: "200px" }
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [loadMore, error]);

  if (loading && pins.length === 0) {
    return (
      <div className="px-6">
        <MasonryGrid>
          {Array.from({ length: 8 }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </MasonryGrid>
      </div>
    );
  }

  if (!loading && pins.length === 0 && !error) {
    return <EmptyState />;
  }

  return (
    <div className="px-6">
      {error && (
        <div className="mb-4 p-4 bg-surface rounded-md border-l-3 border-error text-sm">
          {error}
          <button
            onClick={() => {
              setError(null);
              setHasMore(true);
              if (pins.length === 0) {
                reloadMediaType(mediaType);
              }
            }}
            className="ml-3 text-accent hover:underline cursor-pointer"
          >
            다시 시도
          </button>
        </div>
      )}

      <MasonryGrid>
        {pins.map((pin) => (
          <PinCard key={pin.id} pin={pin} />
        ))}
      </MasonryGrid>

      {hasMore && <div ref={sentinelRef} className="h-4" />}

      {loading && pins.length > 0 && (
        <div className="flex justify-center py-8">
          <div className="w-6 h-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
        </div>
      )}

      <noscript>
        {hasMore && (
          <div className="flex justify-center py-8">
            <a
              href={`?${mediaType ? `media_type=${mediaType}&` : ""}offset=${offsetRef.current}`}
              className="px-6 py-3 bg-surface border border-border rounded-full text-sm text-text-muted hover:text-text-primary transition-colors"
            >
              다음 페이지
            </a>
          </div>
        )}
      </noscript>
    </div>
  );
}
