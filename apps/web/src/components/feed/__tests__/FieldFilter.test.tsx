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

describe("FieldFilter", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset search params
    for (const key of [...mockSearchParams.keys()]) {
      mockSearchParams.delete(key);
    }
  });

  it("renders all filter chips", () => {
    render(<FieldFilter />);

    expect(screen.getByText("전체")).toBeInTheDocument();
    expect(screen.getByText("일러스트")).toBeInTheDocument();
    expect(screen.getByText("음악")).toBeInTheDocument();
    expect(screen.getByText("영상")).toBeInTheDocument();
    expect(screen.getByText("코드")).toBeInTheDocument();
    expect(screen.getByText("글")).toBeInTheDocument();
  });

  it("highlights 전체 chip when no field param", () => {
    render(<FieldFilter />);

    const allChip = screen.getByText("전체");
    expect(allChip.className).toContain("bg-text-primary");
  });

  it("updates URL when field chip is clicked", () => {
    render(<FieldFilter />);

    screen.getByText("음악").click();
    // URLSearchParams URL-encodes Korean characters
    expect(mockPush).toHaveBeenCalledWith(
      expect.stringContaining("field="),
      { scroll: false }
    );
    const calledUrl = mockPush.mock.calls[0][0] as string;
    expect(decodeURIComponent(calledUrl)).toBe("?field=음악");
  });

  it("removes field param when 전체 is clicked", () => {
    mockSearchParams.set("field", "음악");
    render(<FieldFilter />);

    screen.getByText("전체").click();
    expect(mockPush).toHaveBeenCalledWith("?", { scroll: false });
  });
});
