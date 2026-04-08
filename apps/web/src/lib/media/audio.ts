import type { ProgressCallback } from "./index";

// Audio normalization was previously done client-side with FFmpeg.wasm.
// Since FFmpeg.wasm has been removed, audio files pass through as-is.
// Server-side processing can be added later if needed.
export async function normalizeAudio(
  file: File,
  onProgress?: ProgressCallback
): Promise<File> {
  onProgress?.({ stage: "오디오 원본 준비", progress: 100 });
  return file;
}
