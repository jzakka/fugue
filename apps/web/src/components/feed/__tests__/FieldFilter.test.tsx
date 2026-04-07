import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock next/navigation
const mockPush = vi.fn();
const mockSearchParams = new URLSearchParams();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => mockSearchParams,
}));

import FieldFilter from "../FieldFilter";

describe("FieldFilter (MediaType)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    for (const key of [...mockSearchParams.keys()]) {
      mockSearchParams.delete(key);
    }
  });

  it("renders all media type filter chips", () => {
    render(<FieldFilter />);

    expect(screen.getByText("전체")).toBeInTheDocument();
    expect(screen.getByText("이미지")).toBeInTheDocument();
    expect(screen.getByText("음악")).toBeInTheDocument();
    expect(screen.getByText("영상")).toBeInTheDocument();
  });

  it("highlights 전체 chip when no media_type param", () => {
    render(<FieldFilter />);

    const allChip = screen.getByText("전체");
    expect(allChip.className).toContain("bg-text-primary");
  });

  it("updates URL when media type chip is clicked", () => {
    render(<FieldFilter />);

    screen.getByText("음악").click();
    expect(mockPush).toHaveBeenCalledWith(
      expect.stringContaining("media_type=audio"),
      { scroll: false }
    );
  });

  it("preserves tags param when changing media type", () => {
    mockSearchParams.set("tags", "cyberpunk,fantasy");
    render(<FieldFilter />);

    screen.getByText("이미지").click();
    const calledUrl = mockPush.mock.calls[0][0] as string;
    expect(calledUrl).toContain("tags=cyberpunk%2Cfantasy");
    expect(calledUrl).toContain("media_type=image");
  });

  it("removes media_type param when 전체 is clicked", () => {
    mockSearchParams.set("media_type", "audio");
    render(<FieldFilter />);

    screen.getByText("전체").click();
    const calledUrl = mockPush.mock.calls[0][0] as string;
    expect(calledUrl).not.toContain("media_type");
  });
});
