const PR = "https://github.com/navikt/copilot/pull/";

export interface Milestone {
  /** Merge date on main (UTC), or the repo creation date. */
  date: string;
  title: string;
  text: string;
  /** Where the date and the claim can be checked. */
  source: { label: string; url: string };
}

// Dates come from `gh pr list --json mergedAt` and `git log` on main. Add new ones at the end.
export const MILESTONES: Milestone[] = [
  {
    date: "2025-01-10",
    title: "Repoet blir opprettet",
    text: "navikt/copilot starter som et åpent repo. Alt som følger, ligger her.",
    source: { label: "commit a083419c", url: "https://github.com/navikt/copilot/commit/a083419c" },
  },
  {
    date: "2025-12-27",
    title: "MCP-registeret",
    text: "Et register over godkjente MCP-servere, med endepunkter for helse og metrikker.",
    source: { label: "#61", url: `${PR}61` },
  },
  {
    date: "2025-12-28",
    title: "MCP-onboarding",
    text: "Første versjon av mcp-onboarding. Agenter, skills og instruksjoner blir mulige å finne.",
    source: { label: "#64", url: `${PR}64` },
  },
  {
    date: "2026-03-10",
    title: "copilot-metrics",
    text: "Bruksdata fra GitHub hentes inn hver dag, med historikk bakover.",
    source: { label: "#95", url: `${PR}95` },
  },
  {
    date: "2026-03-13",
    title: "Adopsjonsskanning",
    text: "Vi skanner navikt-repoene for KI-tilpasninger og viser resultatet i et dashbord.",
    source: { label: "#110", url: `${PR}110` },
  },
  {
    date: "2026-04-09",
    title: "Sandkassen cplt får eget repo",
    text: "navikt/cplt kjører Copilot CLI i en sandkasse på maskinen til utvikleren.",
    source: { label: "navikt/cplt", url: "https://github.com/navikt/cplt" },
  },
  {
    date: "2026-04-13",
    title: "nav-pilot",
    text: "Et kommandolinjeverktøy som installerer Navs agenter og regler og holder dem oppdatert.",
    source: { label: "#149", url: `${PR}149` },
  },
  {
    date: "2026-06-15",
    title: "Telemetri i nav-pilot",
    text: "nav-pilot sender OpenTelemetry-data, så vi ser hva verktøyet faktisk gjør.",
    source: { label: "#297", url: `${PR}297` },
  },
  {
    date: "2026-07-23",
    title: "Modellvalg blir dokumentert",
    text: "docs/modellvalg.md forklarer hvilken modell som passer til hvilken oppgave.",
    source: { label: "#379", url: `${PR}379` },
  },
  {
    date: "2026-08-24",
    title: "Agentpakker",
    text: "Team kan pakke sine egne agenter og regler, med kontrakt og validering.",
    source: { label: "#436", url: `${PR}436` },
  },
  {
    date: "2026-08-24",
    title: "Golden-prompt-testene",
    text: "Faste oppgaver kjøres mot agentene, så vi merker når en endring gjør dem dårligere.",
    source: { label: "#442", url: `${PR}442` },
  },
  {
    date: "2026-09-30",
    title: "Gjentakbar modellbenchmark",
    text: "Benchmarken kan kjøres på nytt, og resultatene ligger i repoet.",
    source: { label: "#1373", url: `${PR}1373` },
  },
  {
    date: "2026-09-30",
    title: "Siden Modellvalg",
    text: "Viser hvilken modell hver agent bruker, og målingene bak valget.",
    source: { label: "#1376", url: `${PR}1376` },
  },
  {
    date: "2026-09-30",
    title: "Agentpakker holder seg oppdatert",
    text: "En arbeidsflyt foreslår ny versjon av basen. nais/pilot hadde ligget flere uker bak.",
    source: { label: "#1375", url: `${PR}1375` },
  },
  {
    date: "2026-10-02",
    title: "Realistisk kodebenchmark",
    text: "En kodeoppgave fra virkelig arbeid blir tatt vare på i repoet.",
    source: { label: "#1415", url: `${PR}1415` },
  },
  {
    date: "2026-10-07",
    title: "Støtte for OpenCode 2",
    text: "nav-pilot virker også med OpenCode 2, ikke bare med Copilot.",
    source: { label: "#1446", url: `${PR}1446` },
  },
  {
    date: "2026-10-08",
    title: "En språkmodell som dommer",
    text: "En språkmodell vurderer planleggingssvarene i benchmarken.",
    source: { label: "#1470", url: `${PR}1470` },
  },
  {
    date: "2026-10-08",
    title: "@rust bytter til Claude Haiku 5.5",
    text: "Målingene viste at en mindre modell holder mål for @rust.",
    source: { label: "#1471", url: `${PR}1471` },
  },
  {
    date: "2026-10-08",
    title: "Koordinator og arbeider",
    text: "@worker tar avgrensede oppgaver på en billigere modell, og @nav-pilot fordeler arbeidet.",
    source: { label: "#1488, #1496", url: `${PR}1496` },
  },
];
