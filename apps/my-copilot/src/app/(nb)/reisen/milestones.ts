const PR = "https://github.com/navikt/copilot/pull/";

export interface Milestone {
  /** First merge date on main (UTC), or the repo creation date. */
  date: string;
  /** Last merge date, for an item that spans several PRs. */
  end?: string;
  /** Major milestones get a larger marker and heading. */
  major: boolean;
  title: string;
  text: string;
  /** Where the dates and the claims can be checked. */
  sources: { label: string; url: string }[];
}

const pr = (n: number) => ({ label: `#${n}`, url: `${PR}${n}` });

// Dates come from `gh pr list --json mergedAt` and `git log` on main. Keep the list chronological by `date`.
export const MILESTONES: Milestone[] = [
  {
    date: "2025-01-10",
    major: true,
    title: "Repoet blir opprettet",
    text: "navikt/copilot starter som et åpent repo. Alt som følger, ligger her.",
    sources: [{ label: "commit a083419c", url: "https://github.com/navikt/copilot/commit/a083419c" }],
  },
  {
    date: "2025-12-27",
    end: "2025-12-28",
    major: true,
    title: "MCP: register og onboarding",
    text: "Et register over godkjente MCP-servere, og en tjeneste som gjør agenter, skills og instruksjoner lette å finne.",
    sources: [pr(61), pr(64)],
  },
  {
    date: "2026-03-10",
    end: "2026-03-13",
    major: false,
    title: "Innsikt i bruk",
    text: "Bruksdata fra GitHub hentes inn hver dag, og vi skanner navikt-repoene for KI-tilpasninger.",
    sources: [pr(95), pr(110)],
  },
  {
    date: "2026-04-09",
    major: true,
    title: "Sandkassen cplt",
    text: "navikt/cplt kjører Copilot CLI i en sandkasse på maskinen til utvikleren.",
    sources: [{ label: "navikt/cplt", url: "https://github.com/navikt/cplt" }],
  },
  {
    date: "2026-04-13",
    end: "2026-06-15",
    major: true,
    title: "nav-pilot",
    text: "Et kommandolinjeverktøy som installerer Navs agenter og regler og holder dem oppdatert. Fra juni sender det telemetri, så vi ser hva det faktisk gjør.",
    sources: [pr(149), pr(297)],
  },
  {
    date: "2026-07-23",
    end: "2026-10-08",
    major: true,
    title: "Modellvalg på målinger",
    text: "Vi dokumenterte modellvalgene og bygde faste testoppgaver for agentene. Så kom en benchmark som kan kjøres på nytt, siden /modeller, en realistisk kodeoppgave og en språkmodell som dommer. Målingene flyttet @rust til Claude Haiku 5.5.",
    sources: [pr(379), pr(442), pr(1373), pr(1376), pr(1415), pr(1470), pr(1471)],
  },
  {
    date: "2026-08-24",
    end: "2026-09-30",
    major: true,
    title: "Agentpakker",
    text: "Team kan pakke sine egne agenter og regler, med kontrakt og validering. En arbeidsflyt foreslår ny versjon av basen, etter at nais/pilot hadde ligget flere uker bak.",
    sources: [pr(436), pr(1375)],
  },
  {
    date: "2026-10-07",
    major: false,
    title: "Støtte for OpenCode 2",
    text: "nav-pilot virker også med OpenCode 2, ikke bare med Copilot.",
    sources: [pr(1446)],
  },
  {
    date: "2026-10-08",
    major: true,
    title: "Koordinator og arbeider",
    text: "@worker tar avgrensede oppgaver på GPT-6 Luna, som koster mindre. @nav-pilot fordeler arbeidet.",
    sources: [pr(1488), pr(1496)],
  },
];
