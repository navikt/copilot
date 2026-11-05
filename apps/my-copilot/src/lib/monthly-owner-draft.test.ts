import assert from "node:assert/strict";
import { describe, it } from "vitest";
import { buildMonthlyOwnerDraft, previousUtcMonth } from "./monthly-owner-draft";
import type { AdminMonth, AdminMonthlyOverview } from "./types";

const row = (month = "2026-10", basis: AdminMonth["basis"] = "net"): AdminMonth => ({
  month,
  basis,
  enterprise_net_usd: 100.25,
  assigned_net_usd: 80,
  residual_net_usd: 20.25,
  residual_pct: null,
  no_team_net_usd: null,
  no_usage_net_usd: null,
  gross_usd: null,
  estimated_net_usd: null,
  seats: null,
  licence_usd_estimate: null,
  total_usd: null,
  loaded_at: "2026-11-04T12:00:00Z",
});
const overview = (months: AdminMonth[]): AdminMonthlyOverview => ({
  year: 2026,
  months,
  coverage: {
    membership_from: "",
    last_gross_day: "",
    gross_stale: false,
    last_net_month: months.at(-1)?.month ?? "",
    seats_snapshot: "",
    seats_from: "",
  },
});
const input = () => ({
  now: new Date("2026-11-05T00:00:00Z"),
  admin: overview([row("2026-09"), row()]),
  cohorts: [{ cohort_month: "2026-09", cohort_size: 10, m1: 0, m3: null, m6: null }],
  prs: [{ month: "2026-10", created_by_copilot: 0, reviewed_by_copilot: 20, days: 31 }],
});

describe("monthly owner draft", () => {
  it("uses the immediately previous UTC month across year and timezone boundaries", () => {
    assert.equal(previousUtcMonth(new Date("2027-01-01T00:00:00Z")), "2026-12");
    assert.equal(previousUtcMonth(new Date("2026-11-01T00:30:00+02:00")), "2026-09");
  });
  it("waits for the target month, never an older completed month", () => {
    for (const months of [[row("2026-09")], [row("2026-09"), row("2026-10", "gross")], []]) {
      const draft = buildMonthlyOwnerDraft({ ...input(), admin: overview(months) });
      assert.equal(draft.month, "2026-10");
      assert.equal(draft.ready, false);
      assert.deepEqual(draft.findings, []);
    }
  });
  it("requires completion evidence and all cost totals, including for premium", () => {
    for (const patch of [{ loaded_at: "" }, { enterprise_net_usd: null }, { residual_net_usd: null }]) {
      assert.equal(buildMonthlyOwnerDraft({ ...input(), admin: overview([{ ...row(), ...patch }]) }).ready, false);
    }
    const data = input();
    data.admin.coverage.last_net_month = "2026-09";
    assert.equal(buildMonthlyOwnerDraft(data).ready, false);
    assert.equal(
      buildMonthlyOwnerDraft({ ...input(), now: new Date("2026-06-05Z"), admin: overview([row("2026-05", "premium")]) })
        .ready,
      true
    );
  });
  it("becomes ready after delayed publication, preserving amounts and observed zero", () => {
    const draft = buildMonthlyOwnerDraft(input());
    assert.equal(draft.ready, true);
    assert.equal(draft.loadedAt, "2026-11-04T12:00:00Z");
    assert.match(draft.findings[0], /0 %/);
    assert.match(draft.findings[1], /0 PR-er.*20.*31 dager/);
    assert.match(draft.findings[2], /100,25 USD/);
    assert.match(draft.findings[3], /80 USD.*20,25 USD/);
    assert.equal(
      buildMonthlyOwnerDraft({
        ...input(),
        admin: overview([{ ...row(), enterprise_net_usd: 0, assigned_net_usd: 0, residual_net_usd: 0 }]),
      }).ready,
      true
    );
  });
  it("does not calculate change across missing, partial or different billing bases", () => {
    for (const months of [[row()], [row("2026-09", "gross"), row()], [row("2026-09", "premium"), row()]]) {
      assert.match(
        buildMonthlyOwnerDraft({ ...input(), admin: overview(months) }).findings.at(-1)!,
        /ikke tilgjengelig/
      );
    }
    assert.match(
      buildMonthlyOwnerDraft(input()).findings.at(-1)!,
      /endret med 0 USD.*samme innsamlingsgrunnlag.*Bruker- og lisensgrunnlaget kan ha endret seg/
    );
  });
  it("keeps missing, small and partial usage distinct from zero and never claims PR changes", () => {
    const missing = buildMonthlyOwnerDraft({ ...input(), cohorts: null, prs: null });
    assert.match(missing.findings[0], /ikke tilgjengelig/);
    assert.match(missing.findings[1], /ikke tilgjengelige/);
    const partial = buildMonthlyOwnerDraft({
      ...input(),
      cohorts: [{ cohort_month: "2026-09", cohort_size: 4, m1: 50, m3: null, m6: null }],
      prs: [{ month: "2026-10", days: 30, created_by_copilot: 0, reviewed_by_copilot: 0 }],
    });
    assert.match(partial.findings[0], /ikke tilgjengelig/);
    assert.match(partial.findings[1], /ikke tilgjengelige/);
    assert.doesNotMatch(buildMonthlyOwnerDraft(input()).findings[1], /endret|produktivitet|USD/);
  });
  it("excludes July's partial PR window even if a fixture claims 31 days", () => {
    const draft = buildMonthlyOwnerDraft({
      ...input(),
      now: new Date("2026-08-05Z"),
      admin: overview([row("2026-07")]),
      prs: [{ month: "2026-07", days: 31, created_by_copilot: 3, reviewed_by_copilot: 9 }],
    });
    assert.match(draft.findings[1], /ikke tilgjengelige/);
  });
  it("distinguishes failed usage requests from successful requests without rows", () => {
    const failed = buildMonthlyOwnerDraft({ ...input(), cohorts: null, prs: null, cohortError: true, prError: true });
    assert.match(failed.findings[0], /Kunne ikke hente kohortretensjon/);
    assert.match(failed.findings[1], /Kunne ikke hente PR-tall/);
    const empty = buildMonthlyOwnerDraft({ ...input(), cohorts: [], prs: [] });
    assert.match(empty.findings[0], /ikke tilgjengelig/);
    assert.match(empty.findings[1], /ikke tilgjengelige/);
  });
  it("uses separate target-minus-1/3/6 cohorts and only their relevant mature shares", () => {
    const draft = buildMonthlyOwnerDraft({
      ...input(),
      cohorts: [
        { cohort_month: "2026-09", cohort_size: 10, m1: 0, m3: 99, m6: 99 },
        { cohort_month: "2026-07", cohort_size: 20, m1: 99, m3: 65, m6: 99 },
        { cohort_month: "2026-04", cohort_size: 30, m1: 99, m3: 99, m6: 80 },
        { cohort_month: "2026-03", cohort_size: 100, m1: 99, m3: 99, m6: 99 },
      ],
    });
    assert.match(draft.findings[0], /10 brukere.*2026-09.*0 %.*2026-10.*1 måned senere, M1/);
    assert.match(draft.findings[1], /20 brukere.*2026-07.*65 %.*2026-10.*3 måneder senere, M3/);
    assert.match(draft.findings[2], /30 brukere.*2026-04.*80 %.*2026-10.*6 måneder senere, M6/);
    assert.doesNotMatch(draft.findings.join(" "), /99 %|100 brukere/);
  });
  it("suppresses null, small, invalid and left-censored cohorts, preserving available mature observations", () => {
    for (const patch of [{ cohort_size: 4 }, { m6: null }, { m6: NaN }, { m6: 101 }, { m6: -1 }]) {
      const draft = buildMonthlyOwnerDraft({
        ...input(),
        cohorts: [{ cohort_month: "2026-04", cohort_size: 5, m1: null, m3: null, m6: 0, ...patch }],
      });
      assert.match(draft.findings[0], /ikke tilgjengelig/);
    }
    const draft = buildMonthlyOwnerDraft({
      ...input(),
      now: new Date("2026-05-05Z"),
      admin: overview([row("2026-04")]),
      cohorts: [
        { cohort_month: "2025-10", cohort_size: 100, m1: 80, m3: 80, m6: 80 },
        { cohort_month: "2026-01", cohort_size: 5, m1: 0, m3: 0, m6: null },
      ],
    });
    assert.match(draft.findings[0], /5 brukere.*2026-01.*0 %.*3 måneder senere, M3/);
    assert.doesNotMatch(draft.findings.join(" "), /100 brukere|M6/);
  });
});
