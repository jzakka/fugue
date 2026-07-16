import { notFound } from "next/navigation";
import NavBar from "@/components/nav/NavBar";
import { fetchPin, fetchRelatedPins, fetchPinBoards } from "@/lib/api";
import type { Pin, PinBoardInfo } from "@/lib/api";
import { getMediaTypeLabel } from "@/lib/card-type";
import PinCard from "@/components/feed/PinCard";
import MasonryGrid from "@/components/feed/MasonryGrid";
import PinDetailTracker from "./PinDetailTracker";
import AddToBoardButton from "@/components/board/AddToBoardButton";
import HideOnErrorImage from "@/components/ui/HideOnErrorImage";
import { getAuthUser } from "@/lib/auth";
import type { Metadata } from "next";
import Link from "next/link";

type Props = {
  params: Promise<{ id: string }>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  try {
    const pin = await fetchPin(id, { serverSide: true });
    return {
      title: `${pin.title} — Fugue`,
      description: pin.description || `${pin.title} by ${pin.creator.nickname}`,
      openGraph: {
        title: `${pin.title} — Fugue`,
        description: pin.description || `${pin.title} by ${pin.creator.nickname}`,
        images: pin.media_type === "image" ? [pin.media_url] : pin.og_image ? [pin.og_image] : undefined,
      },
    };
  } catch {
    return { title: "작품 — Fugue" };
  }
}

export const dynamic = "force-dynamic";

function MediaPlayer({ pin }: { pin: Pin }) {
  switch (pin.media_type) {
    case "image":
      return (
        <div className="overflow-hidden">
          <HideOnErrorImage
            src={pin.media_url}
            alt={pin.title}
            className="w-full object-cover max-h-[480px]"
          />
        </div>
      );
    case "audio":
      return (
        <div className="p-8 bg-surface-elevated">
          <div className="flex items-center gap-4 mb-4">
            <div aria-hidden="true" className="w-16 h-16 rounded-lg bg-accent-subtle flex items-center justify-center text-2xl text-accent">
              ♪
            </div>
            <div className="min-w-0">
              <div className="text-lg font-semibold break-words">{pin.title}</div>
              <div className="text-sm text-text-muted break-words">{pin.creator.nickname}</div>
            </div>
          </div>
          <audio controls className="w-full" preload="metadata">
            <source src={pin.media_url} />
          </audio>
        </div>
      );
    case "video":
      return (
        <div className="overflow-hidden bg-black">
          <video
            controls
            className="w-full max-h-[480px]"
            preload="metadata"
            poster={pin.og_image || undefined}
          >
            <source src={pin.media_url} />
          </video>
        </div>
      );
    default:
      return null;
  }
}

export default async function PinDetailPage({ params }: Props) {
  const { id } = await params;

  const uuidRegex =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  if (!uuidRegex.test(id)) {
    notFound();
  }

  let pin: Pin;
  try {
    pin = await fetchPin(id, { serverSide: true });
  } catch {
    notFound();
  }

  const user = await getAuthUser();

  let relatedPins: Pin[] = [];
  try {
    const related = await fetchRelatedPins(id, { serverSide: true });
    relatedPins = related.pins;
  } catch (err) {
    console.error("Failed to fetch related pins:", err);
  }

  let pinBoards: PinBoardInfo[] = [];
  try {
    pinBoards = await fetchPinBoards(id, { serverSide: true });
  } catch {
    // Proceed without boards
  }

  return (
    <>
      <NavBar />
      <main id="main" className="flex-1 max-w-4xl mx-auto w-full px-6 py-8">
        {/* Pin Detail */}
        <article className="bg-surface rounded-[16px] border border-border overflow-hidden">
          {/* Media Player */}
          <MediaPlayer pin={pin} />

          {/* Content */}
          <div className="p-6 sm:p-8 space-y-5">
            {/* Media Type Badge */}
            <span className="inline-block px-3 py-1 bg-accent-subtle text-accent rounded-full text-3xs font-medium font-mono">
              {getMediaTypeLabel(pin.media_type)}
            </span>

            {/* Title */}
            <h1 className="text-2xl sm:text-3xl font-bold tracking-tight leading-tight font-display break-words">
              {pin.title}
            </h1>

            {/* Description */}
            {pin.description && (
              <p className="text-text-muted text-sm sm:text-base leading-relaxed break-words">
                {pin.description}
              </p>
            )}

            {/* Tags */}
            {pin.tags.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {pin.tags.map((tag) => (
                  <span
                    key={tag.id}
                    className="px-2.5 py-1 bg-accent-subtle text-text-muted rounded-full text-3xs font-mono"
                  >
                    {tag.name}
                  </span>
                ))}
              </div>
            )}

            {/* Boards this pin belongs to */}
            {pinBoards.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {pinBoards.map((board) => (
                  <Link
                    key={board.id}
                    href={`/boards/${board.id}`}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 border border-border rounded-full text-xs text-text-muted hover:text-accent hover:border-accent focus-visible:text-accent focus-visible:border-accent transition-colors"
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
                      <rect x="3" y="3" width="7" height="7" />
                      <rect x="14" y="3" width="7" height="7" />
                      <rect x="3" y="14" width="7" height="7" />
                      <rect x="14" y="14" width="7" height="7" />
                    </svg>
                    <span>{board.name}</span>
                    <span className="text-text-dim">{board.creator_nickname}</span>
                  </Link>
                ))}
              </div>
            )}

            {/* Divider */}
            <div className="border-t border-border" />

            {/* Creator */}
            <div className="flex items-center justify-between flex-wrap gap-4">
              <Link
                href={`/creators/${pin.creator.id}`}
                className="flex items-center gap-3 group"
              >
                {pin.creator.avatar_url ? (
                  <HideOnErrorImage
                    src={pin.creator.avatar_url}
                    alt=""
                    className="w-10 h-10 rounded-full border-2 border-border object-cover group-hover:border-accent group-focus-visible:border-accent transition-colors"
                  />
                ) : (
                  <div className="w-10 h-10 rounded-full bg-gradient-to-br from-accent to-accent-hover border-2 border-border group-hover:border-accent group-focus-visible:border-accent transition-colors" />
                )}
                <span className="text-sm font-medium text-text-primary group-hover:text-accent group-focus-visible:text-accent transition-colors">
                  {pin.creator.nickname}
                </span>
              </Link>
            </div>

            {/* Action buttons */}
            <div className="flex gap-3 flex-wrap">
              {pin.url && (
                <a
                  href={pin.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 px-5 py-2.5 bg-accent text-white rounded-full text-sm font-semibold hover:bg-accent-hover focus-visible:bg-accent-hover transition-colors"
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
                  >
                    <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                    <polyline points="15 3 21 3 21 9" />
                    <line x1="10" y1="14" x2="21" y2="3" />
                  </svg>
                  원본 보기
                </a>
              )}
              <AddToBoardButton pinId={pin.id} userId={user?.id ?? null} />
            </div>
          </div>
        </article>

        {/* Related Pins */}
        {relatedPins.length > 0 && (
          <section className="mt-12">
            <h2 className="text-lg font-bold tracking-tight mb-6 font-display">
              관련 작품
            </h2>
            <MasonryGrid>
              {relatedPins.map((rp) => (
                <PinCard key={rp.id} pin={rp} />
              ))}
            </MasonryGrid>
          </section>
        )}

        {/* View Tracker (client component) */}
        <PinDetailTracker pinId={pin.id} />
      </main>
    </>
  );
}
