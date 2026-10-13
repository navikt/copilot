import { describe, expect, it } from "vitest";
import { budgetComparison } from "./budget-days";
import type { BudgetDay } from "./types";

const day = (d: string, low: number, rest: number, label = "0–10 %"): BudgetDay => ({
  day: d,
  users: low + rest,
  snapshots: 0,
  bands: [
    { first: 0, last: label.startsWith("0–10") ? 0 : 1, label, users: low },
    { first: 6, last: 6, label: "Over 100 %", users: rest },
  ],
});

describe("budgetComparison", () => {
  it("compares the last day with the same day the month before", () => {
    const s = budgetComparison(
      [day("2026-09-10", 58, 42), day("2026-10-09", 50, 50), day("2026-10-10", 63, 37)],
      "2026-10"
    );
    expect(s).toMatch(/^Per 10\. okt\.?: 63\s%\sunder 10 % \(sep\.?: 58\s%\)$/);
  });

  it("leaves out the comparison without a matching day or band", () => {
    expect(budgetComparison([day("2026-10-10", 63, 37)], "2026-10")).not.toContain("(");
    const s = budgetComparison([day("2026-09-10", 58, 42), day("2026-10-10", 63, 37, "0–25 %")], "2026-10");
    expect(s).not.toContain("(");
    expect(s).toContain("på 0–25");
  });

  it("is null for a month without bands", () => {
    expect(budgetComparison([{ day: "2026-10-01", users: 0, snapshots: 0, bands: null }], "2026-10")).toBeNull();
  });
});
