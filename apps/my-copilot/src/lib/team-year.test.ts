import { describe, expect, it } from "vitest";
import type { TeamYearOverview } from "./types";
import { hasNoUsageAmounts, teamYearCaveats, teamYearCsv, teamYearRows } from "./team-year";

const month = { users: 3, gross_usd: null, seat_months: 3, total_basis: "net" } as const;
const data: TeamYearOverview = {
  team_id: "1",
  team_slug: "teamkuhr",
  year: 2026,
  coverage: {
    membership_from: "2026-05-06",
    gross_from: "2026-06-15",
    last_usage_day: "2026-10-11",
    net_months: ["2026-09"],
    seats_snapshot: "2026-10-11",
    team_members_from: "2026-10-11",
  },
  months: [
    {
      ...month,
      month: "2026-09",
      basis: "net",
      net_usd: 10.5,
      no_usage_net_usd: 1,
      licence_usd_estimate: 57,
      total_usd: 68.5,
    },
    {
      ...month,
      month: "2026-10",
      basis: "gross",
      net_usd: null,
      gross_usd: 20,
      no_usage_net_usd: null,
      licence_usd_estimate: 57,
      total_usd: 77,
      total_basis: "gross",
    },
  ],
};

describe("teamYearRows", () => {
  it("are the CSV's data rows, so the report and the CSV cannot drift", () => {
    const csvRows = teamYearCsv(data)
      .trimEnd()
      .split("\n")
      .filter((l) => !l.startsWith("#"))
      .slice(1);
    expect(teamYearRows(data).map((r) => r.join(","))).toEqual(csvRows);
  });

  it("keep month order with the preliminary month before a total labelled with the billed span", () => {
    expect(teamYearRows(data).map((r) => r[0])).toEqual(["2026-09", "2026-10", "Totalt september"]);
    expect(teamYearRows(data).at(-1)).toEqual(["Totalt september", "", "", "10.50", "", "1.00", "", "57.00", "68.50"]);
    const three = { ...data, months: [{ ...data.months[0], month: "2026-06" }, ...data.months] };
    expect(teamYearRows(three).at(-1)![0]).toBe("Totalt juni–september");
  });

  it("show «netto uten bruk» only when some month has an amount", () => {
    expect(hasNoUsageAmounts(teamYearRows(data))).toBe(true);
    const zero = { ...data, months: data.months.map((m) => ({ ...m, no_usage_net_usd: m.no_usage_net_usd && 0 })) };
    expect(hasNoUsageAmounts(teamYearRows(zero))).toBe(false);
  });

  it("explain «Ingen data» only when a month has no data", () => {
    expect(teamYearCaveats(data).some((c) => c.startsWith("Ingen data"))).toBe(false);
    const none = { ...data, months: [{ ...data.months[0], basis: "none" as const }] };
    expect(teamYearCaveats(none).some((c) => c.startsWith("Ingen data"))).toBe(true);
  });
});
