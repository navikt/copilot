---
applyTo: "**/db/migration/**/*.sql"
---

# Database Migration Standards (Flyway)

Standarder for databasemigrasjoner med Flyway: navnekonvensjoner, sikre endringer og idempotente skript.

## Migration File Naming

Follow Flyway naming convention: `V{version}__{description}.sql`

### Examples

```
V1__initial_schema.sql
V2__add_status_column.sql
V3__add_user_indexes.sql
V4__alter_table_constraints.sql
```

### Rules

- Version numbers must be sequential (1, 2, 3, ...)
- Use double underscore `__` between version and description
- Description should be lowercase with underscores
- **NEVER modify existing migrations** - always create new ones

## Migration File Structure

```sql
-- V1__initial_schema.sql

CREATE TABLE rapporteringsperiode (
    id BIGSERIAL PRIMARY KEY,
    ident VARCHAR(11) NOT NULL,
    periode_id UUID NOT NULL,
    fom DATE NOT NULL,
    tom DATE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_ident ON rapporteringsperiode(ident);
CREATE INDEX idx_periode ON rapporteringsperiode(periode_id);
CREATE INDEX idx_fom_tom ON rapporteringsperiode(fom, tom);

-- Updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_updated_at
BEFORE UPDATE ON rapporteringsperiode
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();
```

## Best Practices

### Primary Keys

```sql
-- Use BIGSERIAL for auto-incrementing primary keys
id BIGSERIAL PRIMARY KEY,

-- Use UUID for distributed systems
id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
```

### Timestamps

```sql
-- Always include timestamps
created_at TIMESTAMP NOT NULL DEFAULT NOW(),
updated_at TIMESTAMP NOT NULL DEFAULT NOW()

-- Add trigger for automatic updated_at
CREATE TRIGGER update_updated_at
BEFORE UPDATE ON table_name
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();
```

### Indexes

```sql
-- Index foreign keys
CREATE INDEX idx_user_id ON orders(user_id);

-- Index frequently queried columns
CREATE INDEX idx_created_at ON orders(created_at);

-- Composite indexes for multi-column queries
CREATE INDEX idx_user_status ON orders(user_id, status);

-- Partial indexes for filtered queries
CREATE INDEX idx_active_orders ON orders(user_id)
WHERE status = 'active';
```

### Constraints

```sql
-- Foreign keys with ON DELETE CASCADE
user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,

-- Check constraints
CONSTRAINT check_positive_amount CHECK (amount > 0),
CONSTRAINT check_valid_status CHECK (status IN ('pending', 'active', 'completed')),

-- Unique constraints
CONSTRAINT unique_email UNIQUE (email),
CONSTRAINT unique_user_period UNIQUE (user_id, period_id)
```

### Data Types

```sql
-- Prefer specific types
VARCHAR(n)      -- For strings with known max length
TEXT            -- For strings with unknown length
BIGINT          -- For large numbers
NUMERIC(10,2)   -- For decimal numbers (money)
TIMESTAMP       -- For date/time
DATE            -- For dates only
BOOLEAN         -- For true/false
UUID            -- For unique identifiers
JSONB           -- For structured JSON data
```

## Migration Patterns

### Adding a Column

```sql
-- V2__add_status_column.sql

ALTER TABLE rapporteringsperiode
ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending';

CREATE INDEX idx_status ON rapporteringsperiode(status);
```

### Adding a Table with Foreign Key

```sql
-- V3__create_aktivitet_table.sql

CREATE TABLE aktivitet (
    id BIGSERIAL PRIMARY KEY,
    rapporteringsperiode_id BIGINT NOT NULL REFERENCES rapporteringsperiode(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    dato DATE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rapporteringsperiode_id ON aktivitet(rapporteringsperiode_id);
CREATE INDEX idx_dato ON aktivitet(dato);
```

### Altering a Column

```sql
-- V4__alter_ident_length.sql

ALTER TABLE rapporteringsperiode
ALTER COLUMN ident TYPE VARCHAR(20);
```

## Kotlin Integration

```kotlin
object PostgresDataSourceBuilder {
    val dataSource by lazy {
        HikariDataSource().apply {
            jdbcUrl = getOrThrow(DB_URL_KEY)
            maximumPoolSize = 5 // Start low in K8s; scale up if needed
            minimumIdle = 1
        }
    }

    fun runMigration() {
        Flyway.configure()
            .dataSource(dataSource)
            .load()
            .migrate()
    }
}

// Run migrations on startup
fun main() {
    PostgresDataSourceBuilder.runMigration()
    // Start application
}
```

## PostgreSQL Query Optimization

### EXPLAIN ANALYZE

Always analyze new or changed queries:

```sql
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT * FROM vedtak
WHERE bruker_id = '12345678901' AND status = 'aktiv'
ORDER BY opprettet_dato DESC LIMIT 10;
```

Red flags: `Seq Scan` on large tables, `Sort external merge`, high discrepancy between estimated/actual rows.

### JSONB Patterns

```sql
-- GIN index for containment queries
CREATE INDEX idx_metadata_gin ON hendelser USING GIN (metadata);

-- Query JSONB
SELECT * FROM hendelser WHERE metadata @> '{"type": "vedtak"}';

-- Extract fields
SELECT id, metadata->>'type' AS type FROM hendelser;
```

### Window Functions

```sql
-- Latest per group
SELECT * FROM (
    SELECT *, ROW_NUMBER() OVER (
        PARTITION BY bruker_id ORDER BY opprettet_dato DESC
    ) AS rn
    FROM vedtak
) sub WHERE rn = 1;
```

### Large Table Migrations

```sql
-- Add column with default (instant in PostgreSQL 11+)
ALTER TABLE stor_tabell ADD COLUMN ny_kolonne BOOLEAN DEFAULT false;

-- Standard Flyway migration (runs in a transaction)
CREATE INDEX idx_ny ON stor_tabell(ny_kolonne);

-- Use CREATE INDEX CONCURRENTLY only in its own dedicated migration
-- with no other statements in the file.
-- Example Flyway migration:
--      V5__add_idx_ny_concurrently.sql
-- Example migration content:
--      CREATE INDEX CONCURRENTLY idx_ny ON stor_tabell(ny_kolonne);
-- This migration must run without being wrapped in a transaction,
-- so configure Flyway's transaction handling accordingly for your setup.
```

## Testing Migrations

```kotlin
@Testcontainers
class MigrationTest {
    companion object {
        @Container
        val postgres = PostgreSQLContainer<Nothing>("postgres:15")
    }

    @Test
    fun `migrations should run successfully`() {
        val dataSource = HikariDataSource().apply {
            jdbcUrl = postgres.jdbcUrl
            username = postgres.username
            password = postgres.password
        }

        val flyway = Flyway.configure()
            .dataSource(dataSource)
            .load()

        val result = flyway.migrate()
        result.migrationsExecuted shouldBeGreaterThan 0
    }
}
```

## Boundaries

### ✅ Always

- Follow V{n}\_\_{description}.sql naming
- Add indexes for foreign keys
- Include created_at and updated_at timestamps
- Use appropriate data types
- Test migrations in dev environment first

### ⚠️ Ask First

- Schema changes affecting multiple tables
- Dropping columns or tables
- Changing primary keys
- Large data migrations

### 🚫 Never

- Modify existing migration files
- Skip version numbers
- Use single underscore in naming
- Deploy untested migrations to production
- Commit migration files without testing

## Related

| Resource | Use For |
|----------|---------|
| `flyway-migration` skill | Flyway migration patterns and best practices |
| `@nais-agent` | GCP Cloud SQL configuration in Nais manifests |
| `postgresql-review` skill | Query optimization and indexing strategy |
