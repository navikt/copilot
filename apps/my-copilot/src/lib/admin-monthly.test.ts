import { describe, expect, it } from "vitest";
import type { AdminMonth, AdminMonthlyOverview } from "./types";
import { adminMonthlyCsv, adminMonthlyHeader, adminMonthlyRows, rebuiltSeatMonths } from "./admin-monthly";

const net = (
  month: string,
  basis: AdminMonth["basis"],
  enterprise: number,
  residual: number,
  noTeam: number | null
) => ({
  month,
  basis,
  enterprise_net_usd: enterprise,
  assigned_net_usd: enterprise - residual,
  residual_net_usd: residual,
  residual_pct: (residual / enterprise) * 100,
  no_team_net_usd: noTeam,
  gross_usd: null,
  estimated_net_usd: null,
  seats: 10,
  licence_usd_estimate: 190,
  total_usd: enterprise + 190,
  loaded_at: "2026-10-11",
});

const data: AdminMonthlyOverview = {
  year: 2026,
  coverage: {
    membership_from: "2026-05-06",
    last_gross_day: "2026-10-11",
    gross_stale: false,
    last_net_month: "2026-09",
    seats_snapshot: "2026-10-12",
    seats_from: "2026-10-12",
  },
  months: [
    net("2026-01", "premium", 1267.62, 152.79, null),
    net("2026-09", "net", 64558.85, 170.51, 7215.43),
    {
      ...net("2026-10", "gross", 1, 0, null),
      enterprise_net_usd: null,
      assigned_net_usd: null,
      residual_net_usd: null,
      residual_pct: null,
      gross_usd: 24517.32,
      estimated_net_usd: 9290.14,
      total_usd: 9480.14,
    },
  ],
};

describe("adminMonthlyRows", () => {
  const rows = adminMonthlyRows(data);
  const total = rows.at(-1)!;

  it("puts one row per month and the total last", () => {
    expect(rows.map((r) => r[0])).toEqual(["2026-01", "2026-09", "2026-10", "Totalt januar–september"]);
  });

  it("reconciles: assigned plus residual is enterprise net, per month and in total", () => {
    for (const row of [...rows.slice(0, 2), total]) {
      expect(Number(row[3]) + Number(row[4])).toBeCloseTo(Number(row[2]), 2);
    }
  });

  it("sums billed months only and leaves the preliminary gross out", () => {
    expect(total[2]).toBe("65826.47");
    expect(total[5]).toBe(((323.3 / 65826.47) * 100).toFixed(2));
    expect(total[6]).toBe("7215.43");
    expect(total[7]).toBe("");
    expect(total[9]).toBe("380.00");
    expect(total[10]).toBe("66206.47");
  });
});

describe("current month and licence history", () => {
  const rows = adminMonthlyRows(data);
  it("gives the current month the estimated billed amount, and the total uses it, not gross", () => {
    const oct = rows[2];
    expect(oct[adminMonthlyHeader.indexOf("estimert_fakturert_hittil_usd")]).toBe("9290.14");
    expect(oct[adminMonthlyHeader.indexOf("totalt_inkl_lisenser_usd")]).toBe("9480.14");
  });
  it("labels licences before the first daily snapshot as rebuilt", () => {
    expect(rebuiltSeatMonths(data)).toEqual(["2026-01", "2026-09", "2026-10"]);
    expect(adminMonthlyCsv(data)).toContain(
      "# Lisenser og lisenskostnad for januar–oktober er beregnet ut fra dagens lisenser, for lavt for tidligere måneder."
    );
  });
});

describe("adminMonthlyCsv", () => {
  it("has caveats as # lines before the header", () => {
    const lines = adminMonthlyCsv(data).trimEnd().split("\n");
    const header = lines.indexOf(adminMonthlyHeader.join(","));
    expect(header).toBeGreaterThan(1);
    expect(lines.slice(0, header).every((l) => l.startsWith("# "))).toBe(true);
    expect(lines.slice(header + 1)).toHaveLength(4);
    expect(lines.every((l) => !/\bAI\b/.test(l))).toBe(true);
  });
});

describe("adminMonthlyCsv column names", () => {
  it("uses the shared terms and explains the renamed columns", () => {
    const csv = adminMonthlyCsv(data);
    expect(adminMonthlyHeader).toContain("fakturert_usd");
    expect(adminMonthlyHeader).not.toContain("netto_usd");
    expect(csv).toMatch(/^# Nye kolonnenavn .*netto_usd heter fakturert_usd/m);
    expect(adminMonthlyRows(data)[1][1]).toBe("Fakturert");
  });
});
