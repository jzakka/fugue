import { Suspense } from "react";
import NavBar from "@/components/nav/NavBar";
import FieldFilter from "@/components/feed/FieldFilter";
import FeedContainer from "@/components/feed/FeedContainer";
import { fetchWorks } from "@/lib/api";
import type { Work } from "@/lib/api";

async function getInitialWorks(): Promise<{
  works: Work[];
  hasMore: boolean;
}> {
  try {
    const data = await fetchWorks({ limit: 20 }, { serverSide: true });
    return { works: data.works, hasMore: data.has_more };
  } catch {
    // API is down — render empty, client will retry
    return { works: [], hasMore: false };
  }
}

export const dynamic = "force-dynamic";

export default async function HomePage() {
  const { works, hasMore } = await getInitialWorks();

  return (
    <>
      <NavBar />
      <Suspense>
        <FieldFilter />
      </Suspense>
      <main className="flex-1 pb-12">
        <Suspense>
          <FeedContainer initialWorks={works} initialHasMore={hasMore} />
        </Suspense>
      </main>
    </>
  );
}
