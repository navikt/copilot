import { describe, expect, it } from "vitest";
import { buildRapport, RAPPORT_SECTIONS, type RapportSectionId } from "./rapport";
import type { AdminMonth, AdminMonthlyOverview } from "./types";

const month = (m: string, basis: AdminMonth["basis"], net: number | null, est: number | null = null): AdminMonth => ({
  month: m,
  basis,
  enterprise_net_usd: net,
  assigned_net_usd: net,
  residual_net_usd: 0,
  residual_pct: 0,
  no_team_net_usd: null,
  gross_usd: est,
  estimated_net_usd: est,
  seats: 700,
  licence_usd_estimate: 13300,
  total_usd: null,
  loaded_at: "",
});

const admin: AdminMonthlyOverview = {
  year: 2026,
  months: [month("2026-05", "premium", 1000), month("2026-09", "net", 2000.5), month("2026-10", "gross", null, 300)],
  coverage: {
    membership_from: "2026-05-06",
    last_gross_day: "2026-10-12",
    gross_stale: false,
    last_net_month: "2026-09",
    seats_snapshot: "2026-10-12",
    seats_from: "2026-10-12",
  },
};

const report = buildRapport({
  year: 2026,
  admin,
  active: [
    { month: "2026-09", active_users: 123, billed: true },
    { month: "2026-10", active_users: 3, billed: false },
  ],
  breakdown: [
    { year_month: "2026-09", model: "Claude Opus 5.5", gross_amount: 0, net_amount: 600, pct_of_monthly_net: 0 },
    { year_month: "2026-09", model: "Auto: GPT-6 Sol", gross_amount: 0, net_amount: 300, pct_of_monthly_net: 0 },
    { year_month: "2026-09", model: "Code Review Model", gross_amount: 0, net_amount: 100, pct_of_monthly_net: 0 },
    { year_month: "2025-12", model: "GPT-5", gross_amount: 0, net_amount: 999, pct_of_monthly_net: 0 },
  ],
  volume: [
    {
      month: "2026-05",
      ai_credits: 0,
      premium_requests: 400,
      cli_input_tokens: 10,
      cli_output_tokens: 2,
      app_input_tokens: 0,
      app_output_tokens: 0,
    },
    {
      month: "2026-09",
      ai_credits: 900,
      premium_requests: 0,
      cli_input_tokens: 0,
      cli_output_tokens: 0,
      app_input_tokens: 0,
      app_output_tokens: 0,
    },
  ],
  today: "2026-10-13",
});

const sec = (id: RapportSectionId) => report.sections[id];
const rows = (id: RapportSectionId, table: string) => sec(id).tables.find((t) => t.id === table)!.rows;

describe("buildRapport", () => {
  it("has a stable, versioned schema with topical sections", () => {
    expect(Object.keys(report.sections)).toEqual([...RAPPORT_SECTIONS]);
    for (const s of Object.values(report.sections)) {
      expect(Object.keys(s).sort()).toEqual(["facts", "notes", "summary", "tables", "title"]);
      expect(s.summary.length).toBeGreaterThan(0);
      for (const t of s.tables) expect(t.caption.length).toBeGreaterThan(0);
    }
    expect(report).toMatchObject({
      schema: "copilot-rapport",
      version: 1,
      title: "Copilot i Nav: bruk og kostnad 2026",
      currency: "USD",
      period: { from: "2026-01-01", to: "2026-10-12" },
    });
  });

  it("sums billed months only, like the cost page total row, and marks the open month foreløpig", () => {
    expect(sec("kostnad").facts.ytd_billed_usd).toBe(3000.5);
    expect(rows("kostnad", "fakturert").at(-1)).toEqual({ month: "2026-10", billed_usd: 300, preliminary: true });
    expect(rows("kostnad", "lisenser").length).toBe(3);
  });

  it("keeps request and token tables apart", () => {
    expect(rows("forbruk", "foresporsler").map((r) => r.month)).toEqual(["2026-05", "2026-09"]);
    expect(rows("forbruk", "tokens")).toEqual([
      { month: "2026-05", copilot_cli_in: 10, copilot_cli_out: 2, copilot_app_in: null, copilot_app_out: null },
    ]);
  });

  it("groups Auto and code review like the cost page and drops other years", () => {
    expect(rows("modeller", "familier").map((f) => [f.family, f.share_pct])).toEqual([
      ["Claude Opus", 60],
      ["Auto", 30],
      ["Kodegjennomgang", 10],
    ]);
    expect(sec("bruksomrade").facts).toEqual({ development_share_pct: 100, code_review_share_pct: 10 });
  });

  it("hides groups under five and has no budget or forecast", () => {
    expect(rows("brukere", "brukere").map((u) => u.users)).toEqual([123, null]);
    expect(sec("ikke_dekket").facts).toEqual({ budget: null, forecast: null, profession: null, department: null });
  });

  it("leaks no logins, ids, project, SQL, table names or limits", () => {
    const json = JSON.stringify(report);
    for (const bad of [
      /copilot-prod/i,
      /\bselect\b/i,
      /\bfrom `/i,
      /billing_|user_metrics|usage_metrics|copilot_seats/,
      /login|user_id|e-?mail|@/i,
      /limit|budget_usd|spend/i,
    ]) {
      expect(json).not.toMatch(bad);
    }
  });
});
