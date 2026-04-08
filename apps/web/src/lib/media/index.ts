import {
  validateFile,
  validateServerSizeLimit,
  type ValidationError,
} from "./validation";
import { compressImage } from "./image";
import { normalizeAudio } from "./audio";
import { compressVideo } from "./video";

export interface ProgressInfo {
  stage: string;
  progress: number;
}

export type ProgressCallback = (info: ProgressInfo) => void;

export interface OptimizeResult {
  file: File;
  originalSize: number;
  optimizedSize: number;
  originalDuration?: number;
  trimmedDuration?: number;
}

export async function validateAndOptimize(
  file: File,
  onProgress?: ProgressCallback
): Promise<OptimizeResult> {
  // Step 1: Validate type and client-side size limit
  const validationError = validateFile(file);
  if (validationError) {
    throw new Error(validationError.message);
  }

  const originalSize = file.size;

  // Step 2: Optimize based on media type
  let optimized: File;
  let originalDuration: number | undefined;
  let trimmedDuration: number | undefined;

  if (file.type.startsWith("image/")) {
    optimized = await compressImage(file, onProgress);
  } else if (file.type.startsWith("audio/")) {
    optimized = await normalizeAudio(file, onProgress);
  } else if (file.type.startsWith("video/")) {
    const result = await compressVideo(file, onProgress);
    optimized = result.file;
    originalDuration = result.originalDuration;
    trimmedDuration = result.trimmedDuration;
  } else {
    optimized = file;
  }

  // Step 3: Validate server size limit after optimization
  const serverError = validateServerSizeLimit(
    optimized,
    optimized.type
  );
  if (serverError) {
    throw new Error(serverError.message);
  }

  return {
    file: optimized,
    originalSize,
    optimizedSize: optimized.size,
    originalDuration,
    trimmedDuration,
  };
}

export type { ValidationError };
