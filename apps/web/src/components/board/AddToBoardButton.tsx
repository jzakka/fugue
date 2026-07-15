"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import type { Board } from "@/lib/api";
import {
  fetchBoards,
  addPinToBoard,
  createBoard,
  recordInteraction,
} from "@/lib/api";
import Link from "next/link";
import { useRouter } from "next/navigation";
import EmptyState from "@/components/feed/EmptyState";

interface AddToBoardButtonProps {
  pinId: string;
  userId: string | null;
}

export default function AddToBoardButton({
  pinId,
  userId,
}: AddToBoardButtonProps) {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);

  if (!userId) {
    return (
      <Link
        href={`/login?redirect=/pins/${pinId}`}
        className="inline-flex items-center gap-2 px-5 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent focus-visible:text-text-primary focus-visible:border-accent transition-colors"
      >
        <BoardIcon />
        보드에 추가
      </Link>
    );
  }

  return (
    <>
      <button
        ref={triggerRef}
        onClick={() => setIsOpen(true)}
        className="inline-flex items-center gap-2 px-5 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary hover:border-accent focus-visible:text-text-primary focus-visible:border-accent transition-colors cursor-pointer"
      >
        <BoardIcon />
        보드에 추가
      </button>
      {isOpen && (
        <BoardSelectModal
          pinId={pinId}
          userId={userId}
          onClose={() => {
            setIsOpen(false);
            triggerRef.current?.focus();
          }}
        />
      )}
    </>
  );
}

function BoardIcon() {
  return (
    <svg
      aria-hidden="true"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="3" y="3" width="7" height="7" />
      <rect x="14" y="3" width="7" height="7" />
      <rect x="3" y="14" width="7" height="7" />
      <path d="M17.5 14v7M14 17.5h7" />
    </svg>
  );
}

function BoardSelectModal({
  pinId,
  userId,
  onClose,
}: {
  pinId: string;
  userId: string;
  onClose: () => void;
}) {
  const router = useRouter();
  const [boards, setBoards] = useState<Board[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [newBoardName, setNewBoardName] = useState("");
  const [creating, setCreating] = useState(false);

  const panelRef = useRef<HTMLDivElement>(null);

  const loadBoards = useCallback(async () => {
    try {
      const data = await fetchBoards(userId);
      setBoards(data);
    } catch {
      // Silently fail
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    loadBoards();
  }, [loadBoards]);

  // Lock background scroll
  useEffect(() => {
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = "";
    };
  }, []);

  // Initial focus to dialog container (WAI-ARIA Dialog Pattern)
  useEffect(() => {
    panelRef.current?.focus();
  }, []);

  // ESC to close
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  // Tab focus trap (WAI-ARIA Dialog Pattern)
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key !== "Tab") return;
      const panel = panelRef.current;
      if (!panel) return;
      const focusables = panel.querySelectorAll<HTMLElement>(
        'button:not([disabled]), [href], input:not([disabled]), [tabindex]:not([tabindex="-1"])'
      );
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const active = document.activeElement as HTMLElement | null;
      if (e.shiftKey) {
        if (active === first || active === panel) {
          e.preventDefault();
          last.focus();
        }
      } else if (active === last) {
        e.preventDefault();
        first.focus();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Auto-close on success
  useEffect(() => {
    if (feedback?.type === "success") {
      const timer = setTimeout(onClose, 1500);
      return () => clearTimeout(timer);
    }
  }, [feedback, onClose]);

  function handleOverlayClick(e: React.MouseEvent) {
    if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
      onClose();
    }
  }

  async function handleSelectBoard(boardId: string, boardName: string) {
    setAdding(boardId);
    setFeedback(null);
    try {
      await addPinToBoard(boardId, pinId);
      recordInteraction(pinId, "board_add");
      router.refresh();
      setFeedback({
        type: "success",
        message: `"${boardName}" 보드에 추가했습니다`,
      });
    } catch (err) {
      const message =
        err instanceof Error && err.message.includes("409")
          ? "이미 이 보드에 추가된 핀입니다"
          : "보드 추가에 실패했습니다. 다시 시도해주세요";
      setFeedback({ type: "error", message });
    } finally {
      setAdding(null);
    }
  }

  async function handleCreateAndAdd() {
    const trimmed = newBoardName.trim();
    if (!trimmed) return;

    setCreating(true);
    setFeedback(null);
    try {
      const board = await createBoard({ name: trimmed });
      setBoards((prev) => [board, ...prev]);
      setNewBoardName("");
      setShowCreate(false);
      await handleSelectBoard(board.id, board.name);
    } catch (err) {
      setFeedback({
        type: "error",
        message:
          err instanceof Error ? err.message : "보드 생성에 실패했습니다",
      });
    } finally {
      setCreating(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      onClick={handleOverlayClick}
    >
      {/* Overlay */}
      <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" />

      {/* Panel */}
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="add-to-board-modal-title"
        tabIndex={-1}
        className="relative bg-surface-elevated border border-border rounded-[16px] w-full max-w-sm mx-4 max-h-[80vh] flex flex-col"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 pt-6 pb-4 border-b border-border">
          <h2 id="add-to-board-modal-title" className="text-lg font-bold tracking-tight font-display">
            보드에 추가
          </h2>
          <button
            onClick={onClose}
            aria-label="닫기"
            className="w-8 h-8 flex items-center justify-center rounded-full hover:bg-surface-hover focus-visible:bg-surface-hover transition-colors text-text-muted hover:text-text-primary focus-visible:text-text-primary cursor-pointer"
          >
            <svg
              aria-hidden="true"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        {/* Feedback */}
        {feedback && (
          <div
            role={feedback.type === "success" ? "status" : "alert"}
            aria-live="polite"
            className={`mx-6 mt-4 px-3 py-2 rounded-[6px] text-xs ${
              feedback.type === "success"
                ? "bg-success/10 border border-success/30 text-success"
                : "bg-error/10 border border-error/30 text-error"
            }`}
          >
            {feedback.message}
          </div>
        )}

        {/* Board List */}
        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-1">
          {loading ? (
            <div
              role="status"
              aria-label="보드 목록 로딩 중"
              className="flex justify-center py-8"
            >
              <div className="w-6 h-6 border-2 border-accent border-t-transparent rounded-full animate-spin" />
            </div>
          ) : boards.length === 0 && !showCreate ? (
            <EmptyState message="아직 생성된 보드가 없습니다" />
          ) : (
            boards.map((board) => (
              <button
                key={board.id}
                onClick={() => handleSelectBoard(board.id, board.name)}
                disabled={adding !== null || feedback?.type === "success"}
                aria-busy={adding === board.id}
                className="w-full flex items-center gap-3 px-3 py-3 rounded-[10px] hover:bg-surface-hover focus-visible:bg-surface-hover transition-colors text-left disabled:opacity-50 cursor-pointer"
              >
                {/* Mini cover */}
                <div className="w-10 h-10 rounded-[6px] bg-surface flex-shrink-0 overflow-hidden border border-border">
                  {board.cover_images.length > 0 ? (
                    <img
                      src={board.cover_images[0]}
                      alt=""
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center">
                      <svg
                        aria-hidden="true"
                        width="14"
                        height="14"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        className="text-text-dim"
                      >
                        <rect x="3" y="3" width="7" height="7" />
                        <rect x="14" y="3" width="7" height="7" />
                        <rect x="3" y="14" width="7" height="7" />
                        <rect x="14" y="14" width="7" height="7" />
                      </svg>
                    </div>
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-text-primary truncate">
                    {board.name}
                  </div>
                  <div className="text-xs text-text-dim font-mono">
                    {board.pin_count} pins
                  </div>
                </div>
                {adding === board.id && (
                  <div
                    role="status"
                    aria-label="보드에 추가 중"
                    className="w-4 h-4 border-2 border-accent border-t-transparent rounded-full animate-spin flex-shrink-0"
                  />
                )}
              </button>
            ))
          )}
        </div>

        {/* Create new board */}
        <div className="px-6 pb-6 pt-2 border-t border-border">
          {showCreate ? (
            <div className="space-y-3">
              <input
                type="text"
                value={newBoardName}
                onChange={(e) => setNewBoardName(e.target.value)}
                placeholder="보드 이름"
                aria-label="새 보드 이름"
                aria-required="true"
                maxLength={100}
                autoFocus
                className="w-full px-3 py-2 bg-bg border border-border rounded-[6px] text-sm text-text-primary outline-none focus:border-accent transition-colors"
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleCreateAndAdd();
                  }
                }}
              />
              <div className="flex gap-2">
                <button
                  onClick={handleCreateAndAdd}
                  disabled={creating || !newBoardName.trim()}
                  aria-busy={creating}
                  className="px-3 py-1.5 bg-accent text-white rounded-full text-xs font-semibold hover:bg-accent-hover focus-visible:bg-accent-hover transition-colors disabled:opacity-50 cursor-pointer"
                >
                  {creating ? "생성 중..." : "생성 및 추가"}
                </button>
                <button
                  onClick={() => {
                    setShowCreate(false);
                    setNewBoardName("");
                  }}
                  className="px-3 py-1.5 border border-border rounded-full text-xs text-text-muted hover:text-text-primary focus-visible:text-text-primary transition-colors cursor-pointer"
                >
                  취소
                </button>
              </div>
            </div>
          ) : (
            <button
              onClick={() => setShowCreate(true)}
              disabled={feedback?.type === "success"}
              className="w-full flex items-center justify-center gap-2 px-3 py-2.5 border border-border border-dashed rounded-[10px] text-sm text-text-muted hover:text-text-primary hover:border-accent focus-visible:text-text-primary focus-visible:border-accent transition-colors disabled:opacity-50 cursor-pointer"
            >
              <svg
                aria-hidden="true"
                width="14"
                height="14"
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
              새 보드 만들기
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
