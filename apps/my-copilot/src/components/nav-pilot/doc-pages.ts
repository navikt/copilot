// The pages in each group, for the overview pages /nav-pilot/guider and
// /nav-pilot/forklaring and for the section menu (SECTION in lib/nav-items.ts).
export type DocLink = { href: string; title: string; desc: string };

export const GUIDE_PAGES: DocLink[] = [
  {
    href: "/nav-pilot/guider/kom-i-gang",
    title: "Kom i gang på 5 minutter",
    desc: "Installer, logg inn med gh, start agenten i et repo og velg hvor mye den skal gjøre selv.",
  },
  {
    href: "/nav-pilot/guider/installere-og-oppgradere",
    title: "Installere og oppgradere",
    desc: "Velg hvor agentpakka skal ligge, installer i CI, oppgrader og avinstaller.",
  },
  {
    href: "/nav-pilot/guider/tilpasse",
    title: "Tilpasse",
    desc: "Endre innstillinger, legg til teamets egne instruksjoner, overstyr og ignorer komponenter, og slå hooks av og på.",
  },
  {
    href: "/nav-pilot/guider/synkronisere",
    title: "Synkronisere",
    desc: "Hold agentpakka oppdatert med en ukentlig pull request eller med nav-pilot sync.",
  },
  {
    href: "/nav-pilot/guider/lokal",
    title: "Lokal modell",
    desc: "Styr utsendingen, bytt modell og bruk alpha decide i hooks og skript.",
  },
  {
    href: "/nav-pilot/guider/worktrees",
    title: "Worktrees",
    desc: "Start nav-pilot i et git-worktree, og la agenten lage worktrees til underagenter i cplt.",
  },
  {
    href: "/nav-pilot/guider/cplt-oppsett",
    title: "Sett opp cplt i et repo",
    desc: "Lag .cplt.toml med cplt init, sjekk den inn og godkjenn den. Med det Go, Gradle, Next.js, pnpm, mise og Docker trenger.",
  },
  {
    href: "/nav-pilot/guider/cplt-gradle",
    title: "Kotlin og Gradle i sandkassen",
    desc: "Få Gradle-bygg og tester til å virke i cplt: daemon, MockK, GitHub Packages, interne verter og Testcontainers.",
  },
  {
    href: "/nav-pilot/guider/cplt-node",
    title: "Node, npm og pnpm i sandkassen",
    desc: "Få npm, pnpm og yarn til å virke i cplt: @navikt-pakker, installasjonsskript, .env-filer, localhost, Playwright og Cypress.",
  },
  {
    href: "/nav-pilot/guider/cplt-git",
    title: "Git og GitHub i sandkassen",
    desc: "Push til egen gren, pull requests, andre repoer og signerte commits når agenten kjører i cplt.",
  },
  {
    href: "/nav-pilot/guider/cplt-nettverk",
    title: "Nettverk i sandkassen",
    desc: "Se hva cplt stopper, og slipp gjennom interne tjenester, andre porter og hoster på en tillatelsesliste.",
  },
  {
    href: "/nav-pilot/guider/cplt-feilmeldinger",
    title: "Feil i sandkassen",
    desc: "Slå opp feilmeldinger fra cplt og verktøy i sandkassen, med kommandoen som løser dem.",
  },
  {
    href: "/nav-pilot/guider/feilsoking",
    title: "Feilsøking",
    desc: "Sjekk maskinen med doctor, se hva cplt blokkerer, og få liv i en lokal modell som henger.",
  },
];

export const EXPLANATION_PAGES: DocLink[] = [
  {
    href: "/nav-pilot/forklaring/planlegging",
    title: "Planlegging",
    desc: "De fire fasene, skillene som driver dem, og hvorfor du skriver kjernelogikken selv.",
  },
  {
    href: "/nav-pilot/forklaring/sandkassen",
    title: "Sandkassen",
    desc: "Hvorfor agenten må kjøre isolert på Nav-utstyr, og hva sikkerhetsnivåene i cplt gjør.",
  },
  {
    href: "/nav-pilot/forklaring/lokal-modell",
    title: "Lokal modell",
    desc: "Hvorfor utsendingen er begrenset, hva nivåene gjør, og hva modellen er godkjent for.",
  },
  {
    href: "/nav-pilot/forklaring/personvern",
    title: "Personvern og telemetri",
    desc: "Hva nav-pilot måler, hva som aldri er med, og hvordan du slår det av.",
  },
  {
    href: "/nav-pilot/forklaring/arkitektur",
    title: "Arkitektur",
    desc: "Hvorfor nav-pilot finnes, hva det vet som Copilot ikke vet, og prinsippene det er bygget på.",
  },
];
