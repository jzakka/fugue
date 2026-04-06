import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import PinCard from "../PinCard";
import type { Pin } from "@/lib/api";

function makePin(overrides: Partial<Pin> = {}): Pin {
  return {
    id: "test-id",
    url: "https://example.com",
    title: "Test Pin",
    description: "A test description",
    field: "미술",
    tags: ["tag1", "tag2"],
    og_image: "https://images.unsplash.com/photo-1.jpg",
    og_data: null,
    pin_count: 0,
    created_at: "2026-04-01T00:00:00Z",
    creator: {
      id: "creator-id",
      nickname: "테스트유저",
      avatar_url: null,
    },
    ...overrides,
  };
}

describe("PinCard", () => {
  it("renders image card with og_image", () => {
    const pin = makePin({ field: "미술", og_image: "https://example.com/img.jpg" });
    render(<PinCard pin={pin} />);

    const img = screen.getByAltText("Test Pin");
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute("src", "https://example.com/img.jpg");
    expect(screen.getByText("Test Pin")).toBeInTheDocument();
    expect(screen.getByText("테스트유저")).toBeInTheDocument();
  });

  it("renders image card placeholder when og_image is null", () => {
    const pin = makePin({ field: "미술", og_image: null });
    render(<PinCard pin={pin} />);

    expect(screen.getByText("🎨")).toBeInTheDocument();
    expect(screen.getByText("Test Pin")).toBeInTheDocument();
  });

  it("renders audio card for 음악 field", () => {
    const pin = makePin({ field: "음악", title: "Dreamscape" });
    render(<PinCard pin={pin} />);

    expect(screen.getByText("Dreamscape")).toBeInTheDocument();
    expect(screen.getByText("▶")).toBeInTheDocument();
    expect(screen.getByText("테스트유저")).toBeInTheDocument();
  });

  it("renders text card for 시나리오 라이터 field", () => {
    const pin = makePin({
      field: "시나리오 라이터",
      title: "잊혀진 계절",
      description: "보이스드라마 시나리오 전 4화 완결",
    });
    render(<PinCard pin={pin} />);

    expect(screen.getByText("잊혀진 계절")).toBeInTheDocument();
    expect(screen.getByText("Writing")).toBeInTheDocument();
    expect(screen.getByText(/보이스드라마/)).toBeInTheDocument();
    expect(screen.getByText(/min read/)).toBeInTheDocument();
  });

  it("renders video card for 영상편집 field", () => {
    const pin = makePin({
      field: "영상편집",
      og_image: "https://example.com/video-thumb.jpg",
    });
    render(<PinCard pin={pin} />);

    const img = screen.getByAltText("Test Pin");
    expect(img).toBeInTheDocument();
    const playButtons = screen.getAllByText("▶");
    expect(playButtons.length).toBeGreaterThan(0);
  });

  it("renders tags", () => {
    const pin = makePin({ tags: ["신스팝", "몽환", "인디"] });
    render(<PinCard pin={pin} />);

    expect(screen.getByText("신스팝")).toBeInTheDocument();
    expect(screen.getByText("몽환")).toBeInTheDocument();
    expect(screen.getByText("인디")).toBeInTheDocument();
  });

  it("links to pin detail page", () => {
    const pin = makePin({ id: "abc-123" });
    render(<PinCard pin={pin} />);

    const links = screen.getAllByRole("link");
    const cardLink = links.find((l) => l.getAttribute("href") === "/pins/abc-123");
    expect(cardLink).toBeTruthy();
  });

  it("shows pin count when > 0", () => {
    const pin = makePin({ pin_count: 5 });
    render(<PinCard pin={pin} />);

    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("hides pin count when 0", () => {
    const pin = makePin({ pin_count: 0 });
    render(<PinCard pin={pin} />);

    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });
});
