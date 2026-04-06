import { Suspense } from "react";
import NavBar from "@/components/nav/NavBar";
import FieldFilter from "@/components/feed/FieldFilter";
import FeedContainer from "@/components/feed/FeedContainer";
import { fetchPins } from "@/lib/api";
import type { Pin } from "@/lib/api";

async function getInitialPins(
  field?: string,
  offset?: number
): Promise<{
  pins: Pin[];
  hasMore: boolean;
  error: boolean;
}> {
  try {
    const data = await fetchPins(
      { field: field || undefined, limit: 20, offset: offset || 0 },
      { serverSide: true }
    );
    return { pins: data.pins, hasMore: data.has_more, error: false };
  } catch {
    return { pins: [], hasMore: false, error: true };
  }
}

export const dynamic = "force-dynamic";

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ field?: string; offset?: string }>;
}) {
  const params = await searchParams;
  const offset = params.offset ? parseInt(params.offset, 10) || 0 : 0;
  const { pins, hasMore, error } = await getInitialPins(params.field, offset);

  return (
    <>
      <NavBar />
      <Suspense>
        <FieldFilter />
      </Suspense>
      <main className="flex-1 pb-12">
        <Suspense>
          <FeedContainer
            initialPins={pins}
            initialHasMore={hasMore}
            initialField={params.field || ""}
            initialOffset={offset}
            initialError={error}
          />
        </Suspense>
      </main>
    </>
  );
}
