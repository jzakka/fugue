"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import type { SearchResult } from "@/lib/api";
import { fetchSearch } from "@/lib/api";
import { getMediaTypeLabel } from "@/lib/card-type";

const STORAGE_KEY = "fugue_recent_searches";
const MAX_RECENT = 5;

function getRecentSearches(): string[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveRecentSearch(query: string) {
  const list = getRecentSearches().filter((s) => s !== query);
  list.unshift(query);
  if (list.length > MAX_RECENT) list.pop();
  localStorage.setItem(STORAGE_KEY, JSON.stringify(list));
}

function removeRecentSearch(query: string) {
  const list = getRecentSearches().filter((s) => s !== query);
  localStorage.setItem(STORAGE_KEY, JSON.stringify(list));
}

export default function SearchBar() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [query, setQuery] = useState(searchParams.get("q") || "");
  const [open, setOpen] = useState(false);
  const [results, setResults] = useState<SearchResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [recentSearches, setRecentSearches] = useState<string[]>([]);

  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  // Load recent searches on mount
  useEffect(() => {
    setRecentSearches(getRecentSearches());
  }, []);

  // Click outside to close
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const doSearch = useCallback(async (q: string) => {
    if (q.length < 3) {
      setResults(null);
      setLoading(false);
      return;
    }

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    try {
      const data = await fetchSearch({ q, type: "all", limit: 5 });
      if (controller.signal.aborted) return;
      setResults(data);
    } catch {
      if (controller.signal.aborted) return;
      setResults(null);
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, []);

  function handleInputChange(value: string) {
    setQuery(value);
    setOpen(true);

    if (debounceRef.current) clearTimeout(debounceRef.current);

    if (value.length < 3) {
      setResults(null);
      setLoading(false);
      return;
    }

    setLoading(true);
    debounceRef.current = setTimeout(() => {
      doSearch(value);
    }, 300);
  }

  function handleSubmit(q?: string) {
    const searchQuery = (q ?? query).trim();
    if (!searchQuery) return;
    saveRecentSearch(searchQuery);
    setRecentSearches(getRecentSearches());
    setOpen(false);
    router.push(`/search?q=${encodeURIComponent(searchQuery)}`);
  }

  function handleDeleteRecent(q: string, e: React.MouseEvent) {
    e.stopPropagation();
    removeRecentSearch(q);
    setRecentSearches(getRecentSearches());
  }

  function handleFocus() {
    setOpen(true);
    setRecentSearches(getRecentSearches());
  }

  const showRecent =
    open && query.length < 3 && recentSearches.length > 0;
  const showResults =
    open && query.length >= 3 && (results || loading);
  const showDropdown = showRecent || showResults;

  const hasPins = (results?.pins?.length ?? 0) > 0;
  const hasCreators = (results?.creators?.length ?? 0) > 0;
  const hasBoards = (results?.boards?.length ?? 0) > 0;
  const hasAnyResults = hasPins || hasCreators || hasBoards;

  return (
    <div className="flex-1 max-w-md relative" ref={containerRef}>
      <span className="absolute left-3.5 top-1/2 -translate-y-1/2 text-sm opacity-40 pointer-events-none">
        <svg
          aria-hidden="true"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
      </span>
      <input
        ref={inputRef}
        type="text"
        value={query}
        onChange={(e) => handleInputChange(e.target.value)}
        onFocus={handleFocus}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            handleSubmit();
          }
          if (e.key === "Escape") {
            setOpen(false);
            inputRef.current?.blur();
          }
        }}
        placeholder="작품, 크리에이터, 태그 검색..."
        aria-label="검색"
        className="w-full py-2.5 pl-10 pr-4 bg-surface border border-border rounded-full text-sm text-text-primary placeholder:text-text-dim outline-none focus:border-accent transition-colors"
      />

      {/* Dropdown */}
      {showDropdown && (
        <div className="absolute top-full left-0 right-0 mt-2 bg-surface-elevated border border-border rounded-[16px] overflow-hidden z-[60]">
          {/* Recent searches */}
          {showRecent && (
            <div className="p-3">
              <div className="text-xs text-text-dim mb-2 px-1">최근 검색</div>
              {recentSearches.map((q) => (
                <div
                  key={q}
                  className="flex items-center justify-between px-3 py-2 rounded-[10px] hover:bg-surface-hover focus-within:bg-surface-hover group transition-colors"
                >
                  <button
                    type="button"
                    onClick={() => {
                      setQuery(q);
                      handleSubmit(q);
                    }}
                    className="flex items-center gap-2 min-w-0 flex-1 cursor-pointer text-left"
                  >
                    <svg
                      aria-hidden="true"
                      width="14"
                      height="14"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="text-text-dim shrink-0"
                    >
                      <polyline points="1 4 1 10 7 10" />
                      <path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10" />
                    </svg>
                    <span className="text-sm text-text-primary truncate">
                      {q}
                    </span>
                  </button>
                  <button
                    type="button"
                    onClick={(e) => handleDeleteRecent(q, e)}
                    aria-label="최근 검색에서 제거"
                    className="opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100 text-text-dim hover:text-text-primary transition-opacity p-1 cursor-pointer"
                  >
                    <svg
                      aria-hidden="true"
                      width="12"
                      height="12"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <line x1="18" y1="6" x2="6" y2="18" />
                      <line x1="6" y1="6" x2="18" y2="18" />
                    </svg>
                  </button>
                </div>
              ))}
            </div>
          )}

          {/* Search results */}
          {showResults && (
            <div className="p-3">
              {loading && !hasAnyResults && (
                <div
                  role="status"
                  aria-label="검색 결과 로딩 중"
                  className="flex justify-center py-4"
                >
                  <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin" />
                </div>
              )}

              {!loading && query.length >= 3 && !hasAnyResults && (
                <div
                  role="status"
                  aria-live="polite"
                  className="py-4 text-center text-sm text-text-dim"
                >
                  검색 결과가 없습니다
                </div>
              )}

              {/* Pins */}
              {hasPins && (
                <div className="mb-2">
                  <div className="text-xs text-text-dim mb-1 px-1">핀</div>
                  {results!.pins!.slice(0, 3).map((pin) => (
                    <Link
                      key={pin.id}
                      href={`/pins/${pin.id}`}
                      onClick={() => setOpen(false)}
                      className="flex items-center gap-3 px-3 py-2 rounded-[10px] hover:bg-surface-hover focus-visible:bg-surface-hover cursor-pointer transition-colors"
                    >
                      {pin.og_image ? (
                        <img
                          src={pin.og_image}
                          alt=""
                          className="w-8 h-8 rounded-[6px] object-cover shrink-0"
                          onError={(e) => {
                            // Cached og_image objects may be evicted by the
                            // harvester image cache TTL; degrade by hiding
                            // the broken image so the layout doesn't show
                            // the browser's broken-image glyph.
                            e.currentTarget.style.display = "none";
                          }}
                        />
                      ) : (
                        <div aria-hidden="true" className="w-8 h-8 rounded-[6px] bg-surface shrink-0 flex items-center justify-center text-text-dim text-xs">
                          {pin.media_type === "audio" ? "♪" : pin.media_type === "video" ? "▶" : "◻"}
                        </div>
                      )}
                      <div className="min-w-0 flex-1">
                        <div className="text-sm text-text-primary truncate">
                          {pin.title}
                        </div>
                        <div className="text-xs text-text-dim truncate">
                          {pin.creator_nickname}
                        </div>
                      </div>
                      <span className="text-3xs px-2 py-0.5 rounded-full bg-accent-subtle text-text-dim shrink-0 font-mono">
                        {getMediaTypeLabel(pin.media_type)}
                      </span>
                    </Link>
                  ))}
                </div>
              )}

              {/* Creators */}
              {hasCreators && (
                <div className="mb-2">
                  <div className="text-xs text-text-dim mb-1 px-1">
                    크리에이터
                  </div>
                  {results!.creators!.slice(0, 1).map((creator) => (
                    <Link
                      key={creator.id}
                      href={`/creators/${creator.id}`}
                      onClick={() => setOpen(false)}
                      className="flex items-center gap-3 px-3 py-2 rounded-[10px] hover:bg-surface-hover focus-visible:bg-surface-hover cursor-pointer transition-colors"
                    >
                      {creator.avatar_url ? (
                        <img
                          src={creator.avatar_url}
                          alt=""
                          className="w-7 h-7 rounded-full object-cover shrink-0"
                        />
                      ) : (
                        <div className="w-7 h-7 rounded-full bg-gradient-to-br from-accent to-accent-hover shrink-0" />
                      )}
                      <span className="text-sm text-text-primary truncate">
                        {creator.nickname}
                      </span>
                    </Link>
                  ))}
                </div>
              )}

              {/* Boards */}
              {hasBoards && (
                <div>
                  <div className="text-xs text-text-dim mb-1 px-1">보드</div>
                  {results!.boards!.slice(0, 1).map((board) => (
                    <Link
                      key={board.id}
                      href={`/boards/${board.id}`}
                      onClick={() => setOpen(false)}
                      className="flex items-center gap-3 px-3 py-2 rounded-[10px] hover:bg-surface-hover focus-visible:bg-surface-hover cursor-pointer transition-colors"
                    >
                      <div className="w-7 h-7 rounded-[6px] bg-surface shrink-0 flex items-center justify-center">
                        <svg
                          aria-hidden="true"
                          width="14"
                          height="14"
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
                        <div className="text-sm text-text-primary truncate">
                          {board.name}
                        </div>
                        <div className="text-xs text-text-dim truncate">
                          {board.creator_nickname}
                        </div>
                      </div>
                    </Link>
                  ))}
                </div>
              )}

              {/* View all results link */}
              {hasAnyResults && (
                <button
                  type="button"
                  onClick={() => handleSubmit()}
                  className="block w-full mt-2 pt-2 border-t border-border text-center text-sm text-accent hover:text-accent-hover focus-visible:text-accent-hover cursor-pointer py-2 transition-colors"
                >
                  전체 결과 보기
                </button>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
