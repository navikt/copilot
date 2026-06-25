# 🧭 nav-pilot

nav-pilot er et CLI-verktøy og en AI-agent for Nav-utvikling med GitHub Copilot og opencode.

📖 **Online docs (primær):** https://ki-utvikling.nav.no/nav-pilot  
📝 **Endringslogg:** [docs/nav-pilot-changelog.md](nav-pilot-changelog.md)

## Kom i gang

```bash
# Installer CLI og nødvendige avhengigheter (anbefalt)
curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash

# ... eller via Homebrew (installerer også rtk og cplt)
brew install navikt/tap/nav-pilot

# I et repo
nav-pilot
nav-pilot install kotlin-backend
```

## Klienter

nav-pilot støtter tre kodingsagenter (`client`-feltet i konfig):

| Klient | Binær | Nav-kontekst | Standard modell |
|---|---|---|---|
| `copilot` (standard) | `cplt` / `copilot` | Installeres i `.github/` | Agentens eget valg |
| `opencode` | `cplt` + `opencode` | Materialiseres automatisk i brukerens OpenCode config-mappe | `github-copilot/claude-sonnet-4.5` |
| `pi` *(eksperimentell)* | `cplt` + `pi` | Via `AGENTS.md` i prosjektroten | Pis eget valg (`model`/`mode` videresendes ikke ennå) |

> **Alle klienter startes i cplt-sandboxen.** nav-pilot kjører klienten via
> `cplt --agent <klient>` slik at agenten er kjerne-nivå-sandboxet (kan lese/skrive
> prosjektfiler, men når ikke SSH-nøkler, sky-credentials eller andre hemmeligheter).
> `cplt` må derfor være installert for å starte `opencode` og `pi` (i tillegg til selve klient-binæren).

### opencode — Nav-kontekst automatisk

Når du bruker `--client opencode` (eller `client = "opencode"` i konfig), gjør
nav-pilot følgende ved hver oppstart:

1. Løser opp Nav-kildeartifaktene (skills, agenter, prompts, instruksjoner)
2. Skriver dem til OpenCode-konfigurasjonsmappen (f.eks. `~/.config/opencode/` eller via `XDG_CONFIG_HOME`) som `AGENTS.md`, `skills/`, `commands/`, `agents/` og `instructions/`
3. Holder dem synkronisert med versjonskontroll (konflikt-deteksjon, ferskhetssjekk)
4. Starter opencode i cplt-sandboxen med Nav-agenten (`cplt --agent opencode -- --agent nav-pilot --model …`)

Den materialiserte `nav-pilot`-agenten er en **primær** opencode-agent, så den dukker
opp i agentvelgeren (Tab) og startes automatisk. De øvrige Nav-agentene
(auth, kafka, aksel, …) materialiseres som **subagenter** du kaller med `@navn`.

Du trenger ikke kjøre `nav-pilot export opencode` manuelt — Nav-konteksten er alltid oppdatert.

```bash
nav-pilot --client opencode           # én gangs override
nav-pilot config set client opencode  # sett permanent
```

`nav-pilot status` og `nav-pilot list --installed` viser opencode-artefaktene og om de er oppdaterte.

#### `export opencode` vs. automatisk materialisering

| Kommando | Mål | Tilstandssporing | Når |
|---|---|---|---|
| `nav-pilot --client opencode` (oppstart) | `~/.config/opencode/` | ✅ konflikt + ferskhet | Personlig kontekst — skjer automatisk |
| `nav-pilot sync` | `~/.config/opencode/` | ✅ oppdaterer sporet tilstand | Frisk opp personlig kontekst |
| `nav-pilot export opencode` (repo-scope) | `<repo>/.opencode/` | — | Sjekk Nav-kontekst inn i et **prosjektrepo** for hele teamet |

For ditt **personlige** oppsett trenger du ikke `export` i det hele tatt — bare kjør
`nav-pilot --client opencode` (materialiserer automatisk) eller `nav-pilot sync` for å friske opp.
Bruk `nav-pilot export opencode` (repo-scope) kun for å versjonskontrollere Nav-konteksten i et prosjektrepo.

> **Avviklet:** `nav-pilot export opencode --user` er erstattet av den automatiske
> materialiseringen ved oppstart + `nav-pilot sync`, som i tillegg gir tilstandssporing
> og konflikt-deteksjon. Repo-scope `export opencode` består.

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

## Telemetry (pilot, default on)

nav-pilot sender OTel-metrikker som standard i pilot.

Standard endpoint er `https://collector-internet.nav.cloud.nais.io/v1/metrics`.
Du kan overstyre med `NAV_PILOT_TELEMETRY_ENDPOINT` ved behov.
Når nav-pilot starter `cplt`/`copilot`, settes `OTEL_EXPORTER_OTLP_ENDPOINT` for Copilot CLI
til samme collector-base (`https://collector-internet.nav.cloud.nais.io`, uten `/v1/metrics`)
slik at Copilot kan sende både metrics og traces. Overstyr med
`NAV_PILOT_COPILOT_OTEL_ENDPOINT` ved behov (den prioriteres over generell
`OTEL_EXPORTER_OTLP_ENDPOINT`). nav-pilot setter også `COPILOT_OTEL_ENABLED=true`
hvis den ikke allerede er satt. nav-pilot injiserer i tillegg resource-attributtene
`nav.pilot.launcher`, `nav.pilot.version` og `nav.pilot.device_id` i Copilots
`OTEL_RESOURCE_ATTRIBUTES` (append-merge, eksisterende nøkler beholdes) for
sporing av Copilot-traces tilbake til nav-pilot.

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

Metrikkene inkluderer også `execution_context` for å skille organisk bruk fra CI
(`organic`, `ci_github_actions`, `ci_other`, `unknown`).

`NAV_PILOT_TELEMETRY_ENABLED=0` (eller `off`) deaktiverer telemetry.

## Konfigurasjon

Du kan lagre standardvalg i `~/.nav-pilot/config.toml`.

```bash
nav-pilot config init
nav-pilot config setup
nav-pilot config show
```

Støttede felt er `client`, `model`, `mode`, `reasoning_effort`, `context_tier`,
`allow_all_tools`, `ask_user`, `auto_launch` og `log_level`. Du kan overstyre dem per kjøring med
globale flagg som `--client`, `--model`, `--mode`, `--effort`, `--context`,
`--allow-all-tools`, `--no-ask-user`, `--auto-launch`/`--no-auto-launch` og `--log-level`.

> **Tips:** Sett `auto_launch = true` (eller bruk `--auto-launch`) for å starte
> cplt/copilot/opencode automatisk uten «Launch X now?»-bekreftelsen.

**Modell per klient:**
- Copilot: `auto`, `claude-sonnet-4.6`, `claude-haiku-4.5`, `claude-opus-4.8`,
  `gpt-5.5`, `gpt-5.4`, `gpt-5.3-codex`, `gpt-5.4-mini`, `gemini-3.1-pro-preview`
- opencode (startes via cplt → GitHub Copilot-provider): bruk `github-copilot/<id>`,
  f.eks. `github-copilot/claude-sonnet-4.5` (Nav-standard), `github-copilot/claude-opus-4.8`,
  `github-copilot/gpt-5.5`. Modellen må være på `provider/model`-format (med `/`);
  `auto` eller tom verdi faller tilbake til Nav-standarden `github-copilot/claude-sonnet-4.5`.

Veiviseren (`nav-pilot config setup`) viser en modellvelger tilpasset valgt klient.
`nav-pilot config explain model` lister opp de kurerte id-ene.

**opencode-mapping:**
`client = "opencode"` mappes til opencode-flagg:
`mode = plan` → `--agent plan` (ellers `--agent nav-pilot`), `model` → `--model`
(prefikses med `github-copilot/` for bare id-er), `reasoning_effort` → `--variant`,
`allow_all_tools` → `--dangerously-skip-permissions`, `log_level` oversettes
til opencodes sett (`DEBUG`/`INFO`/`WARN`/`ERROR`). Felt uten opencode-ekvivalent
(`mode = autopilot` (verdi av `mode`-feltet), `context_tier`, `ask_user = false`) gir en ⚠-advarsel ved oppstart.

## For bidragsytere

- Agent: `.github/agents/nav-pilot.agent.md`
- Design: `docs/nav-pilot-design.md`
- Skills: `.github/skills/<name>/`
- Instruksjoner: `.github/instructions/`

Detaljert bruk, CLI-referanse og arbeidsflyt vedlikeholdes i online docs.
