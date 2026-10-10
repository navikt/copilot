import { describe, expect, it } from "vitest";
import { parseDailyFact } from "./daily-fact";

describe("parseDailyFact", () => {
  it("keeps text and href only", () => {
    expect(parseDailyFact({ text: "Rundt 550 personer.", href: "/innsikt/trender", extra: 1 })).toEqual({
      text: "Rundt 550 personer.",
      href: "/innsikt/trender",
    });
  });

  it("keeps a weekly count rounded to 50, drops any other", () => {
    expect(parseDailyFact({ text: "x", href: "/innsikt", weekUsers: 550 })?.weekUsers).toBe(550);
    expect(parseDailyFact({ text: "x", href: "/innsikt", weekUsers: 532 })?.weekUsers).toBeUndefined();
    expect(parseDailyFact({ text: "x", href: "/innsikt", weekUsers: 0 })?.weekUsers).toBeUndefined();
  });

  it.each([null, {}, { text: "", href: "/innsikt" }, { text: "x", href: "https://evil.example" }])(
    "rejects %j",
    (body) => {
      expect(parseDailyFact(body)).toBeNull();
    }
  );
});
