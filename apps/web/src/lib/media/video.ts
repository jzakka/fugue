import { fetchFile } from "@ffmpeg/util";
import { getFFmpeg, isFFmpegSupported } from "./ffmpeg-loader";
import { SERVER_SIZE_LIMITS } from "./validation";
import type { ProgressCallback } from "./index";

const MAX_VIDEO_DURATION_SECONDS = 15;

interface VideoMeta {
  width: number;
  height: number;
  duration: number;
  isH264MP4: boolean;
}

async function getVideoMeta(file: File): Promise<VideoMeta> {
  const { width, height, duration } = await getVideoDimensions(file);

  // 1st pass: MIME + extension heuristic for H.264/MP4
  const isMP4 = file.type === "video/mp4";
  // We assume MP4 files from user devices use H.264 (most common).
  // If ffmpeg is available, we can verify later via probe.
  const isH264MP4 = isMP4;

  return { width, height, duration, isH264MP4 };
}

function getVideoDimensions(
  file: File
): Promise<{ width: number; height: number; duration: number }> {
  return new Promise((resolve, reject) => {
    const video = document.createElement("video");
    video.preload = "metadata";
    const url = URL.createObjectURL(file);
    video.onloadedmetadata = () => {
      URL.revokeObjectURL(url);
      resolve({
        width: video.videoWidth,
        height: video.videoHeight,
        duration: video.duration,
      });
    };
    video.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("비디오를 로드할 수 없습니다"));
    };
    video.src = url;
  });
}

function isWithin1080p(width: number, height: number): boolean {
  return width <= 1920 && height <= 1080;
}

export interface CompressVideoResult {
  file: File;
  originalDuration?: number;
  trimmedDuration?: number;
}

export async function compressVideo(
  file: File,
  onProgress?: ProgressCallback
): Promise<CompressVideoResult> {
  // Fallback: if ffmpeg.wasm not supported, return original
  if (!isFFmpegSupported()) {
    onProgress?.({
      stage: "브라우저가 비디오 압축을 지원하지 않아 원본 사용",
      progress: 100,
    });
    return { file };
  }

  onProgress?.({ stage: "비디오 분석 중...", progress: 0 });
  const meta = await getVideoMeta(file);

  // Use detected duration, or treat as needing trim if detection failed (NaN/Infinity)
  const duration = Number.isFinite(meta.duration) ? meta.duration : Infinity;
  const needsTrim = duration > MAX_VIDEO_DURATION_SECONDS;

  // Passthrough: H.264 MP4 + 1080p or less + 100MB or less + 15s or less
  if (
    meta.isH264MP4 &&
    isWithin1080p(meta.width, meta.height) &&
    file.size <= SERVER_SIZE_LIMITS.video &&
    !needsTrim
  ) {
    onProgress?.({ stage: "비디오가 이미 최적 조건입니다", progress: 100 });
    return { file, originalDuration: duration };
  }

  const stageLabel = needsTrim
    ? "비디오 트리밍 및 압축 중..."
    : "비디오 압축 중...";

  // Pre-trim notification: inform user before processing starts
  if (needsTrim && Number.isFinite(duration)) {
    onProgress?.({
      stage: `원본 ${Math.round(duration)}초 → ${MAX_VIDEO_DURATION_SECONDS}초로 트리밍됩니다`,
      progress: 3,
    });
  }

  onProgress?.({ stage: `${stageLabel} 준비`, progress: 5 });

  const ffmpeg = await getFFmpeg();

  // Set up progress tracking (named handler for cleanup)
  const progressHandler = ({ progress }: { progress: number }) => {
    const percent = Math.min(Math.round(progress * 85) + 10, 95);
    onProgress?.({ stage: stageLabel, progress: percent });
  };
  ffmpeg.on("progress", progressHandler);

  onProgress?.({ stage: stageLabel, progress: 10 });

  const inputExt = file.type === "video/webm" ? "webm" : "mp4";
  const inputName = `input.${inputExt}`;
  const outputName = "output.mp4";

  try {
    await ffmpeg.writeFile(inputName, await fetchFile(file));

    const args = ["-i", inputName];

    // Add trim flag if duration exceeds limit
    if (needsTrim) {
      args.push("-t", String(MAX_VIDEO_DURATION_SECONDS));
    }

    args.push(
      "-vf",
      "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2",
      "-c:v",
      "libx264",
      "-crf",
      "23",
      "-preset",
      "fast",
      "-c:a",
      "aac",
      "-b:a",
      "128k",
      "-movflags",
      "+faststart",
      outputName
    );

    await ffmpeg.exec(args);

    const data = (await ffmpeg.readFile(outputName)) as Uint8Array;
    // Copy to plain ArrayBuffer to satisfy TypeScript's Blob type constraint
    const copy = new Uint8Array(data.length);
    copy.set(data);
    const blob = new Blob([copy.buffer], { type: "video/mp4" });

    if (blob.size > SERVER_SIZE_LIMITS.video) {
      throw new Error(
        `변환된 비디오 파일이 ${Math.round(SERVER_SIZE_LIMITS.video / (1024 * 1024))}MB를 초과합니다. 더 짧은 비디오를 사용해주세요`
      );
    }

    onProgress?.({
      stage: needsTrim ? "비디오 트리밍 및 압축 완료" : "비디오 압축 완료",
      progress: 100,
    });

    const outputFileName = file.name.replace(/\.\w+$/, ".mp4");
    return {
      file: new File([blob], outputFileName, { type: "video/mp4" }),
      originalDuration: Number.isFinite(duration) ? duration : undefined,
      trimmedDuration: needsTrim ? MAX_VIDEO_DURATION_SECONDS : undefined,
    };
  } finally {
    ffmpeg.off("progress", progressHandler);
    // Cleanup temp files from WASM filesystem
    await ffmpeg.deleteFile(inputName).catch(() => {});
    await ffmpeg.deleteFile(outputName).catch(() => {});
  }
}

export { MAX_VIDEO_DURATION_SECONDS };
