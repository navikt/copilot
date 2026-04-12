# Beslutningstrær for Nav-arkitektur

## Auth-beslutningstre (komplett)

### Steg 1: Hvem initierer forespørselen?

| Caller | Auth-mekanisme | Nais-konfigurasjon |
|--------|---------------|-------------------|
| Innbygger via nettleser | ID-porten | `idporten.enabled: true` |
| Saksbehandler via nettleser | Azure AD | `azure.application.enabled: true` |
| Annen Nav-tjeneste med brukerkontext | TokenX | `tokenx.enabled: true` |
| Annen Nav-tjeneste uten brukerkontext | Azure AD client_credentials | `azure.application.enabled: true` |
| Ekstern partner/system | Maskinporten | `maskinporten.enabled: true` |
| Intern batch/cron | Azure AD client_credentials | `azure.application.enabled: true` |

### Steg 2: Token-validering og exchange

> **Implementasjon:** Bruk `@auth-agent` for komplett oppsett av token-validering.
> For TokenX token exchange, bruk `$tokenx-auth` som har detaljerte Kotlin- og Node.js-eksempler
> med caching, feilhåndtering og Ktor-integrasjon.

**Biblioteker:**
- JVM: `no.nav.security:token-validation-spring` / `token-validation-ktor-v3`
- Node.js: `@navikt/oasis`

### ⚠️ Vanlig feil: Azure client_credentials med brukerkontext

```
❌ FEIL:
Innbygger → Frontend → [Azure client_credentials] → Backend API
Konsekvens: Mister hvem brukeren er, kan ikke autorisere per bruker

✅ RIKTIG:
Innbygger → Frontend → [TokenX exchange] → Backend API  
Konsekvens: Brukerens identitet følger med, kan autorisere per bruker
```

## Kommunikasjons-beslutningstre

### Synkron vs asynkron

| Behov | Mønster | Når bruke |
|-------|---------|-----------|
| Svar trengs umiddelbart | REST API | CRUD, oppslag, brukerinteraksjon |
| Fire-and-forget | Kafka-produsent | Varsling, logging, asynkron prosessering |
| Hendelsekoreografi | Rapids & Rivers | Flertjeneste-flyter, saga-mønster |
| Batch-prosessering | Naisjob + Kafka | Nattlige jobber, rapporter |

### Kafka-topicnavn

```
# Format: {team}.{domene}.v{versjon}
teamdagpenger.rapid.v1          # Rapids & Rivers
teamforeldrepenger.vedtak.v1    # Domene-hendelser
teamtiltak.saksbehandling.v1    # Domene-hendelser
```

### Rapids & Rivers mønster

```kotlin
// River — lytter på spesifikke hendelser
class VedtakRiver(rapidsConnection: RapidsConnection) : River.PacketListener {
    init {
        River(rapidsConnection).apply {
            validate { it.demandValue("@event_name", "vedtak_fattet") }
            validate { it.requireKey("vedtakId", "fnr", "fom", "tom") }
            validate { it.interestedIn("beløp") }
        }.register(this)
    }

    override fun onPacket(packet: JsonMessage, context: MessageContext) {
        val vedtakId = packet["vedtakId"].asText()
        // Prosesser hendelse
        
        // Publiser ny hendelse
        context.publish(JsonMessage.newMessage("utbetaling_beregnet", mapOf(
            "vedtakId" to vedtakId,
            "beløp" to beregnetBeløp
        )).toJson())
    }
}
```

## Database-beslutningstre

### PostgreSQL-konfigurasjon per miljø

```yaml
# Dev (Nais)
gcp:
  sqlInstances:
    - type: POSTGRES_15
      tier: db-f1-micro
      databases:
        - name: myapp-db

# Prod (Nais)
gcp:
  sqlInstances:
    - type: POSTGRES_15
      tier: db-custom-1-3840
      diskAutoresize: true
      highAvailability: true
      databases:
        - name: myapp-db
```

### HikariCP for containere

```kotlin
HikariDataSource().apply {
    maximumPoolSize = 3      // Start smått!
    minimumIdle = 1
    connectionTimeout = 10_000
    idleTimeout = 300_000
    maxLifetime = 1_800_000
    transactionIsolation = "TRANSACTION_READ_COMMITTED"
}
```

**Tommelregel:** `maxPoolSize = antall_replicas × 3 < max_connections (100)`

### Flyway-migrasjoner

```sql
-- V1__initial_schema.sql
CREATE TABLE vedtak (
    id BIGSERIAL PRIMARY KEY,
    fnr VARCHAR(11) NOT NULL,
    fom DATE NOT NULL,
    tom DATE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vedtak_fnr ON vedtak(fnr);
CREATE INDEX idx_vedtak_status ON vedtak(status);
```

## accessPolicy-beslutningstre

### Inbound (hvem kan kalle meg?)

```yaml
accessPolicy:
  inbound:
    rules:
      # Frontend kaller backend
      - application: mitt-frontend
      
      # Annen tjeneste i samme namespace
      - application: annen-tjeneste
      
      # Tjeneste i annet namespace
      - application: pdl-api
        namespace: pdl
      
      # Tjeneste i annet cluster
      - application: arena-adapter
        namespace: teamareana
        cluster: prod-fss
```

### Outbound (hvem kaller jeg?)

```yaml
accessPolicy:
  outbound:
    rules:
      # Intern tjeneste
      - application: pdl-api
        namespace: pdl
      
      # Tjeneste i annet cluster
      - application: oppgave
        namespace: oppgavehandtering
        cluster: prod-fss
    
    external:
      # Ekstern tjeneste (sjelden)
      - host: api.altinn.no
```

### ⚠️ Vanlige feil

```yaml
# ❌ FEIL: Ingen inbound — ingen kan kalle deg
accessPolicy:
  inbound:
    rules: []

# ❌ FEIL: Manglende outbound — kan ikke kalle avhengigheter
accessPolicy:
  outbound:
    rules: []

# ❌ FEIL: Feil namespace/cluster
accessPolicy:
  outbound:
    rules:
      - application: pdl-api  # Mangler namespace: pdl!
```

## Migreringsstrategi-beslutningstre

Bruk dette treet når du moderniserer eller endrer en eksisterende tjeneste.

### Steg 1: Hva endres?

```
Hva moderniserer du?
├── Database (skjema, migrasjon, plattform)
│   ├── Kolonne-endring i aktiv tabell
│   │   └── Trestegs feltmigrasjon (se modernization-patterns.md #1)
│   │       1. Legg til ny kolonne
│   │       2. Dual-write + les fra ny
│   │       3. Fjern gammel kolonne (separat PR)
│   │
│   ├── On-prem → Cloud SQL
│   │   └── pg_dump → GCS → Cloud SQL pipeline
│   │       Endre Nais-manifest: vault → gcp.sqlInstances
│   │       Feature toggle for cutover
│   │
│   └── Stor skjemaendring (millioner rader)
│       └── Online migrasjon med batchet backfill
│           ALTER TABLE ... ADD COLUMN (instant, ingen lås)
│           CREATE INDEX CONCURRENTLY (egen migrasjonsfil)
│           Batchet UPDATE med FOR UPDATE SKIP LOCKED
│
├── API (nytt format, ny tjeneste)
│   ├── Bakoverkompatibel endring
│   │   └── Legg til felt, behold gamle → ingen migrasjon nødvendig
│   │
│   └── Breaking change
│       └── Strangler fig
│           1. Ny tjeneste bak feature toggle
│           2. Gradvis utrulling (per bruker/team/org)
│           3. Redirect gammel → ny når 100%
│           4. Dekommisjonér gammel tjeneste
│
├── Kafka (skjema, topic)
│   ├── Bakoverkompatibelt (legg til optional felt)
│   │   └── interestedIn() i konsument — ingen breaking change
│   │
│   └── Breaking change (nytt skjema/topic)
│       └── Dual-write til v1 + v2
│           1. Produser til begge topics
│           2. Migrer konsumenter én om gangen
│           3. Stopp produksjon til v1
│           4. Dekommisjonér v1 topic
│
└── Frontend (framework, arkitektur)
    ├── Pages Router → App Router
    │   └── Migrer side for side, begge kan eksistere samtidig
    │
    └── Client → Server Components
        └── Flytt datahenting til server, behold interaktivitet som "use client"
```

### Steg 2: Utrullingsstrategi

```
Hvordan ruller du ut endringen?
├── Big bang (alt på én gang)
│   └── ⚠️ Kun for enkle, fullstendig bakoverkompatible endringer
│       Krever: god testdekning + rollback-plan
│
├── Gradvis med feature toggle (Unleash)
│   └── ✅ Anbefalt for de fleste endringer
│       Strategi: ByNavIdent → per team → alle
│       Krever: toggle-opprydding når ferdig!
│
├── Parallellkjøring (shadow/dual)
│   └── ✅ For kritisk forretningslogikk
│       Les fra gammel, skriv til begge, sammenlign
│       Krever: rekonsiliering + observerbarhet
│
└── Blue-green / canary
    └── Nais håndterer dette automatisk via RollingUpdate
        Bruk Recreate kun under database-migrasjoner
```

### Steg 3: Rollback og exit

```
Hva er rollback-planen?
├── Feature toggle → slå av toggle = umiddelbar rollback
├── Kafka dual-write → stopp skriving til ny topic
├── Database-migrasjon → ⚠️ Planlegg FØR du starter
│   └── Kan du rulle tilbake uten datatap?
│       ├── Ja → Ha ferdig rollback-migrasjon
│       └── Nei → Sørg for at gammel kode håndterer nytt skjema
│
Exit criteria (når er migreringen ferdig?):
├── Gammel path har 0 trafikk i 2+ uker
├── Feature toggles er fjernet fra kode og Unleash
├── Berørte team er informert
└── Dokumentasjon er oppdatert
```

## Feature toggle-beslutningstre

```
Trenger du feature toggle?
├── Endringen er bakoverkompatibel og lav-risiko
│   └── Nei, deploy direkte
│
├── Endringen påvirker brukeropplevelse
│   └── Ja, bruk Unleash
│       Strategi: Gradual rollout (%)
│
├── Endringen påvirker forretningslogikk
│   └── Ja, bruk Unleash
│       Strategi: ByNavIdent for testing → Gradual rollout
│
├── Endringen er en migrasjon mellom systemer
│   └── Ja, bruk Unleash
│       Strategi: ByNavIdent → per team → per org → alle
│       Legg til telemetri per toggle-tilstand
│
└── Endringen involverer andre teams konsumenter
    └── Ja, bruk Unleash + koordiner med teamene
        Strategi: Manuell aktivering per konsument-team

Opprydding:
├── Sett frist for fjerning av toggle (maks 3 mnd)
├── Logg en TODO/JIRA for opprydding ved opprettelse
└── Fjern toggle fra kode OG Unleash når migrering er ferdig
```

## Event-evolusjon-beslutningstre

```
Hvordan endrer du Kafka-hendelser?
├── Legg til nytt felt
│   └── Bakoverkompatibelt — bare legg til
│       Produsent: legg til felt i meldingen
│       Konsument: bruk interestedIn() (ikke requireKey)
│
├── Endre feltformat
│   ├── Kan gammelt og nytt eksistere samtidig?
│   │   ├── Ja → Legg til nytt felt, behold gammelt, migrer konsumenter
│   │   └── Nei → Ny topic-versjon (v2)
│   │
│   └── Ny topic: team.domene.v2
│       Dual-write fra produsent
│       Migrer konsumenter én om gangen
│
├── Fjerne felt
│   └── ⚠️ Breaking change
│       1. Sjekk at ingen konsumenter bruker feltet (requireKey/demandKey)
│       2. Fjern fra produsent
│       3. Vent 1 uke, verifiser at ingen feiler
│
├── Splitte topic
│   └── Ny topic per domene
│       1. Produser til gammel + ny
│       2. Migrer konsumenter
│       3. Dekommisjonér gammel topic
│
└── Ny hendelsestype
    └── Bare publiser med ny @event_name
        Eksisterende Rivers ignorerer ukjente event_names automatisk
```

## Teststrategi-beslutningstre

Velg riktig testnivå basert på hva som endres:

### Steg 1: Hva tester du?

```
Hva slags endring er dette?
├── Ny forretningslogikk (beregning, validering, transformasjon)
│   └── Unit-tester (Kotest / Vitest)
│       ✅ Rask feedback, isolert, billig
│       Dekk: normalflyt, edge cases, feilhåndtering
│
├── API-endring (nytt endepunkt, endret kontrakt)
│   ├── Controller-slice-test (@WebMvcTest / ktor testApplication)
│   │   Dekk: routing, validering, serialisering, feilkoder
│   │
│   └── Kontraktstest (hvis konsumenter i andre team)
│       Verifiser at responsen matcher forventet format
│       Spesielt viktig ved breaking changes
│
├── Database-endring (skjema, spørring, migrering)
│   └── Integrasjonstest med Testcontainers
│       ✅ Realistisk — kjører mot ekte PostgreSQL
│       Dekk: CRUD, migreringer, indekser, edge cases
│       Sjekk: Flyway-migrasjoner kjører uten feil
│
├── Auth-endring (ny mekanisme, ny tilgangskontroll)
│   └── Integrasjonstest med MockOAuth2Server
│       Dekk: gyldig token, utløpt token, feil issuer, manglende claims
│       ⚠️ Test ALLTID 401/403-scenarioer
│
├── Kafka-hendelse (ny produsent/konsument)
│   └── Enhetstest med TestRapid (Rapids & Rivers)
│       Dekk: gyldig melding, ugyldig melding, idempotens
│       Sjekk: publiserer riktig hendelse med riktige felt
│
├── Frontend-endring (komponent, side, interaksjon)
│   ├── Komponenttest (Testing Library + Vitest)
│   │   Dekk: rendering, brukerinteraksjon, tilstand
│   │
│   ├── E2E-test (Playwright) — for kritiske brukerreiser
│   │   Dekk: navigasjon, skjemainnsending, feilvisning
│   │
│   └── Tilgjengelighetstest (axe-core)
│       Dekk: WCAG 2.1 AA — kjør på alle sider
│
└── Modernisering / refaktorering av eksisterende kode
    └── Se «Teststrategi for endring» nedenfor
```

### Steg 2: Teststrategi for endring (brownfield)

```
Du endrer eksisterende kode — hva trenger du?

1. Karakteriseringstester (FØRST)
   └── Skriv tester som låser NÅVÆRENDE adferd
       Selv om adferden virker feil — lås den ned
       Gir deg sikkerhet til å endre koden etterpå
       Dekk: alle viktige kodestier i det som skal endres

2. Endringstester
   └── Skriv tester for NY adferd
       Disse vil feile inntil du implementerer endringen
       Dekk: ny adferd, nye edge cases

3. Regresjonstester
   └── Verifiser at UENDRET adferd fortsatt fungerer
       Kjør eksisterende testsuite
       Legg til tester for grenseflater mellom ny og gammel kode

4. Migreringsverifisering (hvis data endres)
   └── Test datamigrasjon med realistiske data
       Verifiser at gammel kode fungerer med nytt skjema
       Verifiser at ny kode fungerer med gammelt skjema (overgangsperiode)
```

### Steg 3: Hvor mye testing?

```
Dekningsgrad etter komponenttype:
├── lib/ (utility, beregning, transformasjon) → 80%+
├── Forretningslogikk (service-lag) → 70%+
├── API-ruter → Happy path + alle feilscenarioer
├── Database-repository → CRUD + edge cases
├── Kafka-konsumenter → Gyldig + ugyldig + idempotens
├── Frontend-komponenter → Rendering + interaksjon
└── E2E → Kritiske brukerreiser (ikke alt!)

⚠️ Test ikke implementasjonsdetaljer — test adferd
⚠️ Test ikke det Nais/plattformen allerede håndterer
```

## Endringskonsekvensanalyse-tre

Bruk dette treet for å kartlegge påvirkning FØR du implementerer:

```
Hva påvirker endringen din?

1. Direkte avhengigheter
   ├── Hvem kaller mitt API?
   │   Sjekk: accessPolicy.inbound i Nais-manifest
   │   Sjekk: Søk etter tjeneste-/endepunktnavnet i navikt org
   │
   ├── Hvem konsumerer mine Kafka-hendelser?
   │   Sjekk: Rapids & Rivers validate() med min @event_name
   │   Sjekk: Kafka-konsument-grupper for mitt topic
   │
   └── Hvem leser min database?
       Sjekk: Er det andre apper med tilgang til samme Cloud SQL?
       ⚠️ Delt database = høy risiko

2. Indirekte avhengigheter
   ├── Påvirker endringen data som vises til innbygger?
   │   → Test brukerreisen ende-til-ende
   │
   ├── Påvirker endringen data som brukes til utbetaling?
   │   → ⚠️ Høy risiko — krever ekstra review og testing
   │
   └── Påvirker endringen data som rapporteres eksternt?
       → Sjekk regelverk og rapporteringsfrister

3. Operasjonelle konsekvenser
   ├── Endres ressursbehov? (CPU, minne, disk)
   ├── Endres trafikkvolum? (flere/færre kall)
   ├── Endres latens? (nye downstream-kall)
   └── Endres feilmønstre? (nye feilkilder)

4. Team-påvirkning
   ├── Hvilke team må informeres?
   ├── Krever det koordinert deploy?
   └── Er det avhengighet til eksterne parter (Altinn, SSB)?
```
