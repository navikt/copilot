package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

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
	// ErrForbidden is returned when a non-admin actor who is not the original submitter attempts to submit a new version.
	ErrForbidden = errors.New("forbidden: only the original submitter or an admin can submit new versions")
	// ErrConflict is returned when an approve or reject targets a non-pending version.
	ErrConflict = errors.New("conflict: version is not pending approval")
	// ErrAlreadyApproved is returned when attempting to reject an already approved version.
	ErrAlreadyApproved = errors.New("cannot reject an already approved version; revocation comes later")
)

// DB holds the write-side storage: entries, versions and audit.
type DB struct {
	pool *pgxpool.Pool
}

// OpenDB connects to Postgres and pings it.
func OpenDB(ctx context.Context, dbURL string) (*DB, error) {
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

	return &DB{pool: pool}, nil
}

// Close closes the database connection pool.
func (s *DB) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Ping verifies the database connection is alive.
func (s *DB) Ping(ctx context.Context) error {
	if s.pool == nil {
		return errors.New("database pool is not initialized")
	}
	return s.pool.Ping(ctx)
}

// Migrate applies embedded sql migrations using a schema_migrations table.
func (s *DB) Migrate(ctx context.Context) error {
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

func (s *DB) applyMigration(ctx context.Context, filename string) error {
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
func (s *DB) Submit(ctx context.Context, actor string, entryJSON []byte, name string, isAdmin bool) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Advisory locks first, in a fixed order (actor, then name), so the rate
	// limit count and MAX(version) below see every committed competitor.
	for _, key := range []string{"actor:" + actor, "name:" + name} {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", key); err != nil {
			return 0, fmt.Errorf("acquiring advisory lock: %w", err)
		}
	}

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

	// Check ownership if existing name
	var v1SubmittedBy string
	err = tx.QueryRow(ctx, `
		SELECT submitted_by
		FROM registry_entry_versions
		WHERE name = $1 AND version = 1
	`, name).Scan(&v1SubmittedBy)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("checking ownership: %w", err)
	}
	if err == nil {
		// Existing name: only actor who submitted v1 or admin is allowed
		if !strings.EqualFold(actor, v1SubmittedBy) && !isAdmin {
			return 0, ErrForbidden
		}
	}

	// Compute MAX(version) after the locks
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

	// Audit row in the same transaction
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
// Only the current pending version can be approved; otherwise returns ErrConflict.
func (s *DB) Approve(ctx context.Context, actor, name string, version int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var currentStatus string
	var currentVersion int
	var currentPendingVersion *int
	err = tx.QueryRow(ctx, `
		SELECT status, version, pending_version
		FROM registry_entries
		WHERE name = $1
		FOR UPDATE
	`, name).Scan(&currentStatus, &currentVersion, &currentPendingVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVersionNotFound
	}
	if err != nil {
		return fmt.Errorf("locking registry entry: %w", err)
	}

	var pendingVer int
	switch {
	case currentPendingVersion != nil:
		pendingVer = *currentPendingVersion
	case currentStatus == "pending":
		pendingVer = currentVersion
	default:
		return fmt.Errorf("%w: entry %q has no pending version", ErrConflict, name)
	}

	if version != pendingVer {
		return fmt.Errorf("%w: version %d is not the pending version (%d)", ErrConflict, version, pendingVer)
	}

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

	// Update registry_entries with approved version
	_, err = tx.Exec(ctx, `
		UPDATE registry_entries
		SET status = 'approved',
		    entry = $2,
		    version = $3,
		    pending_version = NULL,
		    updated_at = now()
		WHERE name = $1
	`, name, entryJSON, version)
	if err != nil {
		return fmt.Errorf("updating approved entry: %w", err)
	}

	// Audit row in the same transaction
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
// or marks registry_entries status='rejected' if the currently served version was pending.
// Rejecting an already approved version returns ErrAlreadyApproved.
// Rejecting any non-pending version returns ErrConflict.
func (s *DB) Reject(ctx context.Context, actor, name string, version int, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var currentStatus string
	var currentVersion int
	var currentPendingVersion *int
	err = tx.QueryRow(ctx, `
		SELECT status, version, pending_version
		FROM registry_entries
		WHERE name = $1
		FOR UPDATE
	`, name).Scan(&currentStatus, &currentVersion, &currentPendingVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVersionNotFound
	}
	if err != nil {
		return fmt.Errorf("locking registry entry: %w", err)
	}

	if currentStatus == "approved" && currentVersion == version {
		return ErrAlreadyApproved
	}

	var pendingVer int
	switch {
	case currentPendingVersion != nil:
		pendingVer = *currentPendingVersion
	case currentStatus == "pending":
		pendingVer = currentVersion
	default:
		return fmt.Errorf("%w: entry %q has no pending version", ErrConflict, name)
	}

	if version != pendingVer {
		return fmt.Errorf("%w: version %d is not the pending version (%d)", ErrConflict, version, pendingVer)
	}

	if currentPendingVersion != nil && *currentPendingVersion == version {
		_, err = tx.Exec(ctx, `
			UPDATE registry_entries
			SET pending_version = NULL,
			    updated_at = now()
			WHERE name = $1
		`, name)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE registry_entries
			SET status = 'rejected',
			    pending_version = NULL,
			    updated_at = now()
			WHERE name = $1
		`, name)
	}
	if err != nil {
		return fmt.Errorf("updating entry on rejection: %w", err)
	}

	// Audit row with reason in the same transaction
	_, err = tx.Exec(ctx, `
		INSERT INTO registry_audit (actor, action, name, version, reason, at)
		VALUES ($1, 'reject', $2, $3, $4, now())
	`, actor, name, version, reason)
	if err != nil {
		return fmt.Errorf("inserting rejection audit: %w", err)
	}

	return tx.Commit(ctx)
}

// GetApprovedEntries is the reader's EntryLoader: approved rows with updated_at.
func (s *DB) GetApprovedEntries(ctx context.Context) ([]DBEntry, error) {
	rows, err := s.pool.Query(ctx, "SELECT name, entry, updated_at FROM registry_entries WHERE status = 'approved' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("querying approved registry_entries: %w", err)
	}
	defer rows.Close()

	var entries []DBEntry
	for rows.Next() {
		var e DBEntry
		if err := rows.Scan(&e.Name, &e.Entry, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning registry entry row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating registry entry rows: %w", err)
	}
	return entries, nil
}
