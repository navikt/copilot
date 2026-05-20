---
title: "Agentbasert observability — når Copilot feilsøker produksjon for deg"
date: 2026-05-20
author: starefosen
category: nav-pilot
excerpt: "Hvordan vi ga Copilot-agenter direkte tilgang til Mimir, Loki og Tempo — slik at de kan feilsøke problemer i produksjon uten at du forlater editoren."
tags:
  - observability
  - skills
  - nav-pilot
  - mimir
  - loki
  - tempo
  - debugging
---

For å feilsøke produksjonshendelser hopper du gjerne mellom Grafana, terminalen og editoren. Du skriver PromQL-spørringer, graver i logger, finner trace-ID-er og kobler det hele sammen i hodet. Hva om agenten kan gjøre denne jobben enklere for deg?

## Fra logger til strukturert observability

Nav har systematisk jobbet med å fjerne persondata fra logger og URLer. Det var nødvendig, men kan i noen tilfeller gjøre loggene vanskeligere å bruke til feilsøking — uten fødselsnummer eller navn i logglinjene er det ikke alltid opplagt å spore en feil gjennom systemet.

Som et kompenserende tiltak har vi tatt et større løft på å ta i bruk metrikker (Prometheus/Mimir) og distribuerte traces (Tempo) med auto-instrumentering for applikasjoner i Nais. Du kan se *hvor* i kallkjeden en feil oppstår, *hvilke* endepunkter som er trege, og *hvordan* systemet oppfører seg over tid — uten å eksponere persondata.

Utfordringen er å klare å bruke dette aktivt. PromQL har bratt læringskurve. Tempo-traces krever at du vet hvilke spørringer du skal stille. Grafana-dashboards tar tid å lage og blir fort utdaterte. Resultatet er at mange fortsatt søker gjennom mengder med logg for å finne nåla i høystakken.

## Tre pilarer, én agent

Navs observability-stack bygger på:

| Pilar | Verktøy | Svarer på |
|-------|---------|-----------|
| Metrikker | Mimir | *Hva* skjer? (rater, kvantiler, metning) |
| Logger | Loki | *Hvorfor* skjer det? (feilmeldinger, kontekst) |
| Traces | Tempo | *Hvor* i kallkjeden? (latens, avhengigheter) |

Alle tre eksponerer HTTP-APIer. Det betyr at en agent med `curl` og `jq` kan gjøre det samme som deg — bare raskere.

`observability-debugging`-skillen gir agenten strukturerte debugging-workflows, ferdige API-kall med riktige headere, og korrelasjonsmønstre som følger en tråd fra metrikk til logg til trace.

## Eksempel — feilsøk en pod som krasjer

Du spør: «min-app i prod restarter hele tiden»

Agenten kjører:

```bash
# Sjekk restart-count
kubectl get pods -n team-x -l app=min-app \
  -o custom-columns=POD:.metadata.name,RESTARTS:.status.containerStatuses[0].restartCount

# Sjekk minnebruk mot limit (Mimir)
curl -s -G -H "X-Scope-OrgID: tenant" \
  "https://mimir.nav.cloud.nais.io/prometheus/api/v1/query" \
  --data-urlencode 'query=container_memory_working_set_bytes{k8s_cluster_name="prod-gcp",app="min-app"}/container_spec_memory_limit_bytes{k8s_cluster_name="prod-gcp",app="min-app"}*100'

# Finn feilmeldinger rundt OOM-tidspunkt (Loki)
curl -s -G -H "X-Scope-OrgID: tenant" \
  "https://loki.nav.cloud.nais.io/loki/api/v1/query_range" \
  --data-urlencode 'query={k8s_cluster_name="prod-gcp",service_name="min-app"} | detected_level="error"'

# Hent trace for å se hvilken downstream-tjeneste som feiler (Tempo)
curl -s -H "X-Scope-OrgID: tenant" \
  "https://tempo.prod-gcp.nav.cloud.nais.io/api/traces/$TRACE_ID"
```

Resultat: «Appen bruker 94 % av minnegrensen, tre OOM-kills siste time. Loggene viser at `/api/rapport`-endepunktet laster hele datasettet i minnet. Forslag: stream resultatet eller øk memory-limit.»

## Ikke bare et dashboard

Dashboards viser data. Agenten *tolker* data og kobler dem til kildekoden din.

- **Dashboard**: Du ser en graf med økende latens og finner selv ut hvilken komponent som forårsaker det.
- **Agent**: Ser latens-økningen, finner tracet, identifiserer at database-spørringen i `VedtakRepository.kt:47` tar 3.8s, og foreslår en indeks.

Agenten har kontekst — den kjenner koden din, vet hvilke tjenester du kaller, og korrelerer på tvers av alle tre pilarene i ett steg. Du trenger ikke kunne PromQL for å dra nytte av metrikkene teamet ditt allerede eksponerer.

## Tilgangsstyring med cplt

Agenten bruker `kubectl` og `curl` mot interne APIer. Det betyr at den trenger tilganger — men ikke fritt spillerom.

`cplt` kjører agenten i en sandbox. Tilgangene styres av `.cplt.toml` i repoet ditt:

```toml
[propose.allow]
read = ["~/.kube/config"]       # kubectl trenger kubeconfig

[propose.proxy]
allow_private_domains = ["mimir.nav.cloud.nais.io", "loki.nav.cloud.nais.io", "tempo.prod-gcp.nav.cloud.nais.io"]
```

All nettverkstrafikk fra agenten går gjennom en filtrerende CONNECT-proxy. Bare domener du eksplisitt har godkjent slipper igjennom. Kjører du `cplt init` i et repo med skillen installert, foreslår cplt disse tilgangene automatisk.

## Kom i gang

```
nav-pilot install observability-debugging
```

Deretter: «Feilsøk høy latens på /api/vedtak i prod» — og se agenten jobbe.

**Forutsetninger:**
- Appen deployet på Nais
- `kubectl`-tilgang til clusteret
- `cplt` installert lokalt
- OpenTelemetry auto-instrumentering aktivert (`spec.observability.autoInstrumentation.enabled: true` i nais.yaml)

Mimir, Loki og Tempo er tilgjengelig for alle Nais-apper uten ekstra konfigurasjon. Og helt til slutt, visste du at agenter kan generere Grafana-dashboards for deg som JSON som kan importeres direkte?

---

Har du tilbakemeldinger eller forslag til forbedringer? Lag gjerne en issue på [navikt/copilot](https://github.com/navikt/copilot/issues) eller ta kontakt med oss i #github-copilot.
