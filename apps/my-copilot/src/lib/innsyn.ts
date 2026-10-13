import { formatDate } from "./format";
import { rebuiltSeatMonths } from "./admin-monthly";
import { monthName as monthLabel } from "./team-year";
import { FAMILY_LABELS, modelFamily, type ModelFamily } from "./model-family";
import type { ActiveUsersMonth, AdminMonthlyOverview, BillingModelBreakdown, VolumeMonth } from "./types";

/**
 * The innsyn (FOI) answer, keyed to kode24's 2026 request. Built only from the
 * data behind /admin/kostnad, so the numbers match the page. Only aggregates:
 * no person, login, project ID, query or table name, no budget or spending limit.
 */
export const INNSYN_QUESTIONS = ["1", "1b", "1c", "2", "3", "3a", "3b"] as const;
export type InnsynQuestionId = (typeof INNSYN_QUESTIONS)[number];

export interface InnsynQuestion {
  id: InnsynQuestionId;
  question: string;
  answer: string;
  data: Record<string, unknown> | null;
  notes: string[];
}

export interface InnsynAnswer {
  schema: "innsyn/v1";
  year: number;
  generated: string;
  period: { from: string; to: string };
  currency: "USD";
  /** In kode24's order, one per INNSYN_QUESTIONS id. An array keeps that order in JSON. */
  questions: InnsynQuestion[];
  method: string[];
}

/** Groups smaller than this are not shown. */
export const MIN_GROUP = 5;

export interface InnsynInput {
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

export function buildInnsyn({ year, admin, active, breakdown, volume, today }: InnsynInput): InnsynAnswer {
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
    // Billed months only, so the shares rest on the billed amount like question 1.
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

  const questions: Record<InnsynQuestionId, Omit<InnsynQuestion, "id">> = {
    "1": {
      question: "Hvor mye har Nav brukt på KI-verktøy hittil i år?",
      answer:
        `Nav har fått fakturert ${nbUsd(ytd)} for bruk av GitHub Copilot` +
        (lastBilled ? ` fra januar til og med ${monthLabel(lastBilled)} ${year}.` : ` i ${year}.`) +
        " Lisenskostnaden står i en egen tabell. Den er et anslag og er ikke med i summen." +
        (preliminary.length ? " Måneder uten faktura er vist som foreløpige og er ikke med i summen." : ""),
      data: {
        months: months
          .filter((m) => m.basis !== "none")
          .map((m) => ({
            month: m.month,
            billed_usd: m.enterprise_net_usd ?? m.estimated_net_usd,
            preliminary: m.basis === "gross",
          })),
        ytd_billed_usd: ytd,
        licence: {
          months: months
            .filter((m) => m.licence_usd_estimate != null)
            .map((m) => ({ month: m.month, usd: m.licence_usd_estimate, calculated: rebuilt.has(m.month) })),
          ytd_usd: licenceTotal,
          estimate: true,
          included_in_billed_total: false,
        },
      },
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
    "1b": {
      question: "Hvor mye har Nav brukt målt i tokens eller tilsvarende?",
      answer:
        "GitHub fakturerer ikke etter tokens. Det nærmeste målet er premiumforespørsler fra januar til mai og KI-kreditter fra juni. " +
        (tokenMonths.length
          ? "GitHub oppgir tokens bare for Copilot CLI og Copilot-appen, ikke for resten av Copilot og ikke per modellfamilie."
          : "Tokens er ikke registrert."),
      data: {
        months: vol.map((v) => ({
          month: v.month,
          premium_requests: nonZero(v.premium_requests),
          ai_credits: nonZero(v.ai_credits),
        })),
        tokens: tokenMonths.map((v) => ({
          month: v.month,
          copilot_cli_in: nonZero(v.cli_input_tokens),
          copilot_cli_out: nonZero(v.cli_output_tokens),
          copilot_app_in: nonZero(v.app_input_tokens),
          copilot_app_out: nonZero(v.app_output_tokens),
        })),
        tokens_by_model_family: null,
      },
      notes: [
        "Én KI-kreditt koster 0,01 USD til listepris.",
        "Inn er tokens sendt til modellen, ut er tokens modellen skrev. Inn kan omfatte kontekst som sendes på nytt i hver forespørsel. GitHub skiller ikke ut hurtigbuffer, så inn-tallet sier lite om hvor mye ny tekst som ble sendt.",
        ...(tokenMonths.length
          ? [
              `Tokens for Copilot CLI finnes fra ${monthLabel(tokenMonths[0].month)} ${tokenMonths[0].month.slice(0, 4)}. Copilot-appen kom senere.`,
            ]
          : []),
      ],
    },
    "1c": {
      question: "Hvor stor del av bruken gjelder utvikling?",
      answer:
        "GitHub Copilot er et utviklingsverktøy, så nær 100 % gjelder utvikling. " +
        (netTotal > 0
          ? `Kodegjennomgang, der Copilot går gjennom endringer i koden, utgjør ${fmtPct(pct(codeReview, netTotal))} av det fakturerte beløpet.`
          : ""),
      data: { development_share_pct: 100, code_review_share_pct: pct(codeReview, netTotal) },
      notes: [],
    },
    "2": {
      question: "Hva er budsjettet og forventet bruk neste år?",
      answer: "Budsjett og prognose for neste år er ikke fastsatt og inngår ikke i dette svaret.",
      data: null,
      notes: [],
    },
    "3": {
      question: "Hvor mange bruker KI-verktøyene?",
      answer:
        (seatsNow != null
          ? past
            ? `Nav hadde ${nb(seatsNow)} Copilot-lisenser ved utgangen av ${year}. `
            : `Nav har i dag ${nb(seatsNow)} Copilot-lisenser. `
          : "") +
        "Tabellen viser fakturerte brukere per måned. For måneder uten faktura viser den aktive brukere så langt.",
      data: {
        licences_now: seatsNow,
        months: users.map((u) => ({
          month: u.month,
          users: u.active_users >= MIN_GROUP ? u.active_users : null,
          kind: u.billed ? "fakturert" : "aktiv",
        })),
      },
      notes: [
        "Fakturerte brukere er personene på fakturaen for måneden. Aktive brukere har brukt Copilot minst én dag.",
      ],
    },
    "3a": {
      question: "Hvilke yrker og avdelinger bruker verktøyene?",
      answer: "Ikke registrert. Bruksdataene inneholder ikke yrke eller avdeling.",
      data: null,
      notes: [],
    },
    "3b": {
      question: "Hvilke modeller er tilgjengelige, og hvilke brukes mest?",
      answer:
        "Nav har ingen egen liste over modeller som tilbys, så vi viser modellene som er brukt. " +
        (families.length
          ? `Mest brukt, målt i andel av fakturert beløp: ${families
              .slice(0, 3)
              .map((f) => `${f.family} (${fmtPct(f.share_pct)})`)
              .join(", ")}.`
          : ""),
      data: { families },
      notes: [
        "Når Copilot velger modell selv, er bruken samlet i «Auto».",
        "Kodegjennomgang er modellen Copilot bruker til å gå gjennom endringer i koden.",
      ],
    },
  };

  return {
    schema: "innsyn/v1",
    year,
    generated: today,
    period: { from: `${year}-01-01`, to: lastDay },
    currency: "USD",
    questions: INNSYN_QUESTIONS.map((id) => ({ id, ...questions[id] })),
    method: [
      "Fakturert er nettobeløpet Nav betaler. Listepris er prisen før kredittene i lisensene er trukket fra. Vi bruker fakturert beløp der ikke annet er oppgitt.",
      "Januar til mai ble fakturert som premiumforespørsler, fra juni som KI-kreditter. Beløpene er lagt sammen.",
      "Lisenskostnaden og foreløpige måneder er anslag.",
      `Tallene gjelder GitHub Copilot i Nav fra ${formatDate(`${year}-01-01`)} til ${formatDate(lastDay)}. Andre KI-verktøy er ikke med.`,
    ],
  };
}

const fmtPct = (v: number | null) => (v == null ? "–" : `${v.toLocaleString("nb-NO")} %`);
