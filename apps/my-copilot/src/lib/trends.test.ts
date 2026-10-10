import { describe, expect, it } from "vitest";
import {
  annotationsFor,
  familyShares,
  formatPp,
  parsePeriod,
  periodStart,
  segmentShares,
  shareChange,
  visibleMonths,
} from "./trends";

describe("segmentShares", () => {
  const keys = [
    { key: "a", label: "A" },
    { key: "b", label: "B" },
  ];
  it("divides by the total and keeps hidden groups null", () => {
    const rows = [
      { month: "2026-07", total: 100, a: 60, b: 40 },
      { month: "2026-08", total: 100, a: 52, b: null },
    ];
    const [a, b] = segmentShares(rows, ["2026-07", "2026-08", "2026-09"], keys, "total");
    expect(a.shares).toEqual([60, 52, null]);
    expect(b.shares).toEqual([40, null, null]);
  });
  it("sums visible keys when there is no total", () => {
    expect(segmentShares([{ month: "2026-07", a: 1, b: 3 }], ["2026-07"], keys)[0].shares).toEqual([25]);
  });
  it("reports change in percentage points across the period", () => {
    expect(shareChange({ label: "A", shares: [null, 60, 52.4, null] })).toBe(-8);
    expect(shareChange({ label: "A", shares: [null, 60] })).toBeNull();
    expect(formatPp(8)).toBe("+8 pp");
    expect(formatPp(-3)).toBe("−3 pp");
  });
});

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
  it("keeps a month without data as a gap", () => {
    expect(visibleMonths(["2026-06", "2026-08"], null).months).toEqual(["2026-06", "2026-07", "2026-08"]);
  });

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
    expect(result.months).toEqual(["2026-07", "2026-08"]);
    expect(result.series.every((s) => s.shares[0] === null)).toBe(true);
    expect(Object.fromEntries(result.series.map((s) => [s.family, s.shares[1]]))).toEqual({
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
