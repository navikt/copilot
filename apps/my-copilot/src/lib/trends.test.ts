import { describe, expect, it } from "vitest";
import { annotationsFor, familyShares, parsePeriod, periodStart, visibleMonths } from "./trends";

describe("period", () => {
  it("defaults to 12 months and rejects unknown values", () => {
    expect(parsePeriod(undefined).value).toBe("12");
    expect(parsePeriod("24").value).toBe("12");
    expect(parsePeriod("alt").months).toBeNull();
  });

  it("counts the current month as the first", () => {
    expect(periodStart(parsePeriod("3"), "2026-10")).toBe("2026-08");
    expect(periodStart(parsePeriod("12"), "2026-10")).toBe("2025-11");
    expect(periodStart(parsePeriod("alt"), "2026-10")).toBeNull();
  });
});

describe("visibleMonths", () => {
  it("clamps to the data start and says so", () => {
    expect(visibleMonths(["2026-07", "2026-06", "2026-08"], "2025-11")).toEqual({
      months: ["2026-06", "2026-07", "2026-08"],
      clamped: true,
      first: "2026-06",
    });
    expect(visibleMonths(["2026-06", "2026-07", "2026-08"], "2026-07")).toEqual({
      months: ["2026-07", "2026-08"],
      clamped: false,
      first: "2026-06",
    });
  });
});

describe("familyShares", () => {
  const row = (year_month: string, model: string, net_amount: number) => ({
    year_month,
    model,
    net_amount,
    gross_amount: net_amount,
    pct_of_monthly_net: 0,
  });

  it("sums models into families as a share of the month's net cost", () => {
    const result = familyShares(
      [
        row("2026-08", "Claude Opus 5.5", 30),
        row("2026-08", "Claude Opus 4.6", 10),
        row("2026-08", "GPT-6 Luna", 50),
        row("2026-08", "Grok Code Fast 1", 10),
        row("2026-07", "Claude Opus 5.5", 0),
      ],
      ["2026-07", "2026-08"]
    );
    expect(result.months).toEqual(["2026-08"]);
    expect(Object.fromEntries(result.series.map((s) => [s.family, s.shares[0]]))).toEqual({
      claude_opus: 40,
      gpt_mini: 50,
      andre: 10,
    });
  });
});

describe("annotationsFor", () => {
  it("groups by month and keeps a data break", () => {
    expect(
      annotationsFor(
        [
          { date: "2026-06-01", label: "A", dataBreak: true },
          { date: "2026-10-07", label: "B" },
          { date: "2026-10-08", label: "C" },
          { date: "2025-01-01", label: "utenfor" },
        ],
        ["2026-06", "2026-10"]
      )
    ).toEqual([
      { month: "2026-06", labels: ["A"], dataBreak: true },
      { month: "2026-10", labels: ["B", "C"], dataBreak: false },
    ]);
  });
});
