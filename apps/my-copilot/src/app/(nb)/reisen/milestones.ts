const PR = "https://github.com/navikt/copilot/pull/";

export interface Milestone {
  /** First merge date on main (UTC), the repo creation date, or just the year («2023»). */
  date: string;
  /** Last merge date, for an item that spans several PRs. */
  end?: string;
  /** Major milestones get a larger marker and heading. */
  major: boolean;
  title: string;
  text: string;
  /** Where the dates and the claims can be checked. */
  sources: Source[];
}

/** A link, or the team's own account of the time before the repo (no link exists). */
type Source = { label: string; url: string } | { team: true };

const TEAM: Source = { team: true };

const DOCS = "https://github.com/navikt/copilot/blob/main/docs/";

const pr = (n: number) => ({ label: `#${n}`, url: `${PR}${n}` });

// Dates come from `gh pr list --json mergedAt` and `git log` on main. Keep the list chronological by `date`.
// The early history (2023–2025) is the team's own account; external dates link to GitHub's announcements.
export const MILESTONES: Milestone[] = [
  {
    date: "2023",
    major: true,
    title: "Pilot i stedet for forbud",
    text: "Noen utviklere i Nav fikk tilgang til den lukkede betaen av GitHub Copilot, den gang mest en smart autofullføring. Nav kunne ha stoppet det, slik store virksomheter ofte gjør. I stedet kjørte Nav en pilot med de samme brukerne.",
    sources: [TEAM],
  },
  {
    date: "2023",
    major: false,
    title: "Lisenser, retningslinjer og juss",
    text: "Pilotbrukerne fikk Copilot Business-lisenser fra Nav. Teamet skrev retningslinjer og gikk gjennom bruksvilkårene og personvernerklæringen med Navs jurister. Det meste av kildekoden vår var allerede åpen, så kodelekkasje bekymret oss lite. Opphavsrett bekymret oss mer: ingen visste hva modellene var trent på.",
    sources: [
      TEAM,
      {
        label: "Copilot Business lansert 14. februar 2023",
        url: "https://github.blog/news-insights/product-news/github-copilot-for-business-is-now-available/",
      },
    ],
  },
  {
    date: "2025-01-10",
    major: true,
    title: "Repoet blir opprettet",
    text: "navikt/copilot ble opprettet som et åpent repo. Alt som følger, ligger her.",
    sources: [{ label: "commit a083419c", url: "https://github.com/navikt/copilot/commit/a083419c" }],
  },
  {
    date: "2025-04-07",
    major: true,
    title: "Agentmodus, og bruken tar av",
    text: "Etter piloten tok vi gradvis inn flere brukere. Da agentmodus kom til alle i VS Code, økte bruken kraftig.",
    sources: [TEAM, { label: "Agentmodus i VS Code", url: "https://code.visualstudio.com/blogs/2025/04/07/agentMode" }],
  },
  {
    date: "2025-09-24",
    major: false,
    title: "SINTEF studerer Copilot i Nav",
    text: "SINTEF sammenlignet Nav-utviklere med og uten Copilot. Et funn: «We did not find any statistically significant changes in commit-based activity for Copilot users after they adopted the tool, although minor increases were observed.» Studien er publisert på HICSS 2026.",
    sources: [
      TEAM,
      { label: "studien på arXiv", url: "https://arxiv.org/abs/2509.20353" },
      { label: "HICSS 2026", url: "https://doi.org/10.24251/HICSS.2026.880" },
    ],
  },
  {
    date: "2025-12-27",
    end: "2026-01-05",
    major: true,
    title: "Et felles knutepunkt og et fellesskap",
    text: "Vi samlet agenter, skills og instruksjoner på ett sted, med et register over godkjente MCP-servere og en side der alle kan installere dem. Rundt dette vokste det fram et fellesskap som deler det som virker.",
    sources: [
      pr(60),
      pr(61),
      pr(64),
      pr(70),
      { label: "#github-copilot på Slack", url: "https://nav-it.slack.com/archives/C055TNXBM17" },
      TEAM,
    ],
  },
  {
    date: "2026-03-10",
    end: "2026-03-13",
    major: false,
    title: "Innsikt i bruk",
    text: "Vi henter bruksdata fra GitHub hver dag og skanner navikt-repoene for KI-tilpasninger.",
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
    text: "Et kommandolinjeverktøy som installerer Navs agenter og regler og holder dem oppdatert. Fra juni sender det telemetri, så vi ser hvordan det faktisk brukes. Underveis spurte vi teknologene i Nav hvordan de bruker KI-verktøy, og skrev ned designvalgene og analysen bak dem.",
    sources: [
      pr(149),
      pr(297),
      { label: "utviklerundersøkelsen 2026", url: `${DOCS}utviklerundersokelsen-2026-oppsummering.md` },
      { label: "designnotatet", url: `${DOCS}nav-pilot-design.md` },
      { label: "analysen av bevisst KI-bruk", url: `${DOCS}bevisst-ai-bruk-analyse.md` },
    ],
  },
  {
    date: "2026-07-23",
    end: "2026-10-08",
    major: true,
    title: "Modellvalg bygget på målinger",
    text: "Vi tok inn nye modeller, begynte å dokumentere valgene og bygde faste testoppgaver for agentene. Så kom en benchmark som kan kjøres på nytt, siden /modeller, en realistisk kodeoppgave og en språkmodell som vurderer planleggingssvarene. Målingene flyttet @rust til Claude Haiku 5.5.",
    sources: [pr(379), pr(442), pr(1373), pr(1376), pr(1415), pr(1470), pr(1471)],
  },
  {
    date: "2026-08-24",
    end: "2026-09-30",
    major: true,
    title: "Agentpakker",
    text: "Team kan pakke egne agenter og regler, med kontrakt og validering. En arbeidsflyt foreslår ny versjon av den felles basen fra navikt/copilot, etter at nais/pilot lå flere uker bak.",
    sources: [pr(436), pr(1375)],
  },
  {
    date: "2026-10-07",
    major: false,
    title: "OpenCode 2 klar, men slått av",
    text: "Støtten for OpenCode 2 er bygget, men slått av til en feil i OpenCode er rettet.",
    sources: [pr(1446), pr(1444)],
  },
  {
    date: "2026-10-08",
    major: true,
    title: "Koordinator og arbeider",
    text: "@nav-pilot fordeler arbeidet. @worker tar avgrensede oppgaver på GPT-6 Luna, som koster mindre.",
    sources: [pr(1488), pr(1496)],
  },
  {
    date: "2026-10",
    major: true,
    title: "I dag",
    text: "Alle teknologer i Nav har nå tilgang til Copilot. Neste steg er de andre rollene i produktteamene, så hele produktteamet får tilgang til de samme KI-verktøyene.",
    sources: [TEAM],
  },
];
