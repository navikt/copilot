package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const testTeam = "team-a"

type fakeTeams struct {
	err  error
	mine []string
}

func (f fakeTeams) GetGitHubTeams(context.Context) ([]GitHubTeam, error) {
	return []GitHubTeam{{Slug: "team-a", Name: "Team A"}, {Slug: "team-b", Name: "Team B"}}, f.err
}
func (f fakeTeams) GetUserTeams(context.Context, string) ([]string, error) { return f.mine, f.err }

// openBudgetTestDB returns a DSN on a clean schema. Skips without
// TEST_DATABASE_URL unless COPILOT_API_REQUIRE_PG=1 (CI).
func openBudgetTestDB(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" && os.Getenv("COPILOT_API_REQUIRE_PG") == "1" {
		t.Fatal("COPILOT_API_REQUIRE_PG=1 needs TEST_DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	return dsn
}

// fakeGitHub is the enterprise budgets endpoint for one enterprise.
type fakeGitHub struct {
	mu       sync.Mutex
	budgets  map[string][]ghBudget // login -> user-scope budgets
	writes   []string              // "POST" / "PATCH id"
	failNext int                   // status for the next write, 0 = ok
	nextID   int
}

func (f *fakeGitHub) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		base := "/enterprises/nav/settings/billing/budgets"
		switch {
		case r.Method == http.MethodGet && r.URL.Path == base:
			u := r.URL.Query().Get("user")
			eff := &effectiveBudget{BudgetAmount: 800}
			for _, b := range f.budgets[u] {
				eff = &effectiveBudget{ID: b.ID, BudgetAmount: b.Amount}
			}
			_ = json.NewEncoder(w).Encode(ghUserBudgets{Budgets: f.budgets[u], Effective: eff})
		case r.Method == http.MethodPost && r.URL.Path == base:
			f.writes = append(f.writes, "POST")
			if f.failNext != 0 {
				w.WriteHeader(f.failNext)
				f.failNext = 0
				return
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["budget_scope"] != "user" || body["budget_entity_name"] != "" || body["prevent_further_usage"] != true {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f.nextID++
			b := ghBudget{ID: fmt.Sprintf("b%d", f.nextID), BudgetScope: "user", Amount: body["budget_amount"].(float64), ExpiresAt: body["expires_at"].(string)}
			login := body["user"].(string)
			f.budgets[login] = append(f.budgets[login], b)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(b)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, base+"/"):
			id := strings.TrimPrefix(r.URL.Path, base+"/")
			f.writes = append(f.writes, "PATCH "+id)
			if f.failNext != 0 {
				w.WriteHeader(f.failNext)
				f.failNext = 0
				return
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			for login, bs := range f.budgets {
				for i := range bs {
					if bs[i].ID == id {
						f.budgets[login][i].Amount = body["budget_amount"].(float64)
						f.budgets[login][i].ExpiresAt = body["expires_at"].(string)
						w.WriteHeader(http.StatusOK)
						return
					}
				}
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

type fakeSeats struct {
	err  error
	none bool
}

func (s fakeSeats) getCopilotSeat(context.Context, string) (*CopilotSeat, error) {
	if s.none {
		return nil, s.err
	}
	return &CopilotSeat{}, s.err
}

// emailResolver maps the token's e-mail to a login: identity only ever
// comes from the authenticated user, as with the real resolvers.
type emailResolver struct{}

func (emailResolver) CanResolve(u *User, _ *http.Request) bool { return u != nil }
func (emailResolver) Resolve(_ context.Context, u *User, _ *http.Request) (*ResolvedIdentity, error) {
	return &ResolvedIdentity{GitHubUsername: strings.TrimSuffix(u.Email, "@nav.no"), Source: "test"}, nil
}

type budgetEnv struct {
	t      *testing.T
	gh     *fakeGitHub
	h      *budgetRaiseHandlers
	router http.Handler
	dsn    string
}

func newBudgetEnv(t *testing.T, dsn string) *budgetEnv {
	t.Helper()
	gh := &fakeGitHub{budgets: map[string][]ghBudget{}}
	srv := httptest.NewServer(gh.handler())
	t.Cleanup(srv.Close)
	h, err := newBudgetRaiseHandlers(&Config{BudgetDatabaseURL: dsn, BudgetWrites: "on", BudgetDefaultUSD: 150, BudgetTimeZone: "Europe/Oslo", BudgetRaisesOpen: true},
		&budgetWriter{httpClient: srv.Client(), baseURL: srv.URL, enterprise: "nav", token: func(context.Context) (string, error) { return "test-token", nil }},
		fakeSeats{})
	if err != nil {
		t.Fatal(err)
	}
	h.limiter.per = 1000
	h.teams = fakeTeams{mine: []string{"team-b"}}
	h.now = func() time.Time { return time.Date(2026, 12, 3, 10, 0, 0, 0, time.UTC) }
	t.Cleanup(func() {
		if h.store.pool != nil {
			h.store.pool.Close()
		}
	})
	env := &budgetEnv{t: t, gh: gh, h: h, dsn: dsn}
	env.router = makeAPIRouter(&Config{}, nil, nil, nil, NewIdentityResolverChain(emailResolver{}), h)
	return env
}

func (e *budgetEnv) do(method, path, login, key, body string, groups ...string) (int, map[string]any) {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Email: login + "@nav.no", Groups: groups}))
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (e *budgetEnv) profile(login string) {
	e.t.Helper()
	if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", login, "", `{"teamId":"`+testTeam+`","role":"utvikler"}`); code != 200 {
		e.t.Fatalf("profile: %d", code)
	}
}

func (e *budgetEnv) raise(login, key string) (int, map[string]any) {
	return e.do("POST", "/api/v1/copilot/budget/raises", login, key, `{"category":"koding","expectedValue":"spare_tid","freeText":"HEMMELIG-TEKST"}`)
}

func (e *budgetEnv) count(sql string, args ...any) int {
	e.t.Helper()
	pool, err := e.h.store.get(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func TestBudgetRaiseFlow(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))

	if code, out := e.do("GET", "/api/v1/copilot/budget/profile", "ola", "", ""); code != 200 || out["exists"] != false || out["role"] != "" || out["suggestedTeamId"] != "team-b" {
		t.Fatalf("missing profile must give defaults: %d %v", code, out)
	}
	if code, out := e.raise("ola", "k0"); code != 409 || !strings.HasSuffix(out["type"].(string), "profile_required") {
		t.Fatalf("raise without profile: %d %v", code, out)
	}
	e.profile("ola")

	code, out := e.raise("ola", "k1")
	if code != 201 || out["budgetUsd"] != float64(300) || out["expiresAt"] != "2027-01-01" || out["raise"].(map[string]any)["status"] != "applied" {
		t.Fatalf("first raise: %d %v", code, out)
	}
	if code, out = e.raise("ola", "k1"); code != 200 || out["replayed"] != true {
		t.Fatalf("replay: %d %v", code, out)
	}
	for i, want := range []float64{450, 600, 750} {
		if code, out = e.raise("ola", fmt.Sprint("k", i+2)); code != 201 || out["budgetUsd"] != want {
			t.Fatalf("raise %d: %d %v", i+2, code, out)
		}
	}
	if code, out = e.raise("ola", "k9"); code != 409 || !strings.HasSuffix(out["type"].(string), "cap_reached") {
		t.Fatalf("over cap: %d %v", code, out)
	}
	if got := strings.Join(e.gh.writes, ","); got != "POST,PATCH b1,PATCH b1,PATCH b1" {
		t.Errorf("GitHub writes: %s", got)
	}
	if b := e.gh.budgets["ola"][0]; b.Amount != 750 || b.ExpiresAt != "2027-01-01" {
		t.Errorf("GitHub budget: %+v", b)
	}
	if n := e.count("SELECT count(*) FROM budget_audit WHERE action = 'raise' AND github_status IS NOT NULL AND NOT dry_run"); n != 4 {
		t.Errorf("audit rows: %d", n)
	}
	if n := e.count("SELECT count(*) FROM budget_github WHERE github_budget_id = 'b1' AND amount_usd = 750"); n != 1 {
		t.Errorf("budget_github: %d", n)
	}
	if code, out = e.do("GET", "/api/v1/copilot/budget/raises", "ola", "", ""); code != 200 || len(out["raises"].([]any)) != 4 {
		t.Errorf("history: %d %v", code, out)
	}
	if code, out = e.do("GET", "/api/v1/copilot/budget/raises", "kari", "", ""); code != 200 || len(out["raises"].([]any)) != 0 {
		t.Errorf("another user's history must be empty: %d %v", code, out)
	}
}

func TestBudgetRaiseOwnBudgetUntouched(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.budgets["ola"] = []ghBudget{{ID: "theirs", BudgetScope: "user", Amount: 1000}}
	if code, out := e.raise("ola", "k1"); code != 409 || !strings.HasSuffix(out["type"].(string), "own_budget") {
		t.Fatalf("own budget: %d %v", code, out)
	}
	t.Setenv(adminGroupEnv, "admins")
	if code, _ := e.do("POST", "/api/v1/copilot/budget/admin/override", "boss", "a1", `{"githubLogin":"ola"}`, "admins"); code != 409 {
		t.Fatalf("admin override must not touch a foreign budget either: %d", code)
	}
	if len(e.gh.writes) != 0 || e.count("SELECT count(*) FROM budget_raises") != 0 {
		t.Errorf("nothing may be written: %v", e.gh.writes)
	}
}

func TestBudgetRaiseDryRunNotCounted(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.h.mode = "dry_run"
	e.profile("ola")
	for i := range 6 {
		if code, out := e.raise("ola", fmt.Sprint("k", i)); code != 201 || out["dryRun"] != true {
			t.Fatalf("dry run %d: %d %v", i, code, out)
		}
	}
	if len(e.gh.writes) != 0 {
		t.Errorf("dry run wrote to GitHub: %v", e.gh.writes)
	}
	if n := e.count("SELECT count(*) FROM budget_audit WHERE dry_run AND github_status IS NULL"); n != 6 {
		t.Errorf("dry-run audit rows: %d", n)
	}
}

func TestBudgetRaiseGitHubFailureIsVisible(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.failNext = 500
	code, out := e.raise("ola", "k1")
	if code != 502 || out["raise"].(map[string]any)["status"] != "failed" {
		t.Fatalf("failed write: %d %v", code, out)
	}
	if n := e.count("SELECT count(*) FROM budget_audit WHERE github_status = 500"); n != 1 {
		t.Errorf("audit of failed call: %d", n)
	}
	// failed counts toward the cap (F7), so the next raise sets 150 + 2 × 150.
	if code, out = e.raise("ola", "k2"); code != 201 || out["budgetUsd"] != float64(450) {
		t.Fatalf("raise after failure: %d %v", code, out)
	}
}

func TestBudgetRaiseConcurrentGivesOneStepEach(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := range 8 {
		key := fmt.Sprint("k", i%4) // every key sent twice
		wg.Go(func() { c, _ := e.raise("ola", key); codes <- c })
	}
	wg.Wait()
	close(codes)
	created := 0
	for c := range codes {
		if c == 201 {
			created++
		}
	}
	if created != 4 || e.count("SELECT count(*) FROM budget_raises") != 4 || e.gh.budgets["ola"][0].Amount != 750 || len(e.gh.budgets["ola"]) != 1 {
		t.Errorf("created=%d budgets=%+v", created, e.gh.budgets["ola"])
	}
}

func TestBudgetAdminOverride(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	for i := range 4 {
		e.raise("ola", fmt.Sprint("k", i))
	}
	t.Setenv(adminGroupEnv, "")
	if code, _ := e.do("POST", "/api/v1/copilot/budget/admin/override", "boss", "a1", `{"githubLogin":"ola"}`, "admins"); code != 404 {
		t.Fatalf("unset admin group must fail closed: %d", code)
	}
	t.Setenv(adminGroupEnv, "admins")
	if code, _ := e.do("POST", "/api/v1/copilot/budget/admin/override", "ola", "a1", `{"githubLogin":"ola"}`); code != 404 {
		t.Fatalf("non-admin: %d", code)
	}
	code, out := e.do("POST", "/api/v1/copilot/budget/admin/override", "boss", "a1", `{"githubLogin":"ola"}`, "admins")
	if code != 201 || out["budgetUsd"] != float64(900) {
		t.Fatalf("override: %d %v", code, out)
	}
	if n := e.count("SELECT count(*) FROM budget_audit WHERE action = 'admin_override' AND actor = 'boss'"); n != 1 {
		t.Errorf("override audit: %d", n)
	}
}

func TestBudgetRaiseTrustBoundaries(t *testing.T) {
	dsn := openBudgetTestDB(t)
	e := newBudgetEnv(t, dsn)
	e.profile("ola")

	// Amounts never from the client.
	if code, _ := e.do("POST", "/api/v1/copilot/budget/raises", "ola", "k1", `{"category":"koding","expectedValue":"spare_tid","amount":5000}`); code != 400 {
		t.Errorf("client amount: %d", code)
	}
	// Identity never from body or query.
	if code, _ := e.do("POST", "/api/v1/copilot/budget/raises?user=kari", "ola", "k2", `{"category":"koding","expectedValue":"spare_tid","githubLogin":"kari"}`); code != 400 {
		t.Errorf("login in body: %d", code)
	}
	if code, _ := e.do("POST", "/api/v1/copilot/budget/raises?user=kari", "ola", "k3", `{"category":"koding","expectedValue":"spare_tid"}`); code != 201 || len(e.gh.budgets["kari"]) != 0 {
		t.Errorf("query user must be ignored: %d %v", code, e.gh.budgets)
	}
	// Missing idempotency key, unknown category, long free text.
	for _, c := range []struct{ key, body string }{
		{"", `{"category":"koding","expectedValue":"spare_tid"}`},
		{"k4", `{"category":"x","expectedValue":"spare_tid"}`},
		{"k5", `{"category":"koding","expectedValue":"spare_tid","freeText":"` + strings.Repeat("ø", 501) + `"}`},
	} {
		if code, _ := e.do("POST", "/api/v1/copilot/budget/raises", "ola", c.key, c.body); code != 400 {
			t.Errorf("%s: %d", c.body[:30], code)
		}
	}

	// Free text never logged.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	e.raise("ola", "k6")
	e.h.mode = "dry_run"
	e.raise("ola", "k7")
	e.gh.failNext = 500
	e.h.mode = "on"
	e.raise("ola", "k8")
	slog.SetDefault(prev)
	if strings.Contains(buf.String(), "HEMMELIG") {
		t.Errorf("free text reached the log: %s", buf.String())
	}

	// Rate limit.
	e.h.limiter = &loginLimiter{per: budgetRatePerMin, window: time.Minute, seen: map[string]*loginWindow{}}
	last := 0
	for range budgetRatePerMin + 1 {
		last, _ = e.do("GET", "/api/v1/copilot/budget/options", "ola", "", "")
	}
	if last != 429 {
		t.Errorf("rate limit: %d", last)
	}
}

func TestBudgetRaiseLicenceAndOpen(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.h.seats = fakeSeats{none: true}
	if code, _ := e.raise("ola", "k1"); code != 403 {
		t.Errorf("no licence: %d", code)
	}
	e.h.seats = fakeSeats{err: fmt.Errorf("down")}
	if code, _ := e.raise("ola", "k1"); code != 503 {
		t.Errorf("licence lookup down: %d", code)
	}
	e.h.seats = fakeSeats{}
	e.h.open = false
	if code, _ := e.raise("ola", "k1"); code != 409 {
		t.Errorf("not open: %d", code)
	}
	e.h.testUsers = map[string]bool{"ola": true}
	if code, _ := e.raise("ola", "k1"); code != 201 {
		t.Errorf("test user while closed: %d", code)
	}

	// With a working database: no token or unknown BUDGET_WRITES writes nothing.
	rows, writes := e.count("SELECT count(*) FROM budget_raises"), len(e.gh.writes)
	w := e.h.writer
	e.h.writer = nil
	if code, _ := e.raise("ola", "k2"); code != 503 {
		t.Errorf("raise without token: %d", code)
	}
	e.h.writer = w
	e.h.mode = parseBudgetWrites("yes")
	if code, _ := e.raise("ola", "k3"); code != 503 {
		t.Errorf("unknown BUDGET_WRITES must be off: %d", code)
	}
	if e.count("SELECT count(*) FROM budget_raises") != rows || len(e.gh.writes) != writes {
		t.Errorf("503 paths wrote something")
	}
}

// Runs without Postgres: these must fail closed with 503 and write nothing.
func TestBudgetRaiseUnavailable(t *testing.T) {
	e := newBudgetEnv(t, "postgres://x:y@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if code, _ := e.do("GET", "/api/v1/copilot/budget/profile", "ola", "", ""); code != 503 {
		t.Errorf("profile with Postgres down: %d", code)
	}
	if code, _ := e.do("GET", "/api/v1/copilot/budget/options", "ola", "", ""); code != 200 {
		t.Errorf("options need no database: %d", code)
	}
	if code, _ := e.raise("ola", "k1"); code != 503 {
		t.Errorf("raise with Postgres down: %d", code)
	}
	e.h.writer = nil
	if code, _ := e.raise("ola", "k1"); code != 503 {
		t.Errorf("raise without token: %d", code)
	}
	e.h.mode = parseBudgetWrites("yes")
	if code, _ := e.raise("ola", "k1"); code != 503 {
		t.Errorf("unknown BUDGET_WRITES must be off: %d", code)
	}
	if len(e.gh.writes) != 0 {
		t.Errorf("GitHub written: %v", e.gh.writes)
	}
	// Not configured at all: own 503, the router still builds.
	r := makeAPIRouter(&Config{}, nil, nil, nil, NewIdentityResolverChain(emailResolver{}), nil)
	req := httptest.NewRequest("GET", "/api/v1/copilot/budget/profile", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Email: "ola@nav.no"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 503 {
		t.Errorf("unconfigured: %d", rec.Code)
	}
}

func TestBudgetTeamsFromGitHub(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	code, out := e.do("GET", "/api/v1/copilot/budget/options", "ola", "", "")
	if teams := out["teams"].([]any); code != 200 || len(teams) != 3 || teams[0].(map[string]any)["id"] != "annet" || teams[1].(map[string]any)["name"] != "Team A" {
		t.Fatalf("options: %d %v", code, out)
	}
	for team, want := range map[string]int{"team-a": 200, "annet": 200, "nope": 400, "0b0d1c3e-1111-4222-8333-444455556666": 400} {
		if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "ola", "", `{"teamId":"`+team+`","role":"utvikler"}`); code != want {
			t.Errorf("profile team %s: %d, want %d", team, code, want)
		}
	}
	if code, _ := e.do("POST", "/api/v1/copilot/budget/raises", "ola", "k1", `{"category":"koding","expectedValue":"spare_tid","teamId":"nope"}`); code != 400 {
		t.Errorf("raise with unknown team: %d", code)
	}
	// Two teams: no suggestion. Team source down: only «annet» accepted.
	e.h.teams = fakeTeams{mine: []string{"team-a", "team-b"}}
	if _, out := e.do("GET", "/api/v1/copilot/budget/profile", "kari", "", ""); out["suggestedTeamId"] != nil {
		t.Errorf("suggestion with two teams: %v", out)
	}
	e.h.teams = fakeTeams{err: fmt.Errorf("bq down")}
	if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "ola", "", `{"teamId":"team-a","role":"utvikler"}`); code != 400 {
		t.Errorf("team source down must fail closed: %d", code)
	}
	if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "ola", "", `{"teamId":"annet","role":"utvikler"}`); code != 200 {
		t.Errorf("annet with team source down: %d", code)
	}
}

func TestBudgetMonthBoundaryOslo(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.h.now = func() time.Time { return time.Date(2026, 11, 30, 23, 30, 0, 0, time.UTC) } // 00:30 1 Dec in Oslo
	if code, out := e.raise("ola", "k1"); code != 201 || out["expiresAt"] != "2027-01-01" || out["raise"].(map[string]any)["month"] != "2026-12-01" {
		t.Fatalf("boundary: %d %v", code, out)
	}
}
