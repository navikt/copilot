import { describe, it, expect } from "vitest";
import { previousMonth, daysInCalendarMonth, isMonthComplete, selectCompleteMonths } from "./month-utils";

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
  it("any non-current month is complete", () => {
    expect(isMonthComplete("2026-05", 15, "2026-07")).toBe(true);
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

  it("treats all months as complete when none is current", () => {
    const result = selectCompleteMonths(data, "2026-08");
    expect(result.completeMonths).toHaveLength(3);
    expect(result.currentMonth).toBeNull();
  });
});
