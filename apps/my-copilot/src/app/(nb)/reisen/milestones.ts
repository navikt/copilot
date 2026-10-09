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
    title: "Fra grasrot til pilot",
    text: "Noen utviklere i Nav fikk tilgang til den lukkede betaen av GitHub Copilot, den gang mest en smart autofullføring. Nav kunne ha stoppet det, slik store virksomheter ofte gjør. I stedet kjørte Nav en pilot med de samme brukerne, og de fikk Copilot Business-lisenser. Fra dag én var dette en grasrotbevegelse og et fellesskap.",
    sources: [
      TEAM,
      {
        label: "Copilot Business lansert 14. februar 2023",
        url: "https://github.blog/news-insights/product-news/github-copilot-for-business-is-now-available/",
      },
    ],
  },
  {
    date: "2023-09-05",
    major: false,
    title: "Retningslinjer og juss",
    text: "De første retningslinjene kom 5. september 2023. De åpnet bare for Copilot Business, ikke private abonnementer. Teamet gikk gjennom bruksvilkårene og personvernerklæringen med Navs jurister. Det meste av kildekoden vår var allerede åpen, så kodelekkasje bekymret oss lite. Opphavsrett bekymret oss mer: ingen visste hva modellene var trent på.",
    sources: [TEAM],
  },
  {
    date: "2023-11-02",
    major: false,
    title: "SINTEF følger utviklerne",
    text: "Høsten 2023 startet SINTEF en studie av Copilot i Nav, som del av forskningsprosjektet Transformit om digital omstilling i offentlig sektor. Forskerne intervjuet brukere og ikke-brukere, gjorde en spørreundersøkelse, observerte og hentet statistikk fra Navs åpne repoer. 2. november 2023 viste vi Copilot fram i en teknisk demo. Studien er publisert på HICSS 2026. Et funn: «We did not find any statistically significant changes in commit-based activity for Copilot users after they adopted the tool, although minor increases were observed.»",
    sources: [
      TEAM,
      { label: "studien på arXiv", url: "https://arxiv.org/abs/2509.20353" },
      { label: "HICSS 2026", url: "https://doi.org/10.24251/HICSS.2026.880" },
    ],
  },
  {
    date: "2024-03-07",
    end: "2024-05",
    major: true,
    title: "Copilot Chat i IntelliJ, og venteliste",
    text: "Mange i Nav skriver Java og Kotlin i IntelliJ. Da Copilot Chat kom til JetBrains, fikk de også en chat, selv om den lå bak VS Code. I mai 2024 var lisensene brukt opp. Vi fikk en venteliste og la fram et budsjettforslag for 2025 om tilgang for alle som trenger det.",
    sources: [
      TEAM,
      {
        label: "Copilot Chat i JetBrains",
        url: "https://github.blog/changelog/2024-03-07-github-copilot-chat-general-availability-in-jetbrains-ide/",
      },
    ],
  },
  {
    date: "2025-01-10",
    end: "2025-01-17",
    major: true,
    title: "Min Copilot og et åpent repo",
    text: "navikt/copilot ble opprettet som et åpent repo. En uke senere kom Min Copilot: selvbetjening av lisenser og bruksstatistikk for hele organisasjonen, aldri per person. Da var vi rundt 111 brukere.",
    sources: [{ label: "commit a083419c", url: "https://github.com/navikt/copilot/commit/a083419c" }, TEAM],
  },
  {
    date: "2025-02-06",
    end: "2025-08",
    major: true,
    title: "Agentmodus, og bruken tar av",
    text: "GitHub viste fram agentmodus. Vi slo på kodegjennomgang med Copilot for navikt og løftet fram egne instruksjoner per repo. I april godtok vi endringer i risikovurderingen og slo av blokkeringen av treff mot offentlig kode, som stoppet svar på Navs egen åpne kode. I august begynte vi å utforske og risikovurdere MCP.",
    sources: [
      TEAM,
      {
        label: "The agent awakens",
        url: "https://github.blog/news-insights/product-news/github-copilot-the-agent-awakens/",
      },
    ],
  },
  {
    date: "2025-09",
    end: "2025-11",
    major: true,
    title: "Over 300 brukere, og flere roller",
    text: "Etter sommeren hadde vi over 300 aktive brukere, de fleste daglig, og Copilot CLI kom. På 100 dager ga Copilot om lag 709 000 kodeforslag og 1,22 millioner genererte linjer, og rundt 190 000 linjer ble tatt i bruk. I november åpnet vi lisensene for flere roller i IT enn utviklere. Derfra rullet det.",
    sources: [TEAM],
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
    date: "2026-06-01",
    major: false,
    title: "Fra premium requests til AI Credits",
    text: "GitHub byttet ut de sterkt subsidierte premium requests med GitHubs AI Credits, som prises etter tokenforbruk. For oss mangedoblet det regningen, og vi begynte å måle hva hver modell koster.",
    sources: [
      {
        label: "GitHubs kunngjøring",
        url: "https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/",
      },
      { label: "vår nyhetssak", url: `${DOCS}news/articles/usage-based-billing.md` },
      TEAM,
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
