import { describe, it, expect } from "vitest";
import { MAX_VIDEO_DURATION_SECONDS } from "./video";

describe("video constants", () => {
  it("MAX_VIDEO_DURATION_SECONDS is 15", () => {
    expect(MAX_VIDEO_DURATION_SECONDS).toBe(15);
  });
});
