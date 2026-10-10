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
  /** One line on where things stood when the phase ended. */
  status: string;
  /** One to three dated key milestones, in date order. */
  milestones: { date: string; text: string; url?: string }[];
  /** At most two small figures, each with Norwegian alt text and a caption naming its source. */
  figures?: { src: string; alt: string; caption: string; href?: string; narrow?: boolean }[];
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
    status: "En liten gruppe pilotbrukere med Copilot Business, autofullføring i editoren og retningslinjer på plass.",
    milestones: [
      { date: "2023", text: "Nav velger pilot framfor forbud" },
      { date: "2023", text: "SINTEF starter studien av Copilot i Nav", url: "https://arxiv.org/abs/2509.20353" },
      { date: "2023-09-05", text: "Nav får sine første retningslinjer for Copilot" },
    ],
    text: "Noen utviklere i Nav fikk tilgang til den lukkede betaen av GitHub Copilot, den gang mest en smart autofullføring. Nav stoppet det ikke, men kjørte en pilot med Copilot Business-lisenser. Teamet skrev retningslinjer og gikk gjennom vilkårene med Navs jurister. Det meste av koden vår var åpen, så opphavsrett bekymret oss mer enn lekkasje. Samme høst startet SINTEF en studie av Copilot i Nav. Fellesskapet var der fra dag én.",
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
    end: "2024-11",
    title: "Fra pilot til hverdag",
    status: "Chat i VS Code og IntelliJ, men lisensene var brukt opp i mai 2024, og nye brukere sto på venteliste.",
    milestones: [{ date: "2024-05", text: "Lisensene er brukt opp, og vi får venteliste" }],
    text: "Copilot Chat kom til IntelliJ, der mange av Navs Java- og Kotlin-utviklere jobber. Etterspørselen vokste forbi lisensene. I mai var de brukt opp, og vi fikk venteliste og et budsjettforslag om tilgang for alle som trenger det. I november kunne vi velge modell i Copilot, blant annet Claude 3.5 Sonnet og OpenAI o1.",
    sources: [
      TEAM,
      {
        label: "Copilot Chat i JetBrains",
        url: "https://github.blog/changelog/2024-03-07-github-copilot-chat-general-availability-in-jetbrains-ide/",
      },
      {
        label: "Claude 3.5 Sonnet",
        url: "https://github.blog/changelog/2024-11-01-claude-3-5-sonnet-is-now-available-to-all-copilot-users-in-public-preview/",
      },
      {
        label: "modellvalg i Copilot",
        url: "https://github.blog/news-insights/product-news/bringing-developer-choice-to-copilot/",
      },
    ],
  },
  {
    date: "2025-01",
    end: "2025-11",
    title: "Agentene kommer",
    figures: [
      {
        src: "/images/reisen/min-copilot-2025.webp",
        alt: "Skjermbilde av Min Copilot i januar 2025: abonnementet med plan, status, siste aktivitet og en knapp for å deaktivere Copilot.",
        caption: "Min Copilot i januar 2025",
      },
      {
        src: "/images/reisen/statistikk-2025.webp",
        alt: "Skjermbilde av statistikken i Min Copilot i januar 2025: 111 aktive brukere, 54 aktive chatbrukere, Kotlin som mest brukte språk og JetBrains som mest brukte editor.",
        caption: "Statistikken i Min Copilot, januar 2025: 111 aktive brukere",
      },
    ],
    status:
      "Over 300 aktive brukere, de fleste daglig. På 100 dager ga Copilot om lag 709 000 kodeforslag og 1,22 millioner genererte linjer. Utviklerne tok rundt 190 000 linjer i bruk.",
    milestones: [
      { date: "2025-01", text: "Over 100 brukere" },
      {
        date: "2025-01-15",
        text: "Min Copilot: selvbetjente lisenser for utviklerne",
        url: "https://github.com/navikt/copilot/commit/3342f8bb",
      },
      {
        date: "2025-02",
        text: "Kodegjennomgang med Copilot (beta) i navikt",
        url: "https://github.blog/changelog/2025-02-26-code-review-in-github-copilot-is-now-in-public-preview/",
      },
      {
        date: "2025-04-07",
        text: "Agent mode til alle i VS Code",
        url: "https://code.visualstudio.com/blogs/2025/04/07/agentMode",
      },
      { date: "2025-09", text: "Over 300 aktive brukere" },
      {
        date: "2025-09-25",
        text: "Copilot CLI i offentlig forhåndsversjon",
        url: "https://github.blog/changelog/2025-09-25-github-copilot-cli-is-now-in-public-preview/",
      },
      {
        date: "2025-11-21",
        text: "Designere, infrastruktur og plattform får tilgang",
        url: "https://github.com/navikt/copilot/commit/816a7bd3",
      },
    ],
    text: "I januar åpnet vi repoet navikt/copilot og Min Copilot med selvbetjente lisenser, for rundt 111 brukere. I februar viste GitHub fram agent mode, og Claude 3.7 Sonnet kom i Copilot. Så kom kodegjennomgang og egne instruksjoner per repo. Vi fjernet blokkeringen av treff mot offentlig kode og risikovurderte MCP. I august og september kom GPT-5 og Claude Sonnet 4.5. Til høsten hadde vi over 300 aktive brukere og Copilot CLI, og i november fikk designere og folk som jobber med infrastruktur og plattform også tilgang.",
    sources: [
      TEAM,
      { label: "commit a083419c", url: "https://github.com/navikt/copilot/commit/a083419c" },
      {
        label: "The agent awakens",
        url: "https://github.blog/news-insights/product-news/github-copilot-the-agent-awakens/",
      },
      {
        label: "Claude 3.7 Sonnet",
        url: "https://github.blog/changelog/2025-02-24-claude-3-7-sonnet-is-now-available-in-github-copilot-in-public-preview/",
      },
      {
        label: "GPT-5",
        url: "https://github.blog/changelog/2025-08-07-openai-gpt-5-is-now-in-public-preview-for-github-copilot/",
      },
      {
        label: "Claude Sonnet 4.5",
        url: "https://github.blog/changelog/2025-09-29-anthropic-claude-sonnet-4-5-is-in-public-preview-for-github-copilot/",
      },
      { label: "Agent mode i VS Code", url: "https://code.visualstudio.com/blogs/2025/04/07/agentMode" },
    ],
  },
  {
    date: "2025-12",
    end: "2026-10",
    title: "Fra verktøy til plattform",
    figures: [
      {
        src: "/images/reisen/nav-pilot-plakat.webp",
        alt: "Toppen av plakaten «Introduserer Nav-Pilot», med en rakett og et kodevindu for @nav-pilot ved siden av en generell Copilot.",
        caption: "Plakaten fra lanseringen av nav-pilot, april 2026",
      },
      {
        src: "/images/github-copilot-vs-code-mcp.jpeg",
        alt: "Illustrasjon av MCP-servere i GitHub Copilot i VS Code",
        caption: "Illustrasjon: GitHub",
      },
    ],
    status:
      "En felles katalog med agenter, skills og instruksjoner, installert med nav-pilot og kjørt i sandkasse med cplt.",
    milestones: [
      { date: "2025-12-27", text: "MCP-registeret kommer", url: `${PR}61` },
      { date: "2026-04-09", text: "Sandkassen cplt får eget repo", url: "https://github.com/navikt/cplt" },
      { date: "2026-04-13", text: "nav-pilot er på plass", url: `${PR}149` },
      { date: "2026-05-05", text: "Nettstedet ki-utvikling.nav.no blir delvis åpent", url: `${PR}219` },
      { date: "2026-05-27", text: "Skills erstatter agenter uten verktøygrenser", url: `${PR}255` },
      { date: "2026-06-02", text: "Backenden skilles ut fra Next.js til copilot-api", url: `${PR}236` },
      {
        date: "2026-06-17",
        text: "mlx-workspace: språkmodeller lokalt på Mac",
        url: "https://github.com/navikt/mlx-workspace",
      },
      { date: "2026-08-24", text: "Agentpakker lar team lage sine egne", url: `${PR}436` },
      {
        date: "2026-09-30",
        text: "Nais bygger nais/pilot på navikt/copilot",
        url: "https://github.com/nais/pilot/issues/14",
      },
    ],
    text: "Vi samlet agenter, skills og instruksjoner i en felles katalog med MCP-register, og fellesskapet deler det som virker. Med sandkassen cplt og kommandolinjeverktøyet nav-pilot ble katalogen noe hvert team installerer. Valgene bygger på en utviklerundersøkelse og et designnotat. I mai åpnet vi nettstedet delvis, skills tok over for agenter uten verktøygrenser, og vi skilte backenden ut i en egen tjeneste. Med agentpakker lager team sine egne, og en arbeidsflyt holder basen oppdatert. Arbeidet sprer seg utenfor Nav: Kartverket, SSB og Cloud Native Bergen kjører agenter i cplt, Altinn viser til ki-utvikling.nav.no, og Nais bygger sin egen pakke på navikt/copilot. Støtten for OpenCode 2 er bygget, men slått av til en feil i OpenCode er rettet.",
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
      { label: "cplt i Kartverket", url: "https://github.com/kartverket/skiperator/blob/main/.cplt.toml" },
      { label: "cplt i SSB", url: "https://github.com/statisticsnorway/vardef/blob/main/.cplt.toml" },
      { label: "cplt i Cloud Native Bergen", url: "https://github.com/CloudNativeBergen/website/blob/main/.cplt.toml" },
      {
        label: "Altinn",
        url: "https://github.com/Altinn/altinn-auth/blob/main/docs/ai/working-agreement-rationale.md",
      },
      { label: "nais/pilot", url: "https://github.com/nais/pilot/issues/14" },
      TEAM,
    ],
  },
  {
    date: "2026-05",
    end: "2026-10",
    title: "Målinger og kostnad",
    figures: [
      {
        src: "/images/nav-pilot-step-count.svg",
        alt: "Diagram: jo flere steg skymodellen trenger alene, jo mer sparer nav-pilot på å sende arbeid til en lokal modell.",
        caption: "Vår måling av lokale modeller i nav-pilot",
        href: "/nyheter/lokale-modeller-i-nav-pilot",
      },
    ],
    status: "Hver agent har en modell valgt ut fra målinger, og vi kjenner kostnaden per modell.",
    milestones: [
      {
        date: "2026-05-13",
        text: "Hver agent låses til en fast modell for å holde kostnaden nede",
        url: "https://github.com/navikt/copilot/commit/7778d409",
      },
      {
        date: "2026-06-01",
        text: "AI Credits erstatter premium requests",
        url: "https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/",
      },
      {
        date: "2026-06-04",
        text: "kode24: Nav må betale tre til fire ganger mer",
        url: "https://www.kode24.no/artikkel/nav-ma-betale-tre-til-fire-ganger-mer-for-sine-600-copilot-brukere/264699",
      },
      {
        date: "2026-08-31",
        text: "nav-pilot kan sende oppgaver til en lokal modell (alfa)",
        url: "/nyheter/lokale-modeller-i-nav-pilot",
      },
      { date: "2026-09-02", text: "Første baseline for benchmarkene", url: `${PR}594` },
      {
        date: "2026-09-25",
        text: "nav-pilot decide gir et svar uten å starte en agent (alfa)",
        url: "/nyheter/nav-pilot-alpha-decide",
      },
      { date: "2026-09-30", text: "Siden /modeller viser valgene og målingene", url: `${PR}1376` },
      { date: "2026-10-08", text: "@nav-pilot fordeler arbeid til @worker", url: `${PR}1496` },
    ],
    text: "Allerede i mai låste vi hver agent til en fast modell for å holde kostnaden nede. Så byttet GitHub ut de subsidierte premium requests med AI Credits etter tokenforbruk. For oss betydde det tre til fire ganger høyere regning for rundt 600 daglige brukere. Vi testet lokale modeller som et billigere alternativ og laget decide for spørsmål som trenger et svar, ikke en agent. Resten av svaret er modellvalg bygget på målinger: benchmarker vi kan kjøre på nytt, siden /modeller og en språkmodell som vurderer planleggingssvar. Målingene flyttet @rust til Claude Haiku 5.5, og @nav-pilot sender nå avgrensede oppgaver til @worker på GPT-6 Luna, som koster mindre. I samme periode kom en ny generasjon modeller: Claude Opus 5.5 og GPT-6 Sol og Luna.",
    sources: [
      {
        label: "GitHubs kunngjøring",
        url: "https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/",
      },
      {
        label: "Claude Opus 5.5",
        url: "https://github.blog/changelog/2026-09-22-claude-opus-5-5-is-now-available-in-github-copilot/",
      },
      {
        label: "GPT-6 Sol og Luna",
        url: "https://github.blog/changelog/2026-09-22-openais-gpt-6-sol-and-gpt-6-luna-now-available/",
      },
      { label: "navikt/mlx-workspace", url: "https://github.com/navikt/mlx-workspace" },
      pr(483),
      { label: "saken om decide", url: `${DOCS}news/articles/nav-pilot-alpha-decide.md` },
      {
        label: "kode24",
        url: "https://www.kode24.no/artikkel/nav-ma-betale-tre-til-fire-ganger-mer-for-sine-600-copilot-brukere/264699",
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
    date: "2026-09",
    end: "2026-10",
    title: "I dag",
    status:
      "Utviklere, designere, infrastruktur og plattform har tilgang, og flere roller i produktutvikling er på vei inn.",
    milestones: [
      {
        date: "2026-09-24",
        text: "Tilgang for flere roller i produktutvikling",
        url: "https://github.com/navikt/copilot/commit/a377208a",
      },
    ],
    text: "Utviklere har hatt tilgang siden Min Copilot kom i januar 2025, og designere, infrastruktur og plattform siden november 2025. I september 2026 åpnet vi for flere roller i produktutvikling. Målet er at hele produktteamet får de samme mulighetene.",
    sources: [TEAM],
  },
];

/**
 * Dated events drawn as vertical rules on the charts in /innsikt/trender. /reisen does not render them.
 * They show timing, not cause. Model pins affect only our own agents; billing applies to the whole enterprise.
 * Dates for our own changes are merge dates from `gh pr view --json mergedAt`;
 * dates for GitHub changes are when GitHub made the change.
 */
export interface ChartAnnotation {
  /** YYYY-MM-DD. */
  date: string;
  /** A few words, shown in the «Hendelser» list and the chart tooltip. */
  label: string;
  url?: string;
  /** What the event means for reading the charts, shown in the «Hendelser» list. */
  note?: string;
  /** A break in the data itself, always marked in the charts. */
  dataBreak?: boolean;
  /** A model event: Nav's model policy, GitHub's default or auto model, or a major model release. Always marked in the model chart. */
  model?: boolean;
}

const CL = "https://github.blog/changelog/";

export const CHART_ANNOTATIONS: ChartAnnotation[] = [
  {
    date: "2025-12-18",
    label: "Claude Opus 4.5 tilgjengelig (usikker)",
    url: `${CL}2025-12-18-claude-opus-4-5-is-now-generally-available-in-github-copilot/`,
    note: "Business- og Enterprise-kunder måtte slå på modellen selv. Vi vet ikke nøyaktig når Nav gjorde det.",
    model: true,
  },
  {
    date: "2026-02-05",
    label: "Claude Opus 4.6 tilgjengelig",
    url: `${CL}2026-02-05-claude-opus-4-6-is-now-generally-available-for-github-copilot/`,
    model: true,
  },
  {
    date: "2026-04-02",
    label: "Nytt metrics-API",
    url: "https://github.blog/changelog/2026-01-29-closing-down-notice-of-legacy-copilot-metrics-apis/",
    note: "Tallene før og etter er hentet på ulike måter, så teamandelene kan ikke sammenlignes direkte.",
    dataBreak: true,
  },
  {
    date: "2026-04-09",
    label: "GitHub begynner å telle CLI",
    note: "Bruk av Copilot CLI regnes som aktivitet fra denne datoen. Det ga 60–80 flere aktive brukere per hverdag.",
    dataBreak: true,
  },
  {
    date: "2026-04-16",
    label: "Claude Opus 4.7 tilgjengelig",
    url: `${CL}2026-04-16-claude-opus-4-7-is-generally-available/`,
    model: true,
  },
  {
    date: "2026-05-17",
    label: "GPT-5.3-Codex blir GitHubs grunnmodell",
    url: `${CL}2026-05-17-gpt-5-3-codex-is-now-the-base-model-for-copilot-business-and-enterprise/`,
    note: "Erstatter GPT-4.1 som grunnmodell for Business og Enterprise.",
    model: true,
  },
  {
    date: "2026-05-28",
    label: "Claude Opus 4.8 tilgjengelig",
    url: `${CL}2026-05-28-claude-opus-4-8-is-generally-available-for-github-copilot/`,
    model: true,
  },
  {
    date: "2026-06-01",
    label: "AI Credits erstatter premium requests",
    url: "https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/",
    note: "Kostnadene før og etter er ikke direkte sammenlignbare.",
    dataBreak: true,
  },
  {
    date: "2026-07-23",
    label: "Nav slår på GPT-5.6 Sol, Terra og Luna",
    url: `${PR}379`,
    model: true,
  },
  {
    date: "2026-07-24",
    label: "Claude Opus 5 tilgjengelig",
    url: `${CL}2026-07-24-claude-opus-5-is-now-available-in-github-copilot/`,
    model: true,
  },
  {
    date: "2026-08-26",
    label: "Nye modeller slås på automatisk (usikker)",
    url: `${CL}2026-07-29-default-model-enablement-for-copilot-business-and-enterprise/`,
    note: "Nye modeller er på fra de blir allment tilgjengelige, med mindre administrator slår dem av. Kunngjort 29. juli. Vi har ikke logget om Nav beholdt standardinnstillingen.",
    model: true,
  },
  {
    date: "2026-09-04",
    label: "GPT-6 Astra tilgjengelig (usikker)",
    url: `${CL}2026-09-04-gpt-6-astra-is-generally-available-in-github-copilot/`,
    note: "GitHub rullet modellen ut gradvis, og den var på som standard. Dette kan forklare hoppet i GPT-andelen i uke 37.",
    model: true,
  },
  {
    date: "2026-09-14",
    label: "Auto-modell får tre nivåer",
    url: `${CL}2026-09-14-configure-cost-and-quality-in-copilot-auto-model-selection/`,
    model: true,
  },
  {
    date: "2026-09-22",
    label: "GPT-6 Sol og GPT-6 Luna tilgjengelig",
    url: `${CL}2026-09-22-openais-gpt-6-sol-and-gpt-6-luna-now-available/`,
    model: true,
  },
  {
    date: "2026-09-22",
    label: "Claude Opus 5.5 tilgjengelig",
    url: `${CL}2026-09-22-claude-opus-5-5-is-now-available-in-github-copilot/`,
    model: true,
  },
  {
    date: "2026-09-29",
    label: "GPT-6.1 Sol tilgjengelig",
    url: `${CL}2026-09-29-gpt-6-1-sol-in-github-copilot/`,
    model: true,
  },
  { date: "2026-09-30", label: "Modellbenchmark", url: `${PR}1373` },
  { date: "2026-09-30", label: "@kafka og @rust til GPT-6 Luna", url: `${PR}1397` },
  { date: "2026-10-02", label: "Benchmark med realistisk kodeoppgave", url: `${PR}1415` },
  {
    date: "2026-10-02",
    label: "Claude Opus 4.7, Gemini 3.5/3.6 Flash og Kimi K2.7 Code fjernes",
    url: `${CL}2026-10-02-selected-models-in-github-copilot-deprecated/`,
    model: true,
  },
  { date: "2026-10-07", label: "@security-champion til Opus 5.5", url: `${PR}1449` },
  {
    date: "2026-10-07",
    label: "Claude Haiku 5.5 tilgjengelig",
    url: `${CL}2026-10-07-claude-haiku-5-5-in-github-copilot/`,
    model: true,
  },
  { date: "2026-10-08", label: "@rust til Claude Haiku 5.5", url: `${PR}1471` },
  { date: "2026-10-08", label: "KI-dommer for planlegging", url: `${PR}1470` },
  { date: "2026-10-08", label: "@worker på GPT-6 Luna", url: `${PR}1488` },
];
