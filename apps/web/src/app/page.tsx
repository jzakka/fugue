import { Suspense } from "react";
import NavBar from "@/components/nav/NavBar";
import FieldFilter from "@/components/feed/FieldFilter";
import TagFilter from "@/components/feed/TagFilter";
import FeedContainer from "@/components/feed/FeedContainer";
import { fetchPins, fetchPopularTags } from "@/lib/api";
import type { Pin, PopularTag } from "@/lib/api";

async function getInitialData(
  mediaType?: string,
  tagSlugs?: string[],
  offset?: number
): Promise<{
  pins: Pin[];
  hasMore: boolean;
  error: boolean;
  popularTags: PopularTag[];
}> {
  // Parallel fetch when no tag filter; sequential when tags need slug→id resolution
  if (!tagSlugs || tagSlugs.length === 0) {
    const [tagRes, pinRes] = await Promise.all([
      fetchPopularTags({ limit: 20 }, { serverSide: true }).catch(() => ({ tags: [] as PopularTag[] })),
      fetchPins({ media_type: mediaType || undefined, limit: 20, offset: offset || 0 }, { serverSide: true })
        .then((d) => ({ pins: d.pins, hasMore: d.has_more, error: false }))
        .catch(() => ({ pins: [] as Pin[], hasMore: false, error: true })),
    ]);
    return { ...pinRes, popularTags: tagRes.tags };
  }

  // Sequential: need tags first for slug→id mapping
  let popularTags: PopularTag[] = [];
  try {
    const tagRes = await fetchPopularTags({ limit: 20 }, { serverSide: true });
    popularTags = tagRes.tags;
  } catch {
    // Tag load failure is non-blocking
  }

  const slugToId = new Map(popularTags.map((t) => [t.slug, t.id]));
  const tagIds = tagSlugs
    .map((s) => slugToId.get(s))
    .filter((id): id is string => !!id);

  try {
    const data = await fetchPins(
      {
        media_type: mediaType || undefined,
        tag_ids: tagIds.length > 0 ? tagIds : undefined,
        limit: 20,
        offset: offset || 0,
      },
      { serverSide: true }
    );
    return { pins: data.pins, hasMore: data.has_more, error: false, popularTags };
  } catch {
    return { pins: [], hasMore: false, error: true, popularTags };
  }
}

export const dynamic = "force-dynamic";

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ media_type?: string; offset?: string; tags?: string }>;
}) {
  const params = await searchParams;
  const offset = params.offset ? parseInt(params.offset, 10) || 0 : 0;
  const tagSlugs = params.tags ? params.tags.split(",").filter(Boolean) : undefined;

  const { pins, hasMore, error, popularTags } = await getInitialData(
    params.media_type,
    tagSlugs,
    offset
  );

  return (
    <>
      <NavBar />
      <Suspense>
        <FieldFilter />
      </Suspense>
      <Suspense>
        <TagFilter tags={popularTags} />
      </Suspense>
      <main id="main" className="flex-1 pb-12">
        <h1 className="sr-only">작품 피드</h1>
        <Suspense>
          <FeedContainer
            initialPins={pins}
            initialHasMore={hasMore}
            initialMediaType={params.media_type || ""}
            initialOffset={offset}
            initialError={error}
            popularTags={popularTags}
          />
        </Suspense>
      </main>
    </>
  );
}
