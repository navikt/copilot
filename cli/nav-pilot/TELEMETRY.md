# Telemetri i nav-pilot CLI

> **Beta-dokument for interne Nav-utviklere.** Dette gjelder nav-pilot CLI-versjonen v0.x+ med OpenTelemetry-støtte.

## 1. Hva samles inn?

nav-pilot sender **pseudonymiserte bruks- og ytelsesmetrikker** via OpenTelemetry (OTLP/HTTP). Ingenting personlig eller kodesensitivt blir logget.

| Metrikk | Type | Beskrivelse | Eksempler på dimensjoner |
|---------|------|-------------|--------------------------|
| `nav_pilot_command_total` | Counter | Antall kommandoer kjørt | `command=install`, `mode=interactive`, `scope=repo`, `result=success` |
| `nav_pilot_command_duration_ms` | Histogram | Kjøringstid per kommando (ms) | Samme som over |
| `nav_pilot_command_error_total` | Counter | Antall kommandoer som feilet | `command=sync`, `scope=user` |
| `nav_pilot_launch_error_total` | Counter | Klient-oppstart som feilet | `client=copilot`, `error_type=launch_failed` |
| `nav_pilot_rtk_setup_total` | Counter | Resultat av interaktiv RTK-prompt | `client=copilot`, `choice=yes`, `result=success` |
| `nav_pilot_install_items_total` | Counter | Antall elementer installert | `command=install`, `scope=repo`, `mode=interactive` |
| `nav_pilot_sync_updates_total` | Counter | Antall oppdateringer funnet ved sync | `command=sync`, `scope=user` |
| `nav_pilot_sync_conflicts_total` | Counter | Antall konflikter ved sync | `command=sync`, `scope=repo` |
| `nav_pilot_info` | Gauge | Prosess-start informasjon (alltid verdi 1) | `version=0.12.3`, `device_id=nav-pilot-abc123`, `execution_context=ci_github_actions`, `os=linux`, `arch=amd64` |
| `nav_pilot_install_present` | Gauge | Om scope har installert state (1/0) | `scope=user`, `collection=all` |
| `nav_pilot_installed_items` | Gauge | Antall installerte items per type/status | `scope=repo`, `type=skill`, `status=active` |
| `nav_pilot_config_info` | Gauge | Resolvert konfigurasjon per oppstart (alltid verdi 1) | `client=opencode`, `config_mode=autopilot`, `model=github-copilot/claude-sonnet-4.5`, `reasoning_effort=high`, `context_tier=unset`, `otel_log_level=none`, `allow_all_tools=false`, `ask_user=true`, `device_id=nav-pilot-abc123` |
| `nav_pilot_client_available` | Gauge | Om en coding-agent-klient finnes på PATH (1/0) | `client=copilot` / `client=opencode` / `client=pi` |
| `nav_pilot_staleness_check_total` | Counter | Antall ferskhetssjekker per resultat | `component=collection`, `scope=user`, `result=stale` |
| `nav_pilot_up_to_date` | Gauge | Om komponent er tilstrekkelig oppdatert (1/0) | `component=cli`, `scope=none` |
| `nav_pilot_version_skew_days` | Histogram | Dager mellom installert og siste tilgjengelig versjon | `component=collection`, `scope=repo` |

`command`-dimensjonen inkluderer også livssyklus-eventer:
- `startup` når brukeren kjører `nav-pilot` uten args (interaktiv flyt)
- `launch` når nav-pilot forsøker å starte `cplt`/`copilot`

**Merk om `nav_pilot_config_info`:**
- `config_mode` er konfig-modus (`default`/`plan`/`autopilot`) — ikke å forveksle
  med `mode` på `nav_pilot_command_total` som er kjøremodus (`interactive`/`non_interactive`).
- `model` kollapses til kjent klient-modell-id (Copilot-modeller som `claude-sonnet-4.6`,
  eller opencode-modeller som `github-copilot/claude-sonnet-4.5`), `custom` (ukjent/egendefinert)
  eller `unset` for å holde kardinaliteten lav.
- Tomme valg (`reasoning_effort`, `context_tier`, `model`) rapporteres som `unset`.
- `nav_pilot_client_available` PATH-sjekker `copilot` (cplt/copilot), `opencode` og `pi`
  ved oppstart, så vi ser hvilke klienter brukere faktisk har installert.

**Alle metrikker inkluderer også (resource-attributter):**
- `service.name` = `"nav-pilot"`
- `service.version` = CLI-versjon (f.eks. `"0.12.3"`, `"dev"`)
- `os` = `"darwin"`, `"linux"`, `"windows"`
- `arch` = `"amd64"`, `"arm64"` etc.
- `device_id` = pseudonymisert maskin-ID (se under)

`execution_context` følger alle løpende metrikker som datapunkt-dimensjon.
`device_id` ligger på `nav_pilot_info` og som resource-attributt.
Det betyr at både resource-attributter og datapunkt-dimensjoner kan brukes i spørringer.

**`execution_context`-verdier:**
- `organic` = vanlig CLI-bruk
- `ci_github_actions` = kjøring i GitHub Actions
- `ci_other` = annen CI
- `unknown` = ikke klassifisert

Klassifisering prioriterer:
1. `NAV_PILOT_EXECUTION_CONTEXT` (eksplisitt override)
2. `GITHUB_ACTIONS=true`
3. generiske CI-signaler (`CI`, `GITLAB_CI`, `JENKINS_URL`, `BUILDKITE`, `CIRCLECI`, `TF_BUILD`, `BUILD_ID`)
4. fallback `organic`

**Hva sendes IKKE:**
- ✗ Filstier, reponavn, eller prosjektkontekst
  (unntak: `nav.repo`-slug for navikt-repoer på agent-sessions, se seksjon 3)
- ✗ Innhold fra Copilot-instruksjoner eller agenter
- ✗ Bruker-ID (aldri NAVident, e-post, GitHub-brukernavn)
- ✗ Git-commit-info eller miljøvariabler

**Device ID (pseudonym):**
- `device_id` = Stabil, deterministisk identifikator per maskin
  - Genereres fra: hostname + CLI-installasjonssti + MAC-adresse (SHA256)
  - Lagret lokalt i `~/.nav-pilot/device-id` (persistent)
  - Samme maskin = alltid samme ID (reproducible)
  - **Inneholder INGEN persondata** (kun hardware/path)

**Dataoppbevaring:**
- Oppbevaringstid styres av backend (Prometheus/OTLP-collector), ikke av CLI-en.
- nav-pilot sender ikke en klientstyrt retention-innstilling.

---

## 2. Hvorfor?

### Adopsjonsovervåking
- Hvor mange Teams bruker `nav-pilot install` hver dag?
- Hvilke kommandoer brukes mest (install vs. sync vs. list)?
- Øker bruken etter nye releaser?

### Feildiagnose
- Høy feiltakt på `sync` i `interactive` mode → bug i konfliktdeteksjon?
- `command_duration_ms` spiker → nettverksproblemer eller IO-bottleneck?
- Mange `sync_conflicts` på `user` scope → dårlig merge-logikk?

### Brukeropplevelse
- Gjennomsnittlig kjøringstid per kommando på tvers av OS
- Andel mislykkede kommandoer → indikator for produktkvalitet
- Mode-fordelinger (interaktiv vs. automatisk) → brukerpreferanser

---

## 3. Hvordan brukes det?

### Aktivering (pilot-fase)

Telemetri er **aktivert som standard** i pilot-fase.

```bash
nav-pilot install @nav-pilot
```

Standard endpoint er:

`https://collector-internet.nav.cloud.nais.io/v1/metrics`

Du kan overstyre endpoint ved behov med `NAV_PILOT_TELEMETRY_ENDPOINT` eller
`OTEL_EXPORTER_OTLP_ENDPOINT`.

Ved launch av `cplt`/`copilot` setter nav-pilot også `OTEL_EXPORTER_OTLP_ENDPOINT`
for Copilot CLI til collector-base uten `/v1/metrics`, slik at Copilot kan sende
både metrics og traces. Egen override for Copilot er `NAV_PILOT_COPILOT_OTEL_ENDPOINT`
(den har høyere prioritet enn en generell `OTEL_EXPORTER_OTLP_ENDPOINT`).
nav-pilot setter også `COPILOT_OTEL_ENABLED=true` hvis den ikke allerede er satt.

I tillegg injiserer nav-pilot egne resource-attributter i Copilots
`OTEL_RESOURCE_ATTRIBUTES`, slik at Copilot-traces kan attribueres tilbake til
nav-pilot. Eksisterende nøkler beholdes (append-merge, ingen overskriving):

| Attributt | Verdi | Hensikt |
| --- | --- | --- |
| `nav.pilot.launcher` | `nav-pilot` | Isolere Copilot-sessions startet via nav-pilot |
| `nav.pilot.version` | nav-pilot-versjon | Adopsjon/versjon av launcheren |
| `nav.pilot.device_id` | pseudonymt `nav-pilot-<hash>` | Join (på verdi) mot nav-pilots egen `device_id`-attributt |
| `nav.repo` | `navikt/<repo>`-slug fra `origin`-remote | Join (server-side, ved spørring) mot repo→team-mapping, slik at sessionsdata kan aggregeres per team (issue #344) |

`nav.pilot.device_id` injiseres kun når nav-pilot-telemetri er aktiv; med
`NAV_PILOT_TELEMETRY_ENABLED=false` utelates den (launcher/version beholdes).

`nav.repo` settes kun når nav-pilot startes inne i et git-repo hvis
`origin`-remote peker på GitHub-organisasjonen `navikt` (både ssh-formen
`git@github.com:navikt/foo.git` og https-formen `https://github.com/navikt/foo`,
med eller uten `.git`, gjenkjennes). Attributten identifiserer en **kodebase,
ikke en person**.

**Hva `nav.repo` IKKE samler inn:**
- ✗ Ingen brukernavn, e-post eller NAVident
- ✗ Ingen hostname
- ✗ Ingen filstier — kun `owner/navn`-slug utledet fra remote-URL-en
- ✗ Utenfor navikt-repoer utelates attributten **helt** (ikke et git-repo,
  ingen `origin`-remote, eller remote utenfor navikt-org) — det gjettes aldri
- ✗ En bruker-satt `nav.repo` i `OTEL_RESOURCE_ATTRIBUTES` overskrives aldri

#### Per-device-spørringer på Copilot-data («vis brukeren egne data»)

Dette er hovednytten for en bruker som vil se **sin egen Copilot-bruk** (modell,
tokens, kostnad — Copilots egne `gen_ai.*`-signaler). nav-pilot kan ikke legge
`device_id` på selve datapunktene i Copilots metrikker (Copilot CLI er en
tredjeparts-binær vi ikke instrumenterer) — vi injiserer den som
**resource-attributt**. Det gir to ulike spørrestier:

- **Tempo (traces):** resource-attributter er direkte søkbare. Fungerer i dag:
  ```traceql
  { resource.nav.pilot.device_id = "nav-pilot-abc123" }
  ```
  Filtrer gjerne videre på Copilot-attributter, f.eks. `gen_ai.request.model`,
  `gen_ai.usage.output_tokens`, `github.copilot.cost`.

- **Mimir (metrics):** OTLP-resource-attributter havner på `target_info`, ikke
  som etikett på `gen_ai.*`-seriene. To alternativer:
  1. **Anbefalt — promotér attributten til etikett.** Konfigurer Mimir
     `promote_resource_attributes` (eller en `transform`/`groupbyattrs`-prosessor
     i OTel-collectoren) slik at `nav.pilot.device_id` blir en metrikk-etikett
     (`nav_pilot_device_id`). Da blir spørringen triviell og robust også for de
     kortlevde Copilot-prosessene:
     ```promql
     sum by (gen_ai_request_model) (
       increase(gen_ai_client_token_usage_sum{nav_pilot_device_id="nav-pilot-abc123"}[$__range])
     )
     ```
  2. **Uten promotering — join mot `target_info`.** Mulig, men skjørt for efemere
     prosesser (samme staleness-problem som for nav-pilots egne tellere):
     ```promql
     increase(gen_ai_client_token_usage_sum[$__range])
       * on (job, instance) group_left(nav_pilot_device_id) target_info
     ```

> Eksakte Copilot-metrikk-/etikettnavn (`gen_ai.*`, suffikser, hvordan punktum og
> bindestrek normaliseres til Prometheus-navn) bør verifiseres mot faktisk data i
> Mimir og justeres. Copilots egen pseudonyme `enduser.pseudo.id` finnes som
> span-attributt, men har ingen dokumentert kobling til GitHub-brukernavn —
> `nav.pilot.device_id` er derfor det stabile håndtaket for egen-data.

For å eksplisitt tvinge på i `~/.bashrc` / `~/.zshrc`:

```bash
# ~/.zshrc
export NAV_PILOT_TELEMETRY_ENABLED=1
```

### Verifisering

Kjør kommando med `--verbose` eller debug-logging for å se telemetri-status:

```bash
# Telemetri sendes stille. Hvis den feiler, ser du advarsel:
$ nav-pilot list
# ingen endpoint kreves; standard endpoint brukes automatisk
```

### Dashboard-eksempler (Grafana / Prometheus)

> **Viktig — OTel Delta Temporality:** Fra og med v0.x er CLI-en konfigurert med
> `DeltaTemporality` for tellere. OTel-collectoren konverterer disse til korrekte 
> kumulative Prometheus-tellere. Bruk standard Prometheus-funksjoner som `increase(<metric>[<range>])` 
> og `rate()` for grafer. Histogrammer aggregeres med `sum by (le) (increase(<metric>_bucket[<range>]))` før `histogram_quantile`.

**Daglige installs per scope:**
```promql
sum by (scope) (increase(nav_pilot_install_items_total[1d]))
```

**Kommando-varighet p95 per kommando:**
```promql
histogram_quantile(0.95, sum by (command, le) (increase(nav_pilot_command_duration_ms_bucket[$__range])))
```

**Feiltakt (% feil av alle kommandoer):**
```promql
100 * sum(increase(nav_pilot_command_error_total[$__range]))
    / clamp_min(sum(increase(nav_pilot_command_total[$__range])), 1)
```

**Sync-konflikter (totalt) per scope:**
```promql
sum by (scope) (increase(nav_pilot_sync_conflicts_total[$__range]))
```

**Antall kommandokjøringer per versjon:**
```promql
sum by (version) (increase(nav_pilot_command_total[$__range]))
```

> En ferdig Grafana-dashboard ligger i [`dashboards/nav-pilot-cli.json`](../../dashboards/nav-pilot-cli.json)
> (uid tildeles ved import). Spørringene er robuste mot manglende data under utrulling
> (`or vector(0)`, `clamp_min(...)`-vakter). Eksakte Prometheus-serienavn (suffiks som
> `_bucket`/`_sum`/`_count`, evt. enhetssuffiks, og `target_info` for ressursattributter)
> bør verifiseres mot den faktiske datakilden og justeres ved behov.

### Alarmer (foreslåtte)

| Alarm | Betingelse | Aksjon |
|-------|-----------|--------|
| Høy feiltakt | `error_rate > 10%` over 1 time | Sjekk feil-logg; rollback hvis kritisk |
| Lang kjøringstid | p95 `command_duration_ms` > 30s | Profilering; nettverksjekk |
| Mange konflikter | `sync_conflicts_total` > 100 per time | Gjennomgå merge-logikk |

---

## 4. Privacy & Security

### Tilgang
- **Nav Pilot-team** (DevOps, Platform): Les-tilgang til Prometheus/Grafana dashboard
- **Telemetry-operator**: Vedlikehold av OTLP-collector
- **Ingen**: Innholdet av filer, instruksjoner eller persondata

### Oppbevaringstid
- **Råmetrikker (Prometheus)**: 15 dager (default retention)
- **Aggregerte metrikker (Grafana dashboards)**: Lagret i repo; historikk beholdes på ubestemt tid
- **Stopp av innsamling**: Brukere kan stoppe videre innsamling ved å deaktivere telemetri (se under).
  Allerede sendte data styres av backend-retention.

### Personvern-garantier
- ✅ Ingen IP-adresser eller User-Agent som OTel-attributter i metrikksdata (merk: transport/ingress kan likevel se og evt. logge IP).
- ✅ Ingen rå maskinidentifikator (hostname/MAC); kun pseudonymisert `device_id` (SHA256-hash, 12 hex-tegn)
- ⚠️ `device_id` gir likevel oppløsning per maskin via `nav_pilot_info` (pseudonymt), ikke kun som globale aggregater.
  Den kan ikke knyttes til person/team uten en ekstern mapping.
- ⚠️ Kardinalitet: `device_id` (og `version`) er høy-kardinalitets-etiketter. I en stor pilot kan
  antall tidsserier vokse raskt i Prometheus — vurder å droppe/aggregere `device_id` i collector
  hvis kostnad/kardinalitet blir et problem.
- ✅ Telemetri kan deaktiveres eksplisitt (`NAV_PILOT_TELEMETRY_ENABLED=0`)
- ✅ Ikke delt med tredjeparter

### Deaktivering

For å **deaktivere telemetri**:

```bash
# Eksplisitt av
export NAV_PILOT_TELEMETRY_ENABLED=0
```

For å **permanent deaktivere** (foreslått for CI/automatisering):

```bash
# Legg i ~/.zshrc eller tilsvarende
export NAV_PILOT_TELEMETRY_ENABLED=0
```

**Effekt av deaktivering:**
- Ingen data sendes til collector
- nav-pilot kjører identisk ellers
- Ingen overhead eller ytelsestap

---

## 5. Aktivering — Steg for steg for pilot-brukere

### A. Enkel aktivering (anbefalt for demo)

```bash
# 1. Kjør nav-pilot som vanlig (telemetri er på som standard)
nav-pilot install @nav-pilot

# 2. Data sendes automatisk til backend
# (ingen output, veldig stille)
```

### B. Permanent eksplisitt aktivering (utviklermaskin, valgfritt)

```bash
# 1. Åpne shell-konfigfil
vim ~/.zshrc  # eller ~/.bashrc, ~/.config/fish/config.fish osv.

# 2. Legg til på slutten:
export NAV_PILOT_TELEMETRY_ENABLED=1

# 3. Last inn shell på nytt
source ~/.zshrc

# 4. Verifiser
echo $NAV_PILOT_TELEMETRY_ENABLED  # → 1
nav-pilot list
```

### C. Deaktivering (hvis du ombestemmer deg)

```bash
# Legg til i ~/.zshrc:
export NAV_PILOT_TELEMETRY_ENABLED=0

# Reload
source ~/.zshrc

# Verifiser
echo $NAV_PILOT_TELEMETRY_ENABLED  # → 0
```

### D. Sjekke status

```bash
# Er telemetri aktivert? (default er aktivert)
if [ "${NAV_PILOT_TELEMETRY_ENABLED:-1}" = "0" ] || [ "${NAV_PILOT_TELEMETRY_ENABLED:-1}" = "off" ]; then
  echo "✗ Telemetri deaktivert"
else
  echo "✓ Telemetri aktivert"
  echo "  Endpoint: ${NAV_PILOT_TELEMETRY_ENDPOINT:-https://collector-internet.nav.cloud.nais.io/v1/metrics}"
fi
```

---

## FAQ

**Sender nav-pilot data når telemetri er deaktivert?**  
Nei. Hvis `NAV_PILOT_TELEMETRY_ENABLED` settes til `0`/`off`, kjører en no-op telemetry recorder. Null overhead.

**Hva om standard-endpoint ikke er nåbar?**  
Telemetri logger en advarsel og feiler gracefully. Kommandoer kjører fortsatt normalt.

**Kan jeg se hva som blir sendt?**  
Ja — se `telemetry.go` i `cli/nav-pilot/` for full liste over metrikker og dimensjoner.

**Hvordan rapporterer jeg telemetri-bug eller privacy-bekymring?**  
Kontakt `@nav-pilot-team` eller lag issue i `navikt/copilot#issues` med tag `telemetry`.

**Brukes telemetri fra CI/CD?**  
Ja. CI-kjøringer klassifiseres med `execution_context` (for eksempel `ci_github_actions`) slik at dashboards kan skille dem fra organisk CLI-bruk. Du kan fortsatt deaktivere telemetri i pipelines:
```yaml
# .github/workflows/ci.yml
env:
  NAV_PILOT_TELEMETRY_ENABLED: "0"
```

**Når avsluttes pilot-programmet?**  
Planlagt: Q4 2026. Da blir telemetri gjort obligatorisk (eller stilt av). Pilot-brukere får varsel.

---

## Teknisk referanse

- **Eksport**: OpenTelemetry (OTLP/HTTP) til NAV sin Prometheus/Grafana-stack
- **Sendefrekvens**: Hver 10. sekund (batch)
- **Timeout**: 2 sekunder per batch
- **Språk**: Go 1.21+
- **Avhengigheter**: `go.opentelemetry.io/otel/*` (se `go.mod`)

For implementeringsdetaljer, se:
- `cli/nav-pilot/telemetry.go` — initialisering og recording
- `cli/nav-pilot/main.go` — integrasjon med kommandoer
- `cli/nav-pilot/telemetry_test.go` — enhetstester
