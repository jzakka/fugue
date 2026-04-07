const ALLOWED_MIME_TYPES = new Set([
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
  "audio/mpeg",
  "audio/wav",
  "audio/ogg",
  "audio/flac",
  "video/mp4",
  "video/webm",
]);

type MediaCategory = "image" | "audio" | "video";

// Client-side original file size limits (generous, before optimization)
const CLIENT_SIZE_LIMITS: Record<MediaCategory, number> = {
  image: 20 * 1024 * 1024, // 20MB
  audio: 100 * 1024 * 1024, // 100MB
  video: 500 * 1024 * 1024, // 500MB
};

// Server-side limits (must match storage.go maxBytes — uses > comparison)
const SERVER_SIZE_LIMITS: Record<MediaCategory, number> = {
  image: 10 * 1024 * 1024, // 10MB
  audio: 50 * 1024 * 1024, // 50MB
  video: 100 * 1024 * 1024, // 100MB
};

function getMediaCategory(mimeType: string): MediaCategory | null {
  if (mimeType.startsWith("image/")) return "image";
  if (mimeType.startsWith("audio/")) return "audio";
  if (mimeType.startsWith("video/")) return "video";
  return null;
}

export interface ValidationError {
  code: "invalid_type" | "size_exceeded" | "server_size_exceeded";
  message: string;
}

export function validateFileType(file: File): ValidationError | null {
  if (!ALLOWED_MIME_TYPES.has(file.type)) {
    return {
      code: "invalid_type",
      message: "지원하지 않는 파일 형식입니다",
    };
  }
  return null;
}

export function validateFileSize(file: File): ValidationError | null {
  const category = getMediaCategory(file.type);
  if (!category) return null;

  const limit = CLIENT_SIZE_LIMITS[category];
  if (file.size > limit) {
    const limitMB = Math.round(limit / (1024 * 1024));
    const labels: Record<MediaCategory, string> = {
      image: "이미지",
      audio: "오디오",
      video: "비디오",
    };
    return {
      code: "size_exceeded",
      message: `${labels[category]}는 ${limitMB}MB 이하만 업로드할 수 있습니다`,
    };
  }
  return null;
}

export function validateServerSizeLimit(
  file: File | Blob,
  mimeType: string
): ValidationError | null {
  const category = getMediaCategory(mimeType);
  if (!category) return null;

  const limit = SERVER_SIZE_LIMITS[category];
  // Match server comparison: size > limit (not >=)
  if (file.size > limit) {
    const limitMB = Math.round(limit / (1024 * 1024));
    const labels: Record<MediaCategory, string> = {
      image: "이미지",
      audio: "오디오",
      video: "비디오",
    };
    return {
      code: "server_size_exceeded",
      message: `최적화 후에도 ${labels[category]} 파일이 ${limitMB}MB를 초과합니다`,
    };
  }
  return null;
}

export function validateFile(file: File): ValidationError | null {
  return validateFileType(file) || validateFileSize(file);
}

export {
  ALLOWED_MIME_TYPES,
  CLIENT_SIZE_LIMITS,
  SERVER_SIZE_LIMITS,
  getMediaCategory,
  type MediaCategory,
};
