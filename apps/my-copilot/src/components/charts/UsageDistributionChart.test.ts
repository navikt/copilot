import { describe, it, expect } from "vitest";
import { countText, populationText } from "./UsageDistributionChart";
import type { UsageDistribution } from "@/lib/types";

const distribution = (num_users: number, total_licensed_seats: number): UsageDistribution => ({
  month: "2026-09",
  num_users,
  total_licensed_seats,
  budget_credits: 3000,
  credits_deciles: [],
  interactions_deciles: [],
  acceptances_deciles: [],
  credits_histogram: [],
});

describe("populationText", () => {
  // num_users includes people whose seat was removed during the month, so it
  // can exceed today's seat count. That must never become "104 % adopsjon".
  it("shows both numbers without a ratio when users exceed seats", () => {
    const text = populationText(distribution(710, 682));
    expect(text).toBe("710 brukere hadde Copilot-aktivitet i september 2026. Nav har 682 lisenser i dag.");
    expect(text).not.toMatch(/%|adopsjon/);
  });

  it("leaves out seats when the seat count is unknown", () => {
    expect(populationText(distribution(710, 0))).toBe("710 brukere hadde Copilot-aktivitet i september 2026.");
  });
});

describe("countText", () => {
  it("hides suppressed small counts", () => {
    expect(countText({ bucket: "100%+", num_users: 0, suppressed: true })).toBe("<5");
  });

  it("shows an empty bucket as 0", () => {
    expect(countText({ bucket: "0%", num_users: 0 })).toBe("0");
  });
});
