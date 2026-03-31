import type { Work } from "@/lib/api";
import { getCardType, getFieldLabel } from "@/lib/card-type";

// Deterministic waveform heights seeded from work ID to avoid hydration mismatch
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

function ImageSection({ work }: { work: Work }) {
  if (work.og_image) {
    return (
      <div className="overflow-hidden">
        <img
          src={work.og_image}
          alt={work.title}
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

function AudioSection({ work }: { work: Work }) {
  return (
    <div className="p-5 relative">
      <div className="absolute inset-0 bg-gradient-to-br from-accent/15 to-transparent" />
      <div className="relative">
        <AudioWaveform seed={work.id} />
        <div className="flex items-center gap-3">
          <button className="w-9 h-9 rounded-full bg-accent text-white flex items-center justify-center text-sm shrink-0">
            ▶
          </button>
          <div className="flex-1 min-w-0">
            <div className="text-sm font-semibold truncate">{work.title}</div>
            <div className="text-xs text-text-muted">{work.creator.nickname}</div>
          </div>
        </div>
      </div>
    </div>
  );
}

function TextSection({ work }: { work: Work }) {
  const readTime = work.description
    ? `${Math.max(1, Math.ceil(work.description.length / 200))} min read`
    : "1 min read";

  return (
    <div className="p-5">
      <div
        className="text-[10px] text-accent uppercase tracking-[1.5px] mb-2"
        style={{ fontFamily: "'Geist Mono', monospace" }}
      >
        {getFieldLabel(work.field)}
      </div>
      <div className="text-lg font-bold leading-tight mb-2">{work.title}</div>
      {work.description && (
        <p className="text-sm text-text-muted leading-relaxed line-clamp-4">
          {work.description}
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

function VideoSection({ work }: { work: Work }) {
  if (work.og_image) {
    return (
      <div className="overflow-hidden relative">
        <img
          src={work.og_image}
          alt={work.title}
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

export default function WorkCard({ work }: { work: Work }) {
  const cardType = getCardType(work.field);

  function handleClick() {
    window.open(work.url, "_blank", "noopener,noreferrer");
  }

  return (
    <div
      onClick={handleClick}
      className="bg-surface rounded-[10px] overflow-hidden cursor-pointer transition-all duration-200 border border-transparent hover:-translate-y-0.5 hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)] hover:border-accent"
      role="link"
      tabIndex={0}
      onKeyDown={(e) => e.key === "Enter" && handleClick()}
    >
      {/* Media section by card type */}
      {cardType === "audio" && <AudioSection work={work} />}
      {cardType === "text" && <TextSection work={work} />}
      {cardType === "video" && <VideoSection work={work} />}
      {cardType === "image" && <ImageSection work={work} />}

      {/* Info section (skip for audio/text — they have inline info) */}
      {cardType !== "audio" && cardType !== "text" && (
        <div className="px-3 pt-2 pb-3">
          <div className="text-sm font-semibold mb-1 line-clamp-2 leading-tight">
            {work.title}
          </div>
          <div className="flex items-center gap-2">
            <div
              className="w-5 h-5 rounded-full shrink-0"
              style={{
                background: `linear-gradient(135deg, var(--accent), #FF8A5C)`,
              }}
            />
            <span className="text-xs text-text-muted">
              {work.creator.nickname}
            </span>
          </div>
        </div>
      )}

      {/* Tags */}
      <div className="px-3 pb-3 flex gap-1 flex-wrap">
        {work.tags.slice(0, 3).map((tag) => (
          <span
            key={tag}
            className="text-[10px] text-text-dim bg-accent-subtle px-2 py-0.5 rounded-full"
            style={{ fontFamily: "'Geist Mono', monospace" }}
          >
            {tag}
          </span>
        ))}
      </div>
    </div>
  );
}
