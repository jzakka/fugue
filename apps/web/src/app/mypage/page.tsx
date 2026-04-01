import { redirect } from "next/navigation";
import NavBar from "@/components/nav/NavBar";
import { fetchWorks } from "@/lib/api";
import { getAuthUser, fetchMe } from "@/lib/auth";
import MyPageClient from "@/components/profile/MyPageClient";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "마이페이지 — Fugue",
};

export const dynamic = "force-dynamic";

export default async function MyPage() {
  // Quick auth check first — avoids redirect loop through /login → /
  const user = await getAuthUser();
  if (!user) {
    redirect("/login?redirect=/mypage");
  }

  const creator = await fetchMe().catch(() => null);
  if (!creator) {
    redirect("/login?redirect=/mypage");
  }

  let works = { works: [] as Awaited<ReturnType<typeof fetchWorks>>["works"], has_more: false };
  try {
    works = await fetchWorks(
      { creator_id: creator.id, limit: 20 },
      { serverSide: true }
    );
  } catch {
    // Proceed with empty works
  }

  return (
    <>
      <NavBar />
      <main className="flex-1 max-w-4xl mx-auto w-full px-6 py-8">
        <MyPageClient
          creator={creator}
          works={works.works}
          hasMore={works.has_more}
        />
      </main>
    </>
  );
}
