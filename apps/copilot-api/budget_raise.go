package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
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
	githubLoginPattern   = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	errBudgetDBDown      = errors.New("budget database unavailable")
)

//go:embed migrations/*.sql
var budgetMigrations embed.FS

// budgetStore connects and migrates on first use, so copilot-api starts and
// serves everything else while Postgres is down. The mutex only guards state:
// one caller connects, the others fail fast, and a failure is remembered for
// budgetDBRetry so a hanging Postgres does not queue requests behind it.
type budgetStore struct {
	dsn        string
	mu         sync.Mutex
	pool       *pgxpool.Pool
	connecting bool
	failedAt   time.Time
}

const (
	budgetDBConnectTimeout = 3 * time.Second
	budgetDBRetry          = 15 * time.Second
	budgetDBMaxConns       = 10 // db-f1-micro allows 25; the pool is the only user
)

func (s *budgetStore) get(ctx context.Context) (*pgxpool.Pool, error) {
	if s == nil || s.dsn == "" {
		return nil, errBudgetDBDown
	}
	s.mu.Lock()
	if s.pool != nil {
		defer s.mu.Unlock()
		return s.pool, nil
	}
	if s.connecting || time.Since(s.failedAt) < budgetDBRetry {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: connecting or recently failed", errBudgetDBDown)
	}
	s.connecting = true
	s.mu.Unlock()

	pool, err := openBudgetPool(ctx, s.dsn)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.connecting = false
	if err != nil {
		s.failedAt = time.Now()
		return nil, fmt.Errorf("%w: %w", errBudgetDBDown, err)
	}
	s.pool = pool
	return pool, nil
}

func openBudgetPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, budgetDBConnectTimeout)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("parse config")
	}
	cfg.MaxConns = budgetDBMaxConns
	cfg.ConnConfig.ConnectTimeout = budgetDBConnectTimeout
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := migrateBudget(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
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
	teams      teamSource    // nil: only «annet» is selectable
	mode       string        // off | dry_run | on (F18)
	defaultUSD int
	loc        *time.Location
	open       bool
	testUsers  map[string]bool
	now        func() time.Time
	limiter    *loginLimiter
	userLocks  sync.Map // login -> *sync.Mutex: one raise at a time per user on this replica
	afterList  func()   // test hook: runs between the GitHub list and the reservation
}

func newBudgetRaiseHandlers(cfg *Config, writer *budgetWriter, seats seatLookup) (*budgetRaiseHandlers, error) {
	loc, err := time.LoadLocation(cfg.BudgetTimeZone)
	if err != nil {
		return nil, fmt.Errorf("BUDGET_TIME_ZONE: %w", err)
	}
	h := &budgetRaiseHandlers{
		store: &budgetStore{dsn: cfg.BudgetDatabaseURL}, writer: writer, seats: seats,
		mode: parseBudgetWrites(cfg.BudgetWrites), defaultUSD: cfg.BudgetDefaultUSD, loc: loc,
		open: cfg.BudgetRaisesOpen, testUsers: cfg.BudgetTestUsers, now: time.Now,
		limiter: &loginLimiter{per: budgetRatePerMin, window: time.Minute, seen: map[string]*loginWindow{}},
	}
	if writer != nil {
		writer.owns = h.ownsBudget
	}
	return h, nil
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
		if r.Method != http.MethodGet && !h.limiter.allow(login, h.now()) {
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
	// One JSON value and nothing after it; the trailing read also enforces the size limit.
	if err := dec.Decode(v); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		respondError(w, "invalid_request", "Invalid request body", http.StatusBadRequest)
		return false
	}
	return true
}

// teamSource lists GitHub org teams (owner decision on #2704: the charged team
// is a GitHub team slug, not Teamkatalogen). CachedBigQueryClient implements it.
type teamSource interface {
	GetGitHubTeams(ctx context.Context) ([]GitHubTeam, error)
	GetUserTeams(ctx context.Context, login string) ([]string, error)
}

// budgetTeams is the selectable list: GitHub teams plus «annet/ukjent». If the
// team source is down, only «annet» is offered and accepted (fails closed).
func (h *budgetRaiseHandlers) budgetTeams(ctx context.Context) []GitHubTeam {
	teams := []GitHubTeam{{Slug: budgetTeamOther, Name: "Annet/ukjent"}}
	if h.teams == nil {
		return teams
	}
	gh, err := h.teams.GetGitHubTeams(ctx)
	if err != nil {
		slog.Warn("GitHub team list unavailable for budget options", "error", err)
		return teams
	}
	return append(teams, gh...)
}

func (h *budgetRaiseHandlers) validTeam(ctx context.Context, slug string) bool {
	return slices.ContainsFunc(h.budgetTeams(ctx), func(t GitHubTeam) bool { return t.Slug == slug })
}

type budgetProfile struct {
	TeamID string `json:"teamId"`
	Role   string `json:"role"`
	Exists bool   `json:"exists"`
	// SuggestedTeamID is the caller's own GitHub team when they are in
	// exactly one; only set while no profile is stored.
	SuggestedTeamID string `json:"suggestedTeamId,omitempty"`
}

func (h *budgetRaiseHandlers) handleOptions(w http.ResponseWriter, r *http.Request, _ string) {
	respondJSON(w, map[string]any{
		"roles": budgetRoles, "categories": budgetCategories, "expectedValues": budgetExpectedValues,
		"teams":   h.budgetTeams(r.Context()),
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
	if !p.Exists && h.teams != nil {
		if mine, err := h.teams.GetUserTeams(r.Context(), login); err == nil && len(mine) == 1 && h.validTeam(r.Context(), mine[0]) {
			p.SuggestedTeamID = mine[0]
		}
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
	if !h.validTeam(r.Context(), in.TeamID) || !slices.Contains(budgetRoles, in.Role) {
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

// budgetGetRaisesTimeout caps what Postgres can add to GET /budget, which
// nav-pilot and my-copilot already call.
const budgetGetRaisesTimeout = time.Second

// monthRaises is the raises part of GET /budget, under its own short deadline.
func (h *budgetRaiseHandlers) monthRaises(ctx context.Context, login string, month time.Time) ([]raiseView, error) {
	ctx, cancel := context.WithTimeout(ctx, budgetGetRaisesTimeout)
	defer cancel()
	return h.listRaises(ctx, login, &month)
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
		(in.TeamID != "" && !h.validTeam(r.Context(), in.TeamID)) || (in.FreeText != nil && !validFreeText(*in.FreeText)) {
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

// validFreeText: at most 500 characters of valid UTF-8, no control
// characters except line breaks and tabs (Postgres rejects NUL).
func validFreeText(s string) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > budgetFreeTextMax {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' })
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

// budgetPendingStale is how long a pending raise may stay pending before the
// next request for the same user reconciles it (process died mid-write).
const budgetPendingStale = 2 * time.Minute

// budgetWriteTimeout bounds the GitHub write and the bookkeeping after it.
// They run detached from the request, so a client that hangs up cannot leave
// a budget in GitHub that we did not record.
const budgetWriteTimeout = 45 * time.Second

// raise runs one raise for target. Every step fails closed: nothing is
// written to GitHub before the raise row exists, and only budget IDs from
// budget_github are ever patched (F6, F8). No pool connection is held across
// a GitHub call.
func (h *budgetRaiseHandlers) raise(w http.ResponseWriter, r *http.Request, actor, target string, in raiseInput, key string, admin bool) {
	ctx := r.Context()
	dry := h.mode == "dry_run"
	// dry_run does not need the write key (owner decision on #2704): it reads
	// GitHub only when the key is there and otherwise computes from the database.
	if h.mode == "off" || (!dry && h.writer == nil) || h.seats == nil {
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
	if !h.validTeam(ctx, in.TeamID) { // a saved team may since have gone
		respondError(w, "invalid_request", "Team is no longer selectable, update your profile", http.StatusBadRequest)
		return
	}

	// F12: active licence. A failed lookup is 503, never a guess. A seat with a
	// pending_cancellation_date is still active until that date, so it counts.
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

	mu, _ := h.userLocks.LoadOrStore(target, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	now := h.now().In(h.loc)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, h.loc)
	expires := month.AddDate(0, 1, 0) // F5: first day of next month, always in the future
	action := "raise"
	if admin {
		action = "admin_override"
	}

	// Read GitHub before taking the database lock (F8 needs the list). Another
	// replica may change things between the two, so reserve checks the user's
	// raise state is what it was before the list, and otherwise we list again.
	var plan raisePlan
	var failCode string
	for attempt := 0; ; attempt++ {
		var fp int64
		if err = pool.QueryRow(ctx, raiseFingerprintSQL, target).Scan(&fp); err != nil {
			dbUnavailable(w, err)
			return
		}
		var list *ghUserBudgets
		if h.writer != nil {
			if list, _, err = h.writer.listUser(ctx, target); err != nil {
				slog.Error("Budget list failed before raise", "error", err)
				respondError(w, "github_error", "GitHub budget lookup failed", http.StatusBadGateway)
				return
			}
		}
		if h.afterList != nil {
			h.afterList()
		}
		plan, failCode, err = h.reserve(ctx, pool, reservation{actor: actor, target: target, key: key, admin: admin, dry: dry, in: in, month: month, expires: expires, list: list, action: action, fingerprint: fp})
		if failCode != "stale" || err != nil {
			break
		}
		if attempt == 2 {
			failCode = "raise_in_progress"
			break
		}
	}
	switch {
	case err != nil:
		dbUnavailable(w, err)
		return
	case plan.replay != nil:
		respondJSON(w, map[string]any{"raise": *plan.replay, "replayed": true}, http.StatusOK)
		return
	case failCode != "":
		msg := map[string]string{
			"cap_reached":       "The monthly ceiling is reached",
			"own_budget":        "You have your own budget. Contact an admin if you need more.",
			"raise_in_progress": "Another raise is in progress, try again shortly",
		}[failCode]
		respondError(w, failCode, msg, http.StatusConflict)
		return
	}

	result := raiseResult{BudgetUSD: plan.amount, ExpiresAt: expires.Format(time.DateOnly), RaisesLeft: max(0, (budgetCapUSD-plan.amount)/budgetStepUSD)}
	audit := func(c context.Context, githubStatus int, id string) {
		if _, err := pool.Exec(c, `INSERT INTO budget_audit (raise_id, actor, action, budget_before_usd, budget_set_usd, expires_at, github_status, github_budget_id, dry_run)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, 0), NULLIF($8, ''), $9)`, plan.raise.ID, actor, action, plan.before, plan.amount, expires, githubStatus, id, dry); err != nil {
			slog.Error("Budget audit insert failed", "error", err)
		}
	}

	if dry { // F17: everything except the GitHub write
		op := "create"
		if plan.ownID != "" {
			op = "patch"
		}
		slog.Info("Budget dry run: would write GitHub budget", "op", op, "budget_amount", plan.amount, "expires_at", result.ExpiresAt, "admin_override", admin)
		audit(ctx, 0, plan.ownID)
		result.Raise, result.DryRun = plan.raise, true
		respondJSON(w, result, http.StatusCreated)
		return
	}

	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), budgetWriteTimeout)
	defer cancel()
	ownID, ghStatus, unknown, err := h.write(wctx, pool, target, plan, month, expires)
	audit(wctx, ghStatus, ownID)
	final := "applied"
	switch {
	case err != nil && unknown:
		// GitHub may hold a budget whose ID we did not store: stay pending,
		// so the next raise reconciles (adopts) it instead of own_budget.
		final = "pending"
		slog.Error("Budget write outcome unknown, left pending", "github_status", ghStatus, "error", err)
	case err != nil:
		final = "failed"
		slog.Error("Budget write failed", "github_status", ghStatus, "error", err)
	}
	if _, dbErr := pool.Exec(wctx, "UPDATE budget_raises SET status = $2 WHERE id = $1", plan.raise.ID, final); dbErr != nil {
		slog.Error("Budget raise status update failed", "error", dbErr)
	}
	plan.raise.Status = final
	result.Raise = plan.raise
	if err != nil {
		respondJSON(w, map[string]any{"type": "https://copilot-api.nav.no/errors/github_error", "status": http.StatusBadGateway, "raise": plan.raise}, http.StatusBadGateway)
		return
	}
	respondJSON(w, result, http.StatusCreated)
}

type reservation struct {
	actor, target, key, action string
	admin, dry                 bool
	in                         raiseInput
	month, expires             time.Time
	list                       *ghUserBudgets // nil in dry_run without write key
	fingerprint                int64          // raiseFingerprintSQL before the list was read
}

// raiseFingerprintSQL changes whenever a raise for the user is inserted or
// leaves pending, i.e. whenever budget_github may have changed.
const raiseFingerprintSQL = "SELECT count(*) + count(*) FILTER (WHERE status <> 'pending') FROM budget_raises WHERE github_login = $1"

type raisePlan struct {
	replay *raiseView
	raise  raiseView
	amount int
	before int
	ownID  string   // our budget to PATCH; empty means POST
	gone   []string // our recorded IDs GitHub no longer lists
}

// reserve does every database check and inserts the raise row in one short
// transaction under a per-user advisory lock (F9 across replicas).
func (h *budgetRaiseHandlers) reserve(ctx context.Context, pool *pgxpool.Pool, rv reservation) (raisePlan, string, error) {
	var plan raisePlan
	var code string
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		plan, code = raisePlan{}, ""
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "budget:"+rv.target); err != nil {
			return err
		}
		if prev, err := scanRaise(tx.QueryRow(ctx, "SELECT "+raiseColumns+" FROM budget_raises WHERE github_login = $1 AND idempotency_key = $2", rv.target, rv.key)); err == nil {
			plan.replay = &prev
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var fp int64
		if err := tx.QueryRow(ctx, raiseFingerprintSQL, rv.target).Scan(&fp); err != nil {
			return err
		}
		if fp != rv.fingerprint {
			code = "stale" // the GitHub list predates another replica's raise
			return nil
		}

		ours := map[string]bool{}
		rows, err := tx.Query(ctx, "SELECT github_budget_id FROM budget_github WHERE github_login = $1 AND deleted_at IS NULL", rv.target)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ours[id] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		var foreign []ghBudget
		if rv.list != nil {
			listed := map[string]bool{}
			for _, b := range rv.list.Budgets {
				listed[b.ID] = true
				switch {
				case b.BudgetScope == "multi_user_customer" || b.BudgetScope == "enterprise":
				case ours[b.ID]:
					plan.ownID = b.ID // review B4: reuse ours, also last month's
				default:
					foreign = append(foreign, b)
				}
			}
			for id := range ours {
				if !listed[id] {
					plan.gone = append(plan.gone, id)
				}
			}
			if rv.list.Effective != nil {
				plan.before = int(rv.list.Effective.BudgetAmount)
			}
		}

		// A pending row younger than budgetPendingStale is a raise in flight on
		// another replica. Older ones died mid-write: reconcile them now.
		type staleRow struct {
			id     string
			amount int
			month  time.Time
		}
		var stale []staleRow
		pend, err := tx.Query(ctx, "SELECT id::text, COALESCE(budget_set_usd, 0), month, created_at < now() - $2::interval FROM budget_raises WHERE github_login = $1 AND status = 'pending'", rv.target, budgetPendingStale.String())
		if err != nil {
			return err
		}
		for pend.Next() {
			var id string
			var amount int
			var m time.Time
			var old bool
			if err := pend.Scan(&id, &amount, &m, &old); err != nil {
				pend.Close()
				return err
			}
			if !old {
				code = "raise_in_progress"
			}
			// month is a date: rebuild it in our zone, as the raise computed it.
			stale = append(stale, staleRow{id, amount, time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, h.loc)})
		}
		pend.Close()
		if err := pend.Err(); err != nil || code != "" {
			return err
		}
		for _, p := range stale {
			status := "failed"
			// The only foreign budget we may adopt is one that matches exactly
			// what the dead raise sent: our amount and our expiry.
			// Its own month and expiry, not this request's: it may be last month's.
			pExpires := p.month.AddDate(0, 1, 0)
			if len(foreign) == 1 && plan.ownID == "" && h.writer != nil && matchesWrite(foreign[0], h.writer.budgetType, p.amount, pExpires) {
				if err := recordOwnBudget(ctx, tx, rv.target, p.month, foreign[0].ID, p.amount, pExpires); err != nil {
					return err
				}
				plan.ownID, foreign, status = foreign[0].ID, nil, "applied"
			}
			if _, err := tx.Exec(ctx, "UPDATE budget_raises SET status = $2 WHERE id = $1", p.id, status); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO budget_audit (raise_id, actor, action, budget_set_usd, github_budget_id, dry_run) VALUES ($1, 'system', 'retry', $2, NULLIF($3, ''), false)`, p.id, p.amount, plan.ownID); err != nil {
				return err
			}
		}

		if len(foreign) > 0 { // F8: a budget we did not create means hands off
			code = "own_budget"
			return nil
		}

		var counted int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM budget_raises WHERE github_login = $1 AND month = $2 AND status IN ('pending', 'applied', 'failed')", rv.target, rv.month).Scan(&counted); err != nil {
			return err
		}
		current := h.defaultUSD + budgetStepUSD*counted
		plan.amount = current + budgetStepUSD
		if !rv.admin {
			if current >= budgetCapUSD {
				code = "cap_reached"
				return nil
			}
			plan.amount = min(plan.amount, budgetCapUSD)
		}

		status := "pending"
		if rv.dry {
			status = "dry_run"
		}
		v, err := scanRaise(tx.QueryRow(ctx, `INSERT INTO budget_raises (github_login, idempotency_key, month, amount_usd, budget_set_usd, category, team_id, expected_value, free_text, status, admin_override)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING `+raiseColumns,
			rv.target, rv.key, rv.month, budgetStepUSD, plan.amount, rv.in.Category, rv.in.TeamID, rv.in.ExpectedValue, rv.in.FreeText, status, rv.admin))
		plan.raise = v
		return err
	})
	return plan, code, err
}

// recordOwnBudget stores id as the user's budget for month. An ID moves with
// the month it is reused in (B4); a replaced ID for the same month is
// overwritten, its history stays in budget_audit.
func recordOwnBudget(ctx context.Context, tx pgx.Tx, login string, month time.Time, id string, amount int, expires time.Time) error {
	if _, err := tx.Exec(ctx, "DELETE FROM budget_github WHERE github_budget_id = $1 AND month <> $2", id, month); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO budget_github (github_login, month, github_budget_id, amount_usd, expires_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (github_login, month) DO UPDATE SET github_budget_id = EXCLUDED.github_budget_id, amount_usd = EXCLUDED.amount_usd, expires_at = EXCLUDED.expires_at, deleted_at = NULL`,
		login, month, id, amount, expires)
	return err
}

// write PATCHes our budget or POSTs a new one, records it and reads it back.
// ctx is detached from the request.
func (h *budgetRaiseHandlers) write(ctx context.Context, pool *pgxpool.Pool, target string, plan raisePlan, month, expires time.Time) (ownID string, status int, unknown bool, err error) {
	for _, id := range plan.gone { // ours, but GitHub no longer lists it: gone
		if _, err := pool.Exec(ctx, "UPDATE budget_github SET deleted_at = now() WHERE github_budget_id = $1", id); err != nil {
			return "", 0, false, err
		}
		if _, err := pool.Exec(ctx, `INSERT INTO budget_audit (raise_id, actor, action, github_budget_id, dry_run) VALUES ($1, 'system', 'expire_delete', $2, false)`, plan.raise.ID, id); err != nil {
			return "", 0, false, err
		}
	}
	ownID = plan.ownID
	created := false
	if ownID != "" {
		status, err = h.writer.patchOwnBudget(ctx, ownID, plan.amount, expires)
	} else {
		created = true
		ownID, status, err = h.writer.createUserBudget(ctx, target, plan.amount, expires)
		unknown = err != nil && status == 0
		if unknown {
			// Result unknown (connection lost). Adopt the one new user budget
			// only if it is exactly what we sent.
			if list, _, lerr := h.writer.listUser(ctx, target); lerr == nil {
				var match []string
				for _, b := range list.Budgets {
					if matchesWrite(b, h.writer.budgetType, plan.amount, expires) {
						match = append(match, b.ID)
					}
				}
				if len(match) == 1 {
					ownID, err, unknown = match[0], nil, false
					status = http.StatusCreated
				}
			}
		}
	}
	if err != nil {
		return ownID, status, unknown, err
	}
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return recordOwnBudget(ctx, tx, target, month, ownID, plan.amount, expires)
	}); err != nil {
		// The budget exists in GitHub but its ID is not stored: the next raise
		// reconciles it as a stale pending row. Loud, so an admin can check.
		slog.Error("Budget written in GitHub but its ID was not stored", "github_budget_id", ownID, "error", err)
		return ownID, status, created, err
	}
	return ownID, status, false, h.verify(ctx, target, ownID, plan.amount, expires)
}

// matchesWrite: the listed budget is exactly what we sent: user scope, amount,
// expiry, SKU, and the configured type when there is one.
func matchesWrite(b ghBudget, budgetType string, amount int, expires time.Time) bool {
	return b.BudgetScope == "user" && b.Amount == float64(amount) && strings.HasPrefix(b.ExpiresAt, expires.Format(time.DateOnly)) &&
		b.SKU == "ai_credits" && (budgetType == "" || b.BudgetType == budgetType)
}

// verify reads the budget back (chapter 7 step 5). The list is the only place
// with expires_at, and a raise without our expiry would never end (R1).
func (h *budgetRaiseHandlers) verify(ctx context.Context, login, id string, amount int, expires time.Time) error {
	list, _, err := h.writer.listUser(ctx, login)
	if err != nil {
		return err
	}
	for _, b := range list.Budgets {
		if b.ID == id {
			if matchesWrite(b, h.writer.budgetType, amount, expires) {
				return nil
			}
			return fmt.Errorf("budget %s read back as amount %v, expires %q, type %q, sku %q", id, b.Amount, b.ExpiresAt, b.BudgetType, b.SKU)
		}
	}
	return fmt.Errorf("budget %s not listed after write", id)
}

// newBudgetRaiseHandlersFromConfig builds the raise handlers. A missing token
// leaves writer nil (raises 503); a broken token config is an error.
func newBudgetRaiseHandlersFromConfig(cfg *Config, seats seatLookup, teams teamSource) (*budgetRaiseHandlers, error) {
	tok, err := newBudgetTokenSource(cfg)
	if err != nil {
		return nil, err
	}
	var writer *budgetWriter
	if tok != nil {
		writer = &budgetWriter{httpClient: &http.Client{Timeout: 15 * time.Second}, baseURL: "https://api.github.com", enterprise: cfg.GitHubEnterprise, token: tok, budgetType: cfg.BudgetGitHubBudgetType}
	}
	slog.Info("Budget raises configured", "writes", parseBudgetWrites(cfg.BudgetWrites), "write_token", tok != nil, "database", cfg.BudgetDatabaseURL != "")
	h, err := newBudgetRaiseHandlers(cfg, writer, seats)
	if h != nil {
		h.teams = teams
	}
	return h, err
}

// ownsBudget backs budgetWriter.owns: an ID is ours only if budget_github has it.
func (h *budgetRaiseHandlers) ownsBudget(ctx context.Context, id string) (bool, error) {
	pool, err := h.store.get(ctx)
	if err != nil {
		return false, err
	}
	var ok bool
	err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM budget_github WHERE github_budget_id = $1 AND deleted_at IS NULL)", id).Scan(&ok)
	return ok, err
}
