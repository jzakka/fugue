"use client";

export default function BoardCover({ images }: { images: string[] }) {
  if (images.length === 0) {
    return (
      <div className="w-full aspect-square bg-surface-elevated rounded-[10px] flex items-center justify-center border border-transparent group-hover:border-accent group-hover:shadow-card-hover group-focus-visible:border-accent group-focus-visible:shadow-card-hover transition-all duration-200">
        <svg
          aria-hidden="true"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="text-text-dim"
        >
          <rect x="3" y="3" width="7" height="7" />
          <rect x="14" y="3" width="7" height="7" />
          <rect x="3" y="14" width="7" height="7" />
          <rect x="14" y="14" width="7" height="7" />
        </svg>
      </div>
    );
  }

  const slots = images.slice(0, 4);

  return (
    <div className="w-full aspect-square rounded-[10px] overflow-hidden grid grid-cols-2 grid-rows-2 gap-0.5 border border-transparent group-hover:border-accent group-hover:shadow-card-hover group-focus-visible:border-accent group-focus-visible:shadow-card-hover transition-all duration-200">
      {slots.map((img, i) => (
        <div key={i} className="overflow-hidden bg-surface-elevated">
          <img
            src={img}
            alt=""
            className="w-full h-full object-cover"
            loading="lazy"
            onError={(e) => {
              e.currentTarget.style.display = "none";
            }}
          />
        </div>
      ))}
      {Array.from({ length: Math.max(0, 4 - slots.length) }).map((_, i) => (
        <div key={`empty-${i}`} className="bg-surface-elevated" />
      ))}
    </div>
  );
}
