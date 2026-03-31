import { Suspense } from "react";
import NavBar from "@/components/nav/NavBar";
import FieldFilter from "@/components/feed/FieldFilter";
import FeedContainer from "@/components/feed/FeedContainer";
import { fetchWorks } from "@/lib/api";
import type { Work } from "@/lib/api";

async function getInitialWorks(field?: string): Promise<{
  works: Work[];
  hasMore: boolean;
}> {
  try {
    const data = await fetchWorks(
      { field: field || undefined, limit: 20 },
      { serverSide: true }
    );
    return { works: data.works, hasMore: data.has_more };
  } catch {
    return { works: [], hasMore: false };
  }
}

export const dynamic = "force-dynamic";

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ field?: string }>;
}) {
  const params = await searchParams;
  const { works, hasMore } = await getInitialWorks(params.field);

  return (
    <>
      <NavBar />
      <Suspense>
        <FieldFilter />
      </Suspense>
      <main className="flex-1 pb-12">
        <Suspense>
          <FeedContainer
            initialWorks={works}
            initialHasMore={hasMore}
            initialField={params.field || ""}
          />
        </Suspense>
      </main>
    </>
  );
}
