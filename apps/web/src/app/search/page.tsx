import { Suspense } from "react";
import NavBar from "@/components/nav/NavBar";
import { fetchSearch } from "@/lib/api";
import type { SearchResult } from "@/lib/api";
import SearchClient from "./SearchClient";
import type { Metadata } from "next";

type Props = {
  searchParams: Promise<{
    q?: string;
    type?: string;
    tag_ids?: string;
    limit?: string;
    offset?: string;
  }>;
};

export async function generateMetadata({ searchParams }: Props): Promise<Metadata> {
  const params = await searchParams;
  const q = params.q || "";
  return {
    title: q ? `"${q}" 검색 결과 — Fugue` : "검색 — Fugue",
    description: q
      ? `"${q}"에 대한 작품, 크리에이터, 보드 검색 결과`
      : "Fugue에서 작품, 크리에이터, 보드를 검색하세요",
  };
}

export const dynamic = "force-dynamic";

export default async function SearchPage({ searchParams }: Props) {
  const params = await searchParams;
  const q = params.q || "";
  const type = (params.type || "all") as "all" | "pins" | "creators" | "boards";
  const tagIds = params.tag_ids ? params.tag_ids.split(",").filter(Boolean) : [];
  const offset = params.offset ? parseInt(params.offset, 10) || 0 : 0;

  let results: SearchResult = {};

  if (q.length > 0) {
    try {
      results = await fetchSearch(
        {
          q,
          type,
          tag_ids: tagIds.length > 0 ? tagIds : undefined,
          limit: 20,
          offset,
        },
        { serverSide: true }
      );
    } catch {
      // Fallback to empty results on error
    }
  }

  return (
    <>
      <NavBar />
      <main className="flex-1 pb-12">
        {q ? (
          <Suspense>
            <SearchClient
              key={`${q}:${type}:${tagIds.join(",")}`}
              initialResults={results}
              query={q}
              initialType={type}
              initialTagIds={tagIds}
            />
          </Suspense>
        ) : (
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <div className="text-5xl mb-4">🐡</div>
            <p className="text-text-muted text-sm mb-1">
              검색어를 입력해주세요
            </p>
            <p className="text-text-dim text-xs">
              작품, 크리에이터, 보드를 검색할 수 있습니다
            </p>
          </div>
        )}
      </main>
    </>
  );
}
