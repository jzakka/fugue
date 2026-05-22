"use client";

import type { Pin } from "@/lib/api";
import { getCardType } from "@/lib/card-type";
import Link from "next/link";

function getDomainFavicon(url: string | null): string | null {
  if (!url) return null;
  try {
    const hostname = new URL(url).hostname;
    return `https://www.google.com/s2/favicons?domain=${hostname}&sz=32`;
  } catch {
    return null;
  }
}

// Deterministic waveform heights seeded from pin ID to avoid hydration mismatch
function seededBars(seed: string): number[] {
  let h = 0;
  for (let i = 0; i < seed.length; i++) {
    h = ((h << 5) - h + seed.charCodeAt(i)) | 0;
  }
  return Array.from({ length: 30 }, (_, i) => {
    h = ((h << 5) - h + i) | 0;
    return (Math.abs(h) % 32) + 8;
  });
}

function AudioWaveform({ seed }: { seed: string }) {
  const bars = seededBars(seed);
  return (
    <div className="flex items-end gap-0.5 h-12 mb-4">
      {bars.map((h, i) => (
        <div
          key={i}
          className="flex-1 rounded-sm bg-text-muted/50 min-h-[3px]"
          style={{ height: `${h}px` }}
        />
      ))}
    </div>
  );
}

function ImageSection({ pin }: { pin: Pin }) {
  return (
    <div className="overflow-hidden">
      <img
        src={pin.media_url}
        alt={pin.title}
        loading="lazy"
        className="w-full block object-cover"
      />
    </div>
  );
}

function AudioSection({ pin }: { pin: Pin }) {
  return (
    <div className="p-5 relative">
      <div className="absolute inset-0 bg-gradient-to-br from-accent/15 to-transparent" />
      <div className="relative">
        <AudioWaveform seed={pin.id} />
        <div className="flex items-center gap-3">
          <span
            aria-hidden="true"
            className="w-9 h-9 rounded-full bg-accent text-white flex items-center justify-center text-sm shrink-0"
          >
            ▶
          </span>
          <div className="flex-1 min-w-0">
            <div className="text-sm font-semibold truncate">{pin.title}</div>
            <div className="text-xs text-text-muted truncate">{pin.creator.nickname}</div>
          </div>
        </div>
      </div>
    </div>
  );
}

function VideoSection({ pin }: { pin: Pin }) {
  return (
    <div className="overflow-hidden relative">
      <img
        src={pin.og_image || pin.media_url}
        alt={pin.title}
        loading="lazy"
        className="w-full block object-cover"
        onError={(e) => {
          // Gracefully degrade when the poster image fails to resolve —
          // e.g., cached og_image evicted by the harvester image cache TTL,
          // or the original source URL is no longer reachable. For video
          // Pins the only meaningful poster is og_image (media_url is the
          // video file itself), so we simply hide the image; the centered
          // play-button overlay still indicates the video card.
          e.currentTarget.style.display = "none";
        }}
      />
      <div aria-hidden="true" className="absolute inset-0 flex items-center justify-center">
        <div className="w-12 h-12 rounded-full bg-black/60 flex items-center justify-center text-white text-lg">
          ▶
        </div>
      </div>
    </div>
  );
}

function ExternalLinkIcon({ url }: { url: string }) {
  return (
    <button
      type="button"
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        window.open(url, "_blank", "noopener,noreferrer");
      }}
      className="inline-flex items-center justify-center w-6 h-6 rounded-full bg-surface-elevated hover:bg-accent-subtle focus-visible:bg-accent-subtle transition-colors shrink-0 cursor-pointer"
      title="원본 보기"
      aria-label="원본 보기"
    >
      <svg
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="text-text-muted"
      >
        <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
        <polyline points="15 3 21 3 21 9" />
        <line x1="10" y1="14" x2="21" y2="3" />
      </svg>
    </button>
  );
}

export default function PinCard({ pin }: { pin: Pin }) {
  const cardType = getCardType(pin.media_type);
  const favicon = getDomainFavicon(pin.url);

  return (
    <Link
      href={`/pins/${pin.id}`}
      className="block bg-surface rounded-[10px] overflow-hidden cursor-pointer transition-all duration-200 border border-transparent hover:-translate-y-0.5 hover:shadow-card-hover hover:border-accent focus-visible:-translate-y-0.5 focus-visible:shadow-card-hover focus-visible:border-accent focus-visible:outline-none"
    >
      {/* Media section by type */}
      {cardType === "audio" && <AudioSection pin={pin} />}
      {cardType === "video" && <VideoSection pin={pin} />}
      {cardType === "image" && <ImageSection pin={pin} />}

      {/* Info section (skip for audio — it has inline info) */}
      {cardType !== "audio" && (
        <div className="px-3 pt-2 pb-3">
          <div className="text-sm font-semibold mb-1 line-clamp-2 leading-tight">
            {pin.title}
          </div>
          <div className="flex items-center gap-2">
            {favicon && (
              <img
                src={favicon}
                alt=""
                className="w-4 h-4 rounded-sm shrink-0"
                loading="lazy"
                onError={(e) => {
                  (e.target as HTMLImageElement).style.display = "none";
                }}
              />
            )}
            {pin.creator.avatar_url ? (
              <img
                src={pin.creator.avatar_url}
                alt=""
                loading="lazy"
                className="w-5 h-5 rounded-full shrink-0 object-cover"
                onError={(e) => {
                  (e.target as HTMLImageElement).style.display = "none";
                }}
              />
            ) : (
              <div className="w-5 h-5 rounded-full shrink-0 bg-gradient-to-br from-accent to-accent-hover" />
            )}
            <span className="text-xs text-text-muted truncate">
              {pin.creator.nickname}
            </span>
          </div>
        </div>
      )}

      {/* Footer: Tags + External link */}
      <div className="px-3 pb-3 flex items-center gap-1 flex-wrap">
        <div className="flex gap-1 flex-wrap flex-1 min-w-0">
          {pin.tags.slice(0, 3).map((tag) => (
            <span
              key={tag.id}
              className="text-3xs text-text-dim bg-accent-subtle px-2 py-0.5 rounded-full font-mono"
            >
              {tag.name}
            </span>
          ))}
        </div>
        {pin.url && (
          <div className="shrink-0">
            <ExternalLinkIcon url={pin.url} />
          </div>
        )}
      </div>
    </Link>
  );
}
