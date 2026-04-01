"use client";

import { useState } from "react";
import type { CreatorPrivate } from "@/lib/api";
import { updateMe } from "@/lib/api";

const AVAILABLE_ROLES = [
  "작곡",
  "보컬",
  "일러스트",
  "영상",
  "글",
  "사진",
  "기획",
  "기타",
];

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
  const [bio, setBio] = useState(creator.bio || "");
  const [roles, setRoles] = useState<string[]>(creator.roles);
  const [contacts, setContacts] = useState<Record<string, string>>(
    creator.contacts || {}
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function toggleRole(role: string) {
    setRoles((prev) =>
      prev.includes(role) ? prev.filter((r) => r !== role) : [...prev, role]
    );
  }

  function handleContactChange(key: string, value: string) {
    setContacts((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    const trimmed = nickname.trim();
    if (!trimmed) {
      setError("닉네임을 입력해주세요");
      return;
    }
    if (roles.length === 0) {
      setError("역할을 최소 하나 선택해주세요");
      return;
    }

    setSaving(true);
    try {
      const cleaned: Record<string, string> = {};
      for (const [k, v] of Object.entries(contacts)) {
        if (v.trim()) cleaned[k] = v.trim();
      }

      const updated = await updateMe({
        nickname: trimmed,
        bio: bio.trim() || undefined,
        roles,
        contacts: cleaned,
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
      <h2 className="text-xl font-bold">프로필 편집</h2>

      {error && (
        <div className="p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error">
          {error}
        </div>
      )}

      {/* Nickname */}
      <div>
        <label className="block text-sm text-text-muted mb-2">닉네임</label>
        <input
          type="text"
          value={nickname}
          onChange={(e) => setNickname(e.target.value)}
          maxLength={200}
          className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors"
        />
      </div>

      {/* Bio */}
      <div>
        <label className="block text-sm text-text-muted mb-2">소개</label>
        <textarea
          value={bio}
          onChange={(e) => setBio(e.target.value)}
          rows={3}
          className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors resize-none"
          placeholder="자신을 소개해주세요..."
        />
      </div>

      {/* Roles */}
      <div>
        <label className="block text-sm text-text-muted mb-2">역할</label>
        <div className="flex flex-wrap gap-2">
          {AVAILABLE_ROLES.map((role) => (
            <button
              key={role}
              type="button"
              onClick={() => toggleRole(role)}
              className={`px-3 py-1.5 rounded-full text-sm transition-colors cursor-pointer ${
                roles.includes(role)
                  ? "bg-accent text-white"
                  : "bg-surface-elevated border border-border text-text-muted hover:border-accent"
              }`}
            >
              {role}
            </button>
          ))}
        </div>
      </div>

      {/* Contacts */}
      <div>
        <label className="block text-sm text-text-muted mb-2">연락처</label>
        <div className="space-y-2">
          {["twitter", "discord", "instagram", "website"].map((key) => (
            <div key={key} className="flex gap-2 items-center">
              <span
                className="w-24 text-xs text-text-dim shrink-0"
                style={{ fontFamily: "'Geist Mono', monospace" }}
              >
                {key}
              </span>
              <input
                type="text"
                value={contacts[key] || ""}
                onChange={(e) => handleContactChange(key, e.target.value)}
                className="flex-1 px-3 py-2 bg-bg border border-border rounded-[6px] text-sm text-text-primary outline-none focus:border-accent transition-colors"
                placeholder={
                  key === "twitter"
                    ? "@username"
                    : key === "discord"
                      ? "username#1234"
                      : key === "website"
                        ? "https://..."
                        : ""
                }
              />
            </div>
          ))}
        </div>
      </div>

      {/* Actions */}
      <div className="flex gap-3 justify-end pt-2">
        <button
          type="button"
          onClick={onCancel}
          disabled={saving}
          className="px-5 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary transition-colors cursor-pointer"
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
