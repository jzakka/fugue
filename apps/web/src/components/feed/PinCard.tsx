import type { Pin } from "@/lib/api";
import { getCardType, getFieldLabel } from "@/lib/card-type";
import Link from "next/link";

function getDomainFavicon(url: string): string | null {
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
    <div className="flex items-end gap-[2px] h-12 mb-4">
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
  if (pin.og_image) {
    return (
      <div className="overflow-hidden">
        <img
          src={pin.og_image}
          alt={pin.title}
          loading="lazy"
          className="w-full block object-cover"
        />
      </div>
    );
  }
  return (
    <div className="h-40 bg-surface-elevated flex items-center justify-center text-4xl text-text-dim">
      🎨
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
          <button className="w-9 h-9 rounded-full bg-accent text-white flex items-center justify-center text-sm shrink-0">
            ▶
          </button>
          <div className="flex-1 min-w-0">
            <div className="text-sm font-semibold truncate">{pin.title}</div>
            <div className="text-xs text-text-muted">{pin.creator.nickname}</div>
          </div>
        </div>
      </div>
    </div>
  );
}

function TextSection({ pin }: { pin: Pin }) {
  const readTime = pin.description
    ? `${Math.max(1, Math.ceil(pin.description.length / 200))} min read`
    : "1 min read";

  return (
    <div className="p-5">
      <div
        className="text-[10px] text-accent uppercase tracking-[1.5px] mb-2"
        style={{ fontFamily: "'Geist Mono', monospace" }}
      >
        {getFieldLabel(pin.field)}
      </div>
      <div className="text-lg font-bold leading-tight mb-2">{pin.title}</div>
      {pin.description && (
        <p className="text-sm text-text-muted leading-relaxed line-clamp-4">
          {pin.description}
        </p>
      )}
      <div
        className="text-[11px] text-text-dim mt-4"
        style={{ fontFamily: "'Geist Mono', monospace" }}
      >
        {readTime}
      </div>
    </div>
  );
}

function VideoSection({ pin }: { pin: Pin }) {
  if (pin.og_image) {
    return (
      <div className="overflow-hidden relative">
        <img
          src={pin.og_image}
          alt={pin.title}
          loading="lazy"
          className="w-full block object-cover"
        />
        <div className="absolute inset-0 flex items-center justify-center">
          <div className="w-12 h-12 rounded-full bg-black/60 flex items-center justify-center text-white text-lg">
            ▶
          </div>
        </div>
      </div>
    );
  }
  return (
    <div className="h-40 bg-surface-elevated flex items-center justify-center relative">
      <span className="text-4xl text-text-dim">🎬</span>
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="w-12 h-12 rounded-full bg-black/60 flex items-center justify-center text-white text-lg">
          ▶
        </div>
      </div>
    </div>
  );
}

function ExternalLinkIcon({ url }: { url: string }) {
  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      onClick={(e) => e.stopPropagation()}
      className="inline-flex items-center justify-center w-6 h-6 rounded-full bg-surface-elevated hover:bg-accent-subtle transition-colors shrink-0"
      title="원본 보기"
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
    </a>
  );
}

export default function PinCard({ pin }: { pin: Pin }) {
  const cardType = getCardType(pin.field);
  const favicon = getDomainFavicon(pin.url);

  return (
    <Link
      href={`/pins/${pin.id}`}
      className="block bg-surface rounded-[10px] overflow-hidden cursor-pointer transition-all duration-200 border border-transparent hover:-translate-y-0.5 hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)] hover:border-accent"
    >
      {/* Media section by card type */}
      {cardType === "audio" && <AudioSection pin={pin} />}
      {cardType === "text" && <TextSection pin={pin} />}
      {cardType === "video" && <VideoSection pin={pin} />}
      {cardType === "image" && <ImageSection pin={pin} />}

      {/* Info section (skip for audio/text — they have inline info) */}
      {cardType !== "audio" && cardType !== "text" && (
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
            <div
              className="w-5 h-5 rounded-full shrink-0"
              style={{
                background: `linear-gradient(135deg, var(--accent), #FF8A5C)`,
              }}
            />
            <span className="text-xs text-text-muted truncate">
              {pin.creator.nickname}
            </span>
          </div>
        </div>
      )}

      {/* Footer: Tags + Pin count + External link */}
      <div className="px-3 pb-3 flex items-center gap-1 flex-wrap">
        <div className="flex gap-1 flex-wrap flex-1 min-w-0">
          {pin.tags.slice(0, 3).map((tag) => (
            <span
              key={tag}
              className="text-[10px] text-text-dim bg-accent-subtle px-2 py-0.5 rounded-full"
              style={{ fontFamily: "'Geist Mono', monospace" }}
            >
              {tag}
            </span>
          ))}
        </div>
        <div className="flex items-center gap-1.5 shrink-0">
          {pin.pin_count > 0 && (
            <span
              className="text-[10px] text-text-dim"
              style={{ fontFamily: "'Geist Mono', monospace" }}
            >
              <svg
                width="10"
                height="10"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="inline-block mr-0.5 -mt-px"
              >
                <path d="M12 2a7 7 0 0 0-7 7c0 5 7 13 7 13s7-8 7-13a7 7 0 0 0-7-7z" />
                <circle cx="12" cy="9" r="2.5" />
              </svg>
              {pin.pin_count}
            </span>
          )}
          <ExternalLinkIcon url={pin.url} />
        </div>
      </div>
    </Link>
  );
}
