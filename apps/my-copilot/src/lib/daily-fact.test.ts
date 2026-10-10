import { describe, expect, it } from "vitest";
import { parseDailyFact } from "./daily-fact";

describe("parseDailyFact", () => {
  it("keeps text and href only", () => {
    expect(parseDailyFact({ text: "Rundt 550 personer.", href: "/innsikt/trender", extra: 1 })).toEqual({
      text: "Rundt 550 personer.",
      href: "/innsikt/trender",
    });
  });

  it.each([null, {}, { text: "", href: "/innsikt" }, { text: "x", href: "https://evil.example" }])(
    "rejects %j",
    (body) => {
      expect(parseDailyFact(body)).toBeNull();
    }
  );
});
