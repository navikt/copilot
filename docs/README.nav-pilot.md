# 🧭 nav-pilot

nav-pilot er et CLI-verktøy og en AI-agent for Nav-utvikling med GitHub Copilot.

📖 **Online docs (primær):** https://ki-utvikling.nav.no/nav-pilot  
📝 **Endringslogg:** [docs/nav-pilot-changelog.md](nav-pilot-changelog.md)

## Kom i gang

```bash
# Installer CLI
brew install navikt/tap/nav-pilot

# I et repo
nav-pilot
nav-pilot install kotlin-backend
```

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

## Telemetry (pilot, opt-in)

nav-pilot sender ikke produkttelemetri som standard. I pilot kan du aktivere OTel-metrikker eksplisitt:

```bash
export NAV_PILOT_TELEMETRY_ENABLED=1
export NAV_PILOT_TELEMETRY_ENDPOINT="https://<otel-endepunkt>"
```

Støttede MVP-metrikker:

- `nav_pilot_command_total`
- `nav_pilot_command_duration_ms`
- `nav_pilot_command_error_total`
- `nav_pilot_install_items_total`
- `nav_pilot_sync_updates_total`
- `nav_pilot_sync_conflicts_total`

`NAV_PILOT_TELEMETRY_ENABLED=0` (eller unset) deaktiverer telemetry.

## For bidragsytere

- Agent: `.github/agents/nav-pilot.agent.md`
- Design: `docs/nav-pilot-design.md`
- Skills: `.github/skills/<name>/`
- Instruksjoner: `.github/instructions/`

Detaljert bruk, CLI-referanse og arbeidsflyt vedlikeholdes i online docs.
