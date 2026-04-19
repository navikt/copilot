# Moderniseringsmønstre for Nav

Konkrete mønstre for vanlige moderniseringsscenarioer i Nav. Bruk sammen med beslutningstrærne i [decision-trees.md](./decision-trees.md).

## 1. Trestegs feltmigrasjon (safe column change)

Bruk når du endrer, omdøper eller fjerner en kolonne i en aktiv tabell.

### Steg A: Legg til ny kolonne, backfill asynkront

```sql
-- V12__add_new_status_column.sql
ALTER TABLE vedtak ADD COLUMN status_v2 VARCHAR(30);

-- Populer fra gammel kolonne (for små tabeller)
UPDATE vedtak SET status_v2 = CASE
  WHEN status = 'pending' THEN 'UNDER_BEHANDLING'
  WHEN status = 'active' THEN 'AKTIV'
  ELSE 'UKJENT'
END;
```

```kotlin
// For store tabeller — batchet backfill via Naisjob
class BackfillStatusTask(private val dataSource: DataSource) {
    fun execute() {
        using(sessionOf(dataSource)) { session ->
            var updated = 0
            do {
                val batch = session.run(queryOf("""
                    UPDATE vedtak SET status_v2 = CASE
                        WHEN status = 'pending' THEN 'UNDER_BEHANDLING'
                        WHEN status = 'active' THEN 'AKTIV'
                        ELSE 'UKJENT'
                    END
                    WHERE status_v2 IS NULL
                    LIMIT 1000
                """).asUpdate)
                updated = batch
            } while (updated > 0)
        }
    }
}
```

### Steg B: Dual-write + les fra ny kolonne

```kotlin
// Skriv til begge kolonner under overgangen
fun oppdaterStatus(id: Long, nyStatus: String) {
    using(sessionOf(dataSource)) { session ->
        session.run(queryOf("""
            UPDATE vedtak SET status = ?, status_v2 = ? WHERE id = ?
        """, gammelMapping(nyStatus), nyStatus, id).asUpdate)
    }
}

// Les fra ny kolonne
fun hentStatus(id: Long): String? =
    session.run(queryOf("SELECT status_v2 FROM vedtak WHERE id = ?", id)
        .map { it.string("status_v2") }.asSingle)
```

### Steg C: Fjern gammel kolonne (separat PR, etter at alt er verifisert)

```sql
-- V14__remove_old_status_column.sql
ALTER TABLE vedtak DROP COLUMN status;
ALTER TABLE vedtak RENAME COLUMN status_v2 TO status;
```

**Exit criteria:** Ingen kode refererer til gammel kolonne, ingen queries bruker gammel kolonne, telemetri bekrefter at ny kolonne brukes 100%.

---

## 2. Strangler fig med redirect

Bruk når du erstatter en eksisterende tjeneste gradvis.

```
Bruker → Ingress/Proxy → [Feature toggle]
                         ├── gammel tjeneste (default)
                         └── ny tjeneste (gradvis utrulling)
```

### Mønster: Redirect-server

```kotlin
// RedirectServer.kt — enkel Ktor-app som ruter basert på toggle
fun Application.configureRouting(unleash: Unleash) {
    routing {
        get("/{path...}") {
            val path = call.parameters.getAll("path")?.joinToString("/") ?: ""
            val target = if (unleash.isEnabled("ny-tjeneste-aktiv")) {
                "https://ny-tjeneste.intern.nav.no/$path"
            } else {
                "https://gammel-tjeneste.intern.nav.no/$path"
            }
            call.respondRedirect(target, permanent = false)
        }
    }
}
```

### Mønster: Gradvis API-migrasjon

```kotlin
// Router som sender trafikk til gammel eller ny implementasjon
class MigrerendeService(
    private val gammel: GammelService,
    private val ny: NyService,
    private val unleash: Unleash
) {
    fun hentVedtak(id: String): Vedtak {
        return if (unleash.isEnabled("bruk-ny-vedtak-service")) {
            ny.hentVedtak(id)
        } else {
            gammel.hentVedtak(id)
        }
    }
}
```

**Exit criteria:** 100% trafikk på ny tjeneste i 2+ uker, ingen feil, gammel tjeneste kan dekommisjoneres.

---

## 3. Feature toggle med Unleash

Nav bruker Unleash for gradvis utrulling. Standard oppsett:

### Nais-konfigurasjon

```yaml
# nais.yaml
spec:
  env:
    - name: UNLEASH_SERVER_API_URL
      value: https://unleash.nais.io/api
    - name: UNLEASH_SERVER_API_TOKEN
      value: ${UNLEASH_API_TOKEN}
```

### Nav-spesifikke strategier

```kotlin
// ByNavIdentStrategy — rull ut per bruker
enum class FeatureToggle(val key: String) {
    NY_VEDTAKSFLYT("team-dagpenger.ny-vedtaksflyt"),
    MODERNISERT_API("team-dagpenger.modernisert-api"),
}

fun erAktivert(toggle: FeatureToggle, navIdent: String? = null): Boolean {
    val context = UnleashContext.builder()
        .apply { navIdent?.let { addProperty("navIdent", it) } }
        .build()
    return unleash.isEnabled(toggle.key, context)
}
```

### Beste praksis

- **Navngivning:** `{team}.{feature}` — f.eks. `team-dagpenger.ny-vedtaksflyt`
- **Opprydding:** Fjern toggles når migrering er fullført (teknisk gjeld!)
- **Telemetri:** Mål ytelse/feil per toggle-tilstand for sammenligning
- **Rollback:** Toggle av = umiddelbar rollback uten deploy

---

## 4. Kafka-topicmigrering

Bruk når du endrer skjema eller splittar topics.

### Evolusjon innenfor topic (bakoverkompatibelt)

```kotlin
// Legg til nye felt som optional — eksisterende konsumenter ignorerer dem
River(rapidsConnection).apply {
    validate { it.demandValue("@event_name", "vedtak_fattet") }
    validate { it.requireKey("vedtakId", "fnr") }
    validate { it.interestedIn("begrunnelse") }  // Nytt optional felt
}.register(this)
```

### Migrering til ny topic-versjon (breaking change)

```
Steg 1: Produser til BEGGE topics (v1 + v2)
Steg 2: Migrer konsumenter til v2 (én om gangen)
Steg 3: Stopp produksjon til v1 når alle er migrert
Steg 4: Dekommisjonér v1 topic
```

```kotlin
// Dual-write til begge versjoner
class VedtakPublisher(private val rapid: RapidsConnection) {
    fun publiser(vedtak: Vedtak) {
        // v1 — gammel format (beholdes under migrasjon)
        rapid.publish(JsonMessage.newMessage("vedtak_fattet", mapOf(
            "vedtakId" to vedtak.id,
            "fnr" to vedtak.fnr,
        )).toJson())

        // v2 — nytt format
        rapid.publish(JsonMessage.newMessage("vedtak_fattet_v2", mapOf(
            "vedtakId" to vedtak.id,
            "fnr" to vedtak.fnr,
            "begrunnelse" to vedtak.begrunnelse,
            "saksbehandler" to vedtak.saksbehandler,
        )).toJson())
    }
}
```

**Exit criteria:** Ingen konsumenter leser v1, topic kan slettes.

---

## 5. Database: On-prem til Cloud SQL

Standard migreringspipeline i Nav:

```bash
# 1. Dump fra on-prem
pg_dump -h on-prem-host -U user -d mydb -Fc > dump.sql

# 2. Last opp til GCS
gsutil cp dump.sql gs://migration-bucket/mydb/

# 3. Importer til Cloud SQL
gcloud sql import sql mydb-instance gs://migration-bucket/mydb/dump.sql \
  --database=mydb

# 4. Verifiser
psql -h cloud-sql-proxy -U user -d mydb -c "SELECT count(*) FROM vedtak;"
```

### Nais-manifestendringer

```yaml
# FØR (on-prem med Vault)
spec:
  vault:
    enabled: true
    paths:
      - mountPath: /var/run/secrets/nais.io/db
        kvPath: /serviceuser/data/prod/srv-myapp

# ETTER (GCP med Cloud SQL)
spec:
  gcp:
    sqlInstances:
      - type: POSTGRES_15
        tier: db-custom-1-3840
        diskAutoresize: true
        highAvailability: true
        insights:
          enabled: true
          queryStringLength: 4500
          recordApplicationTags: true
        maintenance:
          day: 1
          hour: 4
        databases:
          - name: mydb
```

---

## 6. Karakteriseringstester før refaktorering

Lås nåværende adferd før du endrer noe. Bruk TestContainers med gjenbruk for rask feedback.

### Oppsett

```properties
# .testcontainers.properties
testcontainers.reuse.enable=true
```

```kotlin
// Delt test-database med gjenbruk
object TestDatabase {
    val container = PostgreSQLContainer<Nothing>("postgres:15").apply {
        withDatabaseName("testdb")
        withReuse(true)
        start()
    }

    val dataSource = HikariDataSource().apply {
        jdbcUrl = container.jdbcUrl
        username = container.username
        password = container.password
        maximumPoolSize = 3
    }

    init {
        Flyway.configure().dataSource(dataSource).load().migrate()
    }
}
```

### Golden master-test

```kotlin
@Test
fun `vedtakservice returnerer samme resultat før og etter refaktorering`() {
    // Fyll inn kjent testdata
    insertTestVedtak(id = 1, fnr = "12345678901", status = "AKTIV")

    // Lagre nåværende output som "golden master"
    val expected = gammelService.hentVedtak(1)

    // Kjør ny implementasjon
    val actual = nyService.hentVedtak(1)

    // Verifiser at output er identisk
    actual shouldBe expected
}
```

### Rekonsiliering ved dual-write

```kotlin
// Periodisk jobb som sammenligner gammel og ny kilde
class RekonsilieringsJobb(
    private val gammel: GammelRepository,
    private val ny: NyRepository,
    private val meterRegistry: MeterRegistry,
) {
    private val avvikCounter = Counter.builder("rekonsiliering_avvik_total")
        .register(meterRegistry)

    fun kjør() {
        val gamleVedtak = gammel.hentAlle()
        val nyeVedtak = ny.hentAlle()

        val avvik = gamleVedtak.filter { g ->
            val n = nyeVedtak.find { it.id == g.id }
            n == null || n != g
        }

        avvikCounter.increment(avvik.size.toDouble())
        if (avvik.isNotEmpty()) {
            logger.warn { "Rekonsiliering fant ${avvik.size} avvik" }
        }
    }
}
```

---

## 7. Online skjemaendring for store tabeller

For tabeller med millioner rader — unngå låser.

```sql
-- Legg til kolonne med default (instant i PostgreSQL 11+, ingen lås)
ALTER TABLE stor_tabell ADD COLUMN ny_kolonne BOOLEAN DEFAULT false;

-- Opprett indeks uten å låse tabellen
-- VIKTIG: Må være i egen migrasjonsfil uten transaksjon
-- V15__add_index_concurrently.sql
CREATE INDEX CONCURRENTLY idx_ny_kolonne ON stor_tabell(ny_kolonne);
```

### Batchet backfill uten lås

```kotlin
fun backfillBatch(batchSize: Int = 5000): Int {
    return using(sessionOf(dataSource)) { session ->
        session.run(queryOf("""
            UPDATE stor_tabell
            SET ny_kolonne = true
            WHERE id IN (
                SELECT id FROM stor_tabell
                WHERE ny_kolonne IS NULL
                LIMIT ?
                FOR UPDATE SKIP LOCKED
            )
        """, batchSize).asUpdate)
    }
}
```

**`FOR UPDATE SKIP LOCKED`** — hopper over rader som er låst av andre transaksjoner, unngår venting.

---

## Dekommisjonering — sjekkliste

Når migrering er fullført, verifiser disse før du fjerner gammel kode/infrastruktur:

- [ ] **Telemetri:** Gammel path har 0 trafikk i 2+ uker
- [ ] **Feature toggles:** Alle toggles for migrering er fjernet fra kode og Unleash
- [ ] **Konsumenter:** Ingen tjenester leser fra gammel topic/API
- [ ] **Data:** Gammel database/tabell er tom eller arkivert
- [ ] **accessPolicy:** Ingen tjenester har gammel tjeneste i outbound
- [ ] **DNS/ingress:** Gammel URL returnerer 410 Gone eller redirect
- [ ] **Dokumentasjon:** Oppdatert README, ADR markert som erstattet
- [ ] **Team:** Berørte team er informert
