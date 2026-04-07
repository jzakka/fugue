import { describe, it, expect } from "vitest";
import {
  validateFileType,
  validateFileSize,
  validateServerSizeLimit,
  validateFile,
} from "./validation";

function makeFile(name: string, type: string, size: number): File {
  const buffer = new ArrayBuffer(size);
  return new File([buffer], name, { type });
}

describe("validateFileType", () => {
  it.each([
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
  ])("allows %s", (type) => {
    const file = makeFile("test", type, 100);
    expect(validateFileType(file)).toBeNull();
  });

  it.each(["application/pdf", "text/plain", "video/avi", "audio/aac"])(
    "rejects %s",
    (type) => {
      const file = makeFile("test", type, 100);
      expect(validateFileType(file)).toEqual({
        code: "invalid_type",
        message: "지원하지 않는 파일 형식입니다",
      });
    }
  );
});

describe("validateFileSize", () => {
  it("allows image at exactly 20MB", () => {
    const file = makeFile("img.jpg", "image/jpeg", 20 * 1024 * 1024);
    expect(validateFileSize(file)).toBeNull();
  });

  it("rejects image over 20MB", () => {
    const file = makeFile("img.jpg", "image/jpeg", 20 * 1024 * 1024 + 1);
    expect(validateFileSize(file)?.code).toBe("size_exceeded");
  });

  it("allows audio at exactly 100MB", () => {
    const file = makeFile("audio.mp3", "audio/mpeg", 100 * 1024 * 1024);
    expect(validateFileSize(file)).toBeNull();
  });

  it("rejects audio over 100MB", () => {
    const file = makeFile("audio.mp3", "audio/mpeg", 100 * 1024 * 1024 + 1);
    expect(validateFileSize(file)?.code).toBe("size_exceeded");
  });

  it("allows video at exactly 500MB", () => {
    const file = makeFile("video.mp4", "video/mp4", 500 * 1024 * 1024);
    expect(validateFileSize(file)).toBeNull();
  });

  it("rejects video over 500MB", () => {
    const file = makeFile("video.mp4", "video/mp4", 500 * 1024 * 1024 + 1);
    expect(validateFileSize(file)?.code).toBe("size_exceeded");
  });
});

describe("validateServerSizeLimit", () => {
  it("allows image at exactly 10MB (server uses > not >=)", () => {
    const file = makeFile("img.jpg", "image/jpeg", 10 * 1024 * 1024);
    expect(validateServerSizeLimit(file, "image/jpeg")).toBeNull();
  });

  it("rejects image over 10MB", () => {
    const file = makeFile("img.jpg", "image/jpeg", 10 * 1024 * 1024 + 1);
    expect(validateServerSizeLimit(file, "image/jpeg")?.code).toBe(
      "server_size_exceeded"
    );
  });

  it("allows audio at exactly 50MB", () => {
    const file = makeFile("audio.ogg", "audio/ogg", 50 * 1024 * 1024);
    expect(validateServerSizeLimit(file, "audio/ogg")).toBeNull();
  });

  it("rejects audio over 50MB", () => {
    const file = makeFile("audio.ogg", "audio/ogg", 50 * 1024 * 1024 + 1);
    expect(validateServerSizeLimit(file, "audio/ogg")?.code).toBe(
      "server_size_exceeded"
    );
  });

  it("allows video at exactly 100MB", () => {
    const file = makeFile("video.mp4", "video/mp4", 100 * 1024 * 1024);
    expect(validateServerSizeLimit(file, "video/mp4")).toBeNull();
  });

  it("rejects video over 100MB", () => {
    const file = makeFile("video.mp4", "video/mp4", 100 * 1024 * 1024 + 1);
    expect(validateServerSizeLimit(file, "video/mp4")?.code).toBe(
      "server_size_exceeded"
    );
  });
});

describe("validateFile (combined)", () => {
  it("returns type error before size error", () => {
    const file = makeFile("doc.pdf", "application/pdf", 999 * 1024 * 1024);
    expect(validateFile(file)?.code).toBe("invalid_type");
  });

  it("returns null for valid file", () => {
    const file = makeFile("img.jpg", "image/jpeg", 5 * 1024 * 1024);
    expect(validateFile(file)).toBeNull();
  });
});
