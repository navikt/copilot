// Types, labels and messages for /min-side/budsjett. The contract is copilot-api's
// budget raise endpoints (copilot#2704); the design is copilot-intern
// kostnadsinnsikt/prd-budsjett-og-paafyll.md. No server imports: the page is a client component.

export interface BudgetRaise {
  id: string;
  createdAt: string;
  amountUsd: number;
  category: string;
  teamId: string;
  expectedValue: string;
  freeText: string | null;
  status: string;
}

export interface Budget {
  budgetAmount: number;
  consumedAmount: number | null;
  isOverride: boolean;
  defaultBudget: number;
  /** Missing when copilot-api could not read them. */
  raises?: BudgetRaise[];
}

export interface BudgetOptions {
  roles: string[];
  categories: string[];
  expectedValues: string[];
  teams: { id: string; name: string }[];
  stepUsd: number;
  capUsd: number;
  defaultUsd: number;
  freeTextMax: number;
}

export interface BudgetProfile {
  teamId: string;
  role: string;
  exists: boolean;
  /** First visit only: the one GitHub team the user is in, if any. */
  suggestedTeamId?: string;
}

export interface RaiseResult {
  /** True when the same idempotency key was already used: only `raise` is set. */
  replayed?: boolean;
  budgetUsd: number;
  expiresAt: string;
  raisesLeft: number;
  dryRun: boolean;
}

const LABELS: Record<string, string> = {
  // Roles
  utvikler: "Utvikler",
  designer: "Designer",
  produkteier: "Produkteier",
  leder: "Leder",
  analytiker: "Analytiker",
  // Categories
  koding: "Koding",
  feilsoking: "Feilsøking",
  testing: "Testing",
  dokumentasjon: "Dokumentasjon",
  analyse_data: "Analyse og data",
  laering: "Læring",
  // Expected value
  spare_tid: "Spare tid",
  levere_raskere: "Levere raskere",
  bedre_kvalitet: "Bedre kvalitet",
  laere: "Lære",
  utforske: "Utforske",
  annet: "Annet",
};

/** Norwegian label for a code from the options list; unknown codes show as they are. */
export const label = (code: string) => LABELS[code] ?? code;

const STATUS: Record<string, string> = {
  applied: "Lagt inn i GitHub",
  pending: "Venter på GitHub",
  failed: "Feilet, prøves igjen",
  dry_run: "Testkjøring, ikke lagt inn",
  expired: "Ikke lagt inn før månedsslutt",
};
export const statusLabel = (status: string) => STATUS[status] ?? status;

const ERRORS: Record<string, string> = {
  not_open: "Påfyll er ikke åpnet ennå.",
  cap_reached: "Du har nådd taket for denne måneden. Budsjettet går tilbake til standard 1. i neste måned.",
  own_budget:
    "Du har et eget, fast budsjett. Påfyll gjelder ikke for deg. Ta kontakt i #ki-produktutvikling-diskusjon hvis du trenger mer.",
  profile_required: "Lagre team og rolle under «Profil» før du ber om påfyll.",
  forbidden: "Du har ikke en aktiv Copilot-lisens. Påfyll krever lisens.",
  no_github_account: "Vi fant ikke GitHub-brukeren din. Sjekk at Nav-kontoen er koblet til GitHub.",
  budget_writes_unavailable: "Påfyll er ikke tilgjengelig akkurat nå. Prøv igjen om litt.",
  raise_in_progress: "Et annet påfyll for deg behandles nå. Prøv igjen om noen minutter.",
  unauthorized: "Du er logget ut. Last inn siden på nytt for å logge inn.",
  rate_limited: "Du har sendt mange forespørsler på kort tid. Vent ett minutt og prøv igjen.",
  github_error: "GitHub svarte ikke som forventet. Påfyllet kan være registrert. Se tabellen over før du prøver igjen.",
  invalid_request: "Noe i skjemaet ble ikke godtatt. Sjekk feltene og prøv igjen.",
};
/** What to tell the user for an error code from /api/budget/*. */
export const errorMessage = (code: string | undefined) =>
  (code && ERRORS[code]) ?? "Noe gikk galt. Prøv igjen om litt.";

/**
 * A fixed individual budget someone else set: GitHub reports an override, but
 * none of this month's raises is ours. Unknown (null) when the raises are missing.
 */
export function hasOwnBudget(b: Budget): boolean | null {
  if (!b.isOverride) return false;
  if (!b.raises) return null;
  return !b.raises.some((r) => r.status === "applied" || r.status === "pending");
}

export const usd = (n: number) => `${Math.round(n).toLocaleString("nb-NO")} USD`;
