// Data for /nav-pilot/referanse.
//
// CONFIG_KEYS is generated from configKeyDefs in cli/nav-pilot. Do not edit it
// by hand: run
//
//   go test ./internal/cli -run TestConfigKeyDocs -update-config-docs
//
// in cli/nav-pilot.

export const CONFIG_KEYS = [
  {
    key: "version",
    flag: "—",
    values: "1",
    desc: "Skjemaversjon. Mangler den, leses fila som versjon 1, og nav-pilot sier fra med én linje.",
  },
  {
    key: "client",
    flag: "--client",
    values: "copilot · opencode · pi (standard: opencode)",
    desc: "Klient å starte: copilot, opencode eller pi (eksperimentell). Alle kjører i cplt. Mangler nøkkelen i en eksisterende config.toml, betyr det copilot.",
  },
  {
    key: "source",
    flag: "--source",
    values: "owner/name eller en absolutt sti (standard: navikt/copilot)",
    desc: "Hvor agentpakka hentes fra: et GitHub-repo eller en lokal checkout. Settes av install --source --save-source; nav-pilot config unset source går tilbake til standarden.",
  },
  {
    key: "model",
    flag: "--model",
    values: "modell-id, f.eks. claude-opus-4.8",
    desc: "Modell å bruke. En Copilot-id som claude-opus-4.8 virker for copilot og opencode; opencode tar også provider/model. nav-pilot config explain model lister id-ene.",
  },
  {
    key: "mode",
    flag: "--mode",
    values: "default · plan · autopilot (standard: default)",
    desc: "Modus for Copilot-agenten. plan tilsvarer opencode --agent plan; autopilot er bare Copilot.",
  },
  {
    key: "reasoning_effort",
    flag: "--effort",
    values: "none · low · medium · high · xhigh · max",
    desc: "Resonneringsinnsats. Copilot bruker --effort, opencode bruker --variant.",
  },
  {
    key: "context_tier",
    flag: "--context",
    values: "default · long_context",
    desc: "Kontekstnivå. Bare Copilot, og nav-pilot advarer om feltet er satt for opencode.",
  },
  {
    key: "allow_all_tools",
    flag: "--allow-all-tools / --no-allow-all-tools",
    values: "true · false (standard: false)",
    desc: "La agenten kjøre uten å spørre også med autonomy = conservative (Copilot: --allow-all-tools, OpenCode: --auto).",
  },
  {
    key: "ask_user",
    flag: "--ask-user / --no-ask-user",
    values: "true · false (standard: true)",
    desc: "La agenten stoppe og spørre deg. Bare Copilot, og nav-pilot advarer om feltet er satt for opencode.",
  },
  {
    key: "autonomy",
    flag: "—",
    values: "sandbox · conservative (standard: sandbox)",
    desc: "Hvor mye agenten gjør uten å spørre i cplt. sandbox: kjører kommandoer selv, og vaktene i cplt setter grensene. conservative: spør før hver handling.",
  },
  {
    key: "auto_launch",
    flag: "--auto-launch / --no-auto-launch",
    values: "true · false (standard: true)",
    desc: "Start klienten etter sync eller installasjon. Med false skriver nav-pilot bare ut kommandoen.",
  },
  {
    key: "auto_update",
    flag: "—",
    values: "true · false (standard: false)",
    desc: "Oppgrader nav-pilot automatisk når en ny versjon er ute. Feiler det, kjører du videre på versjonen du har.",
  },
  {
    key: "surveys",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Spør av og til etter en økt om du vil svare på en kort brukerundersøkelse. DO_NOT_TRACK og NAV_PILOT_TELEMETRY_ENABLED=false slår det også av.",
  },
  {
    key: "news",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Vis én linje etter en økt når det er kommet en ny nav-pilot-sak, én gang per sak. nav-pilot news lister sakene uansett.",
  },
  {
    key: "log_level",
    flag: "--log-level",
    values: "none · error · warning · info · debug · all · default",
    desc: "Loggnivå for Copilot CLI.",
  },
  {
    key: "otel_log_level",
    flag: "--otel-log-level",
    values: "none · error · warning · warn · info · debug · verbose · all (standard: none)",
    desc: "Loggnivå for OpenTelemetry i Copilot CLI (OTEL_LOG_LEVEL). En OTEL_LOG_LEVEL i skallet vinner, og config show merker den env.",
  },
  {
    key: "local_enabled",
    flag: "—",
    values: "true · false (standard: false)",
    desc: "Send avgrensede oppgaver til en lokal modell (alfa). Settes av alpha local init, nullstilles av alpha local off. Så lenge den er false, ser nav-pilot ingen lokale modeller.",
  },
  {
    key: "local_autostart",
    flag: "—",
    values: "true · false (standard: false)",
    desc: "La en vanlig nav-pilot starte den lokale serveren når den trengs og ingen kjører. Av som standard: en prosess på 21 GB skal ikke starte uten at du har bedt om det.",
  },
  {
    key: "local_loop_guard",
    flag: "—",
    values: "et heltall (standard: 8)",
    desc: "Hvor mange identiske verktøykall på rad som avslutter en lokal tur, uansett hva de returnerer. Gir kallene samme resultat hver gang, holder det med halvparten (minst 2).",
  },
  {
    key: "local_model",
    flag: "—",
    values: "modell-id fra manifestet",
    desc: "Hvilken lokal modell serveren laster (alfa). Tom betyr standardmodellen i manifestet. Sett den med nav-pilot alpha local use <key>.",
  },
  {
    key: "local_endpoint",
    flag: "—",
    values: "en http(s)-URL",
    desc: "Din egen OpenAI-kompatible server (Ollama, llama-server), bare localhost eller privat IP (alfa, uten støtte). Sjekk den med nav-pilot alpha local doctor.",
  },
  {
    key: "local_endpoint_model",
    flag: "—",
    values: "modell-id på serveren",
    desc: "Modell-id-en local_endpoint skal bruke, f.eks. qwen3.6:35b. Påkrevd sammen med local_endpoint.",
  },
  {
    key: "local_dispatch",
    flag: "--local-dispatch",
    values: "off · conservative · balanced · aggressive (standard: aggressive)",
    desc: "Hvor mye hovedagenten i opencode sender til den lokale modellen. aggressive sender mest, men kostet flere KI-kreditter og tok lengre tid i målingene.",
  },
  {
    key: "hook_loop_guard",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Samme løkkeregel i alle Copilot CLI-økter, også i skyen: en postToolUse-hook i ~/.copilot/hooks/ sier fra til modellen når den står fast. false fjerner hooken ved neste oppstart.",
  },
  {
    key: "hook_redact_secrets",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Masker hemmeligheter (GitHub-tokener, AWS-nøkkel-id-er, private nøkler, JWT-er, verdien i password=/api_key=) i verktøyresultater før modellen leser dem, i alle Copilot CLI-økter.",
  },
  {
    key: "hook_redact_fnr",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Masker fødselsnummer, D-nummer og H-nummer i verktøyresultater. nav-pilot maskerer bare elleve sifre der datoen og begge kontrollsifrene stemmer.",
  },
  {
    key: "hook_injection_note",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Sett en merknad foran verktøyresultater som ser ut som instrukser til modellen («ignore previous instructions», rollemarkører), så modellen behandler dem som data. Stopper ingenting.",
  },
  {
    key: "hook_action_check",
    flag: "—",
    values: "off · log (standard: log)",
    desc: "Spør den lokale decide-modellen om en risikabel skallkommando (kubectl, terraform apply, rm -r, git push --force og lignende) står i forhold til formålet. log lagrer svaret og stopper ingenting. Krever lokal modell.",
  },
  {
    key: "mcp_hosts",
    flag: "—",
    values: "ask · off (standard: ask)",
    desc: "Om nav-pilot spør om å slippe gjennom vertene MCP-tjenerne dine trenger i cplt. Vertene hentes fra Navs MCP-register. ask spør én gang per sett, off spør aldri.",
  },
  {
    key: "copilot_auth_mode",
    flag: "—",
    values: "auto · env_only · gh_only (standard: auto)",
    desc: "Hvilken innlogging som når cplt for Copilot. auto begrenser ingenting; env_only krever et token i GH_TOKEN, GITHUB_TOKEN eller COPILOT_GITHUB_TOKEN; gh_only fjerner dem.",
  },
];

export const CLI_COMMANDS = [
  { command: "nav-pilot", description: "Interaktivt: installer, oppgrader eller start Copilot-sandkassen (cplt)" },
  { command: "nav-pilot --client opencode", description: "Start opencode med Nav-konteksten på plass" },
  {
    command: "nav-pilot --verbose",
    description: "Start som vanlig og vis hva oppstarten gjør: sandkassemappe, klient, agent og modell",
  },
  {
    command: 'nav-pilot -- -p "…"',
    description:
      "Start klienten med argumentene etter -- uten meny, spørsmål fra nav-pilot eller sync, med eller uten terminal (CI, skript). Legg til --sync for å synkronisere først. I en terminal viser cplt fortsatt sin egen bekreftelse",
  },
  {
    command: "nav-pilot install nav-pilot",
    description: "Installer agentpakka. Spør om repoet (.github/) eller hjemmekatalogen (~/.copilot/)",
  },
  {
    command: "nav-pilot install --user",
    description: "Installer agenter, skills og instruksjoner til ~/.copilot (alle repoer)",
  },
  { command: "nav-pilot install --dry-run nav-pilot", description: "Forhåndsvis hva som installeres" },
  { command: "nav-pilot install --force nav-pilot", description: "Overskriv lokalt endrede filer" },
  { command: "nav-pilot list", description: "Vis agentpakka og enkeltkomponenter" },
  { command: "nav-pilot list --installed", description: "Vis installerte filer og integritet" },
  { command: "nav-pilot doctor", description: "Sjekk konfig, installasjon, hooks, klienter, cplt og git" },
  {
    command: "nav-pilot mcp list",
    description: "MCP-serverne i Navs register, hvilke du har slått på, og hva som hindrer dem i å virke",
  },
  {
    command: "nav-pilot mcp enable <navn>",
    description: "Slå på en server fra registeret i Copilot CLI og opencode (--client for bare én)",
  },
  {
    command: "nav-pilot mcp disable <navn>",
    description: "Slå av serveren, og slutt å slippe gjennom hostene som bare den trengte",
  },
  { command: "nav-pilot install <name>", description: "Installer én komponent (agent, skill osv.)" },
  {
    command: "nav-pilot install <name> --type <type>",
    description: "Installer med eksplisitt type (agent, skill, instruction, prompt)",
  },
  {
    command: "nav-pilot ignore <type> <name> --user",
    description: "Stopp varsel om en komponent uten å installere den",
  },
  {
    command: "nav-pilot uninstall",
    description: "Fjern det nav-pilot installerte i repoet. --user gjør det samme i ~/.copilot",
  },
  {
    command: "nav-pilot install <name> --frozen",
    description: "Installer nøyaktig det .nav-pilot/agentpakke.lock.json peker på, eller avslutt med kode 3",
  },
  { command: "nav-pilot rollback", description: "Flytt agentpakka i ~/.copilot tilbake til forrige versjon" },
  { command: "nav-pilot init", description: "Lag AGENTS.md og Copilot-instruksjoner med TODO-er i repoet" },
  { command: "nav-pilot sync", description: "Sjekk om oppdateringer finnes (kode 1 hvis ja)" },
  { command: "nav-pilot sync --apply", description: "Oppdater filer direkte" },
  { command: "nav-pilot sync --json", description: "Resultatet som JSON" },
  {
    command: "<command> --json",
    description:
      "JSON på stdout for install, list, sync, export, rollback, validate, ignore, models, usage, version og config (show, path, get, validate). doctor, uninstall og upgrade har ikke JSON",
  },
  { command: "nav-pilot models", description: "Modellene klienten kan bruke, med den du har valgt merket" },
  {
    command: "nav-pilot models claude",
    description: "Filtrer lista: bare modellene med «claude» i id-en eller navnet",
  },
  { command: "nav-pilot models --client opencode", description: "Vis lista for en annen klient" },
  { command: "nav-pilot env", description: "Skriv miljøvariablene for Copilot CLI, til eval i skallprofilen" },
  { command: "nav-pilot upgrade", description: "Oppdater nav-pilot CLI til nyeste versjon" },
  {
    command: "nav-pilot upgrade --dry-run",
    description:
      "Bare sjekk: vis gjeldende → nyeste versjon. Kode 1 når en oppdatering finnes, 0 når du har den nyeste",
  },
  {
    command: "nav-pilot feedback",
    description: "Meld en feil. Åpner et issue i navikt/copilot med versjon og systeminformasjon",
  },
  { command: "nav-pilot feedback --feature", description: "Foreslå ny funksjon" },
  {
    command: "nav-pilot survey",
    description: "Svar på en åpen brukerundersøkelse, også når nav-pilot ikke spør selv",
  },
  { command: "nav-pilot news", description: "Vis de nyeste sakene fra ki-utvikling.nav.no, med lenke" },
  {
    command: "nav-pilot export opencode",
    description: "Skriv agentpakka til .opencode/ i repoet, i formatet til opencode. Trengs ikke for å bruke opencode",
  },
  {
    command: "nav-pilot export opencode --user",
    description: "Skriv til ~/.config/opencode/ (eller $XDG_CONFIG_HOME/opencode/) i stedet, for alle repoer",
  },
  { command: "nav-pilot config", description: "Interaktiv innstillingsside i terminalen" },
  { command: "nav-pilot config init", description: "Opprett ~/.nav-pilot/config.toml med alle valg kommentert ut" },
  {
    command: "nav-pilot config setup",
    description: "Veiviser for klient, modus, modell og hva agenten får gjøre selv",
  },
  {
    command: "nav-pilot config setup --advanced",
    description: "Samme veiviser, pluss spørsmålet om nettverket: cplt-nivået standard eller strict",
  },
  { command: "nav-pilot config show", description: "Vis gjeldende konfig: fila pluss standardverdiene" },
  { command: "nav-pilot config get <key>", description: "Hent én verdi" },
  { command: "nav-pilot config set <key> <value>", description: "Sett én verdi" },
  { command: "nav-pilot config validate", description: "Sjekk konfigfila" },
  { command: "nav-pilot export opencode --dry-run", description: "Forhåndsvis hva som eksporteres" },
  { command: "nav-pilot version", description: "Vis versjonen" },
  {
    command: "nav-pilot alpha local <command>",
    description: "Lokal modell (alfa): init, start, status, models, use, restart, stop, ask, on, off, purge",
  },
  {
    command: 'nav-pilot alpha decide "<spørsmål>" --options a,b',
    description: "Avgjørelse med faste alternativer fra den lokale modellen (alfa). Se --help",
  },
];
