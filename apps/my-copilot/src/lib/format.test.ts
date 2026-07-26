import { formatNumber, formatPercentage, isoWeekLabel, formatMinutes } from "./format";

describe("formatNumber", () => {
  it("should format numbers with Norwegian locale (space as thousands separator)", () => {
    // Norwegian locale uses non-breaking space (U+00A0) as thousands separator
    expect(formatNumber(1000)).toBe("1\u00A0000");
    expect(formatNumber(10000)).toBe("10\u00A0000");
    expect(formatNumber(100000)).toBe("100\u00A0000");
    expect(formatNumber(1000000)).toBe("1\u00A0000\u00A0000");
  });

  it("should handle small numbers", () => {
    expect(formatNumber(0)).toBe("0");
    expect(formatNumber(42)).toBe("42");
    expect(formatNumber(999)).toBe("999");
  });

  it("should handle negative numbers", () => {
    // Norwegian locale uses minus sign (U+2212) and non-breaking space
    expect(formatNumber(-1000)).toBe("\u22121\u00A0000");
    expect(formatNumber(-42)).toBe("\u221242");
  });
});

describe("formatPercentage", () => {
  it("should format percentages correctly", () => {
    expect(formatPercentage(0)).toBe("0%");
    expect(formatPercentage(25)).toBe("25%");
    expect(formatPercentage(100)).toBe("100%");
  });

  it("should handle decimal percentages", () => {
    expect(formatPercentage(25.5)).toBe("25.5%");
    expect(formatPercentage(99.9)).toBe("99.9%");
  });
});

describe("formatMinutes", () => {
  it("returns an en-dash for null, zero and negative durations", () => {
    expect(formatMinutes(null)).toBe("–");
    expect(formatMinutes(0)).toBe("–");
    expect(formatMinutes(-5)).toBe("–");
  });

  it("formats sub-hour durations as minutes", () => {
    expect(formatMinutes(1)).toBe("1 min");
    expect(formatMinutes(45)).toBe("45 min");
    expect(formatMinutes(59.4)).toBe("59 min");
  });

  it("formats hour+minute durations", () => {
    expect(formatMinutes(60)).toBe("1t");
    expect(formatMinutes(192)).toBe("3t 12m");
  });

  it("rounds total minutes before splitting to avoid a 60-minute remainder", () => {
    // 119.7 rounds to 120 total minutes → "2t", never "1t 60m"
    expect(formatMinutes(119.7)).toBe("2t");
  });

  it("formats multi-day durations", () => {
    expect(formatMinutes(24 * 60)).toBe("1d");
    expect(formatMinutes(2 * 24 * 60 + 4 * 60)).toBe("2d 4t");
  });
});

describe("isoWeekLabel", () => {
  it("matches BigQuery's FORMAT_DATE('%G-W%V', day) for known dates", () => {
    // Thursday — unambiguous, always in the ISO week matching the calendar week
    expect(isoWeekLabel("2026-01-01")).toBe("2026-W01");
    expect(isoWeekLabel("2026-07-02")).toBe("2026-W27");
  });

  it("assigns year-end dates to the correct ISO week year", () => {
    // 2025-12-31 is a Wednesday in ISO week 1 of 2026
    expect(isoWeekLabel("2025-12-31")).toBe("2026-W01");
    // 2024-12-30 is a Monday in ISO week 1 of 2025
    expect(isoWeekLabel("2024-12-30")).toBe("2025-W01");
  });

  it("groups every day of the same ISO week under the same label", () => {
    const week = new Set(
      ["2026-06-29", "2026-06-30", "2026-07-01", "2026-07-02", "2026-07-03", "2026-07-04", "2026-07-05"].map(
        isoWeekLabel
      )
    );
    expect(week.size).toBe(1);
  });
});
