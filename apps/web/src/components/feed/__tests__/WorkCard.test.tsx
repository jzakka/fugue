import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import WorkCard from "../WorkCard";
import type { Work } from "@/lib/api";

function makeWork(overrides: Partial<Work> = {}): Work {
  return {
    id: "test-id",
    url: "https://example.com",
    title: "Test Work",
    description: "A test description",
    field: "미술",
    tags: ["tag1", "tag2"],
    og_image: "https://images.unsplash.com/photo-1.jpg",
    og_data: null,
    created_at: "2026-04-01T00:00:00Z",
    creator: {
      id: "creator-id",
      nickname: "테스트유저",
      avatar_url: null,
    },
    ...overrides,
  };
}

describe("WorkCard", () => {
  it("renders image card with og_image", () => {
    const work = makeWork({ field: "미술", og_image: "https://example.com/img.jpg" });
    render(<WorkCard work={work} />);

    const img = screen.getByAltText("Test Work");
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute("src", "https://example.com/img.jpg");
    expect(screen.getByText("Test Work")).toBeInTheDocument();
    expect(screen.getByText("테스트유저")).toBeInTheDocument();
  });

  it("renders image card placeholder when og_image is null", () => {
    const work = makeWork({ field: "미술", og_image: null });
    render(<WorkCard work={work} />);

    expect(screen.getByText("🎨")).toBeInTheDocument();
    expect(screen.getByText("Test Work")).toBeInTheDocument();
  });

  it("renders audio card for 음악 field", () => {
    const work = makeWork({ field: "음악", title: "Dreamscape" });
    render(<WorkCard work={work} />);

    expect(screen.getByText("Dreamscape")).toBeInTheDocument();
    expect(screen.getByText("▶")).toBeInTheDocument();
    expect(screen.getByText("테스트유저")).toBeInTheDocument();
  });

  it("renders text card for 시나리오 라이터 field", () => {
    const work = makeWork({
      field: "시나리오 라이터",
      title: "잊혀진 계절",
      description: "보이스드라마 시나리오 전 4화 완결",
    });
    render(<WorkCard work={work} />);

    expect(screen.getByText("잊혀진 계절")).toBeInTheDocument();
    expect(screen.getByText("Writing")).toBeInTheDocument();
    expect(screen.getByText(/보이스드라마/)).toBeInTheDocument();
    expect(screen.getByText(/min read/)).toBeInTheDocument();
  });

  it("renders video card for 영상편집 field", () => {
    const work = makeWork({
      field: "영상편집",
      og_image: "https://example.com/video-thumb.jpg",
    });
    render(<WorkCard work={work} />);

    const img = screen.getByAltText("Test Work");
    expect(img).toBeInTheDocument();
    // Video cards show play button overlay
    const playButtons = screen.getAllByText("▶");
    expect(playButtons.length).toBeGreaterThan(0);
  });

  it("falls back to image card for unknown field", () => {
    const work = makeWork({ field: "unknown_field", og_image: null });
    render(<WorkCard work={work} />);

    // Should show image placeholder
    expect(screen.getByText("🎨")).toBeInTheDocument();
  });

  it("renders tags", () => {
    const work = makeWork({ tags: ["신스팝", "몽환", "인디"] });
    render(<WorkCard work={work} />);

    expect(screen.getByText("신스팝")).toBeInTheDocument();
    expect(screen.getByText("몽환")).toBeInTheDocument();
    expect(screen.getByText("인디")).toBeInTheDocument();
  });

  it("opens external URL on click", () => {
    const openMock = vi.fn();
    vi.stubGlobal("open", openMock);

    const work = makeWork({ url: "https://soundcloud.com/test" });
    render(<WorkCard work={work} />);

    screen.getByRole("link").click();
    expect(openMock).toHaveBeenCalledWith(
      "https://soundcloud.com/test",
      "_blank",
      "noopener,noreferrer"
    );

    vi.unstubAllGlobals();
  });
});
