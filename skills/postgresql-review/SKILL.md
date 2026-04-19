---
name: postgresql-review
description: PostgreSQL query review, optimalisering og beste praksis for Nav-applikasjoner
license: MIT
compatibility: PostgreSQL database
metadata:
  domain: backend
  tags: postgresql sql optimization review indexing
---

# PostgreSQL Review Skill

Review and optimize PostgreSQL queries, schemas, and patterns for Nav applications. Covers EXPLAIN analysis, index strategies, JSONB patterns, and common anti-patterns.

## Query Analysis

Run `EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)` to analyze queries:

```sql
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT * FROM vedtak
WHERE bruker_id = '12345678901'
  AND status = 'aktiv'
ORDER BY opprettet_dato DESC
LIMIT 10;
```

### Red Flags in EXPLAIN Output

| Sign | Problem | Solution |
|---|---|---|
| `Seq Scan` on large table | Missing index | `CREATE INDEX` |
| `Sort` with `external merge` | Not enough `work_mem` | Increase `work_mem` or add index with correct sort order |
| `Nested Loop` with high `rows` | Cartesian product / missing join index | Add index on join column |
| `Hash Join` with `Batches > 1` | `work_mem` too low | Increase `work_mem` for the session |
| Large difference between `estimated` and `actual` rows | Outdated statistics | `ANALYZE tablename;` |

## Index Strategies

```sql
-- Simple index for lookups
CREATE INDEX idx_vedtak_bruker_id ON vedtak(bruker_id);

-- Composite index — columns in order of selectivity
CREATE INDEX idx_vedtak_bruker_status ON vedtak(bruker_id, status);

-- Partial index — only relevant rows
CREATE INDEX idx_vedtak_aktive ON vedtak(bruker_id)
WHERE status = 'aktiv';

-- Covering index — avoids table lookup
CREATE INDEX idx_vedtak_covering ON vedtak(bruker_id, status)
INCLUDE (opprettet_dato, belop);

-- Concurrent — no table locking (requires outside transaction)
CREATE INDEX CONCURRENTLY idx_vedtak_dato ON vedtak(opprettet_dato);
```

### When to Use What?

| Scenario | Index Type |
|---|---|
| `WHERE a = x` | B-tree on `a` |
| `WHERE a = x AND b = y` | Composite `(a, b)` |
| `WHERE a = x AND status = 'aktiv'` | Partial index `WHERE status = 'aktiv'` |
| `WHERE a LIKE 'prefix%'` | B-tree (prefix only) |
| `WHERE a @> '{"key": "val"}'` | GIN on JSONB |
| Full-text search | GIN with `to_tsvector` |
| Geography | GiST |

## JSONB Patterns

```sql
-- ✅ Correct — GIN index for JSONB queries
CREATE INDEX idx_metadata_gin ON hendelser USING GIN (metadata);

-- Query JSONB
SELECT * FROM hendelser
WHERE metadata @> '{"type": "vedtak", "tema": "dagpenger"}';

-- Fetch nested values
SELECT
    id,
    metadata->>'type' AS type,
    metadata->'detaljer'->>'belop' AS belop
FROM hendelser;

-- ❌ Wrong — casting in WHERE without index
SELECT * FROM hendelser
WHERE (metadata->>'opprettet')::timestamp > NOW() - INTERVAL '7 days';
-- ✅ Better — use expression index
CREATE INDEX idx_metadata_opprettet ON hendelser (((metadata->>'opprettet')::timestamp));
```

## Common Table Expressions (CTEs)

```sql
-- ✅ Correct — CTE for readability
WITH aktive_vedtak AS (
    SELECT bruker_id, COUNT(*) AS antall
    FROM vedtak
    WHERE status = 'aktiv'
    GROUP BY bruker_id
),
siste_aktivitet AS (
    SELECT bruker_id, MAX(opprettet_dato) AS sist_aktiv
    FROM aktivitetslogg
    GROUP BY bruker_id
)
SELECT
    av.bruker_id,
    av.antall,
    sa.sist_aktiv
FROM aktive_vedtak av
JOIN siste_aktivitet sa USING (bruker_id)
WHERE av.antall > 1;
```

## Window Functions

```sql
-- Ranking within group
SELECT
    bruker_id,
    vedtak_id,
    opprettet_dato,
    ROW_NUMBER() OVER (PARTITION BY bruker_id ORDER BY opprettet_dato DESC) AS rn
FROM vedtak
WHERE rn = 1;  -- Latest vedtak per user

-- Running total
SELECT
    dato,
    antall,
    SUM(antall) OVER (ORDER BY dato) AS kumulativt
FROM daglig_statistikk;
```

## Anti-patterns

### N+1 Queries

```kotlin
// ❌ Wrong — N+1: one query per user
val brukere = repository.findAll()
brukere.forEach { bruker ->
    val vedtak = vedtakRepository.findByBrukerId(bruker.id)  // N extra queries
}

// ✅ Correct — JOIN or batch query
val brukereOgVedtak = repository.findAllWithVedtak()  // Single query with JOIN
```

### SELECT *

```sql
-- ❌ Wrong — fetches all columns incl. large JSONB/TEXT
SELECT * FROM dokument WHERE bruker_id = '12345';

-- ✅ Correct — only necessary columns
SELECT id, tittel, opprettet_dato FROM dokument WHERE bruker_id = '12345';
```

### Missing LIMIT on Unbounded Data

```sql
-- ❌ Wrong — can return millions of rows
SELECT * FROM hendelse WHERE type = 'innlogging';

-- ✅ Correct — always limit result set
SELECT * FROM hendelse WHERE type = 'innlogging'
ORDER BY opprettet_dato DESC
LIMIT 100;
```

## Connection Pooling

```kotlin
// HikariCP — recommended configuration for Nais
HikariDataSource().apply {
    jdbcUrl = System.getenv("DB_JDBC_URL")
        ?: "jdbc:postgresql://${System.getenv("DB_HOST")}:5432/${System.getenv("DB_DATABASE")}"
    username = System.getenv("DB_USERNAME")
    password = System.getenv("DB_PASSWORD")
    maximumPoolSize = 5       // Nais: start low, scale up as needed
    minimumIdle = 1
    connectionTimeout = 10_000
    idleTimeout = 300_000
    maxLifetime = 600_000
    validationTimeout = 5_000
}
```

## Migration Strategies for Large Tables

```sql
-- Add column with default (PostgreSQL 11+ — instant, no rewrite)
ALTER TABLE stor_tabell ADD COLUMN ny_kolonne BOOLEAN DEFAULT false;

-- Create index without locking the table
CREATE INDEX CONCURRENTLY idx_ny ON stor_tabell(ny_kolonne);

-- Batch update (avoid long transaction)
-- Run in application code with batches of 10,000 rows:
UPDATE stor_tabell SET ny_kolonne = true WHERE id BETWEEN $1 AND $2;
```

## Checklist

- [ ] Do all `WHERE` columns have indexes?
- [ ] Has `EXPLAIN ANALYZE` been run for new/changed queries?
- [ ] Are we avoiding `SELECT *` in production code?
- [ ] Do we have `LIMIT` on queries that can return many rows?
- [ ] Are JSONB columns indexed with GIN?
- [ ] Is connection pool size appropriate (5-10 for Nais)?
- [ ] Are migrations on large tables run with `CONCURRENTLY`?
