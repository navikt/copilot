import { describe, it, expect } from "vitest";
import {
  previousMonth,
  daysInCalendarMonth,
  isMonthComplete,
  selectCompleteMonths,
  teamInsightMonth,
} from "./month-utils";

describe("teamInsightMonth", () => {
  it("defaults to October rather than the previous closed month", () => {
    expect(teamInsightMonth(undefined, "2026-10")).toBe("2026-10");
  });
  it("preserves an explicitly selected month", () => {
    expect(teamInsightMonth("2026-08", "2026-10")).toBe("2026-08");
  });
  it.each(["2026-13", "2026-11", "2026-04", "", "wrong"])("rejects unavailable selection %s", (month) => {
    expect(teamInsightMonth(month, "2026-10")).toBe("2026-10");
  });
});

describe("previousMonth", () => {
  it("returns correct previous month", () => {
    expect(previousMonth("2026-03")).toBe("2026-02");
    expect(previousMonth("2026-01")).toBe("2025-12");
  });

  it("handles July 31 overflow correctly", () => {
    // The bug: new Date() on July 31, setUTCMonth(-1) → "June 31" → July 1
    // Our fix anchors to day 1 first
    expect(previousMonth("2026-07")).toBe("2026-06");
  });

  it("handles March → February", () => {
    expect(previousMonth("2026-03")).toBe("2026-02");
  });
});

describe("daysInCalendarMonth", () => {
  it("returns 31 for July", () => {
    expect(daysInCalendarMonth("2026-07")).toBe(31);
  });

  it("returns 28 for February (non-leap)", () => {
    expect(daysInCalendarMonth("2026-02")).toBe(28);
  });

  it("returns 29 for February (leap year)", () => {
    expect(daysInCalendarMonth("2024-02")).toBe(29);
  });

  it("returns 30 for April", () => {
    expect(daysInCalendarMonth("2026-04")).toBe(30);
  });
});

describe("isMonthComplete", () => {
  it("a historical month with missing days is incomplete", () => {
    expect(isMonthComplete("2026-05", 15, "2026-07")).toBe(false);
  });

  it("current month with fewer days than calendar is incomplete", () => {
    expect(isMonthComplete("2026-07", 20, "2026-07")).toBe(false);
  });

  it("current month with all days is complete", () => {
    expect(isMonthComplete("2026-07", 31, "2026-07")).toBe(true);
  });

  it("February 28-day month with 28 days is complete if not current", () => {
    expect(isMonthComplete("2026-02", 28, "2026-07")).toBe(true);
  });

  it("does not accept future months or excessive day counts", () => {
    expect(isMonthComplete("2026-08", 31, "2026-07")).toBe(false);
    expect(isMonthComplete("2026-06", 31, "2026-07")).toBe(false);
  });
});

describe("selectCompleteMonths", () => {
  const data = [
    { month: "2026-05", days_in_month: 31, unique_users: 100 },
    { month: "2026-06", days_in_month: 30, unique_users: 110 },
    { month: "2026-07", days_in_month: 15, unique_users: 50 },
  ];

  it("identifies complete vs partial months", () => {
    const result = selectCompleteMonths(data, "2026-07");
    expect(result.completeMonths).toHaveLength(2);
    expect(result.currentMonth?.month).toBe("2026-07");
    expect(result.latestComplete?.month).toBe("2026-06");
    expect(result.prevComplete?.month).toBe("2026-05");
  });

  it("keeps incomplete historical months out of comparisons", () => {
    const result = selectCompleteMonths(data, "2026-08");
    expect(result.completeMonths).toHaveLength(2);
    expect(result.latestComplete?.month).toBe("2026-06");
    expect(result.currentMonth).toBeNull();
  });

  it.each([{ middle: [] }, { middle: [{ month: "2026-06", days_in_month: 29, unique_users: 110 }] }])(
    "does not compare across a missing or incomplete calendar month: $middle",
    ({ middle }) => {
      const result = selectCompleteMonths(
        [data[0], ...middle, { month: "2026-07", days_in_month: 31, unique_users: 120 }],
        "2026-08"
      );
      expect(result.latestComplete?.month).toBe("2026-07");
      expect(result.prevComplete).toBeNull();
    }
  );

  it("compares consecutive complete months across a year boundary", () => {
    const result = selectCompleteMonths(
      [
        { month: "2025-12", days_in_month: 31 },
        { month: "2026-01", days_in_month: 31 },
      ],
      "2026-02"
    );
    expect(result.prevComplete?.month).toBe("2025-12");
  });

  it("returns no comparison when no month has complete coverage", () => {
    const result = selectCompleteMonths([data[2]], "2026-08");
    expect(result.latestComplete).toBeNull();
    expect(result.prevComplete).toBeNull();
  });
});
