# 🧭 nav-pilot

nav-pilot er et CLI-verktøy og en AI-agent for Nav-utvikling med GitHub Copilot og opencode.

📖 **Online docs (primær):** https://ki-utvikling.nav.no/nav-pilot  
📝 **Endringslogg:** [docs/nav-pilot-changelog.md](nav-pilot-changelog.md)

## Kom i gang

```bash
# Anbefalt: Homebrew (macOS), nav-pilot og påkrevd isolasjon
brew install navikt/tap/nav-pilot navikt/tap/cplt

# Linux / CI: last ned og inspiser skriptet manuelt
curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh -o install.sh
cat install.sh   # Se gjennom skriptet før kjøring
bash install.sh
```

> ⚠ **Sikkerhetsmerk:** `curl ... | bash` kjører installasjonsskriptet uten forhåndsverifikasjon.
> Binæren verifiseres med SHA256-checksum og SLSA provenance (krever `gh` CLI), men skriptet
> som laster den ned er ikke signert. Derfor Homebrew på macOS, og manuell nedlasting og
> gjennomlesing på Linux/CI.

```bash
# I et repo
nav-pilot
nav-pilot install kotlin-backend
```

## Hvor skal artefaktene installeres?

Tre former er i bruk i Nav, og de løser ulike problemer. `install` spør hvor den skal
installere, i repoet (`.github/`) eller i hjemmekatalogen (`~/.copilot/`). Svar på forhånd
med `--repo`, `--user` eller `--target <mappe>` for å hoppe over spørsmålet.

| Form | Hvor | Kort sagt |
|---|---|---|
| Repo (`--repo`) | `<repo>/.github/` | Hele teamet får det samme, og Copilot på github.com ser det |
| Personlig (`--user`) | `~/.copilot/` | Følger deg på tvers av alle repoer, ingenting sjekkes inn |
| Hub-repo | ett repo med `.github/` pluss egne artefakter | Ett sted å vedlikeholde teamets egne skills |

De utelukker ikke hverandre. `nav-pilot sync` uten scope-flagg synker alle scope som har en
tilstandsfil, og de spores hver for seg.

### Repo-installasjon

Skriver til `<repo>/.github/`: `agents/`, `skills/`, `instructions/`, `prompts/` og
tilstandsfila `.github/.nav-pilot-state.json`.

Dette får du bare her:

- **Prompts.** Brukerscopet støtter `agent`, `skill` og `instruction`, ikke `prompt`
  (`ScopeUser()` i `cli/nav-pilot/internal/domain/domain.go`). Installerer du en samling med
  `--user`, hoppes promptene over og rapporteres som ikke støttet. Ber du om én enkelt prompt
  med `--type prompt --user`, er det en feilmelding.
- **Copilot på github.com.** Filene er sjekket inn, så det som kjører på GitHub-siden leser
  dem. Det forutsetter at du committer og pusher, og nav-pilot gjør ingen av delene.
- **Automatisk oppdatering uten at noen kjører CLI-et.** Den gjenbrukbare workflowen
  `copilot-customization-sync.yml` kjører `nav-pilot sync` i Actions og åpner PR med
  oppdateringene. Se [README.sync.md](README.sync.md).
- **Alle på teamet, også de som ikke har nav-pilot.** Filene ligger der uansett.

Hva det koster: innholdet blir liggende i repoet og dukker opp i differ og
kodegjennomgang. `kotlin-backend` er 93 filer og rundt 490 KB målt på artefaktene i denne
kilden. Filer teamet vil eie selv kan merkes som overrides i `.github/copilot-sync.json`,
og blir da hoppet over ved sync.

### Personlig installasjon (`--user`)

```bash
nav-pilot install --user --all
eval "$(nav-pilot env)"
```

Skriver til `~/.copilot/`. Ingenting sjekkes inn noe sted.

Dette får du bare her:

- **Alle repoer på maskinen på én gang**, også repoer der du ikke vil eller kan endre
  `.github/`.
- **Ett sted å synce.** Repo-scopet er alltid det repoet du står i. Med repo-installasjon i
  40 repoer må noen innom alle 40, eller sette opp sync-workflowen i alle 40. Med `--user`
  er det én installasjon å holde fersk.
- **`nav-pilot ignore <type> <name> --user`** for å slippe varsler om komponenter du ikke
  vil ha. Kommandoen avviser repo-scope.

Dette når den ikke:

- **Prompts.** Se over.
- **Instruksjoner utenfor nav-pilot.** De havner i `~/.copilot/.github/instructions/`, og
  leses bare når `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` peker på `~/.copilot`. Starter du
  klienten med `nav-pilot`, settes den for deg (`copilotEnv` i
  `cli/nav-pilot/internal/provider/copilot_launch.go`). Starter du `copilot` eller `cplt`
  direkte, må du sette den selv: `eval "$(nav-pilot env)"`. Agenter og skills plukkes opp
  uansett. En staged Tier 2-pakke setter den bevisst ikke.
- **GitHub-siden.** Filene ligger på din maskin, ikke i repoet.
- **Resten av teamet.** En personlig installasjon er personlig.
- **opencode.** Nav-konteksten til opencode materialiseres fra kilden pluss `.github/` i
  repoet du står i, ikke fra `~/.copilot/` (`repoScopeDir()` i
  `cli/nav-pilot/internal/provider/opencode_launch.go`).

### Hooks kan ikke installeres inne i cplt

`nav-pilot install` skriver hooks til `~/.copilot/hooks/` (personlig) eller
`.github/hooks/` (repo). cplt nekter skriving til begge stedene, og det er med vilje: en
hook er kode som kjører senere på maskinen din, utenfor sandboxen, så cplt lar ikke en
prosess inne i sandboxen legge igjen en. Kjører du `nav-pilot install` fra en agentsesjon
i cplt, stopper installasjonen med en feil som forteller deg akkurat det.

Kjør installasjonen fra et vanlig skall i stedet. Alle andre artefakttyper — agenter,
skills, prompts, instruksjoner — installeres helt fint inne i cplt.

### Hub-repo

Mekanisk er et hub-repo en vanlig repo-installasjon i et repo som ikke er en applikasjon,
pluss teamets egne agenter, skills og prompts lagt inn for hånd i det samme `.github/`. Du
kjører nav-pilot fra hub-repoet og jobber derfra.

Det virker fordi begge klientene leser scopet fra arbeidskatalogen:

- Copilot leser `.github/` i repoet direkte, uansett hvem som la fila der.
- opencode materialiserer både kildeartefaktene og det som bare finnes i scopet. Det siste
  kom med [#579](https://github.com/navikt/copilot/pull/579). Før den fikk opencode bare
  det nav-pilot selv hadde installert: et hub-repo med 25 installerte og 3 håndlagde skills
  så 25, uten varsel og uten feilmelding. Kjører teamet en eldre nav-pilot, mangler de tre
  fortsatt.

Skarpe kanter:

- **Konteksten følger arbeidskatalogen.** nav-pilot sender ingen prosjektkatalog med til
  cplt, så klienten arver katalogen du står i. Står du i hub-repoet, har du hub-repoets
  artefakter og hub-repoets filer. Går du til applikasjonsrepoet for å endre kode der, har
  du ikke lenger hub-repoets egne skills i scopet.
- **Kilden vinner ved navnekollisjon.** En håndlagd skill med samme navn som en installert
  taper i opencode. Se «Hva scopet ditt bidrar med» under opencode-avsnittet.
- **Instruksjoner fra `.github/` slås ikke sammen med opencodes globale `AGENTS.md`.**
  `nav-pilot export opencode` er veien når teamet trenger dem der.
- **Sync i hub-repoet oppdaterer hub-repoet.** De andre repoene har fortsatt sitt eget.

`AGENTS.md` og `.github/copilot-instructions.md` synces aldri, uansett form. De er alltid
repo-spesifikke, og er stedet for det som bare gjelder ett repo.

## Sandboxing og isolasjon er påkrevd

Når du bruker en AI-agent på Nav-utstyr, skal agenten kjøre i en sandbox eller tilsvarende
isolasjon. Kravet gjelder både Nav-relatert og personlig agentarbeid.

[`cplt`](https://github.com/navikt/cplt) er den anbefalte og enkleste måten å oppfylle
kravet på. Velger du noe annet, må du selv sette deg inn i hvordan agentklienten isolerer
agenten, og slå på funksjonen. Gir den ikke god nok beskyttelse, må du sørge for tilsvarende
isolasjon selv, for eksempel med en VM eller container. Ikke kjør agenter med ubegrenset
tilgang til Nav-utstyret.

Del [kortversjonen av kravet](https://ki-utvikling.nav.no/nyheter/sandboxing-er-pakrevd-pa-nav-utstyr)
med andre som trenger den.

### Sikkerhetsnivå og versjon i cplt

`nav-pilot doctor` sjekker sikkerhetsnivået til cplt og anbefaler `sandbox.preset = strict`.

Det presetet er en nettverkslås. `gh_guard` og `git_guard` er allerede på i `standard`
(cplt#335), så det strict legger til er nettverket: tvungen proxy, `git_guard` som blokkerer
i stedet for å advare, og `proxy.default_allowlist`. Den siste er den viktige — den gjør at
bare cplt sin innebygde vertsliste pluss det `proxy.allowed_domains` peker på er nåbart.
Alt annet blokkeres.

cplt sin innebygde liste dekker GitHub Copilot og de offentlige pakkeregistrene. Den dekker
ingenting av Navs. Setter du presetet uten å gjøre noe mer, slutter nav-pilot sin telemetri å
komme fram, og skills som `aksel-builder`, `observability-debugging` og `nav-auth` mister
vertene de er bygget rundt — uten at noe på skjermen forteller deg hvorfor.

Derfor skal du sette presetet via nav-pilot, ikke for hånd:

```bash
nav-pilot config     # velg raden «cplt security posture»
```

Den skriver Nav-vertene til `~/.nav-pilot/cplt-allowed-domains.txt`, peker
`proxy.allowed_domains` dit, og setter så presetet — i den rekkefølgen, slik at låsen aldri
rekker å tre i kraft uten vertene. Har du allerede en egen `proxy.allowed_domains`, lar
nav-pilot den være i fred og sier fra at du må ta med vertene selv. cplt-config er personlig,
så nav-pilot setter den aldri stilltiende, og nøkler du har satt selv gjelder fortsatt foran
presetet.

`nav-pilot doctor` sier også fra når cplt selv er utdatert, og foreslår
`brew upgrade navikt/tap/cplt`. nav-pilot laster aldri ned eller oppgraderer cplt for deg.
Svarer ikke GitHub, hopper den bare over versjonssjekken.

## Klienter

nav-pilot støtter tre kodingsagenter (`client`-feltet i konfig):

| Klient | Binær | Nav-kontekst | Standard modell |
|---|---|---|---|
| `copilot` (standard) | `cplt` / `copilot` | Installeres i `.github/` | Agentens eget valg |
| `opencode` | `cplt` + `opencode` | Materialiseres automatisk i brukerens OpenCode config-mappe | `github-copilot/auto` |
| `pi` *(eksperimentell)* | `cplt` + `pi` | Via `AGENTS.md` i prosjektroten | Pis eget valg (`model`/`mode` videresendes ikke ennå) |

> **Bruk cplt-sandboxen.** nav-pilot foretrekker `cplt` og kjører klienten via
> `cplt --agent <klient>`. Agenten kan da lese og skrive prosjektfiler, men når ikke
> SSH-nøkler, tilgangsinformasjon for skytjenester eller andre hemmeligheter. `cplt` må
> være installert for å starte `opencode` og `pi`, i tillegg til selve klient-binæren.

> **Auth-detalj (Copilot/cplt):** nav-pilot henter ikke ut GitHub-tokenet selv.
> Med `cplt`s gh-guard på, som `sandbox.preset = strict` slår på og `nav-pilot
> doctor` anbefaler, skaffer `cplt` tokenet: den bruker `GH_TOKEN`,
> `GITHUB_TOKEN` eller `COPILOT_GITHUB_TOKEN` hvis en av dem er satt, ellers
> `gh auth token` utenfor sandkassen, og leverer det via en 0600-fil som leses
> én gang. Med gh-guarden av gjør `cplt` ingenting her, og Copilot autentiserer
> selv. `copilot_auth_mode` styrer hvilke kilder som slipper fram: `auto`
> (standard) begrenser ingenting, `env_only` avbryter oppstart hvis tokenet ikke
> allerede ligger i miljøet, og `gh_only` fjerner token-variablene fra
> barnemiljøet. Dette er en konfigurasjonskontroll, ikke en sikkerhetsgrense.
> Se [SECURITY.md](../SECURITY.md).

### opencode: Nav-kontekst automatisk

Med `--client opencode` (eller `client = "opencode"` i konfig) gjør nav-pilot dette ved hver
oppstart:

1. Løser opp Nav-kildeartifaktene (skills, agenter, prompts, instruksjoner)
2. Skriver dem til OpenCode-konfigurasjonsmappen (f.eks. `~/.config/opencode/` eller via `XDG_CONFIG_HOME`) som `AGENTS.md`, `skills/`, `commands/`, `agents/` og `instructions/`
3. Holder dem synkronisert med versjonskontroll (konflikt-deteksjon, ferskhetssjekk)
4. Starter opencode i cplt-sandboxen med Nav-agenten (`cplt --agent opencode -- --agent nav-pilot --model …`)

Den materialiserte `nav-pilot`-agenten er en **primær** opencode-agent, så den dukker opp i
agentvelgeren (Tab) og startes automatisk. De øvrige Nav-agentene (auth, kafka, aksel, …)
materialiseres som **subagenter** du kaller med `@navn`.

```bash
nav-pilot --client opencode           # én gangs override
nav-pilot config set client opencode  # sett permanent
```

`nav-pilot status` og `nav-pilot list --installed` viser opencode-artefaktene og om de er
oppdaterte.

##### Hva scopet ditt bidrar med

Skills, prompts og agenter du har lagt inn for hånd i `.github/` i repoet du står i,
materialiseres sammen med Nav-artefaktene. Det er slik et hub-repo får med sine egne
skills i opencode, ikke bare de nav-pilot har installert.

To ting følger ikke med, og det er med vilje:

- **Instruksjoner fra `.github/`** slås ikke sammen med `AGENTS.md`. Utmappa er den
  globale opencode-konfigurasjonen din, og `AGENTS.md` er alltid i kontekst, så
  instruksjonene fra ett repo ville ligget i hver eneste prompt i alle andre repoer.
  Trenger teamet dem i opencode, er `nav-pilot export opencode` veien: den skriver
  `<repo>/.opencode/`, som bare det repoet leser, og tar instruksjonene med.
- **Lokale endringer i en installert artefakt.** Redigerer du en installert skill i
  `.github/`, honorerer Copilot endringen mens opencode får kildeversjonen. Kilden
  vinner ved navnekollisjon. Vil du at opencode skal se den, kopier den til et eget
  navn: både katalognavnet og `name:` i frontmatter må endres, og opencode får da både
  originalen fra kilden og din kopi.

Artefakter fra et annet repo blir liggende i den globale konfigurasjonen til neste sync,
og er synlige ved navn og beskrivelse der. De ryddes bort ved neste oppstart, med mindre
du har endret dem selv: nav-pilot sletter bare det den selv har skrevet og som fortsatt
er uendret. En fil du har redigert blir stående, og forblir sporet, slik at neste sync i
repoet den kom fra melder konflikt framfor å overskrive den.

#### `export opencode` vs. automatisk materialisering

Til ditt **personlige** oppsett trenger du ikke `export` i det hele tatt.

| Kommando | Mål | Tilstandssporing | Når |
|---|---|---|---|
| `nav-pilot --client opencode` (oppstart) | `~/.config/opencode/` | ✅ konflikt + ferskhet | Personlig kontekst, skjer automatisk |
| `nav-pilot sync` | `~/.config/opencode/` | ✅ oppdaterer sporet tilstand | Frisk opp personlig kontekst |
| `nav-pilot export opencode` (repo-scope) | `<repo>/.opencode/` | ingen | Sjekk Nav-kontekst inn i et **prosjektrepo** for hele teamet |

> **Avviklet:** `nav-pilot export opencode --user` er erstattet av automatisk materialisering
> ved oppstart pluss `nav-pilot sync`, som i tillegg gir tilstandssporing og
> konflikt-deteksjon. Repo-scope `export opencode` består.

## Vanlige kommandoer

```bash
nav-pilot list --installed
nav-pilot sync
nav-pilot upgrade
nav-pilot feedback
```

## Lokal modell (alfa, av som standard)

`nav-pilot alpha local` kjører en modell på din egen maskin. Den trekker ingen AI-credits.
Krever en Mac med Apple Silicon og 48 GB minne, og rundt 26 GB ledig disk. `init` gjør resten, inkludert å heve macOS-minnegrensen med `sudo`.

```bash
nav-pilot alpha local init      # gjør alt: miljø, vekter, minnegrense, og starter serveren
nav-pilot alpha local status    # kjører den? svarer den? hvilken modell? hva har den gjort?
nav-pilot alpha local ask -p "..."  # still ett spørsmål rett til modellen
nav-pilot alpha local stop      # og start igjen med start
nav-pilot alpha local on        # skru på igjen etter off
nav-pilot alpha local off       # slutt å sende oppgaver dit; vektene blir liggende
nav-pilot alpha local purge     # fjern alt igjen, viser hva og hvor mye først
```

### Bytte modell

`nav-pilot models` viser hva som er tilgjengelig. De lokale står merket `(local)`.
`local_model` velger hvilken av dem serveren laster; `model` er modellen økten selv kjører på,
og de settes hver for seg.

```bash
nav-pilot models
nav-pilot config set local_model mlx-community/Qwen3.8-27B-4bit
nav-pilot alpha local init      # laster ned vektene for den nye modellen
nav-pilot alpha local start
```

Listen oppdateres når du kjører `init` eller `start` — ikke ved hver kommando, fordi
et nettverkskall der ville lagt seg foran alt annet nav-pilot gjør. Har du nettopp hørt
om en ny modell og ikke ser den, er `start` det som henter listen på nytt.

**Qwen 3.6 er standard fordi den er rask, og fordi ingen av de andre løser målbart flere oppgaver.**
Over fire kjøringer av de samme åtte oppgavene løser den 3, 2, 4 og 4. Qwen 3.8 4-bit løser 4, 4,
3 og 4. Spennene overlapper helt, og forskjellen er ikke målbar (p = 0,71). Til gjengjeld bruker
3.8 omtrent sju ganger så lang tid — median 58–104 sekunder mot 10–12 — og traff
sju-minutterstaket ti ganger der standarden traff det én gang.

**Vi skrev tidligere at 3.8 løser mer.** Det holdt ikke. De tallene ble målt før vi oppdaget at
sandkassen aldri ga modellene tilgang til byggverktøyene: ingen av dem kunne kompilere eller
kjøre tester, og målet var heller ikke pinnet, så de to modellene jobbet på kodebaser fire dager
fra hverandre. Da det ble rettet, forsvant forspranget. Hele historikken står i
[MODELS.md](https://github.com/navikt/mlx-workspace/blob/main/MODELS.md).

Valget er altså hastighet mot ingenting målbart — som er en grunn til å beholde standarden. `nav-pilot config explain model`
sier det samme kortere, og
[MODELS.md](https://github.com/navikt/mlx-workspace/blob/main/MODELS.md) har tallene.

Bytter du modell, må vektene lastes ned én gang til — 16 GB for 3.8 4-bit, 30 GB for
8-bit. `purge` fjerner det du ikke vil beholde.

Vil du slippe å starte serveren selv, kan en vanlig `nav-pilot` gjøre det når den trenger den:

```bash
nav-pilot config set local_autostart true
```

Av som standard, med vilje: å starte en 21 GB prosess uten å bli bedt om det er ikke greit.

Ingenting av dette skjer med mindre du kjører `init` selv. Gjør du ikke det, er nav-pilot
uendret.

### Utsending til en lokal underagent krever opencode

**Under opencode** blir modellen en underagent (`local-worker`) som hovedagenten i skyen
kan sende avgrensede oppgaver til. Hovedagenten bestemmer fortsatt alt. Den sender videre
det som er mekanisk og spesifisert, og gjør resten selv.

**Under Copilot CLI finnes ingen slik underagent, og kan ikke finnes i dag.** Copilot CLI
er standardklienten i nav-pilot, så dette gjelder deg med mindre du har byttet. Der er
valget hele økten på den lokale modellen eller ingenting lokalt. Grunnen er at klienten
setter modelleverandøren som en miljøvariabel for hele prosessen, så én leverandør betjener
hele økten. Vi har verifisert det mot Copilot CLI 1.0.83-3. Hele økten lokalt passer til
arbeid som allerede er spesifisert, ikke til oppgaver der modellen må finne ut hva som skal
gjøres.

Vil du ha utsending, bytt klient:

```bash
nav-pilot config set client opencode
```

Vi har bedt GitHub om å kunne velge modelleverandør per agent i Copilot CLI. Det ligger som
en feature request hos dem.

### Hva den er god og dårlig til

Målt i et kontrollert testoppsett, på én maskin, og nesten alt på ett Ktor-repo. På den ene Spring-appen vi målte kostet lokal utsending mer enn å la være. Den utfører en avgjørelse godt og tar en
avgjørelse dårlig.

| Fungerer | Fungerer ikke |
|---|---|
| Slå opp noe i koden | Skrive en ny fil fra bunnen |
| Legge til kommentarer og loggsetninger | Finne ut hvilke filer en endring treffer |
| Døpe om et symbol i mange filer | Endringer som krever en vurdering per fil |
| Legge til et felt og oppdatere mapperen | Oppgaver der en feil endring er dyr |

Tiden varierer: fra omtrent som skyen på små endringer til rundt fire ganger så lenge på en omdøping. På den største mekaniske endringen vi målte var den raskere enn skyen.

### Når noe henger

```bash
nav-pilot alpha local status
```

Den skiller «treg» fra «død». Sier den `hung`, restart med `stop` og `start`. Serveren
svarer på én forespørsel om gangen, så flere samtidige oppgaver står i kø framfor å kjøre
parallelt.

Dette er alfa. Si fra om noe henger, om en endring kompilerer men er feil, eller om
ventetiden ikke er verdt det: `nav-pilot feedback`.

## Agentpakker fra andre team

Et team kan distribuere sitt eget innhold som en **agentpakke**, et repo med manifest på
`.nav-pilot/agentpakke.json`. Installer det med `--source`. Kilden huskes per scope til du
tømmer den.

```bash
nav-pilot install --source navikt/<repo> <pakkenavn>
nav-pilot config set source ""     # tilbake til navikt/copilot
nav-pilot validate --source navikt/<repo>   # sjekk en pakke mot kontrakten
```

**[Lag din egen agentpakke →](README.agentpakke.md)** med manifestreferanse,
kompatibilitetsregler og CI-validering.

## Telemetry (pilot, default on)

nav-pilot sender OTel-metrikker som standard i pilot. Standard endpoint er
`https://collector-internet.nav.cloud.nais.io/v1/metrics`, og du kan overstyre den med
`NAV_PILOT_TELEMETRY_ENDPOINT`. `NAV_PILOT_TELEMETRY_ENABLED=0` (eller `off`) slår av
telemetry.

Når nav-pilot starter `cplt`/`copilot`, setter den `OTEL_EXPORTER_OTLP_ENDPOINT` for Copilot
CLI til samme collector-base (`https://collector-internet.nav.cloud.nais.io`, uten
`/v1/metrics`), slik at Copilot kan sende både metrics og traces. Overstyr med
`NAV_PILOT_COPILOT_OTEL_ENDPOINT`. Den har forrang over generell
`OTEL_EXPORTER_OTLP_ENDPOINT`. nav-pilot setter også `COPILOT_OTEL_ENABLED=true` hvis den
ikke allerede er satt, og injiserer resource-attributtene `nav.pilot.launcher`,
`nav.pilot.version` og `nav.pilot.device_id` i Copilots `OTEL_RESOURCE_ATTRIBUTES`
(append-merge, eksisterende nøkler beholdes) for å spore Copilot-traces tilbake til nav-pilot.

Støttede MVP-metrikker:

- `nav_pilot_command_duration_ms` (`_count` er også antall kommandoer)
- `nav_pilot_command_error_total`
- `nav_pilot_install_items_total`
- `nav_pilot_sync_updates_total`
- `nav_pilot_sync_conflicts_total`
- `nav_pilot_info`
- `nav_pilot_install_present`
- `nav_pilot_installed_items`
- `nav_pilot_staleness_check_total`
- `nav_pilot_up_to_date`
- `nav_pilot_version_skew_days`

Metrikkene bærer også `execution_context` for å skille organisk bruk fra CI (`organic`,
`ci_github_actions`, `ci_other`, `unknown`).

## Konfigurasjon

Du kan lagre standardvalg i `~/.nav-pilot/config.toml`.

```bash
nav-pilot config          # interaktiv innstillingsside, alle valg med forklaring
nav-pilot config init
nav-pilot config setup
nav-pilot config show
```

Støttede felt er `client`, `model`, `mode`, `reasoning_effort`, `context_tier`,
`allow_all_tools`, `ask_user`, `auto_launch` og `log_level`. Du kan overstyre dem per kjøring
med globale flagg som `--client`, `--model`, `--mode`, `--effort`, `--context`,
`--allow-all-tools`, `--no-ask-user`, `--auto-launch`/`--no-auto-launch` og `--log-level`.

`--payload-context <id>` gjelder bare kilder som er en agentpakke med ferdigbygde payloads,
og velger hvilken kontekst som stages ved launch. Den har ingen config-nøkkel, standarden er
`defaultContext` i pakkas manifest. Den er ikke det samme som `--context`, som fortsatt er
Copilots long-context-nivå. Se [README.agentpakke.md](README.agentpakke.md).

Etter synk eller installasjon starter nav-pilot kodeagenten automatisk. Sett
`auto_launch = false` (eller bruk `--no-auto-launch`) hvis du heller vil starte den selv.
Da skriver nav-pilot bare ut kommandoen du kan kjøre.

**Modell per klient:**
- Copilot: `auto`, `claude-opus-5`, `claude-fable-5`, `claude-sonnet-5`,
  `claude-sonnet-4.6`, `claude-haiku-4.5`, `claude-opus-4.8`, `claude-opus-4.6`,
  `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-5.6-luna`, `gpt-5.5`, `gpt-5.4`,
  `gpt-5.3-codex`, `gpt-5.4-mini`, `gpt-5-mini`, `gemini-3.6-flash`,
  `gemini-3.1-pro-preview`, `gemini-3.5-flash`, `kimi-k2.7-code`, `kimi-k3`
- opencode (startes via cplt mot GitHub Copilot-provideren): bruk `github-copilot/<id>`,
  f.eks. `github-copilot/auto` (Nav-standard), `github-copilot/claude-opus-4.8`,
  `github-copilot/gpt-5.5`. Modellen i config må være på `provider/model`-format (med `/`).
  `--model auto` på CLI (eller tom CLI-verdi) normaliseres til Nav-standarden
  `github-copilot/auto`.

Veiviseren (`nav-pilot config setup`) viser en modellvelger tilpasset valgt klient, og
`nav-pilot config explain model` lister opp de kurerte id-ene.

**opencode-mapping:** `client = "opencode"` mappes til opencode-flagg. `mode = plan` gir
`--agent plan` (ellers `--agent nav-pilot`), `model` gir `--model` (prefikses med
`github-copilot/` for bare id-er), `reasoning_effort` gir `--variant`, `allow_all_tools` gir
`--dangerously-skip-permissions`, og `log_level` oversettes til opencodes sett
(`DEBUG`/`INFO`/`WARN`/`ERROR`). Felt uten opencode-ekvivalent (`mode = autopilot`,
`context_tier`, `ask_user = false`) gir en ⚠-advarsel ved oppstart.

## For bidragsytere

- Agent: `.github/agents/nav-pilot.agent.md`
- Design: `docs/nav-pilot-design.md`
- Skills: `.github/skills/<name>/`
- Instruksjoner: `.github/instructions/`

Detaljert bruk, CLI-referanse og arbeidsflyt vedlikeholdes i online docs.
