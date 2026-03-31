"use client";

import { useRouter, useSearchParams } from "next/navigation";

const FIELDS = [
  { value: "", label: "전체" },
  { value: "미술", label: "일러스트" },
  { value: "음악", label: "음악" },
  { value: "영상편집", label: "영상" },
  { value: "프로그래밍", label: "코드" },
  { value: "시나리오 라이터", label: "글" },
];

export default function FieldFilter() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentField = searchParams.get("field") || "";

  function handleClick(field: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (field) {
      params.set("field", field);
    } else {
      params.delete("field");
    }
    router.push(`?${params.toString()}`, { scroll: false });
  }

  return (
    <div className="px-6 py-4 flex gap-2 overflow-x-auto scrollbar-hide">
      {FIELDS.map((f) => (
        <button
          key={f.value}
          onClick={() => handleClick(f.value)}
          className={`px-4 py-1.5 rounded-full text-sm font-medium whitespace-nowrap transition-colors cursor-pointer ${
            currentField === f.value
              ? "bg-text-primary text-bg"
              : "bg-transparent border border-border text-text-muted hover:border-text-muted hover:text-text-primary"
          }`}
        >
          {f.label}
        </button>
      ))}
    </div>
  );
}
