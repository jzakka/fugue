"use client";

import { useRouter, useSearchParams } from "next/navigation";

const MEDIA_TYPES = [
  { value: "", label: "전체" },
  { value: "image", label: "이미지" },
  { value: "audio", label: "음악" },
  { value: "video", label: "영상" },
];

export default function FieldFilter() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const current = searchParams.get("media_type") || "";

  function handleClick(mediaType: string) {
    // Preserve existing params (especially tags) — only change media_type
    const params = new URLSearchParams(searchParams.toString());
    if (mediaType) {
      params.set("media_type", mediaType);
    } else {
      params.delete("media_type");
    }
    // Reset pagination when filter changes
    params.delete("offset");
    router.push(`?${params.toString()}`, { scroll: false });
  }

  return (
    <div className="px-6 py-4 flex gap-2 overflow-x-auto scrollbar-hide">
      {MEDIA_TYPES.map((mt) => (
        <button
          key={mt.value}
          onClick={() => handleClick(mt.value)}
          className={`px-4 py-1.5 rounded-full text-sm font-medium whitespace-nowrap transition-colors cursor-pointer ${
            current === mt.value
              ? "bg-text-primary text-bg"
              : "bg-transparent border border-border text-text-muted hover:border-text-muted hover:text-text-primary focus-visible:border-text-muted focus-visible:text-text-primary"
          }`}
        >
          {mt.label}
        </button>
      ))}
    </div>
  );
}
