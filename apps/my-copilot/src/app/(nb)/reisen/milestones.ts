const PR = "https://github.com/navikt/copilot/pull/";
const DOCS = "https://github.com/navikt/copilot/blob/main/docs/";

export interface Phase {
  /** Start of the phase: a year («2023»), a month («2026-10») or a day. */
  date: string;
  /** End of the phase, when it spans more than one month. */
  end?: string;
  title: string;
  /** Two to four sentences on what changed and why it mattered. */
  text: string;
  /** Where the claims can be checked. */
  sources: Source[];
}

/** A link, or the team's own account where no public record exists. */
type Source = { label: string; url: string } | { team: true };

const TEAM: Source = { team: true };

const pr = (n: number) => ({ label: `#${n}`, url: `${PR}${n}` });

// Dates come from `gh pr list --json mergedAt`, `git log` on main and GitHub's own announcements.
// Keep the list chronological by `date`.
export const PHASES: Phase[] = [
  {
    date: "2023",
    title: "Grasrot og pilot",
    text: "Noen utviklere i Nav fikk tilgang til den lukkede betaen av GitHub Copilot, den gang mest en smart autofullføring. I stedet for å stoppe det kjørte Nav en pilot med Copilot Business-lisenser. Teamet skrev retningslinjer og gikk gjennom vilkårene med Navs jurister. Koden vår var stort sett åpen, så opphavsrett bekymret oss mer enn lekkasje. Samme høst startet SINTEF en studie av Copilot i Nav. Det var et fellesskap fra dag én.",
    sources: [
      TEAM,
      {
        label: "Copilot Business",
        url: "https://github.blog/news-insights/product-news/github-copilot-for-business-is-now-available/",
      },
      { label: "SINTEF-studien på arXiv", url: "https://arxiv.org/abs/2509.20353" },
      { label: "HICSS 2026", url: "https://doi.org/10.24251/HICSS.2026.880" },
    ],
  },
  {
    date: "2024-03",
    end: "2024-05",
    title: "Fra pilot til hverdag",
    text: "Copilot Chat kom til IntelliJ, der mange av Navs Java- og Kotlin-utviklere jobber. Etterspørselen vokste forbi lisensene: i mai var de brukt opp, og vi fikk venteliste og et budsjettforslag om tilgang for alle som trenger det.",
    sources: [
      TEAM,
      {
        label: "Copilot Chat i JetBrains",
        url: "https://github.blog/changelog/2024-03-07-github-copilot-chat-general-availability-in-jetbrains-ide/",
      },
    ],
  },
  {
    date: "2025-01",
    end: "2025-11",
    title: "Agentene kommer",
    text: "I januar åpnet vi repoet navikt/copilot og Min Copilot med selvbetjente lisenser, for rundt 111 brukere. Så kom agentmodus, kodegjennomgang og egne instruksjoner per repo. Vi løftet blokkeringen av treff mot offentlig kode og risikovurderte MCP. Til høsten hadde vi over 300 aktive brukere og Copilot CLI, og i november åpnet vi for flere roller i IT. Derfra rullet det.",
    sources: [
      TEAM,
      { label: "commit a083419c", url: "https://github.com/navikt/copilot/commit/a083419c" },
      {
        label: "The agent awakens",
        url: "https://github.blog/news-insights/product-news/github-copilot-the-agent-awakens/",
      },
      { label: "Agentmodus i VS Code", url: "https://code.visualstudio.com/blogs/2025/04/07/agentMode" },
    ],
  },
  {
    date: "2025-12",
    end: "2026-10",
    title: "Fra verktøy til plattform",
    text: "Vi samlet agenter, skills og instruksjoner i en felles katalog med MCP-register, og fellesskapet deler det som virker. Sandkassen cplt og kommandolinjeverktøyet nav-pilot gjorde katalogen til noe hvert team installerer, bygget på en utviklerundersøkelse og skrevne designvalg. Med agentpakker kan team lage sine egne, og en arbeidsflyt holder basen oppdatert. Støtten for OpenCode 2 er bygget, men slått av til en feil i OpenCode er rettet.",
    sources: [
      pr(60),
      pr(61),
      pr(64),
      pr(70),
      { label: "#github-copilot på Slack", url: "https://nav-it.slack.com/archives/C055TNXBM17" },
      pr(95),
      pr(110),
      { label: "navikt/cplt", url: "https://github.com/navikt/cplt" },
      pr(149),
      pr(297),
      { label: "utviklerundersøkelsen 2026", url: `${DOCS}utviklerundersokelsen-2026-oppsummering.md` },
      { label: "designnotatet", url: `${DOCS}nav-pilot-design.md` },
      { label: "analysen av bevisst KI-bruk", url: `${DOCS}bevisst-ai-bruk-analyse.md` },
      pr(436),
      pr(1375),
      pr(1446),
      pr(1444),
      TEAM,
    ],
  },
  {
    date: "2026-06",
    end: "2026-10",
    title: "Målinger og kostnad",
    text: "GitHub byttet ut de subsidierte premium requests med AI Credits etter tokenforbruk, og for oss mangedoblet det regningen. Svaret ble modellvalg bygget på målinger: benchmarker som kan kjøres på nytt, siden /modeller og en språkmodell som vurderer planleggingssvar. Målingene flyttet @rust til Claude Haiku 5.5, og @nav-pilot sender nå avgrensede oppgaver til @worker på GPT-6 Luna, som koster mindre.",
    sources: [
      {
        label: "GitHubs kunngjøring",
        url: "https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/",
      },
      { label: "vår nyhetssak", url: `${DOCS}news/articles/usage-based-billing.md` },
      TEAM,
      pr(379),
      pr(442),
      pr(1373),
      pr(1376),
      pr(1415),
      pr(1470),
      pr(1471),
      pr(1488),
      pr(1496),
    ],
  },
  {
    date: "2026-10",
    title: "I dag",
    text: "Alle teknologer i Nav har tilgang til Copilot. Neste steg er resten av produktteamet, så alle rollene får de samme KI-verktøyene.",
    sources: [TEAM],
  },
];
