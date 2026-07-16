"use client";

import { useCallback, useState, useTransition } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import type {
  SearchResult,
  SearchPinResult,
  SearchCreatorResult,
  SearchBoardResult,
  SearchTopTag,
} from "@/lib/api";
import { fetchSearch } from "@/lib/api";
import PinCard from "@/components/feed/PinCard";
import MasonryGrid from "@/components/feed/MasonryGrid";
import CardSkeleton from "@/components/feed/CardSkeleton";
import EmptyState from "@/components/feed/EmptyState";
import Link from "next/link";

const TABS = [
  { value: "all", label: "전체" },
  { value: "pins", label: "핀" },
  { value: "creators", label: "크리에이터" },
  { value: "boards", label: "보드" },
] as const;

const PAGE_SIZE = 20;

/** Convert SearchPinResult to the Pin shape that PinCard expects */
function toPinCardShape(p: SearchPinResult) {
  return {
    id: p.id,
    title: p.title,
    media_url: p.media_url,
    media_type: p.media_type,
    url: p.url,
    description: p.description,
    og_image: p.og_image,
    og_data: null,
    tags: [],
    created_at: p.created_at,
    creator: {
      id: p.creator_id,
      nickname: p.creator_nickname,
      avatar_url: p.creator_avatar_url,
    },
  };
}

export default function SearchClient({
  initialResults,
  query,
  initialType,
  initialTagIds,
}: {
  initialResults: SearchResult;
  query: string;
  initialType: string;
  initialTagIds: string[];
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [, startTransition] = useTransition();

  const activeType = searchParams.get("type") || initialType || "all";
  const activeTagIds = new Set(
    (searchParams.get("tag_ids") || initialTagIds.join(","))
      .split(",")
      .filter(Boolean)
  );

  const [pins, setPins] = useState<SearchPinResult[]>(
    initialResults.pins ?? []
  );
  const [creators, setCreators] = useState<SearchCreatorResult[]>(
    initialResults.creators ?? []
  );
  const [boards, setBoards] = useState<SearchBoardResult[]>(
    initialResults.boards ?? []
  );
  const [topTags, setTopTags] = useState<SearchTopTag[]>(
    initialResults.top_tags ?? []
  );
  const [hasMore, setHasMore] = useState(initialResults.has_more ?? false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [loadingTab, setLoadingTab] = useState(false);

  // Track offset for pagination
  const [offset, setOffset] = useState(() => {
    const initial = initialResults.pins?.length ?? 0;
    return Math.max(
      initial,
      initialResults.creators?.length ?? 0,
      initialResults.boards?.length ?? 0
    );
  });

  function updateUrl(updates: Record<string, string | undefined>) {
    const params = new URLSearchParams(searchParams.toString());
    for (const [key, value] of Object.entries(updates)) {
      if (value) {
        params.set(key, value);
      } else {
        params.delete(key);
      }
    }
    params.delete("offset"); // Always reset offset on filter change
    startTransition(() => {
      router.push(`/search?${params.toString()}`, { scroll: false });
    });
  }

  const handleTabChange = useCallback(
    async (type: string) => {
      updateUrl({ type: type === "all" ? undefined : type });

      setLoadingTab(true);
      try {
        const tagIds = [...activeTagIds].filter(Boolean);
        const data = await fetchSearch({
          q: query,
          type: type as "all" | "pins" | "creators" | "boards",
          tag_ids: tagIds.length > 0 ? tagIds : undefined,
          limit: PAGE_SIZE,
        });
        setPins(data.pins ?? []);
        setCreators(data.creators ?? []);
        setBoards(data.boards ?? []);
        if (data.top_tags) setTopTags(data.top_tags);
        setHasMore(data.has_more ?? false);
        setOffset(
          Math.max(
            data.pins?.length ?? 0,
            data.creators?.length ?? 0,
            data.boards?.length ?? 0
          )
        );
      } catch {
        // Keep existing state on error
      } finally {
        setLoadingTab(false);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [query, activeTagIds.size]
  );

  const handleTagToggle = useCallback(
    async (tagId: string) => {
      const next = new Set(activeTagIds);
      if (next.has(tagId)) {
        next.delete(tagId);
      } else {
        next.add(tagId);
      }
      const tagIdsStr =
        next.size > 0 ? [...next].join(",") : undefined;
      updateUrl({ tag_ids: tagIdsStr });

      setLoadingTab(true);
      try {
        const tagIds = [...next].filter(Boolean);
        const data = await fetchSearch({
          q: query,
          type: activeType as "all" | "pins" | "creators" | "boards",
          tag_ids: tagIds.length > 0 ? tagIds : undefined,
          limit: PAGE_SIZE,
        });
        setPins(data.pins ?? []);
        setCreators(data.creators ?? []);
        setBoards(data.boards ?? []);
        if (data.top_tags) setTopTags(data.top_tags);
        setHasMore(data.has_more ?? false);
        setOffset(
          Math.max(
            data.pins?.length ?? 0,
            data.creators?.length ?? 0,
            data.boards?.length ?? 0
          )
        );
      } catch {
        // Keep existing state on error
      } finally {
        setLoadingTab(false);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [query, activeType, activeTagIds.size]
  );

  const handleLoadMore = useCallback(async () => {
    if (loadingMore || !hasMore) return;
    setLoadingMore(true);

    try {
      const tagIds = [...activeTagIds].filter(Boolean);
      const data = await fetchSearch({
        q: query,
        type: activeType as "all" | "pins" | "creators" | "boards",
        tag_ids: tagIds.length > 0 ? tagIds : undefined,
        limit: PAGE_SIZE,
        offset,
      });
      if (data.pins) setPins((prev) => [...prev, ...data.pins!]);
      if (data.creators)
        setCreators((prev) => [...prev, ...data.creators!]);
      if (data.boards) setBoards((prev) => [...prev, ...data.boards!]);
      setHasMore(data.has_more ?? false);
      setOffset(
        (prev) =>
          prev +
          Math.max(
            data.pins?.length ?? 0,
            data.creators?.length ?? 0,
            data.boards?.length ?? 0
          )
      );
    } catch {
      // ignore
    } finally {
      setLoadingMore(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, activeType, activeTagIds.size, offset, loadingMore, hasMore]);

  const showPins = activeType === "all" || activeType === "pins";
  const showCreators = activeType === "all" || activeType === "creators";
  const showBoards = activeType === "all" || activeType === "boards";

  return (
    <div className="max-w-5xl mx-auto w-full px-6 py-6">
      {/* Search query display */}
      <h1 className="text-2xl font-bold tracking-tight mb-6 font-display">
        &ldquo;{query}&rdquo; 검색 결과
      </h1>

      {/* Category tabs */}
      <div
        role="group"
        aria-label="검색 카테고리"
        className="flex gap-2 mb-4 overflow-x-auto scrollbar-hide"
      >
        {TABS.map((tab) => (
          <button
            key={tab.value}
            onClick={() => handleTabChange(tab.value)}
            aria-pressed={activeType === tab.value}
            className={`px-4 py-1.5 rounded-full text-sm font-medium whitespace-nowrap transition-colors cursor-pointer ${
              activeType === tab.value
                ? "bg-text-primary text-bg"
                : "bg-transparent border border-border text-text-muted hover:border-text-muted hover:text-text-primary focus-visible:border-text-muted focus-visible:text-text-primary"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Top tags chips */}
      {topTags.length > 0 && (
        <div
          role="group"
          aria-label="인기 태그"
          className="flex gap-2 mb-6 overflow-x-auto scrollbar-hide pb-1"
        >
          {topTags.map((tag) => {
            const selected = activeTagIds.has(tag.id);
            return (
              <button
                key={tag.id}
                onClick={() => handleTagToggle(tag.id)}
                aria-pressed={selected}
                className={`px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap transition-colors cursor-pointer font-mono ${
                  selected
                    ? "bg-accent text-white"
                    : "bg-accent-subtle text-text-muted hover:bg-accent/20 focus-visible:bg-accent/20"
                }`}
              >
                {tag.name}
                <span className="ml-1 opacity-60">{tag.count}</span>
              </button>
            );
          })}
        </div>
      )}

      {/* Loading state */}
      {loadingTab && (
        <div role="status" aria-label="검색 결과 로딩 중">
          <MasonryGrid>
            {Array.from({ length: 8 }).map((_, i) => (
              <CardSkeleton key={i} />
            ))}
          </MasonryGrid>
        </div>
      )}

      {/* Results */}
      {!loadingTab && (
        <>
          {/* Pin results */}
          {showPins && pins.length > 0 && (
            <section className="mb-8">
              {activeType === "all" && (
                <h2 className="text-lg font-bold mb-4 font-display tracking-tight">핀</h2>
              )}
              <MasonryGrid>
                {pins.map((pin) => (
                  <PinCard key={pin.id} pin={toPinCardShape(pin)} />
                ))}
              </MasonryGrid>
            </section>
          )}

          {/* Creator results */}
          {showCreators && creators.length > 0 && (
            <section className="mb-8">
              {activeType === "all" && (
                <h2 className="text-lg font-bold mb-4 font-display tracking-tight">크리에이터</h2>
              )}
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
                {creators.map((creator) => (
                  <Link
                    key={creator.id}
                    href={`/creators/${creator.id}`}
                    className="flex items-center gap-4 p-4 bg-surface rounded-[10px] border border-transparent hover:-translate-y-0.5 hover:shadow-card-hover hover:border-accent focus-visible:-translate-y-0.5 focus-visible:shadow-card-hover focus-visible:border-accent focus-visible:outline-none transition-all duration-200"
                  >
                    {creator.avatar_url ? (
                      <img
                        src={creator.avatar_url}
                        alt=""
                        className="w-12 h-12 rounded-full object-cover shrink-0 border-2 border-border"
                        onError={(e) => {
                          e.currentTarget.style.display = "none";
                        }}
                      />
                    ) : (
                      <div className="w-12 h-12 rounded-full bg-gradient-to-br from-accent to-accent-hover shrink-0 border-2 border-border" />
                    )}
                    <div className="min-w-0">
                      <div className="text-sm font-semibold text-text-primary truncate">
                        {creator.nickname}
                      </div>
                      <div className="text-2xs text-text-dim font-mono">
                        {new Date(creator.created_at).toLocaleDateString(
                          "ko-KR"
                        )}
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </section>
          )}

          {/* Board results */}
          {showBoards && boards.length > 0 && (
            <section className="mb-8">
              {activeType === "all" && (
                <h2 className="text-lg font-bold mb-4 font-display tracking-tight">보드</h2>
              )}
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
                {boards.map((board) => (
                  <Link
                    key={board.id}
                    href={`/boards/${board.id}`}
                    className="block p-4 bg-surface rounded-[10px] border border-transparent hover:-translate-y-0.5 hover:shadow-card-hover hover:border-accent focus-visible:-translate-y-0.5 focus-visible:shadow-card-hover focus-visible:border-accent focus-visible:outline-none transition-all duration-200"
                  >
                    <div className="flex items-start gap-3">
                      <div className="w-10 h-10 rounded-[6px] bg-surface-elevated flex items-center justify-center shrink-0">
                        <svg
                          aria-hidden="true"
                          width="18"
                          height="18"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          className="text-text-dim"
                        >
                          <rect x="3" y="3" width="7" height="7" />
                          <rect x="14" y="3" width="7" height="7" />
                          <rect x="3" y="14" width="7" height="7" />
                          <rect x="14" y="14" width="7" height="7" />
                        </svg>
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="text-sm font-semibold text-text-primary truncate">
                          {board.name}
                        </div>
                        {board.description && (
                          <div className="text-xs text-text-muted mt-1 line-clamp-2">
                            {board.description}
                          </div>
                        )}
                        <div className="text-xs text-text-dim mt-1">
                          {board.creator_nickname}
                        </div>
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </section>
          )}

          {/* Empty state */}
          {pins.length === 0 &&
            creators.length === 0 &&
            boards.length === 0 && (
              <EmptyState
                message={`“${query}”에 대한 검색 결과가 없습니다`}
                description="다른 키워드로 검색해보세요"
              />
            )}

          {/* Load more */}
          {hasMore && (
            <div className="flex justify-center py-8">
              <button
                onClick={handleLoadMore}
                disabled={loadingMore}
                aria-busy={loadingMore}
                className="px-6 py-3 bg-surface border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent focus-visible:text-text-primary focus-visible:border-accent transition-colors cursor-pointer disabled:opacity-50"
              >
                {loadingMore ? (
                  <div
                    role="status"
                    aria-label="검색 결과 추가 로딩 중"
                    className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin mx-auto"
                  />
                ) : (
                  "더보기"
                )}
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
