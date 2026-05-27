import { notFound } from "next/navigation";
import NavBar from "@/components/nav/NavBar";
import { fetchBoard } from "@/lib/api";
import PinCard from "@/components/feed/PinCard";
import MasonryGrid from "@/components/feed/MasonryGrid";
import BoardActions from "./BoardActions";
import LoadMorePins from "./LoadMorePins";
import EmptyState from "@/components/feed/EmptyState";
import type { Metadata } from "next";

type Props = {
  params: Promise<{ id: string }>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  try {
    const data = await fetchBoard(id, { serverSide: true });
    return {
      title: `${data.board.name} — Fugue`,
      description: data.board.description || `${data.board.name} 보드`,
    };
  } catch {
    return { title: "보드 — Fugue" };
  }
}

export const dynamic = "force-dynamic";

export default async function BoardDetailPage({ params }: Props) {
  const { id } = await params;

  const uuidRegex =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  if (!uuidRegex.test(id)) {
    notFound();
  }

  let data;
  try {
    data = await fetchBoard(id, { serverSide: true });
  } catch {
    notFound();
  }

  const { board, pins, has_more } = data;

  return (
    <>
      <NavBar />
      <main id="main" className="flex-1 max-w-5xl mx-auto w-full px-6 py-8">
        {/* Board Header */}
        <div className="bg-surface rounded-[16px] border border-border p-6 sm:p-8 mb-8">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <h1 className="text-2xl sm:text-3xl font-bold tracking-tight font-display">
                {board.name}
              </h1>
              {board.description && (
                <p className="mt-2 text-text-muted text-sm sm:text-base leading-relaxed">
                  {board.description}
                </p>
              )}
              <div className="mt-3 flex items-center gap-3 text-sm text-text-dim">
                <span className="font-mono">
                  {board.pin_count} pins
                </span>
                {!board.is_public && (
                  <span className="px-2 py-0.5 bg-accent-subtle text-accent rounded-full text-xs font-mono">
                    비공개
                  </span>
                )}
              </div>
            </div>

            {/* Owner actions (client component) */}
            <BoardActions boardId={board.id} boardName={board.name} boardDescription={board.description || ""} />
          </div>
        </div>

        {/* Pins Grid */}
        {pins.length > 0 ? (
          <MasonryGrid>
            {pins.map((pin) => (
              <PinCard key={pin.id} pin={pin} />
            ))}
            {has_more && (
              <LoadMorePins boardId={board.id} initialCount={pins.length} />
            )}
          </MasonryGrid>
        ) : (
          <EmptyState
            message="아직 보드에 추가된 작품이 없습니다"
            description="피드에서 마음에 드는 작품을 추가해보세요"
          />
        )}
      </main>
    </>
  );
}
