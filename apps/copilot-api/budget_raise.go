package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Personal budget and self-service raises. Design: copilot-intern
// kostnadsinnsikt/prd-budsjett-og-paafyll.md; F-numbers refer to it.
// Not here yet (after 1 December): F2, F13, F14, F16 retry job, F21, F22, F23.

const (
	budgetStepUSD     = 150 // F4: fixed, server-side
	budgetCapUSD      = 750 // F6/F7: personal ceiling
	budgetFreeTextMax = 500 // F10
	budgetRatePerMin  = 10  // chapter 6: 429 rate_limited
	budgetTeamOther   = "annet"
)

var (
	budgetRoles          = []string{"utvikler", "designer", "produkteier", "leder", "analytiker", "annet"}
	budgetCategories     = []string{"koding", "feilsoking", "testing", "dokumentasjon", "analyse_data", "laering", "annet"}
	budgetExpectedValues = []string{"spare_tid", "levere_raskere", "bedre_kvalitet", "laere", "utforske", "annet"}
	// shortcut: team IDs are checked for Teamkatalogen's UUID shape, not looked
	// up there (no Teamkatalogen client in copilot-api yet); add the lookup with F2.
	teamIDPattern      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	githubLoginPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	errBudgetDBDown    = errors.New("budget database unavailable")
)

//go:embed migrations/*.sql
var budgetMigrations embed.FS

// budgetStore connects and migrates on first use, so copilot-api starts and
// serves everything else while Postgres is down.
type budgetStore struct {
	dsn  string
	mu   sync.Mutex
	pool *pgxpool.Pool
}

func (s *budgetStore) get(ctx context.Context) (*pgxpool.Pool, error) {
	if s == nil || s.dsn == "" {
		return nil, errBudgetDBDown
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pool != nil {
		return s.pool, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(s.dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: parse config", errBudgetDBDown)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errBudgetDBDown, err)
	}
	if err := migrateBudget(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("%w: %w", errBudgetDBDown, err)
	}
	s.pool = pool
	return pool, nil
}

// budgetMigrationLock serialises migrations across replicas ("budget").
const budgetMigrationLock int64 = 0x627564676574

func migrateBudget(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(budgetMigrations, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", budgetMigrationLock); err != nil {
				return err
			}
			var done bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&done); err != nil || done {
				return err
			}
			sql, err := budgetMigrations.ReadFile("migrations/" + name)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, string(sql)); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

type seatLookup interface {
	getCopilotSeat(ctx context.Context, username string) (*CopilotSeat, error)
}

type budgetRaiseHandlers struct {
	store      *budgetStore
	writer     *budgetWriter // nil: no write token, raises answer 503
	seats      seatLookup    // nil: licence check impossible, raises answer 503
	mode       string        // off | dry_run | on (F18)
	defaultUSD int
	loc        *time.Location
	open       bool
	testUsers  map[string]bool
	now        func() time.Time
	limiter    *loginLimiter
}

func newBudgetRaiseHandlers(cfg *Config, writer *budgetWriter, seats seatLookup) (*budgetRaiseHandlers, error) {
	loc, err := time.LoadLocation(cfg.BudgetTimeZone)
	if err != nil {
		return nil, fmt.Errorf("BUDGET_TIME_ZONE: %w", err)
	}
	return &budgetRaiseHandlers{
		store: &budgetStore{dsn: cfg.BudgetDatabaseURL}, writer: writer, seats: seats,
		mode: parseBudgetWrites(cfg.BudgetWrites), defaultUSD: cfg.BudgetDefaultUSD, loc: loc,
		open: cfg.BudgetRaisesOpen, testUsers: cfg.BudgetTestUsers, now: time.Now,
		limiter: &loginLimiter{per: budgetRatePerMin, window: time.Minute, seen: map[string]*loginWindow{}},
	}, nil
}

// loginLimiter is a fixed-window request counter per GitHub login.
// shortcut: per replica (max 2), so the real limit is up to 2x; move to
// Postgres if abuse shows up.
type loginLimiter struct {
	mu     sync.Mutex
	per    int
	window time.Duration
	seen   map[string]*loginWindow
}

type loginWindow struct {
	start time.Time
	n     int
}

func (l *loginLimiter) allow(login string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.seen[login]
	if w == nil || now.Sub(w.start) >= l.window {
		if len(l.seen) > 10000 { // bound memory: drop everything rather than grow
			l.seen = map[string]*loginWindow{}
		}
		w = &loginWindow{start: now}
		l.seen[login] = w
	}
	w.n++
	return w.n <= l.per
}

// callerLogin is the only source of identity: the auth and identity
// middleware. Body and query never name the caller.
func callerLogin(r *http.Request) (string, bool) {
	id, ok := GetResolvedIdentity(r.Context())
	if !ok || id.GitHubUsername == "" {
		return "", false
	}
	return strings.ToLower(id.GitHubUsername), true
}

// wrap answers 503 when the feature is not configured and applies the rate limit.
func (h *budgetRaiseHandlers) wrap(next func(*budgetRaiseHandlers, http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil {
			respondError(w, "budget_writes_unavailable", "Budget raises are not configured", http.StatusServiceUnavailable)
			return
		}
		login, ok := callerLogin(r)
		if !ok {
			respondError(w, "unauthorized", "Caller identity could not be determined", http.StatusUnauthorized)
			return
		}
		if !h.limiter.allow(login, h.now()) {
			respondError(w, "rate_limited", "Too many requests", http.StatusTooManyRequests)
			return
		}
		next(h, w, r, login)
	}
}

func dbUnavailable(w http.ResponseWriter, err error) {
	slog.Error("Budget database unavailable", "error", err)
	respondError(w, "budget_writes_unavailable", "Budget database unavailable", http.StatusServiceUnavailable)
}

func decodeStrict(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	dec.DisallowUnknownFields() // a client-sent amount is rejected, not ignored
	if err := dec.Decode(v); err != nil {
		respondError(w, "invalid_request", "Invalid request body", http.StatusBadRequest)
		return false
	}
	return true
}

func validTeam(id string) bool { return id == budgetTeamOther || teamIDPattern.MatchString(id) }

type budgetProfile struct {
	TeamID string `json:"teamId"`
	Role   string `json:"role"`
	Exists bool   `json:"exists"`
}

func (h *budgetRaiseHandlers) handleOptions(w http.ResponseWriter, _ *http.Request, _ string) {
	respondJSON(w, map[string]any{
		"roles": budgetRoles, "categories": budgetCategories, "expectedValues": budgetExpectedValues,
		"teams":   []map[string]string{{"id": budgetTeamOther, "name": "Annet/ukjent"}},
		"stepUsd": budgetStepUSD, "capUsd": budgetCapUSD, "defaultUsd": h.defaultUSD, "freeTextMax": budgetFreeTextMax,
	}, http.StatusOK)
}

func readProfile(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, login string) (budgetProfile, error) {
	var p budgetProfile
	err := q.QueryRow(ctx, "SELECT team_id, role FROM budget_profiles WHERE github_login = $1", login).Scan(&p.TeamID, &p.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return budgetProfile{}, nil // missing row: defaults
	}
	p.Exists = err == nil
	return p, err
}

func (h *budgetRaiseHandlers) handleGetProfile(w http.ResponseWriter, r *http.Request, login string) {
	pool, err := h.store.get(r.Context())
	if err != nil {
		dbUnavailable(w, err)
		return
	}
	p, err := readProfile(r.Context(), pool, login)
	if err != nil {
		dbUnavailable(w, err)
		return
	}
	noCacheControl(w)
	respondJSON(w, p, http.StatusOK)
}

func (h *budgetRaiseHandlers) handlePutProfile(w http.ResponseWriter, r *http.Request, login string) {
	var in struct {
		TeamID string `json:"teamId"`
		Role   string `json:"role"`
	}
	if !decodeStrict(w, r, &in) {
		return
	}
	if !validTeam(in.TeamID) || !slices.Contains(budgetRoles, in.Role) {
		respondError(w, "invalid_request", "Unknown team or role", http.StatusBadRequest)
		return
	}
	pool, err := h.store.get(r.Context())
	if err != nil {
		dbUnavailable(w, err)
		return
	}
	if _, err = pool.Exec(r.Context(), `INSERT INTO budget_profiles (github_login, team_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (github_login) DO UPDATE SET team_id = EXCLUDED.team_id, role = EXCLUDED.role, updated_at = now()`, login, in.TeamID, in.Role); err != nil {
		dbUnavailable(w, err)
		return
	}
	respondJSON(w, budgetProfile{TeamID: in.TeamID, Role: in.Role, Exists: true}, http.StatusOK)
}

type raiseView struct {
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"createdAt"`
	Month         string    `json:"month"`
	AmountUSD     int       `json:"amountUsd"`
	Category      string    `json:"category"`
	TeamID        string    `json:"teamId"`
	ExpectedValue string    `json:"expectedValue"`
	FreeText      *string   `json:"freeText"`
	Status        string    `json:"status"`
	AdminOverride bool      `json:"adminOverride"`
}

const raiseColumns = "id::text, created_at, to_char(month, 'YYYY-MM-DD'), amount_usd, category, team_id, expected_value, free_text, status, admin_override"

func scanRaise(row pgx.Row) (raiseView, error) {
	var v raiseView
	err := row.Scan(&v.ID, &v.CreatedAt, &v.Month, &v.AmountUSD, &v.Category, &v.TeamID, &v.ExpectedValue, &v.FreeText, &v.Status, &v.AdminOverride)
	return v, err
}

func (h *budgetRaiseHandlers) listRaises(ctx context.Context, login string, month *time.Time) ([]raiseView, error) {
	pool, err := h.store.get(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, "SELECT "+raiseColumns+" FROM budget_raises WHERE github_login = $1 AND ($2::date IS NULL OR month = $2) ORDER BY created_at DESC LIMIT 200", login, month)
	if err != nil {
		return nil, err
	}
	out := []raiseView{}
	defer rows.Close()
	for rows.Next() {
		v, err := scanRaise(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (h *budgetRaiseHandlers) handleListRaises(w http.ResponseWriter, r *http.Request, login string) {
	out, err := h.listRaises(r.Context(), login, nil)
	if err != nil {
		dbUnavailable(w, err)
		return
	}
	noCacheControl(w)
	respondJSON(w, map[string]any{"raises": out}, http.StatusOK)
}

type raiseInput struct {
	Category      string  `json:"category"`
	TeamID        string  `json:"teamId"`
	ExpectedValue string  `json:"expectedValue"`
	FreeText      *string `json:"freeText"`
}

func idempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 200 {
		respondError(w, "invalid_request", "Idempotency-Key header is required (max 200 characters)", http.StatusBadRequest)
		return "", false
	}
	return key, true
}

func (h *budgetRaiseHandlers) handleRaise(w http.ResponseWriter, r *http.Request, login string) {
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var in raiseInput
	if !decodeStrict(w, r, &in) {
		return
	}
	if !slices.Contains(budgetCategories, in.Category) || !slices.Contains(budgetExpectedValues, in.ExpectedValue) ||
		(in.TeamID != "" && !validTeam(in.TeamID)) || (in.FreeText != nil && utf8.RuneCountInString(*in.FreeText) > budgetFreeTextMax) {
		respondError(w, "invalid_request", "Unknown category, team or expected value, or free text too long", http.StatusBadRequest)
		return
	}
	if in.FreeText != nil && strings.TrimSpace(*in.FreeText) == "" {
		in.FreeText = nil
	}
	if !h.open && !h.testUsers[login] {
		respondError(w, "not_open", "Raises are not open yet", http.StatusConflict)
		return
	}
	h.raise(w, r, login, login, in, key, false)
}

func (h *budgetRaiseHandlers) handleAdminOverride(w http.ResponseWriter, r *http.Request, actor string) {
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	// The one place a login comes from the body: the target, admins only (PRD review B5).
	var in struct {
		GitHubLogin string `json:"githubLogin"`
	}
	if !decodeStrict(w, r, &in) {
		return
	}
	if !githubLoginPattern.MatchString(in.GitHubLogin) {
		respondError(w, "invalid_request", "Invalid githubLogin", http.StatusBadRequest)
		return
	}
	target := strings.ToLower(in.GitHubLogin)
	h.raise(w, r, actor, target, raiseInput{Category: "annet", ExpectedValue: "annet"}, key, true)
}

type raiseResult struct {
	Raise      raiseView `json:"raise"`
	BudgetUSD  int       `json:"budgetUsd"`
	ExpiresAt  string    `json:"expiresAt"`
	RaisesLeft int       `json:"raisesLeft"`
	DryRun     bool      `json:"dryRun"`
}

// raise runs one raise for target under a per-user Postgres lock (F9).
// Order matters and every step fails closed: nothing is written to GitHub
// before the raise row exists, and only budget IDs from budget_github are
// ever patched (F6, F8).
func (h *budgetRaiseHandlers) raise(w http.ResponseWriter, r *http.Request, actor, target string, in raiseInput, key string, admin bool) {
	ctx := r.Context()
	if h.mode == "off" || h.writer == nil || h.seats == nil {
		respondError(w, "budget_writes_unavailable", "Budget writes are not available", http.StatusServiceUnavailable)
		return
	}
	pool, err := h.store.get(ctx)
	if err != nil {
		dbUnavailable(w, err)
		return
	}
	profile, err := readProfile(ctx, pool, target)
	if err != nil {
		dbUnavailable(w, err)
		return
	}
	if !profile.Exists && !admin {
		respondError(w, "profile_required", "Fill in your profile first", http.StatusConflict)
		return
	}
	if in.TeamID == "" {
		in.TeamID = budgetTeamOther
		if profile.Exists {
			in.TeamID = profile.TeamID
		}
	}

	// F12: active licence. A failed lookup is 503, never a guess.
	seat, err := h.seats.getCopilotSeat(ctx, target)
	if err != nil {
		slog.Error("Copilot seat lookup failed for budget raise", "error", err)
		respondError(w, "budget_writes_unavailable", "Could not verify Copilot licence", http.StatusServiceUnavailable)
		return
	}
	if seat == nil {
		respondError(w, "forbidden", "No active Copilot licence", http.StatusForbidden)
		return
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		dbUnavailable(w, err)
		return
	}
	defer conn.Release()
	lockKey := "budget:" + target
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1, 0))", lockKey); err != nil {
		dbUnavailable(w, err)
		return
	}
	defer func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock(hashtextextended($1, 0))", lockKey); err != nil {
			_ = conn.Conn().Close(context.Background()) // closing the session drops the lock
		}
	}()

	// F9: the same key gives the same raise.
	if prev, err := scanRaise(conn.QueryRow(ctx, "SELECT "+raiseColumns+" FROM budget_raises WHERE github_login = $1 AND idempotency_key = $2", target, key)); err == nil {
		respondJSON(w, map[string]any{"raise": prev, "replayed": true}, http.StatusOK)
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		dbUnavailable(w, err)
		return
	}

	now := h.now().In(h.loc)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, h.loc)
	expires := month.AddDate(0, 1, 0) // F5: first day of next month, always in the future
	var counted int
	if err = conn.QueryRow(ctx, "SELECT count(*) FROM budget_raises WHERE github_login = $1 AND month = $2 AND status IN ('pending', 'applied', 'failed')", target, month).Scan(&counted); err != nil {
		dbUnavailable(w, err)
		return
	}
	current := h.defaultUSD + budgetStepUSD*counted
	amount := current + budgetStepUSD
	if !admin {
		if current >= budgetCapUSD {
			respondError(w, "cap_reached", "The monthly ceiling is reached", http.StatusConflict)
			return
		}
		amount = min(amount, budgetCapUSD)
	}

	// F8: a user budget we did not create means hands off.
	list, _, err := h.writer.listUser(ctx, target)
	if err != nil {
		slog.Error("Budget list failed before raise", "error", err)
		respondError(w, "github_error", "GitHub budget lookup failed", http.StatusBadGateway)
		return
	}
	ours := map[string]bool{}
	rows, err := conn.Query(ctx, "SELECT github_budget_id FROM budget_github WHERE github_login = $1 AND deleted_at IS NULL", target)
	if err != nil {
		dbUnavailable(w, err)
		return
	}
	var ownID string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ours[id] = true
	}
	rows.Close()
	if err != nil || rows.Err() != nil {
		dbUnavailable(w, errors.Join(err, rows.Err()))
		return
	}
	for _, b := range list.Budgets {
		if b.BudgetScope != "user" {
			continue
		}
		if !ours[b.ID] {
			respondError(w, "own_budget", "You have your own budget. Contact an admin if you need more.", http.StatusConflict)
			return
		}
		ownID = b.ID // review B4: reuse our budget, also last month's not yet removed
	}
	before := 0
	if list.Effective != nil {
		before = int(list.Effective.BudgetAmount)
	}

	status := "pending"
	if h.mode == "dry_run" {
		status = "dry_run"
	}
	v, err := scanRaise(conn.QueryRow(ctx, `INSERT INTO budget_raises (github_login, idempotency_key, month, amount_usd, category, team_id, expected_value, free_text, status, admin_override)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING `+raiseColumns,
		target, key, month, budgetStepUSD, in.Category, in.TeamID, in.ExpectedValue, in.FreeText, status, admin))
	if err != nil {
		dbUnavailable(w, err)
		return
	}
	action := "raise"
	if admin {
		action = "admin_override"
	}
	audit := func(githubStatus int, id string, dry bool) {
		if _, err := conn.Exec(context.WithoutCancel(ctx), `INSERT INTO budget_audit (raise_id, actor, action, budget_before_usd, budget_set_usd, expires_at, github_status, github_budget_id, dry_run)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, 0), NULLIF($8, ''), $9)`, v.ID, actor, action, before, amount, expires, githubStatus, id, dry); err != nil {
			slog.Error("Budget audit insert failed", "error", err)
		}
	}
	result := raiseResult{BudgetUSD: amount, ExpiresAt: expires.Format(time.DateOnly), RaisesLeft: max(0, (budgetCapUSD-amount)/budgetStepUSD)}

	if h.mode == "dry_run" { // F17: everything except the GitHub write
		op := "create"
		if ownID != "" {
			op = "patch"
		}
		slog.Info("Budget dry run: would write GitHub budget", "op", op, "budget_amount", amount, "expires_at", result.ExpiresAt, "admin_override", admin)
		audit(0, ownID, true)
		result.Raise, result.DryRun = v, true
		respondJSON(w, result, http.StatusCreated)
		return
	}

	var ghStatus int
	if ownID != "" {
		ghStatus, err = h.writer.patchOwnBudget(ctx, ownID, amount, expires)
	} else {
		// Our stored IDs that GitHub no longer lists have expired there.
		if _, dbErr := conn.Exec(ctx, "UPDATE budget_github SET deleted_at = now() WHERE github_login = $1 AND deleted_at IS NULL", target); dbErr != nil {
			dbUnavailable(w, dbErr)
			return
		}
		ownID, ghStatus, err = h.writer.createUserBudget(ctx, target, amount, expires)
	}
	if err == nil {
		if _, dbErr := conn.Exec(context.WithoutCancel(ctx), `INSERT INTO budget_github (github_login, month, github_budget_id, amount_usd, expires_at) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (github_budget_id) DO UPDATE SET month = EXCLUDED.month, amount_usd = EXCLUDED.amount_usd, expires_at = EXCLUDED.expires_at, deleted_at = NULL`,
			target, month, ownID, amount, expires); dbErr != nil {
			// The budget exists in GitHub but we lost its ID: the next raise
			// will see own_budget. Loud, so an admin can insert the row.
			slog.Error("Budget written in GitHub but its ID was not stored", "github_budget_id", ownID, "error", dbErr)
		}
		// Read back (chapter 7 step 5): the list is the only place with expires_at.
		err = h.verify(ctx, target, ownID, amount)
	}
	audit(ghStatus, ownID, false)
	final := "applied"
	if err != nil {
		final = "failed"
		slog.Error("Budget write failed", "github_status", ghStatus, "error", err)
	}
	if _, dbErr := conn.Exec(context.WithoutCancel(ctx), "UPDATE budget_raises SET status = $2 WHERE id = $1", v.ID, final); dbErr != nil {
		slog.Error("Budget raise status update failed", "error", dbErr)
	}
	v.Status = final
	result.Raise = v
	if err != nil {
		respondJSON(w, map[string]any{"type": "https://copilot-api.nav.no/errors/github_error", "status": http.StatusBadGateway, "raise": v}, http.StatusBadGateway)
		return
	}
	respondJSON(w, result, http.StatusCreated)
}

func (h *budgetRaiseHandlers) verify(ctx context.Context, login, id string, amount int) error {
	list, _, err := h.writer.listUser(ctx, login)
	if err != nil {
		return err
	}
	for _, b := range list.Budgets {
		if b.ID == id && int(b.Amount) == amount {
			return nil
		}
	}
	return fmt.Errorf("budget %s did not read back with amount %d", id, amount)
}

// newBudgetRaiseHandlersFromConfig builds the raise handlers. A missing token
// leaves writer nil (raises 503); a broken token config is an error.
func newBudgetRaiseHandlersFromConfig(cfg *Config, seats seatLookup) (*budgetRaiseHandlers, error) {
	tok, err := newBudgetTokenSource(cfg)
	if err != nil {
		return nil, err
	}
	var writer *budgetWriter
	if tok != nil {
		writer = &budgetWriter{httpClient: &http.Client{Timeout: 15 * time.Second}, baseURL: "https://api.github.com", enterprise: cfg.GitHubEnterprise, token: tok}
	}
	slog.Info("Budget raises configured", "writes", parseBudgetWrites(cfg.BudgetWrites), "write_token", tok != nil, "database", cfg.BudgetDatabaseURL != "")
	return newBudgetRaiseHandlers(cfg, writer, seats)
}
