"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import type { Work } from "@/lib/api";
import { fetchWorks } from "@/lib/api";
import MasonryGrid from "./MasonryGrid";
import WorkCard from "./WorkCard";
import CardSkeleton from "./CardSkeleton";
import EmptyState from "./EmptyState";

const PAGE_SIZE = 20;

export default function FeedContainer({
  initialWorks,
  initialHasMore,
  initialField,
  initialOffset = 0,
  initialError = false,
}: {
  initialWorks: Work[];
  initialHasMore: boolean;
  initialField: string;
  initialOffset?: number;
  initialError?: boolean;
}) {
  const searchParams = useSearchParams();
  const field = searchParams.get("field") || "";

  const [works, setWorks] = useState<Work[]>(initialWorks);
  const [hasMore, setHasMore] = useState(initialHasMore);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(
    initialError ? "작품을 불러올 수 없습니다" : null
  );

  const sentinelRef = useRef<HTMLDivElement>(null);
  const offsetRef = useRef(initialOffset + initialWorks.length);
  const fieldRef = useRef(initialField);
  const abortRef = useRef<AbortController | null>(null);

  // Reload the full first page for the current field
  const reloadField = useCallback(
    async (targetField: string) => {
      // Abort any in-flight request
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;

      setLoading(true);
      setError(null);
      offsetRef.current = 0;

      try {
        const data = await fetchWorks({ field: targetField || undefined, limit: PAGE_SIZE, offset: 0 });
        if (controller.signal.aborted) return;
        setWorks(data.works);
        setHasMore(data.has_more);
        offsetRef.current = data.works.length;
      } catch {
        if (controller.signal.aborted) return;
        setError("작품을 불러올 수 없습니다");
        setWorks([]);
        setHasMore(false);
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    },
    []
  );

  // Refetch when field changes (skip if SSR already seeded the right field)
  useEffect(() => {
    if (field === fieldRef.current) return;
    fieldRef.current = field;
    reloadField(field);
  }, [field, reloadField]);

  // Infinite scroll — load next page (also tracked by abortRef so field
  // changes cancel in-flight loadMore requests and prevent stale appends)
  const loadMore = useCallback(async () => {
    if (loading || !hasMore) return;

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setError(null);

    try {
      const data = await fetchWorks({
        field: field || undefined,
        limit: PAGE_SIZE,
        offset: offsetRef.current,
      });
      if (controller.signal.aborted) return;
      setWorks((prev) => [...prev, ...data.works]);
      setHasMore(data.has_more);
      offsetRef.current += data.works.length;
    } catch {
      if (controller.signal.aborted) return;
      setError("추가 작품을 불러올 수 없습니다");
      setHasMore(false); // Stop auto-retry — user must click "다시 시도"
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, [field, loading, hasMore]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || error || loading) return; // Don't observe during error or active loading

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

  // Initial loading state
  if (loading && works.length === 0) {
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

  // Empty state
  if (!loading && works.length === 0 && !error) {
    return <EmptyState />;
  }

  return (
    <div className="px-6">
      {/* Error banner */}
      {error && (
        <div className="mb-4 p-4 bg-surface rounded-md border-l-3 border-error text-sm">
          {error}
          <button
            onClick={() => {
              setError(null);
              setHasMore(true); // Re-enable infinite scroll
              if (works.length === 0) {
                reloadField(field); // Full reload if no data at all
              }
              // If we have data, IntersectionObserver will re-trigger loadMore
            }}
            className="ml-3 text-accent hover:underline cursor-pointer"
          >
            다시 시도
          </button>
        </div>
      )}

      {/* Masonry grid */}
      <MasonryGrid>
        {works.map((work) => (
          <WorkCard key={work.id} work={work} />
        ))}
      </MasonryGrid>

      {/* Infinite scroll sentinel */}
      {hasMore && <div ref={sentinelRef} className="h-4" />}

      {/* Loading indicator for infinite scroll */}
      {loading && works.length > 0 && (
        <div className="flex justify-center py-8">
          <div className="w-6 h-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
        </div>
      )}

      {/* Load More fallback (noscript) — navigates to next page of results.
          Without JS, pagination is page-based (standard HTML behavior). */}
      <noscript>
        {hasMore && (
          <div className="flex justify-center py-8">
            <a
              href={`?${field ? `field=${field}&` : ""}offset=${offsetRef.current}`}
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
