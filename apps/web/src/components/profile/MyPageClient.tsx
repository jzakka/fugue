"use client";

import { useState, useEffect, useCallback } from "react";
import type { CreatorPrivate, Pin, Board } from "@/lib/api";
import { fetchBoards, createBoard } from "@/lib/api";
import ProfileHeader from "./ProfileHeader";
import ProfileEditForm from "./ProfileEditForm";
import PinsGrid from "./PinsGrid";
import BoardCover from "@/components/board/BoardCover";
import EmptyState from "@/components/feed/EmptyState";
import Link from "next/link";

function BoardSection({ creatorId }: { creatorId: string }) {
  const [boards, setBoards] = useState<Board[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [newBoardName, setNewBoardName] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadBoards = useCallback(async () => {
    try {
      const data = await fetchBoards(creatorId);
      setBoards(data);
    } catch {
      // Silently fail
    } finally {
      setLoading(false);
    }
  }, [creatorId]);

  useEffect(() => {
    loadBoards();
  }, [loadBoards]);

  async function handleCreate() {
    const trimmed = newBoardName.trim();
    if (!trimmed) return;

    setCreating(true);
    setError(null);
    try {
      const board = await createBoard({ name: trimmed });
      setBoards((prev) => [board, ...prev]);
      setNewBoardName("");
      setShowCreate(false);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "보드 생성에 실패했습니다"
      );
    } finally {
      setCreating(false);
    }
  }

  if (loading) {
    return (
      <div
        role="status"
        aria-label="보드 목록 로딩 중"
        className="flex justify-center py-8"
      >
        <div className="w-6 h-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-bold tracking-tight font-display">
          보드
        </h2>
        <button
          onClick={() => setShowCreate((prev) => !prev)}
          aria-expanded={showCreate}
          aria-controls="mypage-board-create-form"
          className="inline-flex items-center gap-1 px-3 py-1.5 bg-accent text-white rounded-full text-xs font-semibold hover:bg-accent-hover focus-visible:bg-accent-hover transition-colors cursor-pointer"
        >
          <svg
            aria-hidden="true"
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <line x1="12" y1="5" x2="12" y2="19" />
            <line x1="5" y1="12" x2="19" y2="12" />
          </svg>
          새 보드
        </button>
      </div>

      {showCreate && (
        <div
          id="mypage-board-create-form"
          className="mb-4 p-4 bg-surface border border-border rounded-[10px] space-y-3"
        >
          {error && (
            <div
              id="mypage-board-name-error"
              role="alert"
              aria-live="polite"
              className="p-2 bg-error/10 border border-error/30 rounded-[6px] text-xs text-error"
            >
              {error}
            </div>
          )}
          <input
            type="text"
            value={newBoardName}
            onChange={(e) => setNewBoardName(e.target.value)}
            placeholder="보드 이름"
            aria-label="새 보드 이름"
            aria-required="true"
            aria-invalid={!!error && !newBoardName.trim()}
            aria-describedby={error ? "mypage-board-name-error" : undefined}
            maxLength={100}
            className="w-full px-3 py-2 bg-bg border border-border rounded-[6px] text-sm text-text-primary outline-none focus:border-accent transition-colors"
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleCreate();
              }
            }}
          />
          <div className="flex gap-2">
            <button
              onClick={handleCreate}
              disabled={creating}
              aria-busy={creating}
              className="px-3 py-1.5 bg-accent text-white rounded-full text-xs font-semibold hover:bg-accent-hover focus-visible:bg-accent-hover transition-colors disabled:opacity-50 cursor-pointer"
            >
              {creating ? "생성 중..." : "생성"}
            </button>
            <button
              onClick={() => {
                setShowCreate(false);
                setNewBoardName("");
                setError(null);
              }}
              disabled={creating}
              className="px-3 py-1.5 border border-border rounded-full text-xs text-text-muted hover:text-text-primary focus-visible:text-text-primary transition-colors cursor-pointer disabled:opacity-50"
            >
              취소
            </button>
          </div>
        </div>
      )}

      {boards.length === 0 ? (
        <EmptyState message="아직 생성된 보드가 없습니다" />
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-4">
          {boards.map((board) => (
            <Link
              key={board.id}
              href={`/boards/${board.id}`}
              className="group block transition-transform duration-200 hover:-translate-y-0.5 focus-visible:-translate-y-0.5 focus-visible:outline-none"
            >
              <BoardCover images={board.cover_images} />
              <div className="mt-2">
                <div className="text-sm font-semibold text-text-primary group-hover:text-accent group-focus-visible:text-accent transition-colors truncate">
                  {board.name}
                </div>
                <div className="text-xs text-text-dim font-mono">
                  {board.pin_count} pins
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

export default function MyPageClient({
  creator: initialCreator,
  pins,
  hasMore,
}: {
  creator: CreatorPrivate;
  pins: Pin[];
  hasMore: boolean;
}) {
  const [creator, setCreator] = useState(initialCreator);
  const [editing, setEditing] = useState(false);

  return (
    <div className="space-y-8">
      {editing ? (
        <ProfileEditForm
          creator={creator}
          onSave={(updated) => {
            setCreator(updated);
            setEditing(false);
          }}
          onCancel={() => setEditing(false)}
        />
      ) : (
        <ProfileHeader
          creator={creator}
          isOwner
          onEdit={() => setEditing(true)}
        />
      )}

      <BoardSection creatorId={creator.id} />

      <PinsGrid
        creatorId={creator.id}
        initialPins={pins}
        initialHasMore={hasMore}
      />
    </div>
  );
}
