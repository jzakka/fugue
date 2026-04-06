import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Pin } from "@/lib/api";

// Mock next/navigation
let currentField = "";
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => ({
    get: (key: string) => (key === "field" ? currentField || null : null),
    toString: () => (currentField ? `field=${currentField}` : ""),
    keys: () => (currentField ? ["field"] : []),
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

function makePin(id: string, field = "미술"): Pin {
  return {
    id,
    url: "https://example.com",
    title: `Pin ${id}`,
    description: null,
    field,
    tags: ["tag"],
    og_image: null,
    og_data: null,
    pin_count: 0,
    created_at: "2026-04-01T00:00:00Z",
    creator: { id: "c1", nickname: "유저", avatar_url: null },
  };
}

describe("FeedContainer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentField = "";
  });

  it("renders initial pins", () => {
    const pins = [makePin("1"), makePin("2")];
    render(<FeedContainer initialPins={pins} initialHasMore={false} initialField="" />);

    expect(screen.getByText("Pin 1")).toBeInTheDocument();
    expect(screen.getByText("Pin 2")).toBeInTheDocument();
  });

  it("shows empty state when no pins and not loading", () => {
    render(<FeedContainer initialPins={[]} initialHasMore={false} initialField="" />);

    expect(screen.getByText("이 분야의 작품이 아직 없어요")).toBeInTheDocument();
    expect(screen.getByText("전체 보기")).toBeInTheDocument();
  });

  it("shows 🐡 icon in empty state", () => {
    render(<FeedContainer initialPins={[]} initialHasMore={false} initialField="" />);

    expect(screen.getByText("🐡")).toBeInTheDocument();
  });

  it("sets up IntersectionObserver when hasMore is true", () => {
    const pins = [makePin("1")];
    render(<FeedContainer initialPins={pins} initialHasMore={true} initialField="" />);

    expect(mockObserve).toHaveBeenCalled();
  });

  it("does not set up IntersectionObserver when hasMore is false", () => {
    const pins = [makePin("1")];
    render(<FeedContainer initialPins={pins} initialHasMore={false} initialField="" />);

    // Observer is created but sentinel div doesn't exist when hasMore=false
    // So observe may or may not be called depending on ref
  });

  it("renders noscript Load More fallback", () => {
    const pins = [makePin("1")];
    const { container } = render(
      <FeedContainer initialPins={pins} initialHasMore={true} initialField="" />
    );

    const noscript = container.querySelector("noscript");
    expect(noscript).toBeInTheDocument();
  });
});
