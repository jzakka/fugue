import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Pin } from "@/lib/api";

// Mock next/navigation
let currentMediaType = "";
let currentTags = "";
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => ({
    get: (key: string) => {
      if (key === "media_type") return currentMediaType || null;
      if (key === "tags") return currentTags || null;
      return null;
    },
    toString: () => {
      const parts = [];
      if (currentMediaType) parts.push(`media_type=${currentMediaType}`);
      if (currentTags) parts.push(`tags=${currentTags}`);
      return parts.join("&");
    },
    keys: () => {
      const keys = [];
      if (currentMediaType) keys.push("media_type");
      if (currentTags) keys.push("tags");
      return keys;
    },
  }),
}));

// Mock fetchPins
const mockFetchPins = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return { ...actual, fetchPins: (...args: unknown[]) => mockFetchPins(...args) };
});

// Mock IntersectionObserver
const mockObserve = vi.fn();
const mockDisconnect = vi.fn();

class MockIntersectionObserver {
  observe = mockObserve;
  disconnect = mockDisconnect;
  unobserve = vi.fn();
  constructor(public callback: IntersectionObserverCallback, public options?: IntersectionObserverInit) {}
}

vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);

import FeedContainer from "../FeedContainer";

function makePin(id: string, mediaType = "image"): Pin {
  return {
    id,
    url: "https://example.com",
    title: `Pin ${id}`,
    description: null,
    media_url: `${mediaType}/test-${id}.jpg`,
    media_type: mediaType as "image" | "audio" | "video",
    tags: [{ id: "t1", name: "tag", slug: "tag", category: "스타일" }],
    og_image: null,
    og_data: null,
    created_at: "2026-04-01T00:00:00Z",
    creator: { id: "c1", nickname: "유저", avatar_url: null },
  };
}

describe("FeedContainer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentMediaType = "";
    currentTags = "";
  });

  it("renders initial pins", () => {
    const pins = [makePin("1"), makePin("2")];
    render(<FeedContainer initialPins={pins} initialHasMore={false} initialMediaType="" />);

    expect(screen.getByText("Pin 1")).toBeInTheDocument();
    expect(screen.getByText("Pin 2")).toBeInTheDocument();
  });

  it("shows empty state when no pins and not loading", () => {
    render(<FeedContainer initialPins={[]} initialHasMore={false} initialMediaType="" />);

    expect(screen.getByText("이 분야의 작품이 아직 없습니다")).toBeInTheDocument();
  });

  it("sets up IntersectionObserver when hasMore is true", () => {
    const pins = [makePin("1")];
    render(<FeedContainer initialPins={pins} initialHasMore={true} initialMediaType="" />);

    expect(mockObserve).toHaveBeenCalled();
  });

  it("renders noscript Load More fallback", () => {
    const pins = [makePin("1")];
    const { container } = render(
      <FeedContainer initialPins={pins} initialHasMore={true} initialMediaType="" />
    );

    const noscript = container.querySelector("noscript");
    expect(noscript).toBeInTheDocument();
  });
});
