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

`install` spør hvor den skal installere, i repoet (`.github/`) eller i hjemmekatalogen
(`~/.copilot/`). Svar på forhånd med `--repo`, `--user` eller `--target <mappe>` for å hoppe
over spørsmålet.

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
Det presetet slår på `gh_guard`, `git_guard` og tvungen proxy i én nøkkel, og nøkler du har
satt selv gjelder fortsatt foran presetet. cplt-config er personlig, så nav-pilot setter den
aldri stilltiende. Du velger selv, enten med

```bash
cplt config set sandbox.preset strict
```

eller ved å velge raden `cplt security posture` på innstillingssiden (`nav-pilot config`),
som spør før den setter nøkkelen.

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

## Personlig installasjon (valgfritt)

```bash
nav-pilot install --user --all
eval "$(nav-pilot env)"
```

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

- `nav_pilot_command_total`
- `nav_pilot_command_duration_ms`
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
`github-copilot/` for bare id-er) kun når du selv har satt en modell — er den ikke satt,
skriver nav-pilot Nav-standarden inn som `model` i opencodes egen config, slik at hver
agents eget `model:`-felt styrer agenten sin, `reasoning_effort` gir `--variant`, `allow_all_tools` gir
`--dangerously-skip-permissions`, og `log_level` oversettes til opencodes sett
(`DEBUG`/`INFO`/`WARN`/`ERROR`). Felt uten opencode-ekvivalent (`mode = autopilot`,
`context_tier`, `ask_user = false`) gir en ⚠-advarsel ved oppstart.

## For bidragsytere

- Agent: `.github/agents/nav-pilot.agent.md`
- Design: `docs/nav-pilot-design.md`
- Skills: `.github/skills/<name>/`
- Instruksjoner: `.github/instructions/`

Detaljert bruk, CLI-referanse og arbeidsflyt vedlikeholdes i online docs.
