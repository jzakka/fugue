"use client";

import { useState } from "react";
import type { CreatorPrivate } from "@/lib/api";
import { updateMe } from "@/lib/api";

export default function ProfileEditForm({
  creator,
  onSave,
  onCancel,
}: {
  creator: CreatorPrivate;
  onSave: (updated: CreatorPrivate) => void;
  onCancel: () => void;
}) {
  const [nickname, setNickname] = useState(creator.nickname);
  const [avatarUrl, setAvatarUrl] = useState(creator.avatar_url || "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    const trimmed = nickname.trim();
    if (!trimmed) {
      setError("닉네임을 입력해주세요");
      return;
    }

    setSaving(true);
    try {
      const updated = await updateMe({
        nickname: trimmed,
        avatar_url: avatarUrl.trim() || undefined,
      });
      onSave(updated);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "프로필 업데이트에 실패했습니다"
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="bg-surface rounded-[16px] p-6 sm:p-8 border border-border space-y-6"
    >
      <h2 className="text-xl font-bold font-display tracking-tight">프로필 편집</h2>

      {error && (
        <div className="p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error">
          {error}
        </div>
      )}

      {/* Nickname */}
      <div>
        <label htmlFor="profile-nickname" className="block text-sm text-text-muted mb-2 font-medium">닉네임</label>
        <input
          id="profile-nickname"
          type="text"
          value={nickname}
          onChange={(e) => setNickname(e.target.value)}
          maxLength={200}
          className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors"
        />
      </div>

      {/* Avatar URL */}
      <div>
        <label htmlFor="profile-avatar-url" className="block text-sm text-text-muted mb-2 font-medium">
          아바타 URL
        </label>
        <input
          id="profile-avatar-url"
          type="url"
          value={avatarUrl}
          onChange={(e) => setAvatarUrl(e.target.value)}
          placeholder="https://..."
          className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors"
        />
        {avatarUrl.trim() && (
          <div className="mt-2">
            <img
              src={avatarUrl}
              alt="미리보기"
              className="w-16 h-16 rounded-full border-2 border-border object-cover"
              onError={(e) => {
                (e.target as HTMLImageElement).style.display = "none";
              }}
            />
          </div>
        )}
      </div>

      {/* Actions */}
      <div className="flex gap-3 justify-end pt-2">
        <button
          type="button"
          onClick={onCancel}
          disabled={saving}
          className="px-5 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary transition-colors cursor-pointer disabled:opacity-50"
        >
          취소
        </button>
        <button
          type="submit"
          disabled={saving}
          className="px-5 py-2.5 bg-accent text-white rounded-full text-sm font-semibold hover:bg-accent-hover transition-colors disabled:opacity-50 cursor-pointer"
        >
          {saving ? "저장 중..." : "저장"}
        </button>
      </div>
    </form>
  );
}
