"use client";

import { useState, useRef, useCallback, useEffect } from "react";
import { useRouter } from "next/navigation";
import { fetchOgPreview, createPin } from "@/lib/api";
import type { OgPreview } from "@/lib/api";

const FIELD_OPTIONS = [
  "미술",
  "음악",
  "영상편집",
  "프로그래밍",
  "글",
  "기타",
] as const;

const TAG_MAX_LENGTH = 30;
const TAG_MIN_COUNT = 1;
const TAG_MAX_COUNT = 5;

export default function PinCreateForm() {
  const router = useRouter();

  // URL + OG state
  const [url, setUrl] = useState("");
  const [ogLoading, setOgLoading] = useState(false);
  const [ogData, setOgData] = useState<OgPreview | null>(null);
  const [ogError, setOgError] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Form fields
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [field, setField] = useState<string>("");
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");

  // Submit state
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleOgFetch = useCallback(async (inputUrl: string) => {
    // Cancel previous request
    if (abortRef.current) {
      abortRef.current.abort();
    }

    const trimmed = inputUrl.trim();
    if (!trimmed) {
      setOgData(null);
      setOgError(false);
      return;
    }

    // Simple URL validation
    try {
      new URL(trimmed);
    } catch {
      return;
    }

    setOgLoading(true);
    setOgError(false);

    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const data = await fetchOgPreview(trimmed);

      // Check if this request was aborted
      if (controller.signal.aborted) return;

      setOgData(data);
      setTitle(data.title || "");
      setDescription(data.description || "");
      if (data.detected_field) {
        setField(data.detected_field);
      }
      if (data.suggested_tags?.length) {
        setTags(data.suggested_tags.slice(0, TAG_MAX_COUNT));
      }
    } catch (err) {
      if (controller.signal.aborted) return;
      setOgError(true);
      setOgData(null);
    } finally {
      if (!controller.signal.aborted) {
        setOgLoading(false);
      }
    }
  }, []);

  function handleUrlChange(value: string) {
    setUrl(value);

    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }

    debounceRef.current = setTimeout(() => {
      handleOgFetch(value);
    }, 500);
  }

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      if (abortRef.current) abortRef.current.abort();
    };
  }, []);

  function handleTagKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      addTag();
    }
    if (e.key === "Backspace" && tagInput === "" && tags.length > 0) {
      setTags((prev) => prev.slice(0, -1));
    }
  }

  function addTag() {
    const trimmed = tagInput.trim().replace(/,/g, "");
    if (!trimmed) return;
    if (trimmed.length > TAG_MAX_LENGTH) {
      setError(`태그는 ${TAG_MAX_LENGTH}자 이하여야 합니다`);
      return;
    }
    if (tags.length >= TAG_MAX_COUNT) {
      setError(`태그는 최대 ${TAG_MAX_COUNT}개까지 추가할 수 있습니다`);
      return;
    }
    if (tags.includes(trimmed)) {
      setTagInput("");
      return;
    }
    setTags((prev) => [...prev, trimmed]);
    setTagInput("");
    setError(null);
  }

  function removeTag(index: number) {
    setTags((prev) => prev.filter((_, i) => i !== index));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    const trimmedUrl = url.trim();
    const trimmedTitle = title.trim();

    if (!trimmedUrl) {
      setError("URL을 입력해주세요");
      return;
    }
    if (!trimmedTitle) {
      setError("제목을 입력해주세요");
      return;
    }
    if (!field) {
      setError("분야를 선택해주세요");
      return;
    }
    setSubmitting(true);
    try {
      await createPin({
        url: trimmedUrl,
        title: trimmedTitle,
        description: description.trim() || undefined,
        field,
        tags,
        og_image: ogData?.image || undefined,
        og_data: ogData ? { ...ogData } : undefined,
      });
      router.push("/mypage");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "작품 등록에 실패했습니다"
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <h1
        className="text-2xl font-bold tracking-tight"
        style={{ fontFamily: "'General Sans', sans-serif" }}
      >
        작품 올리기
      </h1>

      {error && (
        <div className="p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error">
          {error}
        </div>
      )}

      {/* URL Input */}
      <div>
        <label className="block text-sm text-text-muted mb-2">URL</label>
        <input
          type="url"
          value={url}
          onChange={(e) => handleUrlChange(e.target.value)}
          placeholder="https://..."
          className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors"
        />
        {ogLoading && (
          <div className="mt-2 flex items-center gap-2 text-sm text-text-muted">
            <div className="w-4 h-4 border-2 border-accent border-t-transparent rounded-full animate-spin" />
            미리보기를 불러오는 중...
          </div>
        )}
      </div>

      {/* OG Preview Card */}
      {ogData && !ogLoading && (
        <div className="bg-surface border border-border rounded-[10px] overflow-hidden">
          {ogData.image && (
            <div className="overflow-hidden max-h-48">
              <img
                src={ogData.image}
                alt={ogData.title || "미리보기"}
                className="w-full object-cover"
              />
            </div>
          )}
          <div className="p-4">
            <div
              className="text-xs text-text-dim mb-1"
              style={{ fontFamily: "'Geist Mono', monospace" }}
            >
              {ogData.site_name || new URL(url).hostname}
            </div>
            <div className="text-sm font-semibold text-text-primary">
              {ogData.title}
            </div>
            {ogData.description && (
              <p className="text-xs text-text-muted mt-1 line-clamp-2">
                {ogData.description}
              </p>
            )}
          </div>
        </div>
      )}

      {ogError && (
        <div className="p-3 bg-warning/10 border border-warning/30 rounded-[6px] text-sm text-warning">
          미리보기를 불러올 수 없습니다. 아래에서 직접 입력해주세요.
        </div>
      )}

      {/* Title */}
      <div>
        <label className="block text-sm text-text-muted mb-2">제목</label>
        <input
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="작품 제목"
          maxLength={200}
          className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors"
        />
      </div>

      {/* Description */}
      <div>
        <label className="block text-sm text-text-muted mb-2">설명</label>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="작품에 대한 설명을 입력해주세요"
          rows={3}
          className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors resize-none"
        />
      </div>

      {/* Field */}
      <div>
        <label className="block text-sm text-text-muted mb-2">분야</label>
        <select
          value={field}
          onChange={(e) => setField(e.target.value)}
          className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors appearance-none cursor-pointer"
        >
          <option value="" disabled>
            분야를 선택해주세요
          </option>
          {FIELD_OPTIONS.map((f) => (
            <option key={f} value={f}>
              {f}
            </option>
          ))}
        </select>
      </div>

      {/* Tags */}
      <div>
        <label className="block text-sm text-text-muted mb-2">
          태그 ({tags.length}/{TAG_MAX_COUNT})
        </label>
        <div className="flex flex-wrap items-center gap-2 p-2.5 bg-bg border border-border rounded-[6px] focus-within:border-accent transition-colors min-h-[44px]">
          {tags.map((tag, i) => (
            <span
              key={tag}
              className="flex items-center gap-1 px-2.5 py-1 bg-accent-subtle text-accent rounded-full text-xs"
              style={{ fontFamily: "'Geist Mono', monospace" }}
            >
              {tag}
              <button
                type="button"
                onClick={() => removeTag(i)}
                className="ml-0.5 text-accent hover:text-accent-hover cursor-pointer"
              >
                x
              </button>
            </span>
          ))}
          {tags.length < TAG_MAX_COUNT && (
            <input
              type="text"
              value={tagInput}
              onChange={(e) => setTagInput(e.target.value)}
              onKeyDown={handleTagKeyDown}
              onBlur={addTag}
              placeholder={tags.length === 0 ? "태그 입력 후 Enter" : ""}
              maxLength={TAG_MAX_LENGTH}
              className="flex-1 min-w-[120px] bg-transparent text-sm text-text-primary outline-none placeholder:text-text-dim"
            />
          )}
        </div>
        <p className="mt-1 text-xs text-text-dim">
          Enter 또는 쉼표로 태그 추가, 최대 {TAG_MAX_COUNT}개
        </p>
      </div>

      {/* Submit */}
      <div className="flex gap-3 justify-end pt-2">
        <button
          type="button"
          onClick={() => router.back()}
          disabled={submitting}
          className="px-5 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary transition-colors cursor-pointer"
        >
          취소
        </button>
        <button
          type="submit"
          disabled={submitting}
          className="px-6 py-2.5 bg-accent text-white rounded-full text-sm font-semibold hover:bg-accent-hover transition-colors disabled:opacity-50 cursor-pointer"
        >
          {submitting ? "등록 중..." : "등록하기"}
        </button>
      </div>
    </form>
  );
}
