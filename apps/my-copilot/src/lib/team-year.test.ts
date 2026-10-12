import { describe, expect, it } from "vitest";
import type { TeamYearOverview } from "./types";
import { teamYearCsv, teamYearRows } from "./team-year";

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
    expect(csvRows.at(-1)).toBe("Totalt hittil i år,,,10.50,,1.00,,57.00,68.50");
  });
});
