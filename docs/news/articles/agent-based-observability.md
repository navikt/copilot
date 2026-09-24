---
title: "Observability-skillen bruker én inngang til Mimir, Loki og Tempo"
date: 2026-05-20
author: starefossen
category: nav-pilot
excerpt: "Ett skript setter org-header og forklarer HTTP-feil ved spørringer mot Mimir, Loki og Tempo. Tilgangen krever naisdevice og en godkjent waiver."
tags:
  - observability
  - skills
  - nav-pilot
  - mimir
  - loki
  - tempo
  - debugging
---

`observability-debugging` lar en agent undersøke metrikker, logger og traces fra samme workflow. Alle spørringer går nå gjennom `obs-query.sh`, som følger med skillen.

Skriptet setter riktig `X-Scope-OrgID`, URL-koder spørringen og skiller nettverksfeil fra avvisninger i cplt. Bare et 2xx-svar sendes til `jq`.

## Kom i gang

Standardpakken inneholder skillen. Bruk cplt `2026.09.14-105131` eller nyere, og koble naisdevice til Nav. Synk pakka og godkjenn waiveren for de fire observability-hostene når nav-pilot spør:

```bash
nav-pilot sync --apply
```

Ved ny installasjon kommer spørsmålet under installasjonen.

Start deretter nav-pilot og be agenten feilsøke et konkret symptom, for eksempel høy feilrate eller treg responstid.

Agenten bruker samme skript som de dokumenterte eksemplene:

```bash
OBS="$NAV_PILOT_SKILLS_DIR/observability-debugging/obs-query.sh"

bash "$OBS" mimir 'up{app="min-app"}' | jq .
bash "$OBS" loki '{k8s_cluster_name="prod",service_name="min-app"} |= "ERROR"' | jq .
bash "$OBS" tempo-search prod-gcp '{resource.service.name="min-app"}' | jq .
```

Nav-pilot setter `NAV_PILOT_SKILLS_DIR` til riktig katalog for klienten du bruker.

## To typer data i Mimir og Loki

`--org tenant` er standard og spør etter data fra teamenes workloads. Bruk `--org nais` for plattformdata som `nais-system`, node-exporter og alarmer.

Feil verdi kan gi et tomt, men teknisk gyldig svar. Derfor er valget eksplisitt:

```bash
bash "$OBS" mimir 'up{namespace="nais-system"}' --org nais | jq .
```

Tempo bruker miljø i vertsnavnet. Skillen støtter `dev-gcp` og `prod-gcp`.

## Hva cplt må tillate

Mimir, Loki og Tempo svarer på private adresser gjennom naisdevice. Standardpakken foreslår derfor disse fire vertene:

- `mimir.nav.cloud.nais.io`
- `loki.nav.cloud.nais.io`
- `tempo.dev-gcp.nav.cloud.nais.io`
- `tempo.prod-gcp.nav.cloud.nais.io`

Godkjenningen lagres per installert scope. Nav-pilot endrer ikke den globale cplt-konfigurasjonen.

Hvis spørringen feiler, sier skriptet hvilken grense som stoppet den:

| Svar                           | Vanlig årsak                                  |
| ------------------------------ | --------------------------------------------- |
| timeout eller nettverksfeil    | naisdevice er ikke koblet til                 |
| `403 Resolved to a private IP` | waiveren er ikke godkjent                     |
| `403 Domain not in allowlist`  | verten er ikke i pakkas liste                 |
| `401`                          | `X-Scope-OrgID` mangler eller nådde ikke fram |

## Hva agenten kan gjøre

Skillen har workflows for å:

- bekrefte feilrate eller ressursbruk i Mimir
- finne relevante logger i Loki
- følge en `trace_id` gjennom Tempo
- koble funnet til kode og Nais-konfigurasjon

Agenten kan hjelpe med analysen. Den skal ikke endre produksjon eller utvide sandbox-tilganger uten at du tar beslutningen.

**Kilder:**

- [Standardpakken ber om waiver for observability-vertene](https://github.com/navikt/copilot/pull/910) (navikt/copilot, 17. september 2026)
- [Samle observability-spørringene i ett skript](https://github.com/navikt/copilot/pull/914) (navikt/copilot, 18. september 2026)

_Oppdatert 24. september 2026 med gjeldende sandbox- og spørringsflyt._
