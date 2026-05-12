import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("@/lib/api", () => ({
  fetchOgPreview: vi.fn(),
  fetchTags: vi.fn().mockResolvedValue({ tags: [] }),
  createPin: vi.fn(),
}));

vi.mock("@/lib/media", () => ({
  validateAndOptimize: vi.fn(),
}));

vi.mock("@/lib/media/video", () => ({
  getVideoDuration: vi.fn(),
  MAX_VIDEO_DURATION_SECONDS: 15,
}));

vi.mock("@/components/pin/VideoTrimModal", () => ({
  default: () => null,
}));

vi.mock("@/components/pin/VideoThumbnailPicker", () => ({
  default: () => null,
}));

import PinCreateForm from "../PinCreateForm";

describe("PinCreateForm dropzone 안내 텍스트", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("파일 미선택 상태에서 허용 미디어 타입 안내가 표시된다", () => {
    render(<PinCreateForm />);
    expect(screen.getByText("이미지 / 오디오 / 비디오")).toBeInTheDocument();
  });

  it("파일 미선택 상태에서 자동 최적화 적용 안내가 표시된다", () => {
    render(<PinCreateForm />);
    expect(
      screen.getByText("업로드 시 자동 최적화가 적용됩니다"),
    ).toBeInTheDocument();
  });
});
