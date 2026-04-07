import imageCompression from "browser-image-compression";
import type { ProgressCallback } from "./index";

const MAX_WIDTH_OR_HEIGHT = 2000;
const MAX_SIZE_MB = 5;
const INITIAL_QUALITY = 0.85;

export async function compressImage(
  file: File,
  onProgress?: ProgressCallback
): Promise<File> {
  // GIF: skip compression to preserve animation
  if (file.type === "image/gif") {
    onProgress?.({ stage: "GIF는 압축 없이 원본 사용", progress: 100 });
    return file;
  }

  // Check if compression is needed: small enough and dimensions unknown at this point
  // browser-image-compression handles the dimension check internally,
  // but we can skip if file is already under maxSizeMB
  if (file.size <= MAX_SIZE_MB * 1024 * 1024) {
    // Still need to check dimensions - load image to check
    const dimensions = await getImageDimensions(file);
    if (
      dimensions.width <= MAX_WIDTH_OR_HEIGHT &&
      dimensions.height <= MAX_WIDTH_OR_HEIGHT
    ) {
      onProgress?.({ stage: "이미지가 이미 최적 크기입니다", progress: 100 });
      return file;
    }
  }

  onProgress?.({ stage: "이미지 압축 중...", progress: 0 });

  const compressed = await imageCompression(file, {
    maxWidthOrHeight: MAX_WIDTH_OR_HEIGHT,
    maxSizeMB: MAX_SIZE_MB,
    initialQuality: INITIAL_QUALITY,
    useWebWorker: true,
    onProgress: (percent: number) => {
      onProgress?.({ stage: "이미지 압축 중...", progress: percent });
    },
  });

  onProgress?.({ stage: "이미지 압축 완료", progress: 100 });

  // Return as File (browser-image-compression returns Blob, wrap as File)
  return new File([compressed], file.name, { type: compressed.type });
}

function getImageDimensions(
  file: File
): Promise<{ width: number; height: number }> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    const url = URL.createObjectURL(file);
    img.onload = () => {
      URL.revokeObjectURL(url);
      resolve({ width: img.naturalWidth, height: img.naturalHeight });
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("이미지를 로드할 수 없습니다"));
    };
    img.src = url;
  });
}
