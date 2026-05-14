"use client";

import { useState, useRef, useCallback, useEffect } from "react";

const MAX_CLIP = 15;
const MIN_CLIP = 1;

interface VideoTrimModalProps {
  file: File;
  videoDuration: number;
  onConfirm: (trimStart: number, trimEnd: number) => void;
  onCancel: () => void;
}

function fmt(s: number): string {
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m}:${sec.toFixed(1).padStart(4, "0")}`;
}

type DragTarget = "start" | "end" | "window" | null;

export default function VideoTrimModal({
  file,
  videoDuration,
  onConfirm,
  onCancel,
}: VideoTrimModalProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const [videoUrl, setVideoUrl] = useState<string | null>(null);

  const initEnd = Math.min(videoDuration, MAX_CLIP);
  const [start, setStart] = useState(0);
  const [end, setEnd] = useState(initEnd);
  const [drag, setDrag] = useState<DragTarget>(null);
  const dragOffsetRef = useRef(0);

  const clip = end - start;

  useEffect(() => {
    const url = URL.createObjectURL(file);
    // URL.createObjectURL must be paired with revokeObjectURL on cleanup,
    // which only useEffect can express. The setState here is required to
    // surface the URL to render.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setVideoUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  useEffect(() => {
    if (videoRef.current && !drag) {
      videoRef.current.currentTime = start;
    }
  }, [start, drag]);

  const pxToTime = useCallback(
    (clientX: number) => {
      if (!trackRef.current) return 0;
      const r = trackRef.current.getBoundingClientRect();
      return Math.max(0, Math.min(videoDuration, ((clientX - r.left) / r.width) * videoDuration));
    },
    [videoDuration]
  );

  const onHandleDown = useCallback(
    (target: "start" | "end") => (e: React.PointerEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setDrag(target);
      (e.target as HTMLElement).setPointerCapture(e.pointerId);
    },
    []
  );

  const onWindowDown = useCallback(
    (e: React.PointerEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setDrag("window");
      (e.target as HTMLElement).setPointerCapture(e.pointerId);
      dragOffsetRef.current = pxToTime(e.clientX) - start;
    },
    [pxToTime, start]
  );

  const onMove = useCallback(
    (e: React.PointerEvent) => {
      if (!drag) return;
      const t = pxToTime(e.clientX);

      if (drag === "start") {
        let s = Math.max(0, Math.min(t, end - MIN_CLIP));
        if (end - s > MAX_CLIP) s = end - MAX_CLIP;
        setStart(s);
        if (videoRef.current) videoRef.current.currentTime = s;
      } else if (drag === "end") {
        let en = Math.min(videoDuration, Math.max(t, start + MIN_CLIP));
        if (en - start > MAX_CLIP) en = start + MAX_CLIP;
        setEnd(en);
      } else if (drag === "window") {
        const gap = end - start;
        let s = t - dragOffsetRef.current;
        s = Math.max(0, Math.min(s, videoDuration - gap));
        setStart(s);
        setEnd(s + gap);
        if (videoRef.current) videoRef.current.currentTime = s;
      }
    },
    [drag, start, end, videoDuration, pxToTime]
  );

  const onUp = useCallback(() => setDrag(null), []);

  const startPct = (start / videoDuration) * 100;
  const endPct = (end / videoDuration) * 100;
  const widthPct = endPct - startPct;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="bg-surface-elevated border border-border rounded-[12px] w-full max-w-xl mx-4 overflow-hidden">
        <div className="px-5 py-4 border-b border-border">
          <h2 className="text-lg font-bold text-text-primary">
            비디오 구간 선택
          </h2>
          <p className="text-xs text-text-muted mt-1">
            양쪽 주황 핸들로 시작/종료 조정 · 가운데 드래그로 전체 이동 (최대 {MAX_CLIP}초)
          </p>
        </div>

        <div className="px-5 py-4">
          {videoUrl && (
            <video
              ref={videoRef}
              src={videoUrl}
              className="w-full rounded-[8px] bg-black max-h-[280px] object-contain"
              preload="auto"
              muted
              playsInline
            />
          )}

          <div className="mt-4 px-1">
            <div
              className="flex justify-between items-center text-2xs text-text-dim mb-2"
              style={{ fontFamily: "'Geist Mono', monospace" }}
            >
              <span>{fmt(start)}</span>
              <span className="text-accent font-semibold">{clip.toFixed(1)}초 / {MAX_CLIP}초</span>
              <span>{fmt(end)}</span>
            </div>

            {/* Track */}
            <div
              ref={trackRef}
              className="relative h-12 bg-border/30 rounded-lg select-none touch-none"
              onPointerMove={onMove}
              onPointerUp={onUp}
              onPointerLeave={onUp}
            >
              {/* Dimmed outside */}
              <div
                className="absolute top-0 left-0 h-full bg-black/40 rounded-l-lg pointer-events-none"
                style={{ width: `${startPct}%` }}
              />
              <div
                className="absolute top-0 right-0 h-full bg-black/40 rounded-r-lg pointer-events-none"
                style={{ width: `${100 - endPct}%` }}
              />

              {/* Selected window — drag middle to move entire range */}
              <div
                className="absolute top-0 h-full bg-accent/15 cursor-grab active:cursor-grabbing"
                style={{ left: `calc(${startPct}% + 14px)`, width: `calc(${widthPct}% - 28px)` }}
                onPointerDown={onWindowDown}
              >
                <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                  <div className="flex gap-1 opacity-50">
                    <div className="w-0.5 h-5 bg-accent rounded-full" />
                    <div className="w-0.5 h-5 bg-accent rounded-full" />
                    <div className="w-0.5 h-5 bg-accent rounded-full" />
                  </div>
                </div>
              </div>

              {/* Start handle — tall, visible, easy to grab */}
              <div
                className="absolute top-0 h-full w-[14px] cursor-ew-resize z-10 flex items-center justify-center bg-accent rounded-l-md"
                style={{ left: `${startPct}%` }}
                onPointerDown={onHandleDown("start")}
              >
                <div className="w-[3px] h-5 bg-white/70 rounded-full" />
              </div>

              {/* End handle — tall, visible, easy to grab */}
              <div
                className="absolute top-0 h-full w-[14px] cursor-ew-resize z-10 flex items-center justify-center bg-accent rounded-r-md"
                style={{ left: `calc(${endPct}% - 14px)` }}
                onPointerDown={onHandleDown("end")}
              >
                <div className="w-[3px] h-5 bg-white/70 rounded-full" />
              </div>
            </div>

            <div
              className="text-right mt-1 text-2xs text-text-dim"
              style={{ fontFamily: "'Geist Mono', monospace" }}
            >
              전체 {fmt(videoDuration)}
            </div>
          </div>
        </div>

        <div className="px-5 py-4 border-t border-border flex justify-end gap-3">
          <button
            type="button"
            onClick={onCancel}
            className="px-5 py-2.5 border border-border rounded-full text-sm text-text-muted hover:text-text-primary transition-colors cursor-pointer"
          >
            취소
          </button>
          <button
            type="button"
            onClick={() => onConfirm(start, end)}
            disabled={clip < MIN_CLIP || clip > MAX_CLIP + 0.5}
            className="px-6 py-2.5 bg-accent text-white rounded-full text-sm font-semibold hover:bg-accent-hover transition-colors disabled:opacity-50 cursor-pointer"
          >
            선택 완료
          </button>
        </div>
      </div>
    </div>
  );
}
