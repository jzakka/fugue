"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { updateBoard, deleteBoard } from "@/lib/api";

export default function BoardActions({
  boardId,
  boardName,
  boardDescription,
}: {
  boardId: string;
  boardName: string;
  boardDescription: string;
}) {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(boardName);
  const [description, setDescription] = useState(boardDescription);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSave() {
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("보드 이름을 입력해주세요");
      return;
    }

    setSaving(true);
    setError(null);
    try {
      await updateBoard(boardId, {
        name: trimmedName,
        description: description.trim() || undefined,
      });
      setEditing(false);
      router.refresh();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "보드 수정에 실패했습니다"
      );
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!confirm("정말 이 보드를 삭제하시겠습니까?")) return;

    try {
      await deleteBoard(boardId);
      router.push("/mypage");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "보드 삭제에 실패했습니다"
      );
    }
  }

  if (editing) {
    return (
      <div className="space-y-3 shrink-0">
        {error && (
          <div
            id="board-name-error"
            role="alert"
            aria-live="polite"
            className="p-2 bg-error/10 border border-error/30 rounded-[6px] text-xs text-error"
          >
            {error}
          </div>
        )}
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="보드 이름"
          aria-label="보드 이름"
          aria-required="true"
          aria-invalid={!!error && !name.trim()}
          aria-describedby={error ? "board-name-error" : undefined}
          className="w-full px-3 py-2 bg-bg border border-border rounded-[6px] text-sm text-text-primary outline-none focus:border-accent transition-colors"
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              handleSave();
            }
          }}
        />
        <input
          type="text"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="보드 설명 (선택)"
          aria-label="보드 설명"
          className="w-full px-3 py-2 bg-bg border border-border rounded-[6px] text-sm text-text-primary outline-none focus:border-accent transition-colors"
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              handleSave();
            }
          }}
        />
        <div className="flex gap-2">
          <button
            onClick={handleSave}
            disabled={saving}
            aria-busy={saving}
            className="px-3 py-1.5 bg-accent text-white rounded-full text-xs font-semibold hover:bg-accent-hover focus-visible:bg-accent-hover transition-colors disabled:opacity-50 cursor-pointer"
          >
            {saving ? "저장 중..." : "저장"}
          </button>
          <button
            onClick={() => {
              setEditing(false);
              setName(boardName);
              setDescription(boardDescription);
              setError(null);
            }}
            disabled={saving}
            className="px-3 py-1.5 border border-border rounded-full text-xs text-text-muted hover:text-text-primary focus-visible:text-text-primary transition-colors cursor-pointer disabled:opacity-50"
          >
            취소
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2 shrink-0">
      <button
        onClick={() => setEditing(true)}
        className="px-3 py-1.5 border border-border rounded-full text-xs text-text-muted hover:text-text-primary hover:border-accent focus-visible:text-text-primary focus-visible:border-accent transition-colors cursor-pointer"
      >
        편집
      </button>
      <button
        onClick={handleDelete}
        className="px-3 py-1.5 border border-border rounded-full text-xs text-text-muted hover:text-error hover:border-error focus-visible:text-error focus-visible:border-error transition-colors cursor-pointer"
      >
        삭제
      </button>
    </div>
  );
}
