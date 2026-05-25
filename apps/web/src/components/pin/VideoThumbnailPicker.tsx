"use client";

import { useState, useEffect, useRef } from "react";

interface VideoThumbnailPickerProps {
  file: File;
  trimStart: number;
  trimEnd: number;
  onSelect: (thumbnail: Blob) => void;
}

const THUMB_COUNT = 3;

function extractFrames(
  file: File,
  timestamps: number[]
): Promise<Blob[]> {
  return new Promise((resolve, reject) => {
    const video = document.createElement("video");
    video.preload = "auto";
    video.muted = true;
    video.playsInline = true;
    const url = URL.createObjectURL(file);
    video.src = url;

    const canvas = document.createElement("canvas");
    const ctx = canvas.getContext("2d")!;
    const blobs: Blob[] = [];
    let idx = 0;

    video.onloadeddata = () => {
      canvas.width = video.videoWidth;
      canvas.height = video.videoHeight;
      seekNext();
    };

    video.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("비디오 프레임 추출 실패"));
    };

    function seekNext() {
      if (idx >= timestamps.length) {
        URL.revokeObjectURL(url);
        resolve(blobs);
        return;
      }
      video.currentTime = timestamps[idx];
    }

    video.onseeked = () => {
      ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
      canvas.toBlob(
        (blob) => {
          if (blob) blobs.push(blob);
          idx++;
          seekNext();
        },
        "image/jpeg",
        0.85
      );
    };
  });
}

export default function VideoThumbnailPicker({
  file,
  trimStart,
  trimEnd,
  onSelect,
}: VideoThumbnailPickerProps) {
  const [thumbnails, setThumbnails] = useState<string[]>([]);
  const [thumbBlobs, setThumbBlobs] = useState<Blob[]>([]);
  const [selected, setSelected] = useState(0);
  const [loading, setLoading] = useState(true);
  const calledRef = useRef(false);

  useEffect(() => {
    if (calledRef.current) return;
    calledRef.current = true;

    const rangeStart = trimStart;
    const rangeEnd = trimEnd;
    const rangeDuration = rangeEnd - rangeStart;

    // Pick evenly spaced timestamps within the range
    const timestamps: number[] = [];
    for (let i = 0; i < THUMB_COUNT; i++) {
      const t = rangeStart + (rangeDuration / (THUMB_COUNT + 1)) * (i + 1);
      timestamps.push(t);
    }

    extractFrames(file, timestamps)
      .then((blobs) => {
        setThumbBlobs(blobs);
        const urls = blobs.map((b) => URL.createObjectURL(b));
        setThumbnails(urls);
        setLoading(false);
        if (blobs.length > 0) onSelect(blobs[0]);
      })
      .catch(() => setLoading(false));

    return () => {
      thumbnails.forEach((u) => URL.revokeObjectURL(u));
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function handleSelect(idx: number) {
    setSelected(idx);
    if (thumbBlobs[idx]) onSelect(thumbBlobs[idx]);
  }

  if (loading) {
    return (
      <div
        role="status"
        aria-live="polite"
        className="flex items-center gap-2 text-xs text-text-muted py-2"
      >
        <div className="w-4 h-4 border-2 border-accent border-t-transparent rounded-full animate-spin" />
        썸네일 추출 중...
      </div>
    );
  }

  if (thumbnails.length === 0) return null;

  return (
    <div>
      <label className="block text-sm text-text-muted mb-2 font-medium">
        썸네일 선택
      </label>
      <div className="flex gap-2">
        {thumbnails.map((url, i) => (
          <button
            key={i}
            type="button"
            onClick={() => handleSelect(i)}
            aria-pressed={selected === i}
            className={`relative flex-1 aspect-video rounded-[6px] overflow-hidden border-2 transition-colors cursor-pointer ${
              selected === i ? "border-accent" : "border-border hover:border-accent/50 focus-visible:border-accent/50"
            }`}
          >
            <img src={url} alt={`썸네일 ${i + 1}`} className="w-full h-full object-cover" />
            {selected === i && (
              <div className="absolute top-1 right-1 w-5 h-5 bg-accent rounded-full flex items-center justify-center text-white text-xs">
                ✓
              </div>
            )}
          </button>
        ))}
      </div>
    </div>
  );
}
