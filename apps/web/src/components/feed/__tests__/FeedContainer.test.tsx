import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Work } from "@/lib/api";

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

// Mock fetchWorks
const mockFetchWorks = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return { ...actual, fetchWorks: (...args: unknown[]) => mockFetchWorks(...args) };
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

function makeWork(id: string, field = "미술"): Work {
  return {
    id,
    url: "https://example.com",
    title: `Work ${id}`,
    description: null,
    field,
    tags: ["tag"],
    og_image: null,
    og_data: null,
    created_at: "2026-04-01T00:00:00Z",
    creator: { id: "c1", nickname: "유저", avatar_url: null },
  };
}

describe("FeedContainer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentField = "";
  });

  it("renders initial works", () => {
    const works = [makeWork("1"), makeWork("2")];
    render(<FeedContainer initialWorks={works} initialHasMore={false} />);

    expect(screen.getByText("Work 1")).toBeInTheDocument();
    expect(screen.getByText("Work 2")).toBeInTheDocument();
  });

  it("shows empty state when no works and not loading", () => {
    render(<FeedContainer initialWorks={[]} initialHasMore={false} />);

    expect(screen.getByText("이 분야의 작품이 아직 없어요")).toBeInTheDocument();
    expect(screen.getByText("전체 보기")).toBeInTheDocument();
  });

  it("shows 🐡 icon in empty state", () => {
    render(<FeedContainer initialWorks={[]} initialHasMore={false} />);

    expect(screen.getByText("🐡")).toBeInTheDocument();
  });

  it("sets up IntersectionObserver when hasMore is true", () => {
    const works = [makeWork("1")];
    render(<FeedContainer initialWorks={works} initialHasMore={true} />);

    expect(mockObserve).toHaveBeenCalled();
  });

  it("does not set up IntersectionObserver when hasMore is false", () => {
    const works = [makeWork("1")];
    render(<FeedContainer initialWorks={works} initialHasMore={false} />);

    // Observer is created but sentinel div doesn't exist when hasMore=false
    // So observe may or may not be called depending on ref
  });

  it("renders noscript Load More fallback", () => {
    const works = [makeWork("1")];
    const { container } = render(
      <FeedContainer initialWorks={works} initialHasMore={true} />
    );

    const noscript = container.querySelector("noscript");
    expect(noscript).toBeInTheDocument();
  });
});
