"use client";

import { useState, useRef, useCallback, useEffect } from "react";
import { useRouter } from "next/navigation";
import { fetchOgPreview, fetchTags, createPin } from "@/lib/api";
import type { OgPreview, TagInfo } from "@/lib/api";
import {
  validateAndOptimize,
  type ProgressInfo,
  type OptimizeResult,
} from "@/lib/media";
import { getVideoDuration, MAX_VIDEO_DURATION_SECONDS } from "@/lib/media/video";
import VideoTrimModal from "@/components/pin/VideoTrimModal";
import VideoThumbnailPicker from "@/components/pin/VideoThumbnailPicker";

const TAG_MAX_COUNT = 10;

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
}

function formatTime(s: number): string {
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m}:${sec.toFixed(1).padStart(4, "0")}`;
}

export default function PinCreateForm() {
  const router = useRouter();

  // Media file
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Video trim state
  const [showTrimModal, setShowTrimModal] = useState(false);
  const [pendingVideoFile, setPendingVideoFile] = useState<File | null>(null);
  const [videoDuration, setVideoDuration] = useState(0);
  const [trimStart, setTrimStart] = useState<number | null>(null);
  const [trimEnd, setTrimEnd] = useState<number | null>(null);
  const [thumbnail, setThumbnail] = useState<Blob | null>(null);

  // Optimization state
  const [optimizing, setOptimizing] = useState(false);
  const [optimizeProgress, setOptimizeProgress] =
    useState<ProgressInfo | null>(null);
  const [optimizeResult, setOptimizeResult] =
    useState<OptimizeResult | null>(null);

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
    const controller = new AbortController();
    fetchTags(undefined, { signal: controller.signal })
      .then((res) => setAllTags(res.tags))
      .catch(() => {});
    return () => controller.abort();
  }, []);

  // File handling
  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (!f) return;

    // Revoke previous preview URL
    if (preview) URL.revokeObjectURL(preview);
    setPreview(null);
    setError(null);
    setOptimizeResult(null);
    setOptimizeProgress(null);
    setTrimStart(null);
    setTrimEnd(null);

    // Video: check duration and show trim modal if > 15s
    if (f.type.startsWith("video/")) {
      try {
        const dur = await getVideoDuration(f);
        if (!Number.isFinite(dur)) {
          setError("지원하지 않는 비디오 형식입니다");
          if (fileInputRef.current) fileInputRef.current.value = "";
          return;
        }
        setVideoDuration(dur);
        if (dur > MAX_VIDEO_DURATION_SECONDS) {
          // Show trim modal
          setPendingVideoFile(f);
          setShowTrimModal(true);
          return;
        }
        // <= 15s: use as-is, no trim needed
      } catch {
        setError("비디오를 로드할 수 없습니다");
        if (fileInputRef.current) fileInputRef.current.value = "";
        return;
      }
    }

    // Non-video or short video: run optimization pipeline
    await processFile(f);
  }

  async function processFile(f: File) {
    setOptimizing(true);
    try {
      const result = await validateAndOptimize(f, (info) => {
        setOptimizeProgress(info);
      });

      setFile(result.file);
      setOptimizeResult(result);

      if (
        result.file.type.startsWith("image/") ||
        result.file.type.startsWith("video/")
      ) {
        setPreview(URL.createObjectURL(result.file));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "파일 처리에 실패했습니다");
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
    } finally {
      setOptimizing(false);
      setOptimizeProgress(null);
    }
  }

  function handleTrimConfirm(start: number, end: number) {
    setShowTrimModal(false);
    setTrimStart(start);
    setTrimEnd(end);
    if (pendingVideoFile) {
      processFile(pendingVideoFile);
    }
    setPendingVideoFile(null);
  }

  function handleTrimCancel() {
    setShowTrimModal(false);
    setPendingVideoFile(null);
    setFile(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  function removeFile() {
    setFile(null);
    if (preview) URL.revokeObjectURL(preview);
    setPreview(null);
    setOptimizeResult(null);
    setTrimStart(null);
    setTrimEnd(null);
    setThumbnail(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  // OG fetch
  const handleOgFetch = useCallback(
    async (inputUrl: string) => {
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
    },
    [title, description]
  );

  function handleUrlChange(value: string) {
    setUrl(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => handleOgFetch(value), 500);
  }

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      abortRef.current?.abort();
      if (preview) URL.revokeObjectURL(preview);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Tag helpers
  const categories = [...new Set(allTags.map((t) => t.category))];

  const filteredTags = allTags.filter((t) => {
    if (activeCategory && t.category !== activeCategory) return false;
    if (tagSearch && !t.name.toLowerCase().includes(tagSearch.toLowerCase()))
      return false;
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
    const formData = new FormData();
    formData.append("media", file);
    formData.append("title", title.trim());
    if (description.trim()) formData.append("description", description.trim());
    if (url.trim()) formData.append("url", url.trim());
    if (ogData?.image) formData.append("og_image", ogData.image);
    for (const tagId of selectedTagIds) {
      formData.append("tag_ids", tagId);
    }

    // Include trim info for server-side processing
    if (trimStart != null && trimEnd != null) {
      formData.append("trim_start", String(trimStart));
      formData.append("trim_end", String(trimEnd));
    }

    // Include video thumbnail
    if (thumbnail) {
      formData.append("thumbnail", thumbnail, "thumbnail.jpg");
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

  const isDisabled = submitting || optimizing;

  return (
    <>
      {/* Video Trim Modal */}
      {showTrimModal && pendingVideoFile && (
        <VideoTrimModal
          file={pendingVideoFile}
          videoDuration={videoDuration}
          onConfirm={handleTrimConfirm}
          onCancel={handleTrimCancel}
        />
      )}

      <form onSubmit={handleSubmit} className="space-y-6" aria-labelledby="pin-create-form-title">
        <h1 id="pin-create-form-title" className="text-2xl font-bold tracking-tight font-display">
          핀 생성
        </h1>

        {error && (
          <div
            id="pin-error"
            role="alert"
            aria-live="polite"
            className="p-3 bg-error/10 border border-error/30 rounded-[6px] text-sm text-error"
          >
            {error}
          </div>
        )}

        {/* Media Upload */}
        <div>
          <div className="block text-sm text-text-muted mb-2 font-medium">
            미디어 파일 <span className="text-error">*</span>
          </div>
          {!file && !optimizing ? (
            <div
              onClick={() => fileInputRef.current?.click()}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  fileInputRef.current?.click();
                }
              }}
              role="button"
              tabIndex={0}
              aria-label="미디어 파일 선택"
              className="border-2 border-dashed border-border rounded-[10px] p-8 text-center cursor-pointer hover:border-accent focus-visible:border-accent focus-visible:outline-none transition-colors"
            >
              <div className="text-3xl mb-2">📁</div>
              <div className="text-sm text-text-muted">
                클릭하여 파일을 선택하세요
              </div>
              <div className="text-xs text-text-dim mt-1 font-mono">
                이미지 / 오디오 / 비디오
              </div>
              <div className="text-xs text-text-dim mt-1">
                업로드 시 자동 최적화가 적용됩니다
              </div>
            </div>
          ) : optimizing ? (
            <div className="border border-border rounded-[10px] p-6">
              <div
                role="status"
                aria-live="polite"
                className="flex items-center gap-3 mb-3"
              >
                <div className="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin" />
                <span className="text-sm text-text-muted">
                  {optimizeProgress?.stage || "파일 처리 중..."}
                </span>
              </div>
              {optimizeProgress && (
                <div
                  role="progressbar"
                  aria-valuenow={Math.round(optimizeProgress.progress)}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-label="파일 최적화 진행률"
                  className="w-full bg-border rounded-full h-2"
                >
                  <div
                    className="bg-accent h-2 rounded-full transition-all duration-300"
                    style={{ width: `${optimizeProgress.progress}%` }}
                  />
                </div>
              )}
            </div>
          ) : (
            <div className="border border-border rounded-[10px] overflow-hidden">
              {preview && file!.type.startsWith("image/") && (
                <img
                  src={preview}
                  alt="미리보기"
                  className="w-full max-h-48 object-cover"
                />
              )}
              {preview && file!.type.startsWith("video/") && (
                <video
                  src={preview}
                  className="w-full max-h-48"
                  controls
                  preload="metadata"
                />
              )}
              {file!.type.startsWith("audio/") && (
                <div className="p-4 bg-surface-elevated flex items-center gap-3">
                  <span aria-hidden="true" className="text-2xl">♪</span>
                  <span className="text-sm truncate flex-1">{file!.name}</span>
                </div>
              )}
              <div className="px-4 py-3 flex items-center justify-between bg-surface">
                <div className="text-xs text-text-muted flex items-center gap-2">
                  {file!.name}
                  <span className="px-2 py-0.5 bg-accent-subtle text-accent rounded-full font-mono text-3xs">
                    {mediaType}
                  </span>
                  {trimStart != null && trimEnd != null && (
                    <span className="text-text-dim font-mono">
                      {formatTime(trimStart)} ~ {formatTime(trimEnd)} ({(trimEnd - trimStart).toFixed(1)}초)
                    </span>
                  )}
                  {optimizeResult &&
                    optimizeResult.originalSize !== optimizeResult.optimizedSize && (
                      <span className="text-text-dim font-mono">
                        {formatSize(optimizeResult.originalSize)} →{" "}
                        {formatSize(optimizeResult.optimizedSize)}
                      </span>
                    )}
                </div>
                <button
                  type="button"
                  onClick={removeFile}
                  className="text-xs text-error hover:underline focus-visible:underline cursor-pointer"
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

        {/* Video Thumbnail Selection */}
        {file && file.type.startsWith("video/") && (
          <VideoThumbnailPicker
            file={file}
            trimStart={trimStart ?? 0}
            trimEnd={trimEnd ?? videoDuration}
            onSelect={setThumbnail}
          />
        )}

        {/* Title */}
        <div>
          <label htmlFor="pin-title" className="block text-sm text-text-muted mb-2 font-medium">
            제목 <span className="text-error" aria-hidden="true">*</span>
          </label>
          <input
            id="pin-title"
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="작품 제목"
            maxLength={200}
            aria-required="true"
            aria-invalid={!!error && !title.trim()}
            aria-describedby={error ? "pin-error" : undefined}
            className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors"
          />
        </div>

        {/* Description */}
        <div>
          <label htmlFor="pin-description" className="block text-sm text-text-muted mb-2 font-medium">설명</label>
          <textarea
            id="pin-description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="작품에 대한 설명"
            rows={3}
            className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors resize-none"
          />
        </div>

        {/* URL (optional) */}
        <div>
          <label htmlFor="pin-url" className="block text-sm text-text-muted mb-2 font-medium">
            원본 URL <span className="text-text-dim">(선택)</span>
          </label>
          <input
            id="pin-url"
            type="url"
            value={url}
            onChange={(e) => handleUrlChange(e.target.value)}
            placeholder="https://..."
            className="w-full px-4 py-2.5 bg-bg border border-border rounded-[6px] text-text-primary outline-none focus:border-accent transition-colors"
          />
          {ogLoading && (
            <div
              role="status"
              aria-live="polite"
              className="mt-2 flex items-center gap-2 text-sm text-text-muted"
            >
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
                <img
                  src={ogData.image}
                  alt={ogData.title || "미리보기"}
                  className="w-full object-cover"
                />
              </div>
            )}
            <div className="p-3">
              <div className="text-xs text-text-dim font-mono">
                {ogData.site_name || (url ? new URL(url).hostname : "")}
              </div>
              <div className="text-sm font-semibold text-text-primary">
                {ogData.title}
              </div>
            </div>
          </div>
        )}

        {/* Tag Selection */}
        <div>
          <div className="block text-sm text-text-muted mb-2 font-medium">
            태그 ({selectedTagIds.size}/{TAG_MAX_COUNT}){" "}
            <span className="text-text-dim">(선택)</span>
          </div>

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
                    aria-label={`${tag.name} 태그 제거`}
                    className="flex items-center gap-1 px-2.5 py-1 bg-accent text-white rounded-full text-xs cursor-pointer font-mono"
                  >
                    {tag.name}
                    <span aria-hidden="true" className="ml-0.5">×</span>
                  </button>
                );
              })}
            </div>
          )}

          <input
            type="text"
            value={tagSearch}
            onChange={(e) => setTagSearch(e.target.value)}
            placeholder="태그 검색..."
            aria-label="태그 검색"
            className="w-full px-4 py-2 bg-bg border border-border rounded-[6px] text-sm text-text-primary outline-none focus:border-accent transition-colors mb-3"
          />

          <div
            role="group"
            aria-label="태그 카테고리 필터"
            className="flex gap-1.5 overflow-x-auto mb-3 scrollbar-hide"
          >
            <button
              type="button"
              onClick={() => setActiveCategory("")}
              aria-pressed={activeCategory === ""}
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
                aria-pressed={activeCategory === cat}
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
                    aria-pressed={selected}
                    className={`px-2.5 py-1 rounded-full text-xs cursor-pointer transition-colors font-mono ${
                      selected
                        ? "bg-accent text-white"
                        : "bg-accent-subtle text-text-muted hover:bg-accent/20 focus-visible:bg-accent/20 disabled:opacity-50 disabled:cursor-not-allowed"
                    }`}
                  >
                    {tag.name}
                  </button>
                );
              })}
              {filteredTags.length === 0 && (
                <div
                  role="status"
                  aria-live="polite"
                  className="text-xs text-text-dim py-4 w-full text-center"
                >
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
            disabled={isDisabled}
            className="px-5 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary focus-visible:text-text-primary transition-colors cursor-pointer disabled:opacity-50"
          >
            취소
          </button>
          <button
            type="submit"
            disabled={isDisabled}
            aria-busy={isDisabled}
            className="px-6 py-2.5 bg-accent text-white rounded-full text-sm font-semibold hover:bg-accent-hover focus-visible:bg-accent-hover transition-colors disabled:opacity-50 cursor-pointer"
          >
            {submitting ? "등록 중..." : optimizing ? "처리 중..." : "등록하기"}
          </button>
        </div>
      </form>
    </>
  );
}
