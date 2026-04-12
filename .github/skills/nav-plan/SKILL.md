---
name: nav-plan
description: Arkitekturplanlegging med beslutningstrær for auth, kommunikasjon, database og Nais-konfigurasjon
---

# Nav Architecture Plan

Gå fra en vag idé til en konkret, Nav-kompatibel arkitekturplan. Bruker beslutningstrær for å velge riktig auth, kommunikasjon, database og Nais-oppsett.

## Workflow

1. **Velg arketype** — hva slags ting bygges
2. **Gå gjennom beslutningstrær** — auth, kommunikasjon, data, observerbarhet
3. **Velg teststrategi** — riktig testnivå per komponent og endringstype
4. **Generer plan** — Nais-manifest, prosjektstruktur, CI/CD
5. **Generer leveransedokumenter** — endringsdokument, utrullingsplan, runbook
6. **Review** — verifiser at valgene henger sammen

## Steg 1: Arketype

| Arketype | Stack | Nais-type |
|----------|-------|-----------|
| Backend API | Kotlin/Ktor eller Spring Boot | Application |
| Hendelsekonsument | Kotlin + Kafka + Rapids & Rivers | Application |
| Frontend (innbygger) | Next.js + ID-porten + Wonderwall | Application |
| Frontend (saksbehandler) | Next.js + Azure AD + Wonderwall | Application |
| Batchjobb | Kotlin + scheduled | Naisjob |
| BFF (Backend-for-Frontend) | Next.js API routes | Application |

### Endringstype

I tillegg til arketype, avklar om dette er nytt eller en endring:

| Endringstype | Ekstra beslutninger |
|-------------|-------------------|
| **Nybygg** | Standard beslutningstrær (auth, data, kommunikasjon) |
| **Modernisering** | Migreringsstrategi, bakoverkompatibilitet, utrulling, dekommisjonering |
| **Refaktorering** | Karakteriseringstester, rollback-plan, parallellkjøring |

For modernisering og refaktorering — se [modernization-patterns.md](./references/modernization-patterns.md) for konkrete mønstre og [decision-trees.md](./references/decision-trees.md) for migreringsbeslutningstrær.

## Steg 2: Beslutningstrær

Se [decision-trees.md](./references/decision-trees.md) for alle beslutningstrær.

### Autentisering

```
Hvem kaller tjenesten?
├── Innbygger via nettleser
│   └── ID-porten + Wonderwall
│       idporten.enabled: true
│       Nais-sidecar: Wonderwall
│       Node.js: @navikt/oasis
│       JVM: token-support
│
├── Saksbehandler via nettleser
│   └── Azure AD + Wonderwall
│       azure.application.enabled: true
│       Nais-sidecar: Wonderwall
│       Node.js: @navikt/oasis
│       JVM: token-support
│
├── Annen Nav-tjeneste (med brukerkontext)
│   └── TokenX (token exchange)
│       tokenx.enabled: true
│       Kaller: exchangeToken(subjectToken, targetAudience)
│       Mottaker: valider TokenX-token
│
├── Annen Nav-tjeneste (uten brukerkontext / batch)
│   └── Azure AD client_credentials
│       azure.application.enabled: true
│       Kaller: getClientCredentialsToken(scope)
│       ⚠️ Kun når brukerkontext ikke er tilgjengelig!
│
└── Ekstern partner
    └── Maskinporten
        maskinporten.enabled: true
        Definerer scopes for ekstern tilgang
```

### Kommunikasjon

```
Hva slags kommunikasjon?
├── Synkron (kaller og venter på svar)
│   ├── REST API (standard)
│   │   Kotlin: Ktor routing eller Spring @RestController
│   │   OpenAPI: Swagger/SpringDoc
│   │
│   └── GraphQL (komplekse, fleksible queries)
│       Sjelden brukt i Nav, foretrekk REST
│
├── Asynkron (fire-and-forget, hendelsedrevet)
│   ├── Kafka (standard for Nav)
│   │   Topic: {team}.{domene}.v1
│   │   Dev: 1 partisjon, Prod: 6+
│   │   Rapids & Rivers for hendelseskoreografi
│   │
│   └── Pub/Sub (GCP-native, sjelden)
│       Bare hvis Kafka ikke passer
│
└── Sanntid (push til klient)
    └── Server-Sent Events
        Ktor: respondSse {}
        Next.js: ReadableStream
```

### Database

```
Trenger du persistent lagring?
├── Ja, relasjonell data
│   └── PostgreSQL via Nais (Cloud SQL)
│       gcp.sqlInstances[].type: POSTGRES_15
│       Flyway for migrasjoner
│       HikariCP pool: 3-5 (containere!)
│       
│       Primærnøkkel?
│       ├── Domene-ID (fnr, saksnummer) → VARCHAR
│       └── Systemgenerert → BIGSERIAL eller UUID
│
├── Ja, analytisk / read-heavy
│   └── BigQuery
│       Separat Nais-konfigurasjon
│       Bruk @google-cloud/bigquery
│
├── Ja, cache / sesjon
│   └── Redis via Nais
│       gcp.redis[].tier: BASIC
│       
└── Nei
    └── Stateless tjeneste (bra! enklere drift)
```

## Steg 3: Generer plan

Basert på beslutningene, generer disse artefaktene:

### Nais-manifest

Se [nais-templates.md](./references/nais-templates.md) for komplette maler per arketype.

```yaml
# Generert manifest basert på beslutningstrær
apiVersion: nais.io/v1alpha1
kind: Application
metadata:
  name: {app-name}
  namespace: {team}
  labels:
    team: {team}
spec:
  image: "{{ image }}"
  port: 8080
  
  # Auth — fra beslutningstre
  # [genereres basert på valgt mekanisme]
  
  # Database — fra beslutningstre  
  # [genereres hvis PostgreSQL valgt]
  
  # Kafka — fra beslutningstre
  # [genereres hvis Kafka valgt]
  
  # Alltid inkluder:
  liveness:
    path: /isalive
  readiness:
    path: /isready
  prometheus:
    enabled: true
    path: /metrics
  resources:
    requests:
      cpu: 15m
      memory: 256Mi
    limits:
      memory: 512Mi
  replicas:
    min: 2
    max: 4
  
  # accessPolicy — ALLTID eksplisitt
  accessPolicy:
    inbound:
      rules: []  # Fyll inn basert på hvem som kaller
    outbound:
      rules: []  # Fyll inn basert på avhengigheter
```

### Prosjektstruktur

**Kotlin/Ktor:**
```
{app-name}/
├── .nais/
│   ├── nais.yaml
│   └── nais-dev.yaml
├── .github/workflows/
│   └── build-deploy.yml
├── src/main/kotlin/no/nav/{team}/{app}/
│   ├── Application.kt
│   ├── Config.kt          # Sealed class config
│   ├── api/
│   │   └── Routes.kt
│   ├── db/
│   │   └── Repository.kt  # Kotliquery
│   └── kafka/
│       └── Consumer.kt    # Rapids & Rivers
├── src/main/resources/
│   └── db/migration/
│       └── V1__initial_schema.sql
├── src/test/kotlin/
├── build.gradle.kts
└── Dockerfile
```

**Next.js:**
```
{app-name}/
├── .nais/
│   ├── nais.yaml
│   └── nais-dev.yaml
├── .github/workflows/
│   └── build-deploy.yml
├── src/
│   ├── app/
│   │   ├── layout.tsx
│   │   ├── page.tsx
│   │   └── api/          # BFF proxy routes
│   ├── components/
│   └── lib/
│       ├── auth.ts       # @navikt/oasis
│       └── api-client.ts
├── next.config.ts
├── package.json
└── Dockerfile
```

### CI/CD-workflow

```yaml
name: Build and deploy
on:
  push:
    branches: [main]

permissions:
  contents: read
  id-token: write

jobs:
  build:
    runs-on: ubuntu-latest
    outputs:
      image: ${{ steps.docker-build-push.outputs.image }}
    steps:
      - uses: actions/checkout@v4
      - uses: nais/docker-build-push@v0
        id: docker-build-push
        with:
          team: {team}
          identity_provider: ${{ secrets.NAIS_WORKLOAD_IDENTITY_PROVIDER }}
          project_id: ${{ vars.NAIS_MANAGEMENT_PROJECT_ID }}

  deploy-dev:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: nais/deploy/actions/deploy@v2
        env:
          CLUSTER: dev-gcp
          RESOURCE: .nais/nais-dev.yaml
          VAR: image=${{ needs.build.outputs.image }}

  deploy-prod:
    needs: [build, deploy-dev]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: nais/deploy/actions/deploy@v2
        env:
          CLUSTER: prod-gcp
          RESOURCE: .nais/nais.yaml
          VAR: image=${{ needs.build.outputs.image }}
```

## Steg 3b: Teststrategi

Velg riktig testnivå basert på arketype og endringstype. Se [Teststrategi-beslutningstre](./references/decision-trees.md#teststrategi-beslutningstre) for detaljerte trær.

**Minimum for alle arketyper:**

| Arketype | Unit | Integrasjon | E2E |
|----------|------|-------------|-----|
| Backend API | Forretningslogikk | DB + auth | — |
| Hendelsekonsument | Meldingsbehandling | TestRapid | — |
| Frontend | Komponenter | — | Kritiske reiser |
| BFF | Transformasjoner | Auth-proxy | — |
| Batchjobb | Beregninger | DB | — |

**Tillegg for modernisering:**
- Karakteriseringstester **før** du endrer kode
- Migreringsverifisering (gammel kode + nytt skjema og omvendt)
- Regresjonstester for grenseflater mellom ny og gammel kode

Se [Endringskonsekvensanalyse-tre](./references/decision-trees.md#endringskonsekvensanalyse-tre) for kartlegging av påvirkning.

## Steg 3c: Leveransedokumenter

Generer relevante dokumenter basert på endringstype og risiko. Se [dokumentasjon og leveransemaler](./references/documentation-templates.md) for fullstendige maler.

| Dokument | Når | Formål |
|----------|-----|--------|
| Endringsdokument | Alltid for ikke-trivielle endringer | Beslutning, påvirkning, utrulling, rollback |
| Runbook-oppdatering | Ny tjeneste eller endret driftsadferd | Feilsøking, eskalering, helsesjekk |
| API-endringsdokument | Breaking changes eller nye API-er | Migrasjonsveiledning for konsumenter |
| Observerbarhet | Alle endringer i produksjonsadferd | Suksesskriterier, dashboards, alarmer |

**Minimum output for enhver plan:**
1. Endringsdokument med rollback-plan
2. Post-deploy-verifiseringssjekkliste
3. Observerbarhetsplan (hva måles, hva alarmeres)

## Steg 4: Review-sjekkliste

Verifiser at valgene henger sammen:

- [ ] Auth-mekanisme matcher caller-type (ikke Azure client_credentials med brukerkontext)
- [ ] `accessPolicy.inbound` lister alle kallere
- [ ] `accessPolicy.outbound` lister alle avhengigheter
- [ ] Ressurser er riktige for forventet last
- [ ] Health-endepunkter er implementert (/isalive, /isready, /metrics)
- [ ] Database-pooling er tilpasset containere (3-5, ikke 10)
- [ ] PII er identifisert og beskyttet (ikke logget, tilgangsstyrt)
- [ ] Observerbarhet dekker forretningsmetrikker, ikke bare tekniske
- [ ] CI/CD deployer til dev før prod

### Ekstra for modernisering/refaktorering

- [ ] Rollback-plan er definert og testet
- [ ] Feature toggle er satt opp for gradvis utrulling
- [ ] Bakoverkompatibilitet er verifisert (gammel kode + nytt skjema fungerer)
- [ ] Karakteriseringstester låser nåværende adferd
- [ ] Exit criteria er definert (når er migreringen ferdig?)
- [ ] Berørte team/konsumenter er identifisert og informert
- [ ] Dekommisjoneringsplan for gammel kode/infrastruktur

### Testing og dokumentasjon

- [ ] Teststrategi er definert per komponent/lag
- [ ] Endringsdokument er generert med rollback-plan
- [ ] Observerbarhetsplan definerer suksesskriterier og alarmer
- [ ] Post-deploy-verifiseringssjekkliste er utfylt
- [ ] API-endringsdokument er laget (hvis breaking changes)
- [ ] Runbook er oppdatert (hvis ny tjeneste eller endret drift)

## Boundaries

### ✅ Always

- Gå gjennom alle beslutningstrær systematisk
- Generer Nais-manifest med eksplisitt accessPolicy
- Inkluder CI/CD-workflow i planen
- Verifiser auth mot caller-type
- Start med små ressurser (15m CPU, 256Mi memory)

### ⚠️ Ask First

- Avvik fra standard arketyper
- Introduksjon av ny teknologi (GraphQL, gRPC)
- Komplekse multi-service arkitekturer

### 🚫 Never

- Generer plan uten å ha avklart auth
- Bruk default HikariCP pool-størrelse
- Utelat accessPolicy fra Nais-manifest
- Foreslå CPU-limits i Nais
