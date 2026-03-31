"use client";

import { useRouter } from "next/navigation";

export default function EmptyState() {
  const router = useRouter();

  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="text-5xl mb-4">🐡</div>
      <p className="text-text-muted text-sm mb-4">
        이 분야의 작품이 아직 없어요
      </p>
      <button
        onClick={() => router.push("/", { scroll: false })}
        className="text-accent text-sm hover:underline cursor-pointer"
      >
        전체 보기
      </button>
    </div>
  );
}
