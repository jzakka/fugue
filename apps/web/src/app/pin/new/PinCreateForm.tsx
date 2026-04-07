"use client";

import { useState, useRef, useCallback, useEffect } from "react";
import { useRouter } from "next/navigation";
import { fetchOgPreview, fetchTags, createPin } from "@/lib/api";
import type { OgPreview, TagInfo } from "@/lib/api";

const TAG_MAX_COUNT = 10;

export default function PinCreateForm() {
  const router = useRouter();

  // Media file
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // URL + OG state (optional)
  const [url, setUrl] = useState("");
  const [ogLoading, setOgLoading] = useState(false);
  const [ogData, setOgData] = useState<OgPreview | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  // Form fields
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");

  // Tag selection
  const [allTags, setAllTags] = useState<TagInfo[]>([]);
  const [selectedTagIds, setSelectedTagIds] = useState<Set<string>>(new Set());
  const [tagSearch, setTagSearch] = useState("");
  const [activeCategory, setActiveCategory] = useState("");

  // Submit state
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load tags on mount
  useEffect(() => {
    fetchTags().then((res) => setAllTags(res.tags)).catch(() => {});
  }, []);

  // File handling
  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (!f) return;
    setFile(f);

    // Generate preview for images/video
    if (f.type.startsWith("image/") || f.type.startsWith("video/")) {
      const objectUrl = URL.createObjectURL(f);
      setPreview(objectUrl);
    } else {
      setPreview(null);
    }
  }

  function removeFile() {
    setFile(null);
    if (preview) URL.revokeObjectURL(preview);
    setPreview(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  // OG fetch (optional, triggered when URL changes)
  const handleOgFetch = useCallback(async (inputUrl: string) => {
    abortRef.current?.abort();
    const trimmed = inputUrl.trim();
    if (!trimmed) {
      setOgData(null);
      return;
    }
    try {
      new URL(trimmed);
    } catch {
      return;
    }
    setOgLoading(true);
    const controller = new AbortController();
    abortRef.current = controller;
    try {
      const data = await fetchOgPreview(trimmed);
      if (controller.signal.aborted) return;
      setOgData(data);
      if (!title && data.title) setTitle(data.title);
      if (!description && data.description) setDescription(data.description);
    } catch {
      if (controller.signal.aborted) return;
      setOgData(null);
    } finally {
      if (!controller.signal.aborted) setOgLoading(false);
    }
  }, [title, description]);

  function handleUrlChange(value: string) {
    setUrl(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => handleOgFetch(value), 500);
  }

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      abortRef.current?.abort();
    };
  }, []);

  // Tag helpers
  const categories = [...new Set(allTags.map((t) => t.category))];

  const filteredTags = allTags.filter((t) => {
    if (activeCategory && t.category !== activeCategory) return false;
    if (tagSearch && !t.name.toLowerCase().includes(tagSearch.toLowerCase())) return false;
    return true;
  });

  function toggleTag(id: string) {
    setSelectedTagIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        if (next.size >= TAG_MAX_COUNT) return prev;
        next.add(id);
      }
      return next;
    });
  }

  // Submit
  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!file) {
      setError("미디어 파일을 선택해주세요");
      return;
    }
    if (!title.trim()) {
      setError("제목을 입력해주세요");
      return;
    }
    if (selectedTagIds.size === 0) {
      setError("태그를 1개 이상 선택해주세요");
      return;
    }

    const formData = new FormData();
    formData.append("media", file);
    formData.append("title", title.trim());
    if (description.trim()) formData.append("description", description.trim());
    if (url.trim()) formData.append("url", url.trim());
    if (ogData?.image) formData.append("og_image", ogData.image);
    for (const tagId of selectedTagIds) {
      formData.append("tag_ids", tagId);
    }

    setSubmitting(true);
    try {
      await createPin(formData);
      router.push("/mypage");
    } catch (err) {
      setError(err instanceof Error ? err.message : "핀 등록에 실패했습니다");
    } finally {
      setSubmitting(false);
    }
  }

  const mediaType = file
    ? file.type.startsWith("image/")
      ? "이미지"
      : file.type.startsWith("audio/")
      ? "오디오"
      : file.type.startsWith("video/")
      ? "비디오"
      : file.type
    : null;

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <h1
        className="text-2xl font-bold tracking-tight"
        style={{ fontFamily: "'General Sans', sans-serif" }}
      >
        핀 생성
      </h1>

      {error && (
        <div className="p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error">
          {error}
        </div>
      )}

      {/* Media Upload */}
      <div>
        <label className="block text-sm text-text-muted mb-2">
          미디어 파일 <span className="text-error">*</span>
        </label>
        {!file ? (
          <div
            onClick={() => fileInputRef.current?.click()}
            className="border-2 border-dashed border-border rounded-[10px] p-8 text-center cursor-pointer hover:border-accent transition-colors"
          >
            <div className="text-3xl mb-2">📁</div>
            <div className="text-sm text-text-muted">
              클릭하여 파일을 선택하세요
            </div>
            <div
              className="text-xs text-text-dim mt-1"
              style={{ fontFamily: "'Geist Mono', monospace" }}
            >
              이미지 (10MB) / 오디오 (50MB) / 비디오 (100MB)
            </div>
          </div>
        ) : (
          <div className="border border-border rounded-[10px] overflow-hidden">
            {preview && file.type.startsWith("image/") && (
              <img src={preview} alt="미리보기" className="w-full max-h-48 object-cover" />
            )}
            {preview && file.type.startsWith("video/") && (
              <video src={preview} className="w-full max-h-48" controls preload="metadata" />
            )}
            {file.type.startsWith("audio/") && (
              <div className="p-4 bg-surface-elevated flex items-center gap-3">
                <span className="text-2xl">♪</span>
                <span className="text-sm truncate flex-1">{file.name}</span>
              </div>
            )}
            <div className="px-4 py-3 flex items-center justify-between bg-surface">
              <div className="text-xs text-text-muted">
                {file.name}{" "}
                <span
                  className="ml-2 px-2 py-0.5 bg-accent-subtle text-accent rounded-full"
                  style={{ fontFamily: "'Geist Mono', monospace" }}
                >
                  {mediaType}
                </span>
              </div>
              <button
                type="button"
                onClick={removeFile}
                className="text-xs text-error hover:underline cursor-pointer"
              >
                제거
              </button>
            </div>
          </div>
        )}
        <input
          ref={fileInputRef}
          type="file"
          accept="image/jpeg,image/png,image/gif,image/webp,audio/mpeg,audio/wav,audio/ogg,audio/flac,video/mp4,video/webm"
          onChange={handleFileChange}
          className="hidden"
        />
      </div>

      {/* Title */}
      <div>
        <label className="block text-sm text-text-muted mb-2">
          제목 <span className="text-error">*</span>
        </label>
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
          placeholder="작품에 대한 설명"
          rows={3}
          className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors resize-none"
        />
      </div>

      {/* URL (optional) */}
      <div>
        <label className="block text-sm text-text-muted mb-2">
          원본 URL <span className="text-text-dim">(선택)</span>
        </label>
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

      {/* OG Preview */}
      {ogData && !ogLoading && (
        <div className="bg-surface border border-border rounded-[10px] overflow-hidden">
          {ogData.image && (
            <div className="overflow-hidden max-h-32">
              <img src={ogData.image} alt={ogData.title || "미리보기"} className="w-full object-cover" />
            </div>
          )}
          <div className="p-3">
            <div className="text-xs text-text-dim" style={{ fontFamily: "'Geist Mono', monospace" }}>
              {ogData.site_name || (url ? new URL(url).hostname : "")}
            </div>
            <div className="text-sm font-semibold text-text-primary">{ogData.title}</div>
          </div>
        </div>
      )}

      {/* Tag Selection */}
      <div>
        <label className="block text-sm text-text-muted mb-2">
          태그 ({selectedTagIds.size}/{TAG_MAX_COUNT}){" "}
          <span className="text-error">*</span>
        </label>

        {/* Selected tags */}
        {selectedTagIds.size > 0 && (
          <div className="flex flex-wrap gap-2 mb-3">
            {[...selectedTagIds].map((id) => {
              const tag = allTags.find((t) => t.id === id);
              if (!tag) return null;
              return (
                <button
                  key={id}
                  type="button"
                  onClick={() => toggleTag(id)}
                  className="flex items-center gap-1 px-2.5 py-1 bg-accent text-white rounded-full text-xs cursor-pointer"
                  style={{ fontFamily: "'Geist Mono', monospace" }}
                >
                  {tag.name}
                  <span className="ml-0.5">×</span>
                </button>
              );
            })}
          </div>
        )}

        {/* Search */}
        <input
          type="text"
          value={tagSearch}
          onChange={(e) => setTagSearch(e.target.value)}
          placeholder="태그 검색..."
          className="w-full px-4 py-2 bg-bg border border-border rounded-[6px] text-sm text-text-primary outline-none focus:border-accent transition-colors mb-3"
        />

        {/* Category tabs */}
        <div className="flex gap-1.5 overflow-x-auto mb-3 scrollbar-hide">
          <button
            type="button"
            onClick={() => setActiveCategory("")}
            className={`px-3 py-1 rounded-full text-xs whitespace-nowrap cursor-pointer transition-colors ${
              activeCategory === ""
                ? "bg-text-primary text-bg"
                : "bg-surface border border-border text-text-muted"
            }`}
          >
            전체
          </button>
          {categories.map((cat) => (
            <button
              key={cat}
              type="button"
              onClick={() => setActiveCategory(cat)}
              className={`px-3 py-1 rounded-full text-xs whitespace-nowrap cursor-pointer transition-colors ${
                activeCategory === cat
                  ? "bg-text-primary text-bg"
                  : "bg-surface border border-border text-text-muted"
              }`}
            >
              {cat}
            </button>
          ))}
        </div>

        {/* Tag grid */}
        <div className="max-h-48 overflow-y-auto border border-border rounded-[6px] p-2">
          <div className="flex flex-wrap gap-1.5">
            {filteredTags.map((tag) => {
              const selected = selectedTagIds.has(tag.id);
              return (
                <button
                  key={tag.id}
                  type="button"
                  onClick={() => toggleTag(tag.id)}
                  disabled={!selected && selectedTagIds.size >= TAG_MAX_COUNT}
                  className={`px-2.5 py-1 rounded-full text-xs cursor-pointer transition-colors ${
                    selected
                      ? "bg-accent text-white"
                      : "bg-accent-subtle text-text-muted hover:bg-accent/20 disabled:opacity-40 disabled:cursor-not-allowed"
                  }`}
                  style={{ fontFamily: "'Geist Mono', monospace" }}
                >
                  {tag.name}
                </button>
              );
            })}
            {filteredTags.length === 0 && (
              <div className="text-xs text-text-dim py-4 w-full text-center">
                일치하는 태그가 없습니다
              </div>
            )}
          </div>
        </div>
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
