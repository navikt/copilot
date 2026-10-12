import { describe, expect, it } from "vitest";
import { formatDuration } from "./modeller-content";

describe("formatDuration", () => {
  it("shows seconds alone under a minute", () => {
    expect(formatDuration(48)).toBe("48 s");
    expect(formatDuration(59.4)).toBe("59 s");
  });
  it("shows minutes and seconds from a minute up", () => {
    expect(formatDuration(59.6)).toBe("1 min 0 s");
    expect(formatDuration(83)).toBe("1 min 23 s");
  });
});
