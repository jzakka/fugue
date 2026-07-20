import { notFound } from "next/navigation";
import NavBar from "@/components/nav/NavBar";
import ProfileHeader from "@/components/profile/ProfileHeader";
import PinsGrid from "@/components/profile/PinsGrid";
import BoardGrid from "@/components/board/BoardGrid";
import { fetchCreator, fetchPins, fetchBoards } from "@/lib/api";
import type { Board } from "@/lib/api";
import type { Metadata } from "next";

type Props = {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ offset?: string }>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  try {
    const creator = await fetchCreator(id, { serverSide: true });
    return {
      title: `${creator.nickname} — Fugue`,
      description: `${creator.nickname}의 큐레이션`,
      openGraph: {
        title: `${creator.nickname} — Fugue`,
        description: `${creator.nickname}의 큐레이션`,
      },
    };
  } catch {
    return { title: "크리에이터 — Fugue" };
  }
}

export const dynamic = "force-dynamic";

export default async function CreatorProfilePage({
  params,
  searchParams,
}: Props) {
  const { id } = await params;
  const sp = await searchParams;
  const offset = sp.offset ? parseInt(sp.offset, 10) || 0 : 0;

  // Validate UUID format
  const uuidRegex =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  if (!uuidRegex.test(id)) {
    notFound();
  }

  let creator;
  try {
    creator = await fetchCreator(id, { serverSide: true });
  } catch {
    notFound();
  }

  let pinsData = {
    pins: [] as Awaited<ReturnType<typeof fetchPins>>["pins"],
    has_more: false,
  };
  try {
    pinsData = await fetchPins(
      { creator_id: id, limit: 20, offset },
      { serverSide: true }
    );
  } catch {
    // Proceed with empty pins
  }

  let boards: Board[] = [];
  try {
    boards = await fetchBoards(id, { serverSide: true });
  } catch {
    // Proceed with empty boards
  }

  return (
    <>
      <NavBar />
      <main id="main" className="flex-1 max-w-4xl mx-auto w-full px-6 py-8">
        <div className="space-y-6">
          <ProfileHeader creator={creator} />
          <BoardGrid boards={boards} />
          <PinsGrid
            creatorId={id}
            initialPins={pinsData.pins}
            initialHasMore={pinsData.has_more}
            initialOffset={offset}
          />
        </div>
      </main>
    </>
  );
}
