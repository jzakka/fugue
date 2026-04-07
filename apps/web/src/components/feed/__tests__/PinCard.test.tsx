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
    media_url: "image/test.jpg",
    media_type: "image",
    tags: [
      { id: "t1", name: "tag1", slug: "tag1", category: "스타일" },
      { id: "t2", name: "tag2", slug: "tag2", category: "장르" },
    ],
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

describe("PinCard", () => {
  it("renders image card with media_url", () => {
    const pin = makePin({ media_type: "image", media_url: "image/test.jpg" });
    render(<PinCard pin={pin} />);

    const img = screen.getByAltText("Test Pin");
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute("src", "image/test.jpg");
    expect(screen.getByText("Test Pin")).toBeInTheDocument();
    expect(screen.getByText("테스트유저")).toBeInTheDocument();
  });

  it("renders audio card for audio media_type", () => {
    const pin = makePin({ media_type: "audio", title: "Dreamscape" });
    render(<PinCard pin={pin} />);

    expect(screen.getByText("Dreamscape")).toBeInTheDocument();
    expect(screen.getByText("▶")).toBeInTheDocument();
    expect(screen.getByText("테스트유저")).toBeInTheDocument();
  });

  it("renders video card for video media_type", () => {
    const pin = makePin({
      media_type: "video",
      media_url: "video/test.mp4",
    });
    render(<PinCard pin={pin} />);

    const img = screen.getByAltText("Test Pin");
    expect(img).toBeInTheDocument();
    const playButtons = screen.getAllByText("▶");
    expect(playButtons.length).toBeGreaterThan(0);
  });

  it("renders tags", () => {
    const pin = makePin({
      tags: [
        { id: "t1", name: "신스팝", slug: "synthpop", category: "장르" },
        { id: "t2", name: "몽환", slug: "dreamy", category: "스타일" },
        { id: "t3", name: "인디", slug: "indie", category: "장르" },
      ],
    });
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

  it("shows external link when url is present", () => {
    const pin = makePin({ url: "https://example.com" });
    render(<PinCard pin={pin} />);

    const extLink = screen.getByTitle("원본 보기");
    expect(extLink).toBeInTheDocument();
  });

  it("hides external link when url is null", () => {
    const pin = makePin({ url: null });
    render(<PinCard pin={pin} />);

    expect(screen.queryByTitle("원본 보기")).not.toBeInTheDocument();
  });
});
