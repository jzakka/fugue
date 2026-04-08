import {
  validateFile,
  validateServerSizeLimit,
  type ValidationError,
} from "./validation";
import { compressImage } from "./image";
import { normalizeAudio } from "./audio";

export interface ProgressInfo {
  stage: string;
  progress: number;
}

export type ProgressCallback = (info: ProgressInfo) => void;

export interface OptimizeResult {
  file: File;
  originalSize: number;
  optimizedSize: number;
}

export async function validateAndOptimize(
  file: File,
  onProgress?: ProgressCallback
): Promise<OptimizeResult> {
  const validationError = validateFile(file);
  if (validationError) {
    throw new Error(validationError.message);
  }

  const originalSize = file.size;
  let optimized: File;

  if (file.type.startsWith("image/")) {
    optimized = await compressImage(file, onProgress);
  } else if (file.type.startsWith("audio/")) {
    optimized = await normalizeAudio(file, onProgress);
  } else if (file.type.startsWith("video/")) {
    // Video processing is handled server-side (trim + encode).
    // Client just sends the original file.
    onProgress?.({ stage: "비디오 원본 준비", progress: 100 });
    optimized = file;
  } else {
    optimized = file;
  }

  // Skip server size check for video (server trims and may re-encode)
  if (!file.type.startsWith("video/")) {
    const serverError = validateServerSizeLimit(optimized, optimized.type);
    if (serverError) {
      throw new Error(serverError.message);
    }
  }

  return {
    file: optimized,
    originalSize,
    optimizedSize: optimized.size,
  };
}

export type { ValidationError };
