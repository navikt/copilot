# nav-pilot design

nav-pilot is both a CLI tool and an AI agent. The CLI installs agents, skills, and instructions into a repository. The agent (`@nav-pilot`) uses that knowledge in Copilot Chat.

Designmønstre og konvensjoner i nav-pilot CLI. Nye kommandoer og endringer skal følge disse.

## Arkitektur

nav-pilot er én Go-pakke (`package main`) uten interne moduler. Koden er delt i filer etter ansvarsområde, ikke etter lag.

```
main.go          CLI-parsing, dispatch til cmd*-funksjoner
install.go       install, install --auto-detect, list, status, uninstall
init.go          scaffold repo-lokale Copilot-konfigurasjonsfiler
add.go           add (enkeltartifakt — deprecated alias for install)
export.go        export (formatkonvertering)
sync.go          sync (oppdateringssjekk)
interactive.go   TUI-flyt med charmbracelet/huh
update.go        upgrade / update (selvoppdatering av binæren)
feedback.go      åpner GitHub issue med diagnostikk
env.go           shell-eksport for Copilot CLI-integrasjon
scope.go         InstallScope (repo vs. user)
source.go        Source (lokal repo eller git clone)
state.go         StateFile (JSON-basert installasjonstilstand)
files.go         filkopiering, hashing, symlink-sjekk
manifest.go      manifest.json-parsing og validering
frontmatter.go   YAML-frontmatter splitting og transformasjon
output.go        fargefunksjoner (red, green, yellow, dim, bold)
suggest.go       Levenshtein-avstand og did-you-mean-forslag
staleness.go     bakgrunnssjekk av ny versjon
syncconfig.go    copilot-sync.json per repo
version.go       versjonsstrengslogikk
```

## Config-system

User-specific config lives in `~/.nav-pilot/config.toml` (override via `NAV_PILOT_CONFIG`).
See `config.go`, `config_cmd.go`, `config_setup.go`.

Files:
- `config.go` — `Config` struct, `readConfig()`, `validateConfig()`, `loadConfigForLaunch()` (validate + warn/refuse at launch), `resolve()`, `CLIOverrides`, `ResolvedConfig`
- `config_cmd.go` — `nav-pilot config` subcommands: `init`, `setup`, `show`, `path`, `get`, `set`, `validate`, `explain`
- `config_setup.go` — Interactive first-run wizard (`maybeRunFirstRunSetup`, `runConfigSetup`, `writeSetupConfig`)

### Config fields

| TOML key | Type | Default | CLI flag |
|---|---|---|---|
| `version` | int | (required) | — |
| `client` | string | `copilot` | `--client` |
| `model` | string | unset | `--model` |
| `mode` | string | `default` | `--mode` |
| `reasoning_effort` | string | unset | `--effort` |
| `context_tier` | string | unset | `--context` |
| `allow_all_tools` | bool | `false` | `--allow-all-tools` |
| `ask_user` | bool | `true` | `--no-ask-user` |
| `log_level` | string | unset | `--log-level` |
| `otel_log_level` | string | `none` | `--otel-log-level` (sets `OTEL_LOG_LEVEL`) |

### Per-run CLI override flags

All launch-override flags are global and processed BEFORE command dispatch. They apply only to the interactive flow and `--sync` launch:

```bash
nav-pilot --client opencode         # Use opencode for this run
nav-pilot --model gpt-4o            # Override model
nav-pilot --mode plan               # Run in plan mode
nav-pilot --effort high             # Set reasoning effort
nav-pilot --context long_context    # Use extended context
nav-pilot --allow-all-tools         # Allow all tools
nav-pilot --no-ask-user             # Non-interactive mode
nav-pilot --log-level debug         # Set log level
```

### Modellvalg og validering

`model` formatvalideres lokalt, ikke mot en tillatt liste: Copilot CLI validerer
`--model` server-side, og opencode aksepterer åpne `provider/model`-strenger.
`validateModelValue` krever en ikke-tom id uten mellomrom, som matcher
`^[A-Za-z0-9][A-Za-z0-9._/-]*$` — dekker Copilot-ids (`claude-opus-4.8`,
`gpt-5.5`) og opencode-ids (`anthropic/claude-sonnet-4-5`).

**Per-klient-validering:** `openCodeProvider.ValidateModel` krever i tillegg at id-en
er på `provider/model`-format (nøyaktig én `/`, ikke-tom på begge sider). En bare
Copilot-id som `claude-opus-4.8` gir hard feil for opencode-provideren.

Veiviseren viser en **velger** med Nav-kurerte modeller per provider
(`KnownModels()` fra `Provider`-grensesnittet):
- Copilot: `knownCopilotModels` — inkluderer `auto`, Claude Sonnet/Haiku/Opus, GPT-5.x, Gemini
- opencode: `knownOpenCodeModels` — Nav-anbefalt `anthropic/claude-sonnet-4-5` som standard

En "Custom…"-mulighet i velgeren lar brukeren skrive inn valgfri id med validering.
`nav-pilot config explain model` lister opp de kjente id-ene per provider.

### Validation on launch

`loadConfigForLaunch` reads, validates, and resolves the config before every
launch (both the interactive flow and `--sync`):

- **Refuses to start** on hard-invalid config — an unknown `version`, an invalid
  enum value (`client`, `mode`, `reasoning_effort`, `context_tier`, `log_level`),
  or a malformed `model` identifier. It prints every problem via `validateConfig`
  and points the user at `nav-pilot config setup`.
- **Warns (non-fatal)** on advisory issues via `configAdvisories`, printed to
  stderr without blocking the launch:
  - unknown TOML keys (likely typos the parser silently ignores)
  - Model-ids som er velformet men ikke i den Nav-kurerte listen for den valgte
    klienten (f.eks. `model = "sonnet"` for Copilot → foreslår `claude-sonnet-4.6`).
    Blokkerer ikke oppstart fordi modeller valideres av den underliggende klienten.

The standalone `nav-pilot config validate` command performs the same checks
(including unknown-key detection) on demand without launching.

### Provider-abstraksjon (Provider interface)

`launchProvider(resolved ResolvedConfig)` delegerer til `providerFor(resolved.Client).Launch(resolved)`.
Alle provider-spesifikke valg er samlet i `provider.go`:

```go
type Provider interface {
    ID() string                                       // "copilot" | "opencode" | "pi"
    DisplayName() string                              // brukervendt navn
    Available() bool                                  // PATH-sjekk
    Launch(resolved ResolvedConfig) error             // start klienten
    DefaultModel() string                             // Nav-standard (tom = klient velger selv)
    KnownModels() []ModelChoice                       // kurert modell-liste for veiviseren
    ValidateModel(model string) error                 // provider-spesifikk validering
    ModelAdvisory(model string) string                // advarsel for ukurerte men gyldige ids
    UnsupportedConfigWarnings(ResolvedConfig) []string // advarsel for felt uten ekvivalent
}
```

`providerRegistry` holder de tre implementasjonene i rekkefølge:
1. `copilotProvider` — starter via `cplt`/`copilot` CLI
2. `openCodeProvider` — starter via `opencode run` i `opencode_launch.go`
3. `piProvider` — returnerer "not supported"-feil (stub)

`ValidProviderIDs` er avledet fra registeret — ingen separat hardkodet liste.
Å legge til en fjerde provider krever én struct + ett registre-element, ingen
if/else-grening i andre filer.

nav-pilot bruker et lite, kontrollert sett av klienter (`copilot`, `opencode`,
`pi`). `cplt`-sandkassen støtter flere (`gemini`, `antigravity`, `shell`), men
nav-pilot eksponerer bare klienter med bekreftet launch-sti og Nav-kontekst.

> **Bakoverkompatibilitet:** Eksisterende konfig-filer med `agent = "..."` godtas
> fortsatt og kartlegges til `client`-feltet. En advarsel skrives ut ved oppstart;
> bytt til `client` for å fjerne den.

**cplt agent-pinning:** `buildCopilotArgs` setter alltid `cplt --agent copilot`
slik at en annen agent på PATH ikke plukkes opp, og videreformidler `nav-pilot`-
persona + flagg etter `--`-separatoren: `cplt --agent copilot -- --agent nav-pilot …`.

### OpenCode alternativ-mapping

`openCodeArgs` mapper resolvert konfig til `opencode run`-flagg. opencode sitt
flagg-grensesnitt er annerledes enn Copilots, så flere felt oversettes eller dropper:

| nav-pilot konfig | opencode-flagg | Merknad |
|---|---|---|
| `model` | `--model` | Krever `provider/model` (f.eks. `anthropic/claude-sonnet-4-5`); Nav-standard er `anthropic/claude-sonnet-4-5` når unset |
| `mode = plan` | `--agent plan` | opencode har ingen `--mode`; `autopilot` har ingen opencode-ekvivalent — advarsel ved oppstart |
| `reasoning_effort` | `--variant` | Leverandørspesifikk resonering (f.eks. `high`, `max`) |
| `allow_all_tools` | `--dangerously-skip-permissions` | |
| `log_level` | `--log-level` | Oversettes til opencodes sett: `DEBUG`/`INFO`/`WARN`/`ERROR` (se under) |
| `context_tier` | — | Ingen opencode-ekvivalent — advarsel hvis eksplisitt satt |
| `ask_user` | — | Ingen opencode-ekvivalent — advarsel hvis eksplisitt satt til `false` |

**Advarsler for umappede felt** (`openCodeUnsupportedConfigWarnings`):
Nav-pilot skriver en gul ⚠-advarsel til stderr for felt som er eksplisitt satt
men ikke støttes av opencode. Bare advarsel — oppstarten stopper ikke.

Log-level-oversettelse (`openCodeLogLevel`): `debug`/`all` → `DEBUG`, `info` →
`INFO`, `warning` → `WARN`, `error` → `ERROR`; `none`/`default`/unset utelater
flagget (opencode bruker sin egen standard). opencode aksepterer bare store bokstaver.

### OpenCode OTel

Når et OTel-endepunkt er konfigurert (via `OTEL_EXPORTER_OTLP_ENDPOINT` eller `NAV_PILOT_COPILOT_OTEL_ENDPOINT`), gjør nav-pilot:
1. Kaller `ensureOpenCodeOTelConfig()` for å sette `experimental.openTelemetry = true` i `~/.config/opencode/opencode.json`
2. Injiserer `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_RESOURCE_ATTRIBUTES` og `OPENCODE_CLIENT=nav-pilot` i opencodes miljø

OTel-konfigurasjonssammenslåingen er dyp (beholder eksisterende `experimental.*`-nøkler) og idempotent.

### OpenCode livssyklus — Nav-kontekst

opencode er en fullverdig klient med automatisk Nav-kontekst. Livssyklusen er:

**Materialisering (P1):**  
`ensureOpenCodeNavContext()` (kalt fra `launchOpenCode`) løser opp kildeartifaktene
og skriver dem til `~/.config/opencode/` (bruker-globalt, ikke repo-spesifikt):

| Mål | Innhold |
|---|---|
| `~/.config/opencode/AGENTS.md` | Sammenstilt instruksjonsfil fra alle Nav-instruksjoner |
| `~/.config/opencode/skills/` | Nav-skills (katalogstruktur med `SKILL.md`) |
| `~/.config/opencode/commands/` | Nav-prompts konvertert til opencode commands |
| `~/.config/opencode/agents/` | Nav-agenter |

Valget om bruker-globalt scope (ikke repo-lokalt `.opencode/`) er bevisst:
opencode leser fra sin globale config-katalog, og Nav-konteksten skal virke
på tvers av alle repoer uten at utvikleren må kjøre `export` manuelt.

**Tilstandshåndtering (P2):**  
`syncOpenCodeArtifacts()` er den tilstand-bevisste varianten av materialisering.
Den skriver en tilstandsfil `~/.config/opencode/.nav-pilot-state.json` (samme
`StateFile`-struktur som `.nav-pilot-state.json` for `.github/`) med:
- Kilde-ref, versjon og SHA
- Per-fil-hash av alle forvaltede filer

**Konfliktdeteksjon:** Hvis en bruker har endret en nav-pilot-forvaltet fil lokalt
(hash-avvik mot lagret tilstand), overskrives den ikke. I stedet rapporteres
konflikten med ⚠-advarsel til stderr — speilet med `.github/`-semantikken.

**Ferskhetssjekk og versjonsskjevhet:**  
Sync sammenligner lagret kilde-SHA med gjeldende release og rapporterer
`nav_pilot_staleness_check_total` og `nav_pilot_version_skew_days` for
opencode-artefakter på samme måte som for `.github/`-artefakter.

**List og status:**  
`nav-pilot list` og `nav-pilot status` viser opencode-artefakter (via
`printOpenCodeStatusBlock`) når en forvaltet `~/.config/opencode/.nav-pilot-state.json`
finnes — inkludert ferskhetsindikator og eventuelle konflikter.

## Avhengigheter

Kun `charmbracelet/huh` (TUI-prompts) som direkte avhengighet. Alt annet er standardbiblioteket. Hold det slik — ikke legg til nye avhengigheter uten god grunn.

Ingen YAML-bibliotek. Frontmatter parses linjebasert. Ingen HTTP-rammeverk. Ingen DI-rammeverk.

## Kommandomønster

Hver kommando er en `cmd*`-funksjon som tar parsed argumenter og returnerer `error`. `run()` i main.go parser flagg og dispatcher.

```go
func cmdExport(format string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error
func cmdInstallAuto(name, itemType string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error
func cmdInstall(collection string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error
func cmdSync(scope *InstallScope, ref, sourceRepo string, apply, jsonOutput bool) error
func cmdAdd(itemType, name string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error  // deprecated alias
func cmdInit(targetDir string, dryRun, force bool) error
```

Nye kommandoer følger dette mønsteret:

1. Ta alle nødvendige parametre som funksjonsargumenter (ikke les `os.Args` direkte)
2. Returner `error` — la `main()` håndtere exit-koder
3. Legg til en `case` i `switch command` i `run()`
4. Oppdater `usage()` med ny kommando
5. Legg til i `--user`-allowlisten om kommandoen støtter user scope

## Scope

`InstallScope` kapsler forskjellen mellom repo-installasjon (`.github/`) og brukerinstallasjon (`~/.copilot/`). Bruk scope-metoder for å bygge stier:

```go
scope.DstPath("agents", "nav-pilot.agent.md")  // full målsti
scope.RelPath("agents", "nav-pilot.agent.md")   // relativ sti for state
scope.SupportsType("prompt")                     // false for user scope
scope.IsUser()                                   // true for --user
scope.Label()                                    // "~/.copilot (user-wide)"
```

Nye kommandoer som skriver filer skal bruke scope-metodene — ikke bygg stier manuelt med `filepath.Join(rootDir, ".github", ...)`.

## Artifact Resolution

Alle artefakttyper (skills, agents, instructions, prompts) kan ligge på to steder i kilderepoet (navikt/copilot):

| Plassering | Formål | Auto-discovery |
|---|---|---|
| `<type>/<name>` | Ny root-plassering (awesome-copilot-konvensjon) | ✅ Ja |
| `.github/<type>/<name>` | Legacy-plassering | ❌ Nei |

**Root vinner når den finnes.** For skills valideres at `SKILL.md` finnes. For andre typer sjekkes fileksistens direkte.

### SourceResolver (resolver.go)

All artifact resolution is centralized in `SourceResolver`. Never build source paths manually.

```go
resolver := NewSourceResolver(sourceDir)
```

**Types:**
```go
// ArtifactKind describes the shape and naming of one artifact type.
var KindAgent       = &ArtifactKind{Name: "agent",       Dir: "agents",       Suffix: ".agent.md",        Sidecars: []string{".metadata.json"}}
var KindSkill       = &ArtifactKind{Name: "skill",       Dir: "skills",       IsDir: true, Marker: "SKILL.md"}
var KindInstruction = &ArtifactKind{Name: "instruction",  Dir: "instructions", Suffix: ".instructions.md"}
var KindPrompt      = &ArtifactKind{Name: "prompt",       Dir: "prompts",      Suffix: ".prompt.md", CanBeDir: true}

// Resolved holds the result of resolving a single artifact.
type Resolved struct {
    Kind    *ArtifactKind
    Name    string   // e.g. "nais" or "api-design"
    AbsPath string   // full filesystem path
    RelPath string   // relative to source root
    IsDir   bool     // actual shape on disk
}
```

**Methods:**
```go
resolver.Get(kind, name)                          // → (Resolved, bool) — resolve one artifact by name
resolver.GetFile(typeDir, fileName)               // → (absPath, relPath, bool) — resolve a specific file (sidecars, sync)
resolver.List(kind)                               // → []Resolved — discover all artifacts, sorted, deduped
resolver.MapLocalPath(localPath, isUserScope)     // → string — map installed path back to source path
```

**Generic install:**
```go
installArtifact(resolver, scope, kind, name, dryRun, force, result)  // replaces 5 old install functions
```

### Oppløsningsrekkefølge

**Skills:**
```
1. Sjekk skills/<name>/SKILL.md finnes?     → bruk skills/<name>/
2. Sjekk .github/skills/<name>/SKILL.md?    → bruk .github/skills/<name>/
3. Ingen funnet                              → skill finnes ikke
```

**Agents/Instructions:**
```
1. Sjekk <type>/<fileName> finnes?           → bruk <type>/<fileName>
2. Sjekk .github/<type>/<fileName> finnes?   → bruk .github/<type>/<fileName>
3. Ingen funnet                              → artefakt finnes ikke
```

**Prompts (strengere presedenslogikk):**
```
1. Sjekk prompts/<name>/ er en mappe?        → bruk prompts/<name>/
2. Sjekk prompts/<name>.prompt.md finnes?     → bruk prompts/<name>.prompt.md
3. Sjekk .github/prompts/<name>/ er mappe?    → bruk .github/prompts/<name>/
4. Sjekk .github/prompts/<name>.prompt.md?    → bruk .github/prompts/<name>.prompt.md
5. Ingen funnet                               → prompt finnes ikke
```

### Hvem bruker hva

| Funksjon | Resolver-metode | Fil |
|---|---|---|
| `installArtifact()` | `Get`, `GetFile` (sidecars) | install.go |
| `listAvailableItems()` | `List` (all kinds) | install.go |
| `collectAvailableItems()` | `List` (all kinds) | install.go |
| `collectAllItems()` | `List` (agents, skills, instructions) | manifest.go |
| `exportSkills()` | `List(KindSkill)` | export.go |
| `exportAgents()` | `List(KindAgent)` | export.go |
| `exportInstructions()` | `List(KindInstruction)` | export.go |
| `exportPrompts()` | `List(KindPrompt)` | export.go |
| `autoDetectSyncFiles()` | `GetFile`, `Get` | sync.go |
| `resolveSyncFiles()` | `MapLocalPath` | sync.go |

### Mål- vs. kildestier

**Kilde** (navikt/copilot): `<type>/` eller `.github/<type>/` — oppløses av `SourceResolver`.

**Mål** (brukerens repo): Alltid `.github/<type>/` (repo scope) eller `~/.copilot/<type>/` (user scope). Målstier endres **aldri**.

### Unntak

- `copilot-instructions.md` ligger alltid i `.github/` — det er en operasjonell fil, ikke et distribuerbart artefakt.

## Source

`Source` løser opp kildekodemappa. Prioritet:

1. Eksplisitt `--ref` → `git clone --depth 1 --branch <ref>`
2. Lokal repo (CWD er inne i navikt/copilot) → dev-modus, ingen clone
3. Release-tag som matcher binærens versjon → `git clone --branch nav-pilot/<version>`
4. HEAD (kun for `version=dev`) → `git clone`

Alle kommandoer som leser fra kilden følger dette mønsteret:

```go
src, err := resolveSource(ref, sourceRepo)
if err != nil {
    return err
}
defer src.Cleanup()  // fjerner temp-dir
```

`Cleanup()` er viktig. Temp-dirs lekker ellers.

## Flagg

Alle flagg parses manuelt i `run()`. Ingen flag-bibliotek.

| Flagg | Kort | Verdi | Støttede kommandoer |
|---|---|---|---|
| `--dry-run` | `-n` | nei | install, add, export, uninstall |
| `--force` | `-f` | nei | install, add, export |
| `--target` | `-t` | dir | install, add, export, sync |
| `--ref` | `-r` | ref | install, add, export, sync, list |
| `--source` | `-s` | repo | install, add, export, sync, list |
| `--user` | `-u` | nei | install, add, sync, status, uninstall, export |
| `--apply` | | nei | sync |
| `--json` | | nei | sync, install, add, status, export, list |
| `--items` | | nei | list |
| `--feature` | `-F` | nei | feedback |

Nye flagg: legg til i for-løkka i `run()`, med `--long` og `-short` form. Gjenbruk eksisterende flagg der det gir mening.

`--user` og `--target` er gjensidig utelukkende — `run()` sjekker dette.

## Sikkerhetsregler

### Symlinkbeskyttelse

Alle filskrivinger sjekker symlinker i stikjeden opp til en grense (boundary):

```go
copyFile(src, dst, scope.RootDir)   // sjekker dst-sti
copyDir(src, dst, scope.RootDir)    // sjekker dst-sti
writeStateAt(path, boundary, state) // sjekker state-sti
```

`checkSymlink()` stopper ved boundary for å unngå falske positiver fra system-symlinker (f.eks. `/var → /private/var` på macOS).

`copyDirSimple()` i export.go sjekker kilde-symlinker i stedet (avviser dem).

### Stivalidering

State-filer validerer alle stier ved lesing:

```go
scope.ValidateStatePath(f.Path)  // avviser "..", absolutte stier, stier utenfor scope
```

### Navnevalidering

Alle brukeroppgitte navn valideres med `validateName()`:

```go
validateName(name)  // avviser "", "..", "/", "\", urene stier
```

### Atomiske skrivinger

Filer skrives via temp-fil + rename:

```go
tmp, err := os.CreateTemp(filepath.Dir(dst), ".nav-pilot-*")
// skriv til tmp
os.Rename(tmpPath, dst)
```

## Fil-IO

### Kopiering

To kopieringsfunksjoner for to ulike brukstilfeller:

- `copyFile()` / `copyDir()` — for installasjon. Sjekker mål-symlinker, bruker atomisk skriving, tar `boundary`-parameter
- `copyDirSimple()` — for eksport. Sjekker kilde-symlinker, enklere (ingen boundary)

### Hashing

- `fileHash()` — SHA-256, forkorta til 16 hex-tegn
- `normalizedFileHash()` — normaliserer markdown (CRLF→LF, trailing whitespace, doble blanklinjer) før hashing
- `dirHash()` — hasher hele kataloger rekursivt, med markdown-normalisering

Hashing brukes til:
- Konfliktdeteksjon ved installasjon (`checkConflict()`)
- Integritetskontroll i `status`-kommandoen
- Synkroniseringssjekk i `sync`

## Frontmatter

Frontmatter-parseren i `frontmatter.go` er linjebasert — ingen YAML-avhengighet.

Viktige funksjoner:

```go
fm, body, hasFM := splitFrontmatter(data)      // del opp fil i frontmatter + body
stripped := stripFrontmatterKeys(fm, keys)       // fjern nøkler (inkl. nøstede barn)
val, ok := extractFrontmatterValue(fm, key)      // les én verdi
newFM := buildAgentFrontmatter(desc)             // bygg ny frontmatter
result := reassemble(fm, body)                   // sett sammen igjen
```

- Normaliserer CRLF→LF før parsing
- Tillater trailing whitespace på `---`-delimiter
- `stripFrontmatterKeys` kjenner igjen nøstede YAML-barn (innrykk) og fjerner dem med forelderen
- `yamlQuoteIfNeeded` siterer verdier med `:`, `#` og andre YAML-spesialtegn

## Tilstand (state)

`StateFile` (JSON) sporer hva som er installert:

```json
{
  "collection": "fullstack",
  "version": "2026.04.14-202800-a25f6c3",
  "scope": "repo",
  "source_sha": "a25f6c3",
  "installed_at": "2026-04-14T20:28:00Z",
  "files": [
    {"path": ".github/agents/nav-pilot.agent.md", "hash": "abc123..."}
  ]
}
```

State leses alltid gjennom `readScopedState()` som validerer scope-match og sti-sikkerhet. Skrives gjennom `writeScopedState()` som bruker atomisk skriving med symlink-sjekk.

## Output

### Farger

Bruk hjelpefunksjonene i `output.go`:

```go
green("✓")           // suksess
yellow("⚠")          // advarsel
red("Error:")         // feil
dim("→")              // sekundær info
bold("nav-pilot")     // uthevet
```

Respekterer `NO_COLOR`-miljøvariabelen automatisk.

### Konsollmønstre

Kommandoer som endrer filer bruker konsistente mønstre:

```go
// Dry run
fmt.Printf("  %s %s\n", dim("→"), relPath)

// Installert
fmt.Printf("  %s %s\n", green("✓"), name)

// Advarsel
fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)

// Ferdig-melding
fmt.Printf("%s Installed %d items.\n", green("✓"), count)
```

Skriv informasjonsmeldinger til stdout, feilmeldinger til stderr.

### Exit-koder

Definert som konstanter i `main.go`:

```go
const (
    ExitSuccess          = 0  // alt gikk bra
    ExitError            = 1  // generell feil
    ExitUpdatesAvailable = 1  // sync: oppdateringer tilgjengelig (stille, ingen feilmelding)
    ExitSyncFailed       = 2  // sync: sjekk feilet
)
```

`main()` mapper `error`-verdier til exit-koder. Sentinel-feil (`errUpdatesAvailable`, `errSyncFailed`) har egne exit-koder. Alle andre feil gir `ExitError` (1).

### JSON-output

`--json` støttes på alle kommandoer som produserer strukturerte resultater: sync, install, add, status, export, list.

Mønster:

1. Gate all menneskelesbar output bak `if !jsonOutput { ... }`
2. Samle resultater i en struct/map
3. Kall `outputJSON(result)` helt til slutt — denne skriver prettified JSON til stdout

```go
if jsonOutput {
    return outputJSON(map[string]interface{}{
        "command": "add",
        "type":    itemType,
        "name":    name,
        // ...
    })
}
```

`outputJSON()` er definert i `sync.go` og bruker `json.NewEncoder(os.Stdout)` med innrykk.

### Feilhint (did-you-mean)

Ukjente kommandoer og flagg inkluderer forslag basert på Levenshtein-avstand:

```
unknown command: "statu". Did you mean "status"?
unknown flag: --taget. Did you mean --target?
```

Implementert i `suggest.go`. Terskel: avstand ≤ 2. Returnerer "" om ingen match er nær nok.

### Fremdriftsindikator

`cloneRemote()` i `source.go` skriver en statusmelding til stderr før git clone:

```
→ Fetching navikt/copilot@main...
```

Stderroutput er viktig for CI: det vises i terminalen men forstyrrer ikke stdout-piping.

## Testbarhet

### Testbar arkitektur

`run()` tar `args []string` og returnerer `error` — alt kan testes uten å spawne prosesser:

```go
func TestRun_UnknownCommand(t *testing.T) {
    err := run([]string{"bogus"})
    // assert
}
```

### Overridbare variabler

Globale variabler som gjør funksjoner testbare:

```go
var timeNow = time.Now              // overstyr tid i tester
var forceNonInteractive bool        // forhindrer TUI-blokkering i tester
var openBrowserFn = openBrowser     // unngå å åpne nettleser i tester
var httpClient = &http.Client{...}  // mock HTTP i tester
var cacheHome = ""                  // overstyr cache-sti i tester
```

### Testmønstre

- Table-driven tester for funksjoner med mange tilfeller
- `t.TempDir()` for isolerte filsystem-tester (rydder opp automatisk)
- `setupTestSource(t)` oppretter et midlertidig `.github/`-tre for integrasjonstester
- Hjelpefunksjoner `mustMkdir(t, dir)` og `mustWrite(t, path, content)` for test-setup
- Teste eksportfunksjoner direkte (f.eks. `exportSkills()`) — ikke bare gjennom `run()`
- Sjekk at `--dry-run` ikke skriver noe til disk

### Testkonvensjoner

```go
// filnavn: <modul>_test.go — testfiler ved sida av koden
// testandre: TestXxx / TestXxx_EdgeCase / subtests med t.Run()
// assertions: if/t.Errorf, strings.Contains — ingen ekstra testbibliotek
```

## Init (scaffolding av repo-lokale filer)

`nav-pilot init` oppretter tre repo-lokale filer som Copilot bruker for prosjektspesifikk kontekst:

| Fil | Formål |
|---|---|
| `AGENTS.md` | Prosjektbeskrivelse for kodingsagenter (build-kommandoer, struktur, boundaries) |
| `.github/copilot-instructions.md` | Copilot Chat-instruksjoner (tech stack, nøkkelmønstre) |
| `.github/copilot-review-instructions.md` | Copilot Code Review-instruksjoner (maks 4000 tegn) |

### Stackdeteksjon

`detectStack()` sjekker target-mappen for:

| Signal | Stack |
|---|---|
| `go.mod` | Go |
| `package.json` | Node.js/TypeScript |
| `build.gradle.kts` / `build.gradle` / `pom.xml` | Kotlin |
| `.nais/` | Nais-deployment |

Detektert stack styrer hvilke maler og kommandoer som brukes i filene.

### Templater

Templatene er string-building (ingen template-bibliotek, i tråd med DESIGN-filosofien). Innholdet er:

- **Lean**: Bare prosjektspesifikk kontekst, ikke Nav-brede konvensjoner (de kommer fra installerte instruksjoner)
- **TODO-markører**: `<!-- TODO: ... -->` der teamet må fylle inn
- **Automatisk**: Build-kommandoer og nøkkelkataloger detekteres fra stacken

### Post-install hint

`hintInitIfMissing()` kalles etter `install --user`. Sjekker om cwd er et git-repo som mangler noen av de tre filene, og foreslår `nav-pilot init` i så fall.

### Flagg

- `--dry-run`: Vis hva som ville blitt opprettet, skriv ingenting
- `--force`: Overskriv eksisterende filer
- `--target <dir>`: Målkatalog (standard: `.`)

---

## Legg til ny kommando (sjekkliste)

1. Opprett `<kommando>.go` med `cmd<Kommando>(...) error`
2. Følg signaturmønsteret: scope, ref, sourceRepo, dryRun, force som parametre
3. Bruk `resolveSource()` + `defer src.Cleanup()` for kildetilgang
4. Bruk scope-metoder for alle stier
5. Sjekk symlinker ved filskriving (`copyFile`/`copyDir` med boundary, eller `copyDirSimple` med kilde-sjekk)
6. Valider brukerinput med `validateName()` for filnavn
7. Legg til `case` i `run()` switch
8. Oppdater `usage()` med kommando, flagg og eksempel
9. Legg til i `--user`-allowlist hvis relevant
10. Opprett `<kommando>_test.go` — test alle transformfunksjoner og edge cases
11. Støtt `--dry-run` hvis kommandoen skriver filer (skriv ingenting, vis hva som ville skjedd)
12. Støtt `--force` hvis kommandoen kan overskrive eksisterende filer

---

## Kjente begrensninger

Dokumenterte designbegrensninger som kan endres i fremtidige versjoner.

### Ingen passthrough av argumenter til Copilot CLI

Når nav-pilot starter Copilot CLI interaktivt (etter install/sync-flyt), er det ingen måte å sende ekstra argumenter til `copilot`/`cplt`. Launchen er hardkodet til kun å videresende `--agent nav-pilot`:

```go
// interactive.go — launchCopilotWithAgent()
args := []string{}
if agent != "" {
    args = append(args, "--", "--agent", agent)  // cplt
    // eller: args = append(args, "--agent", agent)  // copilot
}
cmd := exec.Command(cliPath, args...)
```

Brukere som vil sende andre flagg (f.eks. `--model`, egne prompts, eller en annen agent) må kjøre `copilot`/`cplt` direkte etter at nav-pilot har satt opp miljøet.

**Status:** OTel-metrics er aktivert som standard og kan deaktiveres med `NAV_PILOT_TELEMETRY_ENABLED=0`. OTLP-endepunkt kan overstyres ved behov.

---



Analyse av nav-pilot mot populære CLI-verktøy og etablerte retningslinjer:
[clig.dev](https://clig.dev/), 12 Factor CLI Apps, og Go-verktøy som
gh (GitHub CLI), age (FiloSottile), gum (Charmbracelet), gitleaks og Hugo.

### Hva vi gjør bra

| Område | Vurdering | Forklaring |
|---|---|---|
| Minimale avhengigheter | ✅ Beste praksis | 1 direkte avhengighet (huh) — gh har 30+, gum 15. Lav forsyningskjederisiko |
| Sikkerhet (filer) | ✅ Beste i klassen | Symlink-sjekk, atomiske skrivinger, stivalidering — bedre enn gh og de fleste andre |
| Testbar arkitektur | ✅ Eksemplarisk | `run()` returnerer error, overridbare globale variabler, `t.TempDir()` |
| NO_COLOR-støtte | ✅ clig.dev-standard | Respekterer `NO_COLOR`-miljøvariabelen |
| Feilmeldinger til stderr | ✅ UNIX-konvensjon | `Error:`-prefix til stderr, resultater til stdout |
| Flag-konvensjoner | ✅ Gjenkjennelig | `-n`, `-f`, `-u`, `-r` følger etablerte navn (clig.dev anbefaler disse) |
| Dry-run-mønster | ✅ clig.dev-anbefalt | `--dry-run` er en av de mest anbefalte flaggene |
| Scope-abstraksjon | ✅ Gjennomtenkt | `InstallScope` er en ren abstraksjon — uvanlig godt for et lite verktøy |
| Hjelptekst | ✅ God struktur | Viser kommandoer, flagg og eksempler — følger clig.dev "lead with examples" |
| Flat pakkestruktur | ✅ Riktig for størrelse | age, gum og andre småverktøy gjør det samme — nested pakker er for store verktøy |
| Interaktiv modus | ✅ Moderne | Faller tilbake til TUI ved ingen argumenter (som `npm init`) — clig.dev-kompatibelt |

### Sammenligningstabell

| Prinsipp (clig.dev) | nav-pilot | Karakter |
|---|---|---|
| Robusthet | Symlink-sjekk, atomiske skrivinger, crash-safe | A+ |
| Empati | Klare feilmeldinger med did-you-mean-hint | A |
| Komposerbarhet (pipes, CI) | Fungerer uten TTY, NO_COLOR, stderr for feil | A |
| Hjelp | `-h`/`--help` fungerer, eksempler inkludert | A |
| Underkommandoer | Ren dispatch, ingen flagg-eksplosjon | A |
| Output-moduser | `--json` på alle kommandoer | A |
| Exit-koder | Dokumenterte konstanter (0/1/2) | A |
| Miljøvariabler | NO_COLOR respektert, COPILOT_* brukt | A- |
| Fremtidssikring | Stabile flaggnavn, additive endringer | A |

**Samlet: A** — godt designet småverktøy som følger moderne standarder. Tier 1-gap (exit-koder, feilhint, --json, fremdrift) er lukket.

### Designvalg vi beholder bevisst

Noen avvik fra industripraksis er bevisste valg:

1. **Manuell flagg-parsing i stedet for Cobra/Kong** — Gir full kontroll, null implisitt oppførsel, enklere å forstå. Bytt bare hvis vi passerer ~15 kommandoer.

2. **Ingen YAML-bibliotek for frontmatter** — Linjebasert parsing er enklere, raskere, og unngår en avhengighet. Fungerer for vårt begrensede bruk.

3. **Én pakke (package main)** — Hele kodebasen kan leses på under en time. Ikke splitt i internal/pkg med mindre det blir nødvendig.

4. **Farger via ANSI i stedet for lipgloss** — 30 linjer i output.go er nok. lipgloss er overkill for fem fargehjelper-funksjoner.

5. **Ingen konfigurasjonsfil** — CLI-flagg og miljøvariabler er nok for et verktøy som kjøres sjelden. Konfigfiler legger til kompleksitet.
