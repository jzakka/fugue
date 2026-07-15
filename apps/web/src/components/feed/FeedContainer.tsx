"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import type { Pin, PopularTag } from "@/lib/api";
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
  popularTags = [],
}: {
  initialPins: Pin[];
  initialHasMore: boolean;
  initialMediaType: string;
  initialOffset?: number;
  initialError?: boolean;
  popularTags?: PopularTag[];
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const mediaType = searchParams.get("media_type") || "";
  const tagsParam = searchParams.get("tags") || "";

  const [pins, setPins] = useState<Pin[]>(initialPins);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(
    initialError ? "작품을 불러올 수 없습니다" : null
  );

  const sentinelRef = useRef<HTMLDivElement>(null);
  const offsetRef = useRef(initialOffset + initialPins.length);
  const mediaTypeRef = useRef(initialMediaType);
  const tagsRef = useRef(tagsParam);
  const abortRef = useRef<AbortController | null>(null);

  // Convert tag slugs to IDs using popularTags
  const slugToId = new Map(popularTags.map((t) => [t.slug, t.id]));
  function resolveTagIds(slugs: string): string[] {
    if (!slugs) return [];
    return slugs
      .split(",")
      .filter(Boolean)
      .map((s) => slugToId.get(s))
      .filter((id): id is string => !!id);
  }

  const reloadPins = useCallback(
    async (targetMediaType: string, targetTags: string) => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;

      setLoading(true);
      setError(null);
      offsetRef.current = 0;

      const tagIds = resolveTagIds(targetTags);

      try {
        const data = await fetchPins({
          media_type: targetMediaType || undefined,
          tag_ids: tagIds.length > 0 ? tagIds : undefined,
          limit: PAGE_SIZE,
          offset: 0,
        });
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [popularTags]
  );

  useEffect(() => {
    if (mediaType === mediaTypeRef.current && tagsParam === tagsRef.current) return;
    mediaTypeRef.current = mediaType;
    tagsRef.current = tagsParam;
    reloadPins(mediaType, tagsParam);
  }, [mediaType, tagsParam, reloadPins]);

  const loadMore = useCallback(async () => {
    if (loading || !hasMore) return;

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setError(null);

    const tagIds = resolveTagIds(tagsParam);

    try {
      const data = await fetchPins({
        media_type: mediaType || undefined,
        tag_ids: tagIds.length > 0 ? tagIds : undefined,
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mediaType, tagsParam, loading, hasMore, popularTags]);

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
      <div role="status" aria-label="작품 로딩 중" className="px-6">
        <MasonryGrid>
          {Array.from({ length: 8 }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </MasonryGrid>
      </div>
    );
  }

  if (!loading && pins.length === 0 && !error) {
    return (
      <EmptyState message="이 분야의 작품이 아직 없습니다">
        <button
          onClick={() => router.push("/", { scroll: false })}
          className="text-accent text-sm hover:underline focus-visible:underline cursor-pointer"
        >
          전체 보기
        </button>
      </EmptyState>
    );
  }

  // Build noscript href preserving both media_type and tags
  const noscriptParams = new URLSearchParams();
  if (mediaType) noscriptParams.set("media_type", mediaType);
  if (tagsParam) noscriptParams.set("tags", tagsParam);
  noscriptParams.set("offset", String(offsetRef.current));

  return (
    <div className="px-6">
      {error && (
        <div
          role="alert"
          aria-live="polite"
          className="mb-4 p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error"
        >
          {error}
          <button
            onClick={() => {
              setError(null);
              setHasMore(true);
              if (pins.length === 0) {
                reloadPins(mediaType, tagsParam);
              }
            }}
            className="ml-3 text-accent hover:underline focus-visible:underline cursor-pointer"
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
        <div
          role="status"
          aria-label="추가 작품 로딩 중"
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
