package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

var (
	// ErrRateLimitExceeded is returned when an actor exceeds the 10 submits per hour limit.
	ErrRateLimitExceeded = errors.New("rate limit exceeded: max 10 submits per actor per hour")
	// ErrVersionNotFound is returned when the specified version of an entry does not exist.
	ErrVersionNotFound = errors.New("version not found")
)

// DBEntry mirrors apps/mcp-registry's representation of an approved entry.
type DBEntry struct {
	Name  string
	Entry []byte
}

// Store handles persistent storage and transactions for mcp-registry-writer.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore initializes a new Store with a pgxpool.
func NewStore(ctx context.Context, dbURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parsing db url: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating pgx connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close closes the database connection pool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Ping verifies the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	if s.pool == nil {
		return errors.New("database pool is not initialized")
	}
	return s.pool.Ping(ctx)
}

// Migrate applies embedded sql migrations using a schema_migrations table.
func (s *Store) Migrate(ctx context.Context) error {
	if s.pool == nil {
		return errors.New("database pool is not initialized")
	}

	createMigrationsTable := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);`
	if _, err := s.pool.Exec(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	var filenames []string
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 4 && entry.Name()[len(entry.Name())-4:] == ".sql" {
			filenames = append(filenames, entry.Name())
		}
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		if err := s.applyMigration(ctx, filename); err != nil {
			return fmt.Errorf("applying migration %s: %w", filename, err)
		}
	}

	return nil
}

func (s *Store) applyMigration(ctx context.Context, filename string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var exists bool
	err = tx.QueryRow(ctx, "SELECT true FROM schema_migrations WHERE version = $1", filename).Scan(&exists)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("checking migration status: %w", err)
	}
	if exists {
		return nil
	}

	content, err := migrationFS.ReadFile("migrations/" + filename)
	if err != nil {
		return fmt.Errorf("reading migration file: %w", err)
	}

	if _, err := tx.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("executing migration sql: %w", err)
	}

	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", filename); err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}

	return tx.Commit(ctx)
}

// Submit creates a new version of an entry in registry_entry_versions,
// updates registry_entries (keeping an approved version served if present,
// or setting status='pending' otherwise), and logs an audit entry.
// Returns the newly created version number.
func (s *Store) Submit(ctx context.Context, actor string, entryJSON []byte, name string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Rate limit: max 10 submits per actor per hour, counted from registry_audit
	var submitCount int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM registry_audit
		WHERE actor = $1
		  AND action = 'submit'
		  AND at > now() - INTERVAL '1 hour'
	`, actor).Scan(&submitCount)
	if err != nil {
		return 0, fmt.Errorf("checking rate limit: %w", err)
	}
	if submitCount >= 10 {
		return 0, ErrRateLimitExceeded
	}

	// Compute next version
	var maxVersion int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0)
		FROM registry_entry_versions
		WHERE name = $1
	`, name).Scan(&maxVersion)
	if err != nil {
		return 0, fmt.Errorf("querying max version: %w", err)
	}
	newVersion := maxVersion + 1

	// Insert into registry_entry_versions (append-only)
	_, err = tx.Exec(ctx, `
		INSERT INTO registry_entry_versions (name, version, entry, submitted_by)
		VALUES ($1, $2, $3, $4)
	`, name, newVersion, entryJSON, actor)
	if err != nil {
		return 0, fmt.Errorf("inserting entry version: %w", err)
	}

	// Lock and check current entry row
	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM registry_entries
		WHERE name = $1
		FOR UPDATE
	`, name).Scan(&currentStatus)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// No existing row: create with status pending
		_, err = tx.Exec(ctx, `
			INSERT INTO registry_entries (name, status, entry, version, pending_version, updated_at)
			VALUES ($1, 'pending', $2, $3, NULL, now())
		`, name, entryJSON, newVersion)
		if err != nil {
			return 0, fmt.Errorf("inserting new registry entry: %w", err)
		}
	case err != nil:
		return 0, fmt.Errorf("locking registry entry: %w", err)
	case currentStatus == "approved":
		// Existing row is approved: keep it served and set pending_version
		_, err = tx.Exec(ctx, `
			UPDATE registry_entries
			SET pending_version = $2,
			    updated_at = now()
			WHERE name = $1
		`, name, newVersion)
		if err != nil {
			return 0, fmt.Errorf("updating pending version for approved entry: %w", err)
		}
	default:
		// Previous status was pending or rejected: switch/update to pending
		_, err = tx.Exec(ctx, `
			UPDATE registry_entries
			SET status = 'pending',
			    entry = $2,
			    version = $3,
			    pending_version = NULL,
			    updated_at = now()
			WHERE name = $1
		`, name, entryJSON, newVersion)
		if err != nil {
			return 0, fmt.Errorf("updating pending registry entry: %w", err)
		}
	}

	// Audit row
	_, err = tx.Exec(ctx, `
		INSERT INTO registry_audit (actor, action, name, version, reason, at)
		VALUES ($1, 'submit', $2, $3, NULL, now())
	`, actor, name, newVersion)
	if err != nil {
		return 0, fmt.Errorf("inserting audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}

	return newVersion, nil
}

// Approve copies the specified version's entry from registry_entry_versions
// into registry_entries with status approved, and logs an audit entry.
func (s *Store) Approve(ctx context.Context, actor, name string, version int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Fetch version entry
	var entryJSON []byte
	err = tx.QueryRow(ctx, `
		SELECT entry
		FROM registry_entry_versions
		WHERE name = $1 AND version = $2
	`, name, version).Scan(&entryJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVersionNotFound
	}
	if err != nil {
		return fmt.Errorf("fetching version entry: %w", err)
	}

	// Upsert registry_entries with approved version
	_, err = tx.Exec(ctx, `
		INSERT INTO registry_entries (name, status, entry, version, pending_version, updated_at)
		VALUES ($1, 'approved', $2, $3, NULL, now())
		ON CONFLICT (name) DO UPDATE
		SET status = 'approved',
		    entry = EXCLUDED.entry,
		    version = EXCLUDED.version,
		    pending_version = CASE
		        WHEN registry_entries.pending_version = EXCLUDED.version THEN NULL
		        ELSE registry_entries.pending_version
		    END,
		    updated_at = now()
	`, name, entryJSON, version)
	if err != nil {
		return fmt.Errorf("upserting approved entry: %w", err)
	}

	// Audit row
	_, err = tx.Exec(ctx, `
		INSERT INTO registry_audit (actor, action, name, version, reason, at)
		VALUES ($1, 'approve', $2, $3, NULL, now())
	`, actor, name, version)
	if err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}

	return tx.Commit(ctx)
}

// Reject records a rejection audit row, clears pending_version if it matches,
// and marks registry_entries status='rejected' if the currently served version was pending.
func (s *Store) Reject(ctx context.Context, actor, name string, version int, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Verify that the specified version exists
	var exists bool
	err = tx.QueryRow(ctx, `
		SELECT true
		FROM registry_entry_versions
		WHERE name = $1 AND version = $2
	`, name, version).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVersionNotFound
	}
	if err != nil {
		return fmt.Errorf("verifying version existence: %w", err)
	}

	// Update registry_entries
	_, err = tx.Exec(ctx, `
		UPDATE registry_entries
		SET status = CASE
		        WHEN version = $2 AND status = 'pending' THEN 'rejected'
		        ELSE status
		    END,
		    pending_version = CASE
		        WHEN pending_version = $2 THEN NULL
		        ELSE pending_version
		    END,
		    updated_at = now()
		WHERE name = $1
	`, name, version)
	if err != nil {
		return fmt.Errorf("updating entry on rejection: %w", err)
	}

	// Audit row with reason
	_, err = tx.Exec(ctx, `
		INSERT INTO registry_audit (actor, action, name, version, reason, at)
		VALUES ($1, 'reject', $2, $3, $4, now())
	`, actor, name, version, reason)
	if err != nil {
		return fmt.Errorf("inserting rejection audit: %w", err)
	}

	return tx.Commit(ctx)
}

// GetApprovedEntries queries the database matching the reader's query:
// SELECT name, entry FROM registry_entries WHERE status = 'approved' ORDER BY name.
func (s *Store) GetApprovedEntries(ctx context.Context) ([]DBEntry, error) {
	rows, err := s.pool.Query(ctx, "SELECT name, entry FROM registry_entries WHERE status = 'approved' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("querying approved registry_entries: %w", err)
	}
	defer rows.Close()

	var entries []DBEntry
	for rows.Next() {
		var e DBEntry
		if err := rows.Scan(&e.Name, &e.Entry); err != nil {
			return nil, fmt.Errorf("scanning registry entry row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating registry entry rows: %w", err)
	}
	return entries, nil
}
