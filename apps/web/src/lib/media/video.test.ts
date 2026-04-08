import { describe, it, expect } from "vitest";
import { MAX_VIDEO_DURATION_SECONDS } from "./video";

describe("video constants", () => {
  it("MAX_VIDEO_DURATION_SECONDS is 15", () => {
    expect(MAX_VIDEO_DURATION_SECONDS).toBe(15);
  });
});

describe("duration threshold logic", () => {
  it("duration <= 15 does not need trim", () => {
    expect(10 > MAX_VIDEO_DURATION_SECONDS).toBe(false);
    expect(15 > MAX_VIDEO_DURATION_SECONDS).toBe(false);
  });

  it("duration > 15 needs trim", () => {
    expect(15.001 > MAX_VIDEO_DURATION_SECONDS).toBe(true);
    expect(30 > MAX_VIDEO_DURATION_SECONDS).toBe(true);
  });

  it("NaN/Infinity treated as needing trim (safety fallback)", () => {
    const infDuration = Infinity;
    expect(infDuration > MAX_VIDEO_DURATION_SECONDS).toBe(true);

    // NaN comparison returns false, so we use the isFinite check pattern
    const nanDuration = NaN;
    const duration = Number.isFinite(nanDuration) ? nanDuration : Infinity;
    expect(duration > MAX_VIDEO_DURATION_SECONDS).toBe(true);
  });
});
