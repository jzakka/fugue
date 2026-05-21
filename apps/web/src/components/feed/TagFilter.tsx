"use client";

import { useRouter, useSearchParams } from "next/navigation";
import type { PopularTag } from "@/lib/api";

export default function TagFilter({ tags }: { tags: PopularTag[] }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const selectedSlugs = new Set(
    (searchParams.get("tags") || "").split(",").filter(Boolean)
  );

  function handleToggle(slug: string) {
    const params = new URLSearchParams(searchParams.toString());
    const next = new Set(selectedSlugs);

    if (next.has(slug)) {
      next.delete(slug);
    } else {
      next.add(slug);
    }

    if (next.size > 0) {
      params.set("tags", [...next].join(","));
    } else {
      params.delete("tags");
    }
    // Reset pagination
    params.delete("offset");

    router.push(`?${params.toString()}`, { scroll: false });
  }

  function handleReset() {
    const params = new URLSearchParams(searchParams.toString());
    params.delete("tags");
    params.delete("offset");
    router.push(`?${params.toString()}`, { scroll: false });
  }

  if (tags.length === 0) return null;

  return (
    <div className="px-6 pb-2 flex gap-2 items-center overflow-x-auto scrollbar-hide">
      {selectedSlugs.size > 0 && (
        <button
          onClick={handleReset}
          className="px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap cursor-pointer bg-error/10 text-error border border-error/30 hover:bg-error/20 focus-visible:bg-error/20 transition-colors font-mono"
        >
          초기화
        </button>
      )}
      {tags.map((tag) => {
        const selected = selectedSlugs.has(tag.slug);
        return (
          <button
            key={tag.id}
            onClick={() => handleToggle(tag.slug)}
            className={`px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap transition-colors cursor-pointer font-mono ${
              selected
                ? "bg-accent text-white"
                : "bg-accent-subtle text-text-muted hover:bg-accent/20 focus-visible:bg-accent/20"
            }`}
          >
            {tag.name}
            <span className="ml-1 opacity-60">{tag.pin_count}</span>
          </button>
        );
      })}
    </div>
  );
}
