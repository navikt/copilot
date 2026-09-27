// The pages in each group, for the overview pages and for the block
// «Sider i denne delen» at the bottom of each page. The section menu (§6.4 in
// docs/nav-pilot-dokumentasjon-forslag.md) replaces the block.
export type DocLink = { href: string; title: string; desc: string };

export const GUIDE_PAGES: DocLink[] = [
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
    desc: "Styr utsendingen, bytt modell, bruk egen server, og bruk alpha decide i hooks og skript.",
  },
  {
    href: "/nav-pilot/guider/feilsoking",
    title: "Feilsøking",
    desc: "Sjekk maskinen med doctor, se hva cplt blokkerer, og få liv i en lokal modell som henger.",
  },
];

export const REFERENCE_PAGES: DocLink[] = [
  {
    href: "/nav-pilot/referanse",
    title: "Kommandoer, konfig og tabeller",
    desc: "Kommandoene, avslutningskodene, konfignøklene, sikkerhetsnivåene, telemetrien, de lokale modellene og filene nav-pilot installerer.",
  },
  {
    href: "/nav-pilot/klienter",
    title: "Klienter",
    desc: "Hva Copilot CLI, opencode og pi kan, Navs MCP-register i opencode, og hva som mangler.",
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
    desc: "Hvorfor utsendingen er begrenset, og hva modellene klarer i målingene våre.",
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
