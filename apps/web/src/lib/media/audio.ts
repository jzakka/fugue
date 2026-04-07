import { fetchFile } from "@ffmpeg/util";
import { getFFmpeg, isFFmpegSupported } from "./ffmpeg-loader";
import { SERVER_SIZE_LIMITS } from "./validation";
import type { ProgressCallback } from "./index";

const COMPRESSED_AUDIO_TYPES = new Set(["audio/mpeg", "audio/ogg"]);
const LOSSLESS_AUDIO_TYPES = new Set(["audio/wav", "audio/flac"]);

export async function normalizeAudio(
  file: File,
  onProgress?: ProgressCallback
): Promise<File> {
  // MP3/OGG: passthrough (already compressed and web-playable)
  if (COMPRESSED_AUDIO_TYPES.has(file.type)) {
    if (file.size > SERVER_SIZE_LIMITS.audio) {
      throw new Error(
        `오디오 파일이 ${Math.round(SERVER_SIZE_LIMITS.audio / (1024 * 1024))}MB를 초과합니다`
      );
    }
    onProgress?.({ stage: "압축 오디오 — 원본 사용", progress: 100 });
    return file;
  }

  // WAV/FLAC: convert to OGG Vorbis
  if (!LOSSLESS_AUDIO_TYPES.has(file.type)) {
    onProgress?.({ stage: "오디오 원본 사용", progress: 100 });
    return file;
  }

  // Fallback: if ffmpeg.wasm not supported, return original
  if (!isFFmpegSupported()) {
    onProgress?.({
      stage: "브라우저가 오디오 변환을 지원하지 않아 원본 사용",
      progress: 100,
    });
    return file;
  }

  onProgress?.({ stage: "오디오 변환 중...", progress: 0 });

  const ffmpeg = await getFFmpeg();
  onProgress?.({ stage: "오디오 변환 중...", progress: 10 });

  const inputExt = file.type === "audio/wav" ? "wav" : "flac";
  const inputName = `input.${inputExt}`;
  const outputName = "output.ogg";

  try {
    await ffmpeg.writeFile(inputName, await fetchFile(file));
    onProgress?.({ stage: "오디오 변환 중...", progress: 30 });

    await ffmpeg.exec([
      "-i",
      inputName,
      "-c:a",
      "libvorbis",
      "-ar",
      "44100",
      "-sample_fmt",
      "s16",
      "-q:a",
      "6",
      outputName,
    ]);
    onProgress?.({ stage: "오디오 변환 중...", progress: 90 });

    const data = (await ffmpeg.readFile(outputName)) as Uint8Array;
    // Copy to plain ArrayBuffer to satisfy TypeScript's Blob type constraint
    const copy = new Uint8Array(data.length);
    copy.set(data);
    const blob = new Blob([copy.buffer], { type: "audio/ogg" });

    if (blob.size > SERVER_SIZE_LIMITS.audio) {
      throw new Error(
        `변환된 오디오 파일이 ${Math.round(SERVER_SIZE_LIMITS.audio / (1024 * 1024))}MB를 초과합니다. 더 짧은 오디오를 사용해주세요`
      );
    }

    onProgress?.({ stage: "오디오 변환 완료", progress: 100 });

    const outputFileName = file.name.replace(/\.\w+$/, ".ogg");
    return new File([blob], outputFileName, { type: "audio/ogg" });
  } finally {
    // Cleanup temp files from WASM filesystem
    await ffmpeg.deleteFile(inputName).catch(() => {});
    await ffmpeg.deleteFile(outputName).catch(() => {});
  }
}
