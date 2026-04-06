import { notFound } from "next/navigation";
import NavBar from "@/components/nav/NavBar";
import { fetchPin, fetchRelatedPins } from "@/lib/api";
import type { Pin } from "@/lib/api";
import { getFieldLabel } from "@/lib/card-type";
import PinCard from "@/components/feed/PinCard";
import MasonryGrid from "@/components/feed/MasonryGrid";
import PinDetailTracker from "./PinDetailTracker";
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
        images: pin.og_image ? [pin.og_image] : undefined,
      },
    };
  } catch {
    return { title: "작품 — Fugue" };
  }
}

export const dynamic = "force-dynamic";

const FIELD_FALLBACK_ICONS: Record<string, string> = {
  "미술": "palette",
  "음악": "music_note",
  "영상편집": "movie",
  "프로그래밍": "code",
  "글": "description",
  "기타": "auto_awesome",
};

function FieldFallback({ field }: { field: string }) {
  const label = getFieldLabel(field);
  return (
    <div className="w-full h-64 bg-surface-elevated flex flex-col items-center justify-center gap-3">
      <div className="w-16 h-16 rounded-full bg-accent-subtle flex items-center justify-center">
        <span className="text-2xl text-accent">{label.charAt(0)}</span>
      </div>
      <span
        className="text-xs text-text-dim uppercase tracking-wider"
        style={{ fontFamily: "'Geist Mono', monospace" }}
      >
        {label}
      </span>
    </div>
  );
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

  let relatedPins: Pin[] = [];
  try {
    const related = await fetchRelatedPins(id, { serverSide: true });
    relatedPins = related.pins;
  } catch {
    // Proceed without related pins
  }

  return (
    <>
      <NavBar />
      <main className="flex-1 max-w-4xl mx-auto w-full px-6 py-8">
        {/* Pin Detail */}
        <article className="bg-surface rounded-[16px] border border-border overflow-hidden">
          {/* OG Image or Fallback */}
          {pin.og_image ? (
            <div className="overflow-hidden">
              <img
                src={pin.og_image}
                alt={pin.title}
                className="w-full object-cover max-h-[480px]"
              />
            </div>
          ) : (
            <FieldFallback field={pin.field} />
          )}

          {/* Content */}
          <div className="p-6 sm:p-8 space-y-5">
            {/* Field Badge */}
            <span
              className="inline-block px-3 py-1 bg-accent-subtle text-accent rounded-full text-xs font-medium"
              style={{ fontFamily: "'Geist Mono', monospace" }}
            >
              {getFieldLabel(pin.field)}
            </span>

            {/* Title */}
            <h1
              className="text-2xl sm:text-3xl font-bold tracking-tight leading-tight"
              style={{ fontFamily: "'General Sans', sans-serif" }}
            >
              {pin.title}
            </h1>

            {/* Description */}
            {pin.description && (
              <p className="text-text-muted text-sm sm:text-base leading-relaxed">
                {pin.description}
              </p>
            )}

            {/* Tags */}
            {pin.tags.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {pin.tags.map((tag) => (
                  <span
                    key={tag}
                    className="px-2.5 py-1 bg-accent-subtle text-text-muted rounded-full text-xs"
                    style={{ fontFamily: "'Geist Mono', monospace" }}
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}

            {/* Divider */}
            <div className="border-t border-border" />

            {/* Creator + Meta */}
            <div className="flex items-center justify-between flex-wrap gap-4">
              {/* Creator */}
              <Link
                href={`/creators/${pin.creator.id}`}
                className="flex items-center gap-3 group"
              >
                {pin.creator.avatar_url ? (
                  <img
                    src={pin.creator.avatar_url}
                    alt={pin.creator.nickname}
                    className="w-10 h-10 rounded-full border-2 border-border object-cover group-hover:border-accent transition-colors"
                  />
                ) : (
                  <div className="w-10 h-10 rounded-full bg-gradient-to-br from-accent to-orange-400 border-2 border-border group-hover:border-accent transition-colors" />
                )}
                <span className="text-sm font-medium text-text-primary group-hover:text-accent transition-colors">
                  {pin.creator.nickname}
                </span>
              </Link>

              {/* Pin count */}
              <div className="flex items-center gap-1.5 text-text-muted">
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M12 2a7 7 0 0 0-7 7c0 5 7 13 7 13s7-8 7-13a7 7 0 0 0-7-7z" />
                  <circle cx="12" cy="9" r="2.5" />
                </svg>
                <span
                  className="text-xs"
                  style={{ fontFamily: "'Geist Mono', monospace" }}
                >
                  {pin.pin_count}
                </span>
              </div>
            </div>

            {/* Action buttons */}
            <div className="flex gap-3 flex-wrap">
              <a
                href={pin.url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-5 py-2.5 bg-accent text-white rounded-full text-sm font-semibold hover:bg-accent-hover transition-colors"
              >
                <svg
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
              <Link
                href={`/boards?add=${pin.id}`}
                className="inline-flex items-center gap-2 px-5 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent transition-colors"
              >
                <svg
                  width="14"
                  height="14"
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
                  <path d="M17.5 14v7M14 17.5h7" />
                </svg>
                보드에 추가
              </Link>
            </div>
          </div>
        </article>

        {/* Related Pins */}
        {relatedPins.length > 0 && (
          <section className="mt-12">
            <h2
              className="text-lg font-bold tracking-tight mb-6"
              style={{ fontFamily: "'General Sans', sans-serif" }}
            >
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
