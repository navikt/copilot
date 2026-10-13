import { formatDate } from "./format";
import { rebuiltSeatMonths } from "./admin-monthly";
import { monthName as monthLabel } from "./team-year";
import { FAMILY_LABELS, modelFamily, type ModelFamily } from "./model-family";
import type { ActiveUsersMonth, AdminMonthlyOverview, BillingModelBreakdown, VolumeMonth } from "./types";

/**
 * «Copilot i Nav: bruk og kostnad»: a general data report built only from the
 * data behind /admin/kostnad, so the numbers match the page. It also answers
 * kode24's 2026 innsyn request. Only aggregates: no person, login, project ID,
 * query or table name, no budget or spending limit.
 */
export const RAPPORT_SECTIONS = [
  "kostnad",
  "forbruk",
  "brukere",
  "modeller",
  "bruksomrade",
  "metode",
  "ikke_dekket",
] as const;
export type RapportSectionId = (typeof RAPPORT_SECTIONS)[number];

export interface RapportTable {
  /** Stable key, e.g. "fakturert". */
  id: string;
  caption: string;
  rows: Record<string, unknown>[];
}

export interface RapportSection {
  title: string;
  summary: string;
  /** Scalar facts, e.g. ytd_billed_usd. */
  facts: Record<string, unknown>;
  tables: RapportTable[];
  notes: string[];
}

export interface Rapport {
  schema: "copilot-rapport";
  version: 1;
  title: string;
  year: number;
  generated: string;
  period: { from: string; to: string };
  currency: "USD";
  /** In reading order; keys never change within a version. */
  sections: Record<RapportSectionId, RapportSection>;
}

/** Groups smaller than this are not shown. */
export const MIN_GROUP = 5;

export interface RapportInput {
  year: number;
  admin: AdminMonthlyOverview;
  active: ActiveUsersMonth[];
  breakdown: BillingModelBreakdown[];
  volume: VolumeMonth[];
  today: string; // YYYY-MM-DD
}

const usd = (v: number) => Math.round(v * 100) / 100;
const pct = (part: number, whole: number) => (whole > 0 ? Math.round((part / whole) * 1000) / 10 : null);
const nb = (v: number) => v.toLocaleString("nb-NO", { maximumFractionDigits: 0 });
const nbUsd = (v: number) => `${v.toLocaleString("nb-NO", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} USD`;
const nonZero = (v: number) => (v > 0 ? v : null);

export function buildRapport({ year, admin, active, breakdown, volume, today }: RapportInput): Rapport {
  const inYear = (month: string) => month.startsWith(`${year}-`);
  const months = admin.months.filter((m) => inYear(m.month));
  const billed = months.filter((m) => m.basis === "net" || m.basis === "premium");
  const preliminary = months.filter((m) => m.basis === "gross");
  const lastDay = [admin.coverage.last_gross_day || today, `${year}-12-31`].sort()[0];
  const past = lastDay < today.slice(0, 4) + "-01-01";
  const ytd = usd(billed.reduce((a, m) => a + (m.enterprise_net_usd ?? 0), 0));
  const rebuilt = new Set(rebuiltSeatMonths(admin));
  const licenceTotal = usd(billed.reduce((a, m) => a + (m.licence_usd_estimate ?? 0), 0));
  const lastBilled = billed.at(-1)?.month;

  // Net per family over the year, the «Auto» and code review rules from the cost page.
  const familyNet = new Map<ModelFamily, number>();
  const familyModels = new Map<ModelFamily, Set<string>>();
  const billedMonths = new Set(billed.map((m) => m.month));
  for (const row of breakdown) {
    // Billed months only, so the shares rest on the billed amount in «Kostnad».
    if (!billedMonths.has(row.year_month.slice(0, 7))) continue;
    const family = modelFamily(row.model, true);
    familyNet.set(family, (familyNet.get(family) ?? 0) + row.net_amount);
    // Only models in a known family: names from billing are otherwise unvetted.
    if (row.net_amount > 0 && family !== "auto" && family !== "andre")
      familyModels.set(family, (familyModels.get(family) ?? new Set()).add(row.model.trim()));
  }
  const netTotal = [...familyNet.values()].reduce((a, b) => a + b, 0);
  const families = [...familyNet.entries()]
    .map(([family, net]) => ({
      family: FAMILY_LABELS[family],
      share_pct: pct(net, netTotal),
      models: [...(familyModels.get(family) ?? [])].sort(),
    }))
    .sort((a, b) => (b.share_pct ?? 0) - (a.share_pct ?? 0));
  const codeReview = familyNet.get("kodegjennomgang") ?? 0;

  const vol = volume.filter((v) => inYear(v.month));
  const tokenMonths = vol.filter(
    (v) => v.cli_input_tokens + v.cli_output_tokens + v.app_input_tokens + v.app_output_tokens > 0
  );
  const users = active.filter((a) => inYear(a.month));
  const seatsNow = months.at(-1)?.seats ?? null;

  const febSpike = tokenMonths.some((v) => v.month === "2026-02");

  const sections: Record<RapportSectionId, RapportSection> = {
    kostnad: {
      title: "Kostnad",
      summary:
        `Nav har fått fakturert ${nbUsd(ytd)} for bruk av GitHub Copilot` +
        (lastBilled ? ` fra januar til og med ${monthLabel(lastBilled)} ${year}.` : ` i ${year}.`) +
        " Lisenskostnaden står i en egen tabell. Den er et anslag og er ikke med i summen." +
        (preliminary.length ? " Måneder uten faktura er vist som foreløpige og er ikke med i summen." : ""),
      facts: { ytd_billed_usd: ytd, licence_ytd_usd_estimate: licenceTotal },
      tables: [
        {
          id: "fakturert",
          caption: "Fakturert per måned",
          rows: months
            .filter((m) => m.basis !== "none")
            .map((m) => ({
              month: m.month,
              billed_usd: m.enterprise_net_usd ?? m.estimated_net_usd,
              preliminary: m.basis === "gross",
            })),
        },
        {
          id: "lisenser",
          caption: "Lisenskostnad per måned (anslag, ikke med i summen)",
          rows: months
            .filter((m) => m.licence_usd_estimate != null)
            .map((m) => ({ month: m.month, usd: m.licence_usd_estimate, calculated: rebuilt.has(m.month) })),
        },
      ],
      notes: [
        "Nav blir fakturert i amerikanske dollar (USD). Beløpene er ikke regnet om til kroner.",
        "Fakturert er beløpet etter at kredittene som følger med lisensene, er trukket fra.",
        "Foreløpig betyr at fakturaen for måneden ikke er kommet. Beløpet er et anslag og kan endre seg.",
        "Lisenskostnad er et anslag: antall lisenser × listepris (19/39 USD), uten rabatt, beregnet ut fra dagens lisenser. Ikke med i det fakturerte beløpet.",
        ...(rebuilt.size && admin.coverage.seats_from
          ? [
              `Lisenstallet før ${formatDate(admin.coverage.seats_from)} er regnet ut fra dagens antall, fordi vi ikke har daglige tellinger fra før.`,
            ]
          : []),
      ],
    },
    forbruk: {
      title: "Forbruk",
      summary:
        "GitHub fakturerer ikke etter tokens. Det nærmeste målet er premiumforespørsler fra januar til mai og KI-kreditter fra juni. " +
        (tokenMonths.length
          ? "GitHub oppgir tokens bare for Copilot CLI og Copilot-appen, ikke for resten av Copilot og ikke per modellfamilie."
          : "Tokens er ikke registrert."),
      facts: { tokens_by_model_family: null },
      tables: [
        {
          id: "foresporsler",
          caption: "Premiumforespørsler og KI-kreditter per måned",
          rows: vol.map((v) => ({
            month: v.month,
            premium_requests: nonZero(v.premium_requests),
            ai_credits: nonZero(v.ai_credits),
          })),
        },
        {
          id: "tokens",
          caption: "Tokens per måned for Copilot CLI og Copilot-appen",
          rows: tokenMonths.map((v) => ({
            month: v.month,
            copilot_cli_in: nonZero(v.cli_input_tokens),
            copilot_cli_out: nonZero(v.cli_output_tokens),
            copilot_app_in: nonZero(v.app_input_tokens),
            copilot_app_out: nonZero(v.app_output_tokens),
          })),
        },
      ],
      notes: [
        "Én KI-kreditt koster 0,01 USD til listepris.",
        "Inn er tokens sendt til modellen, ut er tokens modellen skrev. Inn kan omfatte kontekst som sendes på nytt i hver forespørsel. GitHub skiller ikke ut hurtigbuffer, så inn-tallet sier lite om hvor mye ny tekst som ble sendt.",
        ...(tokenMonths.length
          ? [
              `Tokens for Copilot CLI finnes fra ${monthLabel(tokenMonths[0].month)} ${tokenMonths[0].month.slice(0, 4)}. Copilot-appen kom senere.`,
            ]
          : []),
        ...(febSpike
          ? [
              "Fra 16. til 24. februar 2026 oppga GitHub omtrent like mange tokens ut som inn for Copilot CLI. Det er trolig en feil hos GitHub, så tallet for ut i februar er for høyt. Vi viser tallene slik GitHub oppga dem.",
            ]
          : []),
      ],
    },
    brukere: {
      title: "Brukere",
      summary:
        (seatsNow != null
          ? past
            ? `Nav hadde ${nb(seatsNow)} Copilot-lisenser ved utgangen av ${year}. `
            : `Nav har i dag ${nb(seatsNow)} Copilot-lisenser. `
          : "") +
        "Tabellen viser fakturerte brukere per måned. For måneder uten faktura viser den aktive brukere så langt.",
      facts: { licences_now: seatsNow },
      tables: [
        {
          id: "brukere",
          caption: "Brukere per måned",
          rows: users.map((u) => ({
            month: u.month,
            users: u.active_users >= MIN_GROUP ? u.active_users : null,
            kind: u.billed ? "fakturert" : "aktiv",
          })),
        },
      ],
      notes: [
        "Fakturerte brukere er personene på fakturaen for måneden. Aktive brukere har brukt Copilot minst én dag.",
        `Grupper på færre enn ${MIN_GROUP} personer vises ikke.`,
      ],
    },
    modeller: {
      title: "Modeller",
      summary:
        "Nav har ingen egen liste over modeller som tilbys, så vi viser modellene som er brukt. " +
        (families.length
          ? `Mest brukt, målt i andel av fakturert beløp: ${families
              .slice(0, 3)
              .map((f) => `${f.family} (${fmtPct(f.share_pct)})`)
              .join(", ")}.`
          : ""),
      facts: {},
      tables: [{ id: "familier", caption: "Modellfamilier og andel av fakturert beløp", rows: families }],
      notes: [
        "Når Copilot velger modell selv, er bruken samlet i «Auto».",
        "Kodegjennomgang er modellen Copilot bruker til å gå gjennom endringer i koden.",
      ],
    },
    bruksomrade: {
      title: "Bruksområde",
      summary:
        "GitHub Copilot er et utviklingsverktøy, så nær 100 % av bruken gjelder utvikling. " +
        (netTotal > 0
          ? `Kodegjennomgang, der Copilot går gjennom endringer i koden, utgjør ${fmtPct(pct(codeReview, netTotal))} av det fakturerte beløpet.`
          : ""),
      facts: { development_share_pct: 100, code_review_share_pct: pct(codeReview, netTotal) },
      tables: [],
      notes: [],
    },
    metode: {
      title: "Slik har vi talt",
      summary: `Tallene gjelder GitHub Copilot i Nav fra ${formatDate(`${year}-01-01`)} til ${formatDate(lastDay)}. Andre KI-verktøy er ikke med.`,
      facts: {},
      tables: [],
      notes: [
        "Fakturert er nettobeløpet Nav betaler. Listepris er prisen før kredittene i lisensene er trukket fra. Vi bruker fakturert beløp der ikke annet er oppgitt.",
        "Januar til mai ble fakturert som premiumforespørsler, fra juni som KI-kreditter. Beløpene er lagt sammen.",
        "Lisenskostnaden og foreløpige måneder er anslag.",
      ],
    },
    ikke_dekket: {
      title: "Hva rapporten ikke dekker",
      summary: "Rapporten viser ikke budsjett, prognose, yrke eller avdeling.",
      facts: { budget: null, forecast: null, profession: null, department: null },
      tables: [],
      notes: [
        "Budsjett og prognose for neste år er ikke fastsatt og inngår ikke i rapporten.",
        "Bruksdataene inneholder ikke yrke eller avdeling.",
      ],
    },
  };

  return {
    schema: "copilot-rapport",
    version: 1,
    title: `Copilot i Nav: bruk og kostnad ${year}`,
    year,
    generated: today,
    period: { from: `${year}-01-01`, to: lastDay },
    currency: "USD",
    sections,
  };
}

const fmtPct = (v: number | null) => (v == null ? "–" : `${v.toLocaleString("nb-NO")} %`);
