import type { AdminMonth, AdminMonthlyOverview, CohortRetention, CopilotPRMonth } from "./types";

export function previousUtcMonth(now: Date): string {
  return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1)).toISOString().slice(0, 7);
}

function collected(row: AdminMonth | undefined): row is AdminMonth & {
  enterprise_net_usd: number;
  assigned_net_usd: number;
  residual_net_usd: number;
} {
  // Both bases come only from completed runs in adminMonthlyOverview, not legacy daily billing.
  return (
    !!row &&
    (row.basis === "net" || row.basis === "premium") &&
    row.enterprise_net_usd != null &&
    Number.isFinite(row.enterprise_net_usd) &&
    row.assigned_net_usd != null &&
    Number.isFinite(row.assigned_net_usd) &&
    row.residual_net_usd != null &&
    Number.isFinite(row.residual_net_usd) &&
    Number.isFinite(Date.parse(row.loaded_at))
  );
}

const usd = (value: number) => `${value.toLocaleString("nb-NO", { maximumFractionDigits: 2 })} USD`;

export function buildMonthlyOwnerDraft({
  now,
  admin,
  previous,
  previousError = false,
  cohorts,
  prs,
  cohortError = false,
  prError = false,
}: {
  now: Date;
  admin: AdminMonthlyOverview;
  previous?: AdminMonthlyOverview;
  previousError?: boolean;
  cohorts: CohortRetention[] | null;
  prs: CopilotPRMonth[] | null;
  cohortError?: boolean;
  prError?: boolean;
}) {
  const month = previousUtcMonth(now);
  const priorMonth = previousUtcMonth(new Date(`${month}-01T00:00:00Z`));
  const row = admin.months.find((m) => m.month === month);
  const ready = collected(row) && admin.coverage.last_net_month >= month;
  if (!ready) return { month, ready: false, findings: [], loadedAt: null };

  const findings: string[] = [];
  if (cohortError) {
    findings.push("Kunne ikke hente kohortretensjon. Prøv igjen før gjennomgang.");
  } else {
    for (const offset of [1, 3, 6] as const) {
      const cohortMonth = new Date(Date.UTC(Number(month.slice(0, 4)), Number(month.slice(5)) - 1 - offset, 1))
        .toISOString()
        .slice(0, 7);
      const cohort = cohorts?.find((c) => c.cohort_month === cohortMonth);
      const share = cohort?.[`m${offset}`];
      if (
        cohort &&
        cohortMonth > "2025-10" &&
        cohort.cohort_size >= 5 &&
        share != null &&
        Number.isFinite(share) &&
        share >= 0 &&
        share <= 100
      ) {
        findings.push(
          `Av ${cohort.cohort_size} brukere med første observerte aktivitet i ${cohortMonth} ble ${share} % observert aktive igjen i ${month}, ${offset} ${offset === 1 ? "måned" : "måneder"} senere, M${offset}.`
        );
      }
    }
    if (!findings.length)
      findings.push(
        "Kohortretensjon er ikke tilgjengelig for denne måneden. Manglende data eller små grupper vises ikke som null."
      );
  }

  const pr = prs?.find((p) => p.month === month);
  const days = new Date(Date.UTC(Number(month.slice(0, 4)), Number(month.slice(5)), 0)).getUTCDate();
  findings.push(
    prError
      ? "Kunne ikke hente PR-tall. Prøv igjen før gjennomgang."
      : pr && month >= "2026-08" && pr.days === days
        ? `I offentlige og interne repoer er ${pr.created_by_copilot} PR-er opprettet av Copilot og ${pr.reviewed_by_copilot} gjennomgått av Copilot observert over ${pr.days} dager.`
        : "PR-tall er ikke tilgjengelige med full månedsdekning for offentlige og interne repoer. Kilden starter 17. juli 2026; manglende dager er ikke null aktivitet."
  );
  findings.push(
    `Innsamlet nettokostnad for Copilot i Nav er ${usd(row.enterprise_net_usd)}. Lisensanslag er ikke inkludert.`,
    `${usd(row.assigned_net_usd)} er fordelt på brukere, og ${usd(row.residual_net_usd)} er restbeløp. Restbeløpet er ikke et mål på ubrukt lisens.`
  );
  const prior = (previous ?? admin).months.find((m) => m.month === priorMonth);
  findings.push(
    previousError
      ? "Kunne ikke hente forrige måned. Prøv igjen før gjennomgang."
      : collected(prior) && prior.basis === row.basis
        ? `Nettokostnaden er endret med ${usd(row.enterprise_net_usd - prior.enterprise_net_usd)} siden ${priorMonth}, med samme innsamlingsgrunnlag. Bruker- og lisensgrunnlaget kan ha endret seg; beløpet forklarer ikke årsaken.`
        : "Kostnadsendring er ikke tilgjengelig uten en ferdig innsamlet foregående måned med samme grunnlag."
  );

  return { month, ready: true, findings, loadedAt: row.loaded_at };
}
