# Telemetri i nav-pilot CLI

> **Beta-dokument for interne Nav-utviklere.** Dette gjelder nav-pilot CLI-versjonen v0.x+ med OpenTelemetry-støtte.

## 1. Hva samles inn?

nav-pilot sender **pseudonymiserte bruks- og ytelsesmetrikker** via OpenTelemetry (OTLP/HTTP). Ingenting personlig eller kodesensitivt blir logget.

| Metrikk | Type | Beskrivelse | Eksempler på dimensjoner |
|---------|------|-------------|--------------------------|
| `nav_pilot_command_total` | Counter | Antall kommandoer kjørt | `command=install`, `mode=interactive`, `scope=repo`, `result=success` |
| `nav_pilot_command_duration_ms` | Histogram | Kjøringstid per kommando (ms) | Samme som over |
| `nav_pilot_command_error_total` | Counter | Antall kommandoer som feilet | `command=sync`, `scope=user` |
| `nav_pilot_install_items_total` | Counter | Antall elementer installert | `command=install`, `scope=repo`, `mode=interactive` |
| `nav_pilot_sync_updates_total` | Counter | Antall oppdateringer funnet ved sync | `command=sync`, `scope=user` |
| `nav_pilot_sync_conflicts_total` | Counter | Antall konflikter ved sync | `command=sync`, `scope=repo` |
| `nav_pilot_info` | Gauge | Prosess-start informasjon (alltid verdi 1) | `version=0.12.3`, `device_id=nav-pilot-abc123`, `execution_context=ci_github_actions`, `os=linux`, `arch=amd64` |
| `nav_pilot_install_present` | Gauge | Om scope har installert state (1/0) | `scope=user`, `collection=all` |
| `nav_pilot_installed_items` | Gauge | Antall installerte items per type/status | `scope=repo`, `type=skill`, `status=active` |
| `nav_pilot_staleness_check_total` | Counter | Antall ferskhetssjekker per resultat | `component=collection`, `scope=user`, `result=stale` |
| `nav_pilot_up_to_date` | Gauge | Om komponent er tilstrekkelig oppdatert (1/0) | `component=cli`, `scope=none` |
| `nav_pilot_version_skew_days` | Histogram | Dager mellom installert og siste tilgjengelig versjon | `component=collection`, `scope=repo` |

`command`-dimensjonen inkluderer også livssyklus-eventer:
- `startup` når brukeren kjører `nav-pilot` uten args (interaktiv flyt)
- `launch` når nav-pilot forsøker å starte `cplt`/`copilot`

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

**Daglige installs per scope:**
```promql
increase(nav_pilot_install_items_total[1d]) by (scope)
```

**Gjennomsnittlig kommando-varighet per kommando:**
```promql
histogram_quantile(0.95, sum by (command, le) (rate(nav_pilot_command_duration_ms_bucket[5m])))
```

**Feiltakt (% feil av alle kommandoer):**
```promql
(
  increase(nav_pilot_command_error_total[1h])
  /
  increase(nav_pilot_command_total[1h])
) * 100
```

**Sync-konflikter per dag:**
```promql
increase(nav_pilot_sync_conflicts_total[1d]) by (scope, mode)
```

**Versjon-fordeling av aktive brukere:**
```promql
increase(nav_pilot_command_total[24h]) by (version)
```

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
- **Sletting**: Brukere kan slette ved å deaktivere telemetri (se under)

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
