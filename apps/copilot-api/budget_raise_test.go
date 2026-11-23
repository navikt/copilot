package main

import (
	"bytes"
	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const testTeam = "team-b" // the caller's own team in newBudgetEnv

type fakeTeams struct {
	err      error
	mine     []string      // current roster
	activity []string      // GetUserTeams' 30-day fallback, used when the roster is empty
	delay    time.Duration // per lookup, like a cold BigQuery cache
}

func (f fakeTeams) GetGitHubTeams(context.Context) ([]GitHubTeam, error) {
	time.Sleep(f.delay)
	return []GitHubTeam{{Slug: "team-a", Name: "Team A"}, {Slug: "team-b", Name: "Team B"}}, f.err
}

// GetUserTeams mimics the activity fallback; budget code must not use it.
func (f fakeTeams) GetUserTeams(context.Context, string) ([]string, error) {
	if len(f.mine) == 0 {
		return f.activity, f.err
	}
	return f.mine, f.err
}
func (f fakeTeams) GetRosterTeams(context.Context, string) ([]string, error) {
	time.Sleep(f.delay)
	return f.mine, f.err
}

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
	nextPage bool          // list claims another page
	dropPOST bool          // create, then drop the connection
	postWait chan struct{} // POST blocks until closed
	noExpiry bool          // ignore expires_at like a GitHub that drops it
	failList int           // fail the next N list calls
	skew     float64       // stored amount differs from the one sent
	noSKU    bool          // list without budget_product_sku
	truncate bool          // create, then answer 201 with a cut-off body
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
			if f.failList > 0 {
				f.failList--
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			u := r.URL.Query().Get("user")
			eff := &effectiveBudget{BudgetAmount: 800}
			for _, b := range f.budgets[u] {
				eff = &effectiveBudget{ID: b.ID, BudgetAmount: b.Amount}
			}
			_ = json.NewEncoder(w).Encode(ghUserBudgets{Budgets: f.budgets[u], Effective: eff, HasNextPage: f.nextPage})
		case r.Method == http.MethodPost && r.URL.Path == base:
			if f.postWait != nil {
				f.mu.Unlock()
				<-f.postWait
				f.mu.Lock()
			}
			f.writes = append(f.writes, "POST")
			if f.failNext != 0 {
				w.WriteHeader(f.failNext)
				f.failNext = 0
				return
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["budget_scope"] != "user" || body["budget_entity_name"] != "" || body["prevent_further_usage"] != true ||
				body["budget_type"] != "BundlePricing" || body["budget_product_sku"] != "ai_credits" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f.nextID++
			b := ghBudget{ID: fmt.Sprintf("b%d", f.nextID), BudgetScope: "user", BudgetType: "BundlePricing", SKU: "ai_credits", Amount: body["budget_amount"].(float64), ExpiresAt: body["expires_at"].(string)}
			if f.noExpiry {
				b.ExpiresAt = ""
			}
			b.Amount += f.skew
			if f.noSKU {
				b.SKU = ""
			}
			login := body["user"].(string)
			f.budgets[login] = append(f.budgets[login], b)
			if f.truncate {
				f.truncate = false
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":`))
				return
			}
			if f.dropPOST {
				f.dropPOST = false
				conn, _, _ := w.(http.Hijacker).Hijack()
				_ = conn.Close()
				return
			}
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
		&budgetWriter{httpClient: srv.Client(), baseURL: srv.URL, enterprise: "nav", budgetType: "BundlePricing", token: func(context.Context) (string, error) { return "test-token", nil }},
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
	e.gh.failNext = 422 // a refusal: GitHub did not create anything
	code, out := e.raise("ola", "k1")
	if code != 502 || out["raise"].(map[string]any)["status"] != "failed" {
		t.Fatalf("failed write: %d %v", code, out)
	}
	if n := e.count("SELECT count(*) FROM budget_audit WHERE github_status = 422"); n != 1 {
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
	for range budgetRatePerMin + 5 {
		if code, _ := e.do("GET", "/api/v1/copilot/budget/options", "ola", "", ""); code != 200 {
			t.Fatalf("reads must not be rate limited: %d", code)
		}
	}
	last := 0
	for range budgetRatePerMin + 1 {
		last, _ = e.do("PUT", "/api/v1/copilot/budget/profile", "ola", "", `{"teamId":"annet","role":"utvikler"}`)
	}
	if last != 429 {
		t.Errorf("rate limit on writes: %d", last)
	}
	// Control characters in free text: 400, not a database error.
	e.h.limiter = &loginLimiter{per: 1000, window: time.Minute, seen: map[string]*loginWindow{}}
	for _, ft := range []string{`a\u0000b`, `a\u0007b`} {
		if code, _ := e.do("POST", "/api/v1/copilot/budget/raises", "ola", "kz"+ft, `{"category":"koding","expectedValue":"spare_tid","freeText":"`+ft+`"}`); code != 400 {
			t.Errorf("free text %q: %d", ft, code)
		}
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
	if teams := out["teams"].([]any); code != 200 || len(teams) != 2 || teams[0].(map[string]any)["id"] != "annet" || teams[1].(map[string]any)["name"] != "Team B" {
		t.Fatalf("options: %d %v", code, out)
	}
	for team, want := range map[string]int{"team-b": 200, "team-a": 400, "annet": 200, "nope": 400, "0b0d1c3e-1111-4222-8333-444455556666": 400} {
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
	if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "kari", "", `{"teamId":"team-b","role":"utvikler"}`); code != 400 {
		t.Errorf("team source down must fail closed: %d", code)
	}
	if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "kari", "", `{"teamId":"annet","role":"utvikler"}`); code != 200 {
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

func TestBudgetRaiseOrphanReplaced(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	if code, _ := e.raise("ola", "k1"); code != 201 {
		t.Fatal(code)
	}
	e.gh.budgets["ola"] = nil // an admin deleted our budget in GitHub
	if code, out := e.raise("ola", "k2"); code != 201 || out["budgetUsd"] != float64(450) {
		t.Fatalf("raise after our budget vanished: %d %v", code, out)
	}
	if code, out := e.raise("ola", "k3"); code != 201 {
		t.Fatalf("next raise must patch the new budget, not see own_budget: %d %v", code, out)
	}
	if got := strings.Join(e.gh.writes, ","); got != "POST,POST,PATCH b2" {
		t.Errorf("writes: %s", got)
	}
	if n := e.count("SELECT count(*) FROM budget_audit WHERE action = 'expire_delete' AND github_budget_id = 'b1'"); n != 1 {
		t.Errorf("gone budget audited: %d", n)
	}
}

func TestBudgetRaiseClientDisconnectStillRecorded(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.postWait = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/api/v1/copilot/budget/raises", strings.NewReader(`{"category":"koding","expectedValue":"spare_tid"}`))
	req.Header.Set("Idempotency-Key", "k1")
	req = req.WithContext(context.WithValue(ctx, userContextKey, &User{Email: "ola@nav.no"}))
	done := make(chan struct{})
	go func() { e.router.ServeHTTP(httptest.NewRecorder(), req); close(done) }()
	for e.count("SELECT count(*) FROM budget_raises") == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	cancel() // the client hangs up while GitHub is creating the budget
	time.Sleep(20 * time.Millisecond)
	close(e.gh.postWait)
	<-done
	if n := e.count("SELECT count(*) FROM budget_raises WHERE status = 'applied'"); n != 1 {
		t.Errorf("raise must finish after a disconnect: applied=%d", n)
	}
	if n := e.count("SELECT count(*) FROM budget_github WHERE github_budget_id = 'b1'"); n != 1 {
		t.Errorf("budget ID must be recorded: %d", n)
	}
}

func TestBudgetRaiseLostResponseAdopted(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.dropPOST = true
	if code, out := e.raise("ola", "k1"); code != 201 {
		t.Fatalf("lost create response must adopt the matching budget: %d %v", code, out)
	}
	if code, _ := e.raise("ola", "k2"); code != 201 {
		t.Fatalf("next raise: %d", code)
	}
}

func TestBudgetRaisePendingRows(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	pool, _ := e.h.store.get(context.Background())
	ins := `INSERT INTO budget_raises (github_login, idempotency_key, month, amount_usd, budget_set_usd, category, team_id, expected_value, status, created_at)
		VALUES ('ola', $1, '2026-12-01', 150, 300, 'koding', 'annet', 'spare_tid', 'pending', now() - $2::interval)`
	if _, err := pool.Exec(context.Background(), ins, "fresh", "0s"); err != nil {
		t.Fatal(err)
	}
	if code, out := e.raise("ola", "k1"); code != 409 || !strings.HasSuffix(out["type"].(string), "raise_in_progress") {
		t.Fatalf("fresh pending: %d %v", code, out)
	}
	// The process died after GitHub created the budget: stale pending row,
	// unrecorded budget with exactly our amount and expiry. Adopt, do not lock out.
	if _, err := pool.Exec(context.Background(), "UPDATE budget_raises SET created_at = now() - interval '1 hour'"); err != nil {
		t.Fatal(err)
	}
	e.gh.budgets["ola"] = []ghBudget{{ID: "lost", BudgetScope: "user", BudgetType: "BundlePricing", SKU: "ai_credits", Amount: 300, ExpiresAt: "2027-01-01"}}
	if code, out := e.raise("ola", "k2"); code != 201 || out["budgetUsd"] != float64(450) {
		t.Fatalf("stale pending: %d %v", code, out)
	}
	if got := strings.Join(e.gh.writes, ","); got != "PATCH lost" {
		t.Errorf("writes: %s", got)
	}
	if n := e.count("SELECT count(*) FROM budget_raises WHERE idempotency_key = 'fresh' AND status = 'applied'"); n != 1 {
		t.Errorf("stale row reconciled: %d", n)
	}
}

func TestBudgetRaiseVerifyExpiry(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.noExpiry = true
	if code, out := e.raise("ola", "k1"); code != 502 || out["raise"].(map[string]any)["status"] != "failed" {
		t.Fatalf("budget without our expiry must not be applied: %d %v", code, out)
	}
}

func TestBudgetRaiseListGuards(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.nextPage = true
	if code, _ := e.raise("ola", "k1"); code != 502 {
		t.Errorf("multi-page list: %d", code)
	}
	e.gh.nextPage = false
	e.gh.budgets["ola"] = []ghBudget{{ID: "org", BudgetScope: "organization", Amount: 900}}
	if code, _ := e.raise("ola", "k2"); code != 409 {
		t.Errorf("unknown scope must count as foreign: %d", code)
	}
	// The writer itself refuses an ID that is not in budget_github.
	if _, err := e.h.writer.patchOwnBudget(context.Background(), "org", 1, time.Now()); err == nil {
		t.Error("PATCH on a foreign ID must be refused")
	}
	if len(e.gh.writes) != 0 {
		t.Errorf("writes: %v", e.gh.writes)
	}
}

func TestBudgetRaiseClampAndDryRunWithoutKey(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.h.defaultUSD = 200
	var got []any
	for i := range 4 {
		_, out := e.raise("ola", fmt.Sprint("k", i))
		got = append(got, out["budgetUsd"])
	}
	if fmt.Sprint(got) != "[350 500 650 750]" {
		t.Errorf("clamp to the ceiling: %v", got)
	}
	e.h.writer, e.h.mode = nil, "dry_run"
	e.profile("kari")
	if code, out := e.raise("kari", "d1"); code != 201 || out["dryRun"] != true {
		t.Errorf("dry run without write key: %d %v", code, out)
	}
}

// A Postgres that accepts TCP and never answers must not slow GET /budget.
func TestBudgetStoreHangingPostgres(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		var held []net.Conn // accepted, never answered
		for {
			c, err := ln.Accept()
			if err != nil {
				for _, c := range held {
					_ = c.Close()
				}
				return
			}
			held = append(held, c)
		}
	}()
	h, err := newBudgetRaiseHandlers(&Config{BudgetDatabaseURL: "postgres://x:y@" + ln.Addr().String() + "/db?sslmode=disable", BudgetTimeZone: "Europe/Oslo"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() { _, _ = h.monthRaises(context.Background(), "ola", time.Now()) })
	}
	wg.Wait()
	if d := time.Since(start); d > 2*budgetGetRaisesTimeout {
		t.Errorf("4 concurrent lookups took %v", d)
	}
	start = time.Now()
	if _, err := h.monthRaises(context.Background(), "ola", time.Now()); err == nil || time.Since(start) > 100*time.Millisecond {
		t.Errorf("a recent failure must fail fast: %v %v", err, time.Since(start))
	}
}

func TestBudgetWriteBudgetTypeDefault(t *testing.T) {
	t.Setenv("BUDGET_GITHUB_BUDGET_TYPE", "")
	t.Setenv("BUDGET_GITHUB_APP_ID", "")
	if got := loadConfig().BudgetGitHubBudgetType; got != "BundlePricing" {
		t.Errorf("default budget_type: %q (every live AI-credit budget lists BundlePricing)", got)
	}
}

// Two replicas share Postgres but not their in-process locks: the advisory
// lock and the pending-row check must still give one step per raise.
func TestBudgetRaiseAcrossReplicas(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	h2, err := newBudgetRaiseHandlers(&Config{BudgetDatabaseURL: e.dsn, BudgetWrites: "on", BudgetDefaultUSD: 150, BudgetTimeZone: "Europe/Oslo", BudgetRaisesOpen: true}, e.h.writer, fakeSeats{})
	if err != nil {
		t.Fatal(err)
	}
	h2.now, h2.teams, h2.limiter.per = e.h.now, e.h.teams, 1000
	t.Cleanup(func() {
		if h2.store.pool != nil {
			h2.store.pool.Close()
		}
	})
	r2 := makeAPIRouter(&Config{}, nil, nil, nil, NewIdentityResolverChain(emailResolver{}), h2)
	for i := range 20 {
		login := fmt.Sprint("user", i)
		e.profile(login)
		var wg sync.WaitGroup
		for j, router := range []http.Handler{e.router, r2} {
			wg.Go(func() {
				req := httptest.NewRequest("POST", "/api/v1/copilot/budget/raises", strings.NewReader(`{"category":"koding","expectedValue":"spare_tid"}`))
				req.Header.Set("Idempotency-Key", fmt.Sprint("k", j))
				req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Email: login + "@nav.no"}))
				router.ServeHTTP(httptest.NewRecorder(), req)
			})
		}
		wg.Wait()
		applied := e.count("SELECT count(*) FROM budget_raises WHERE github_login = $1 AND status = 'applied'", login)
		e.gh.mu.Lock()
		bs := e.gh.budgets[login]
		e.gh.mu.Unlock()
		if len(bs) != 1 || int(bs[0].Amount) != 150+150*applied {
			t.Fatalf("%s: %d applied but GitHub has %+v", login, applied, bs)
		}
	}
}

func (e *budgetEnv) exec(sql string, args ...any) {
	e.t.Helper()
	pool, err := e.h.store.get(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

// GitHub created the budget but we could not store its ID: the raise stays
// pending, and once the database is back the next raise adopts it.
func TestBudgetRaiseUnknownOutcomeStaysPending(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.exec(`CREATE FUNCTION fail_insert() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'down'; END $$ LANGUAGE plpgsql`)
	e.exec(`CREATE TRIGGER fail_budget_github BEFORE INSERT ON budget_github FOR EACH ROW EXECUTE FUNCTION fail_insert()`)
	if code, out := e.raise("ola", "k1"); code != 502 || out["raise"].(map[string]any)["status"] != "pending" {
		t.Fatalf("unrecorded create: %d %v", code, out)
	}
	e.exec("DROP TRIGGER fail_budget_github ON budget_github")
	e.exec("UPDATE budget_raises SET created_at = now() - interval '1 hour'")
	if code, out := e.raise("ola", "k2"); code != 201 || out["budgetUsd"] != float64(450) {
		t.Fatalf("after recovery: %d %v", code, out)
	}
	if got := strings.Join(e.gh.writes, ","); got != "POST,PATCH b1" {
		t.Errorf("writes: %s", got)
	}
}

func TestBudgetRaiseLostResponseListDown(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.dropPOST = true
	// The pre-raise list works; the reconciliation list after the lost POST fails.
	e.h.afterList = func() { e.gh.mu.Lock(); e.gh.failList = 1; e.gh.mu.Unlock() }
	if code, out := e.raise("ola", "k1"); code != 502 || out["raise"].(map[string]any)["status"] != "pending" {
		t.Fatalf("lost response, lookup down: %d %v", code, out)
	}
	e.h.afterList = nil
	e.exec("UPDATE budget_raises SET created_at = now() - interval '1 hour'")
	if code, _ := e.raise("ola", "k2"); code != 201 {
		t.Fatalf("next raise must adopt: %d", code)
	}
}

// Replica B lists GitHub, then replica A completes a raise, then B reserves.
// B must notice its list is stale and not treat A's budget as gone.
func TestBudgetRaiseStaleListAcrossReplicas(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	h2, err := newBudgetRaiseHandlers(&Config{BudgetDatabaseURL: e.dsn, BudgetWrites: "on", BudgetDefaultUSD: 150, BudgetTimeZone: "Europe/Oslo", BudgetRaisesOpen: true}, e.h.writer, fakeSeats{})
	if err != nil {
		t.Fatal(err)
	}
	h2.now, h2.teams = e.h.now, e.h.teams
	t.Cleanup(func() {
		if h2.store.pool != nil {
			h2.store.pool.Close()
		}
	})
	reached, release := make(chan struct{}), make(chan struct{})
	h2.afterList = func() {
		h2.afterList = nil // only the first attempt waits
		close(reached)
		<-release
	}
	r2 := makeAPIRouter(&Config{}, nil, nil, nil, NewIdentityResolverChain(emailResolver{}), h2)
	done := make(chan int)
	go func() {
		req := httptest.NewRequest("POST", "/api/v1/copilot/budget/raises", strings.NewReader(`{"category":"koding","expectedValue":"spare_tid"}`))
		req.Header.Set("Idempotency-Key", "b")
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Email: "ola@nav.no"}))
		rec := httptest.NewRecorder()
		r2.ServeHTTP(rec, req)
		done <- rec.Code
	}()
	<-reached
	if code, _ := e.raise("ola", "a"); code != 201 {
		t.Fatalf("replica A: %d", code)
	}
	close(release)
	if code := <-done; code != 201 {
		t.Fatalf("replica B: %d", code)
	}
	if got := strings.Join(e.gh.writes, ","); got != "POST,PATCH b1" || len(e.gh.budgets["ola"]) != 1 {
		t.Errorf("writes %s, budgets %+v", got, e.gh.budgets["ola"])
	}
}

func TestBudgetRaiseAdoptionIsStrict(t *testing.T) {
	for name, b := range map[string]ghBudget{
		"organization scope": {ID: "x", BudgetScope: "organization", BudgetType: "BundlePricing", SKU: "ai_credits", Amount: 300, ExpiresAt: "2027-01-01"},
		"other type":         {ID: "x", BudgetScope: "user", BudgetType: "SkuPricing", SKU: "ai_credits", Amount: 300, ExpiresAt: "2027-01-01"},
	} {
		t.Run(name, func(t *testing.T) {
			e := newBudgetEnv(t, openBudgetTestDB(t))
			e.profile("ola")
			e.exec(`INSERT INTO budget_raises (github_login, idempotency_key, month, amount_usd, budget_set_usd, category, team_id, expected_value, status, created_at)
				VALUES ('ola', 'old', '2026-12-01', 150, 300, 'koding', 'annet', 'spare_tid', 'pending', now() - interval '1 hour')`)
			e.gh.budgets["ola"] = []ghBudget{b}
			if code, _ := e.raise("ola", "k1"); code != 409 || len(e.gh.writes) != 0 {
				t.Errorf("must not adopt: %d %v", code, e.gh.writes)
			}
		})
	}
}

func TestBudgetRaiseStalePendingFromLastMonth(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.exec(`INSERT INTO budget_raises (github_login, idempotency_key, month, amount_usd, budget_set_usd, category, team_id, expected_value, status, created_at)
		VALUES ('ola', 'nov', '2026-11-01', 150, 300, 'koding', 'annet', 'spare_tid', 'pending', now() - interval '1 hour')`)
	e.gh.budgets["ola"] = []ghBudget{{ID: "nov", BudgetScope: "user", BudgetType: "BundlePricing", SKU: "ai_credits", Amount: 300, ExpiresAt: "2026-12-01"}}
	if code, out := e.raise("ola", "k1"); code != 201 {
		t.Fatalf("last month's stale create must be adopted: %d %v", code, out)
	}
	if got := strings.Join(e.gh.writes, ","); got != "PATCH nov" {
		t.Errorf("writes: %s", got)
	}
}

func TestBudgetRaiseReadBackExact(t *testing.T) {
	for name, set := range map[string]func(*fakeGitHub){
		"amount 300.9": func(f *fakeGitHub) { f.skew = 0.9 },
		"no SKU":       func(f *fakeGitHub) { f.noSKU = true },
	} {
		t.Run(name, func(t *testing.T) {
			e := newBudgetEnv(t, openBudgetTestDB(t))
			e.profile("ola")
			set(e.gh)
			if code, out := e.raise("ola", "k1"); code != 502 || out["raise"].(map[string]any)["status"] != "failed" {
				t.Errorf("must not be applied: %d %v", code, out)
			}
		})
	}
}

func TestBudgetBodyStrict(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	ok := `{"teamId":"annet","role":"utvikler"}`
	for name, body := range map[string]string{
		"concatenated": ok + `{"teamId":"team-a","role":"leder"}`,
		"garbage":      ok + `xyz`,
		"oversized":    ok + strings.Repeat(" ", 9<<10) + "x",
	} {
		if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "ola", "", body); code != 400 {
			t.Errorf("%s: %d", name, code)
		}
	}
	if e.count("SELECT count(*) FROM budget_profiles") != 0 {
		t.Error("a rejected body must not be saved")
	}
}

func TestBudgetRaiseProfileTeamRevalidated(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")                                    // team-b
	e.h.teams = fakeTeams{activity: []string{"team-b"}} // ola is in no team now, team-b only in recent activity
	if code, out := e.do("GET", "/api/v1/copilot/budget/profile", "ola", "", ""); code != 200 || out["teamId"] != "annet" || out["leftTeamId"] != "team-b" {
		t.Errorf("left team must read as annet: %d %v", code, out)
	}
	if code, _ := e.raise("ola", "k1"); code != 201 || e.count("SELECT count(*) FROM budget_raises WHERE team_id = 'annet'") != 1 {
		t.Errorf("left team must raise as annet: %d", code)
	}
}

func TestBudgetNewRosterTeamNotInOrgList(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.h.teams = fakeTeams{mine: []string{"team-ny"}} // in the roster, not yet in the cached org list
	if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "ola", "", `{"teamId":"team-ny","role":"utvikler"}`); code != 200 {
		t.Errorf("a roster team must be selectable before the org list catches up: %d", code)
	}
	if _, out := e.do("GET", "/api/v1/copilot/budget/profile", "ola", "", ""); out["leftTeamId"] != nil {
		t.Errorf("a new roster team is not left: %v", out)
	}
}

func TestBudgetRaiseReplayBeforeTeamCheck(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	body := `{"category":"koding","expectedValue":"spare_tid","teamId":"team-b"}`
	if code, _ := e.do("POST", "/api/v1/copilot/budget/raises", "ola", "k1", body); code != 201 {
		t.Fatalf("first raise: %d", code)
	}
	e.h.teams = fakeTeams{mine: []string{"team-a"}} // left team-b, then the client retries
	if code, out := e.do("POST", "/api/v1/copilot/budget/raises", "ola", "k1", body); code != 200 || out["replayed"] != true || len(e.gh.writes) != 1 {
		t.Errorf("retry must replay, not fail on team: %d %v writes=%d", code, out, len(e.gh.writes))
	}
}

func TestRosterTeamsNoSnapshotIsUnknown(t *testing.T) {
	now := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	snap := func(daysAgo int) bigquery.NullDate {
		return bigquery.NullDate{Date: civil.DateOf(now).AddDays(-daysAgo), Valid: true}
	}
	if _, err := rosterTeams([]rosterRow{{}}, now); err == nil {
		t.Error("no snapshot must be an error, not an empty membership")
	}
	if _, err := rosterTeams([]rosterRow{{SnapshotDate: snap(rosterMaxAgeDays + 1)}}, now); err == nil {
		t.Error("a stale snapshot must count as unknown")
	}
	if got, err := rosterTeams([]rosterRow{{SnapshotDate: snap(rosterMaxAgeDays)}}, now); err != nil || len(got) != 0 {
		t.Errorf("recent snapshot without the user: %v %v", got, err)
	}
	if got, _ := rosterTeams([]rosterRow{{SnapshotDate: snap(0), TeamSlug: bigquery.NullString{StringVal: "team-b", Valid: true}}}, now); len(got) != 1 {
		t.Errorf("member: %v", got)
	}
}

func TestCachedRosterRechecksSnapshotAge(t *testing.T) {
	day := time.Date(2026, 11, 23, 23, 55, 0, 0, time.UTC)
	now := day
	c := &CachedBigQueryClient{cache: NewCache(time.Hour), now: func() time.Time { return now }}
	c.cache.Set("roster_rows_ola", []rosterRow{{
		SnapshotDate: bigquery.NullDate{Date: civil.DateOf(day).AddDays(-rosterMaxAgeDays), Valid: true},
		TeamSlug:     bigquery.NullString{StringVal: "team-b", Valid: true},
	}})
	if got, err := c.GetRosterTeams(context.Background(), "ola"); err != nil || len(got) != 1 {
		t.Fatalf("fresh cached roster: %v %v", got, err)
	}
	now = day.Add(10 * time.Minute) // past midnight: the cached snapshot is now too old
	if _, err := c.GetRosterTeams(context.Background(), "ola"); err == nil {
		t.Error("a cache hit must re-check the snapshot age")
	}
}

func TestBudgetTeamLookupsStayUnderBFFDeadline(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	const d = 300 * time.Millisecond
	e.h.teams = fakeTeams{mine: []string{"team-b"}, delay: d}
	start := time.Now()
	if code, _ := e.do("GET", "/api/v1/copilot/budget/options", "ola", "", ""); code != 200 || time.Since(start) >= 2*d {
		t.Errorf("roster and names must load concurrently: %d in %v", code, time.Since(start))
	}
	start = time.Now()
	if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "ola", "", `{"teamId":"team-b","role":"utvikler"}`); code != 200 || time.Since(start) >= 2*d {
		t.Errorf("validation must not wait for display names: %d in %v", code, time.Since(start))
	}
}

func TestBudgetProfileRoleOnlySaveDuringOutage(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola") // team-b
	e.h.teams = fakeTeams{err: fmt.Errorf("bq down")}
	if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "ola", "", `{"teamId":"team-b","role":"leder"}`); code != 200 {
		t.Errorf("unchanged saved team must be accepted during an outage: %d", code)
	}
	if code, _ := e.do("PUT", "/api/v1/copilot/budget/profile", "ola", "", `{"teamId":"team-a","role":"leder"}`); code != 400 {
		t.Errorf("a changed team must still fail closed: %d", code)
	}
}

func TestBudgetProfileTeamSourceDownIsNotLeft(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola") // team-b
	e.h.teams = fakeTeams{err: fmt.Errorf("bq down")}
	if code, out := e.do("GET", "/api/v1/copilot/budget/profile", "ola", "", ""); code != 200 || out["teamId"] != "team-b" || out["leftTeamId"] != nil {
		t.Errorf("an outage must not say the user left: %d %v", code, out)
	}
}

func TestBudgetRaiseTruncatedCreateIsUnknown(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.truncate = true
	if code, out := e.raise("ola", "k1"); code != 201 {
		t.Fatalf("a 2xx with a cut-off body must be reconciled by listing: %d %v", code, out)
	}
	if code, _ := e.raise("ola", "k2"); code != 201 {
		t.Fatalf("next raise: %d", code)
	}
}

func TestBudgetRaiseAuditIsDurable(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.exec(`CREATE FUNCTION fail_audit() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'down'; END $$ LANGUAGE plpgsql`)
	e.exec(`CREATE TRIGGER fail_audit_insert BEFORE INSERT ON budget_audit FOR EACH ROW EXECUTE FUNCTION fail_audit()`)
	if code, _ := e.raise("ola", "k1"); code != 503 || len(e.gh.writes) != 0 || e.count("SELECT count(*) FROM budget_raises") != 0 {
		t.Fatalf("no audit intent, no write: %d %v", code, e.gh.writes)
	}
	e.exec("DROP TRIGGER fail_audit_insert ON budget_audit")
	e.exec(`CREATE TRIGGER fail_audit_update BEFORE UPDATE ON budget_audit FOR EACH ROW EXECUTE FUNCTION fail_audit()`)
	if code, out := e.raise("ola", "k2"); code != 502 || out["raise"].(map[string]any)["status"] != "pending" {
		t.Fatalf("result not audited must not be applied: %d %v", code, out)
	}
	if n := e.count("SELECT count(*) FROM budget_audit"); n != 1 {
		t.Errorf("the intent row must stay: %d", n)
	}
	// Recovery: the budget is already ours (recorded), only the result was
	// not audited. The next raise must reconcile it as applied, audited.
	e.exec("DROP TRIGGER fail_audit_update ON budget_audit")
	e.exec("UPDATE budget_raises SET created_at = now() - interval '1 hour'")
	if code, out := e.raise("ola", "k3"); code != 201 || out["budgetUsd"] != float64(450) {
		t.Fatalf("raise after recovery: %d %v", code, out)
	}
	if n := e.count("SELECT count(*) FROM budget_raises WHERE idempotency_key = 'k2' AND status = 'applied'"); n != 1 {
		t.Error("the recorded raise must be reconciled as applied, not failed")
	}
	if n := e.count("SELECT count(*) FROM budget_audit WHERE action = 'retry' AND github_budget_id = 'b1'"); n != 1 {
		t.Errorf("recovered outcome audited: %d", n)
	}
}

// A 5xx on create or patch may still have taken effect: pending, then reconciled.
func TestBudgetRaiseServerErrorIsUnknown(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.raise("ola", "k1")
	e.gh.mu.Lock()
	e.gh.failNext = 503
	e.gh.mu.Unlock()
	if code, out := e.raise("ola", "k2"); code != 502 || out["raise"].(map[string]any)["status"] != "pending" {
		t.Fatalf("patch 5xx: %d %v", code, out)
	}
	e.gh.failNext = 0
	e.exec("UPDATE budget_raises SET created_at = now() - interval '1 hour'")
	// GitHub did not apply it (fake left 300): reconciled as failed, counted.
	if code, out := e.raise("ola", "k3"); code != 201 || out["budgetUsd"] != float64(600) {
		t.Fatalf("after reconcile: %d %v", code, out)
	}
	// Read-back unavailable after a successful write: pending, then applied.
	e2 := newBudgetEnv(t, openBudgetTestDB(t))
	e2.profile("kari")
	e2.h.afterList = func() { e2.gh.mu.Lock(); e2.gh.failList = 1; e2.gh.mu.Unlock() }
	if code, out := e2.raise("kari", "k1"); code != 502 || out["raise"].(map[string]any)["status"] != "pending" {
		t.Fatalf("read-back down: %d %v", code, out)
	}
	e2.h.afterList = nil
	e2.exec("UPDATE budget_raises SET created_at = now() - interval '1 hour'")
	e2.raise("kari", "k2")
	if n := e2.count("SELECT count(*) FROM budget_raises WHERE idempotency_key = 'k1' AND status = 'applied'"); n != 1 {
		t.Error("written but unverified raise must reconcile as applied")
	}
}

func TestBudgetRaiseCreateServerErrorIsUnknown(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.failNext = 500
	if code, out := e.raise("ola", "k1"); code != 502 || out["raise"].(map[string]any)["status"] != "pending" {
		t.Fatalf("create 5xx may have created the budget: %d %v", code, out)
	}
}

// #2715: replica A has reserved and is writing; replica B lists GitHub before
// A's create and reserves while A finishes. A's bookkeeping must not slip in
// between B's reads, or B counts A's raise but misses A's budget and creates
// a second one.
func TestBudgetRaiseWriteSerialisedWithReserve(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	h2, err := newBudgetRaiseHandlers(&Config{BudgetDatabaseURL: e.dsn, BudgetWrites: "on", BudgetDefaultUSD: 150, BudgetTimeZone: "Europe/Oslo", BudgetRaisesOpen: true}, e.h.writer, fakeSeats{})
	if err != nil {
		t.Fatal(err)
	}
	h2.now, h2.teams = e.h.now, e.h.teams
	t.Cleanup(func() {
		if h2.store.pool != nil {
			h2.store.pool.Close()
		}
	})
	r2 := makeAPIRouter(&Config{}, nil, nil, nil, NewIdentityResolverChain(emailResolver{}), h2)
	post := func(router http.Handler, key string) int {
		req := httptest.NewRequest("POST", "/api/v1/copilot/budget/raises", strings.NewReader(`{"category":"koding","expectedValue":"spare_tid"}`))
		req.Header.Set("Idempotency-Key", key)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Email: "ola@nav.no"}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	e.gh.postWait = make(chan struct{}) // A's create waits in GitHub
	aDone := make(chan int)
	go func() { aDone <- post(e.router, "a") }()
	for e.count("SELECT count(*) FROM budget_raises") == 0 {
		time.Sleep(5 * time.Millisecond)
	}

	inReserve, releaseB := make(chan struct{}), make(chan struct{})
	h2.midReserve = func() {
		h2.midReserve = nil
		close(inReserve)
		<-releaseB
	}
	bDone := make(chan int)
	go func() { bDone <- post(r2, "b") }()
	<-inReserve
	close(e.gh.postWait) // A creates, records and finishes, if nothing stops it
	// Release B once A has either finished (broken code) or is waiting on
	// B's advisory lock (fixed code); no timing involved.
	a, aGot := 0, false
	for !aGot && e.waiters("wait_event = 'advisory'") == 0 {
		select {
		case a = <-aDone:
			aGot = true
		case <-time.After(2 * time.Millisecond):
		}
	}
	close(releaseB)
	if !aGot {
		a = <-aDone
	}
	b := <-bDone
	e.gh.mu.Lock()
	defer e.gh.mu.Unlock()
	if len(e.gh.budgets["ola"]) != 1 || a != 201 || b != 409 {
		t.Fatalf("A=%d B=%d (want 201, 409), GitHub has %+v", a, b, e.gh.budgets["ola"])
	}
}

// waiters counts sessions blocked on a heavyweight lock matching cond.
func (e *budgetEnv) waiters(cond string) int {
	return e.count("SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND " + cond)
}

func (e *budgetEnv) secondReplica() (*budgetRaiseHandlers, func(key string) int) {
	e.t.Helper()
	h2, err := newBudgetRaiseHandlers(&Config{BudgetDatabaseURL: e.dsn, BudgetWrites: "on", BudgetDefaultUSD: 150, BudgetTimeZone: "Europe/Oslo", BudgetRaisesOpen: true}, e.h.writer, fakeSeats{})
	if err != nil {
		e.t.Fatal(err)
	}
	h2.now, h2.teams = e.h.now, e.h.teams
	e.t.Cleanup(func() {
		if h2.store.pool != nil {
			h2.store.pool.Close()
		}
	})
	r2 := makeAPIRouter(&Config{}, nil, nil, nil, NewIdentityResolverChain(emailResolver{}), h2)
	return h2, func(key string) int {
		req := httptest.NewRequest("POST", "/api/v1/copilot/budget/raises", strings.NewReader(`{"category":"koding","expectedValue":"spare_tid"}`))
		req.Header.Set("Idempotency-Key", key)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Email: "ola@nav.no"}))
		rec := httptest.NewRecorder()
		r2.ServeHTTP(rec, req)
		return rec.Code
	}
}

func (e *budgetEnv) waitFor(sql string, args ...any) {
	e.t.Helper()
	for e.count(sql, args...) == 0 {
		time.Sleep(2 * time.Millisecond)
	}
}

// Review of #2716: without the lock on the final step, B can read A's
// recorded ID (missing from B's older list, so "gone"), then see A already
// applied, and create a second budget.
func TestBudgetRaiseFinalStepLocked(t *testing.T) {
	var sawLockWait atomic.Bool
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	h2, postB := e.secondReplica()

	e.gh.postWait = make(chan struct{})
	aDone := make(chan int)
	go func() { c, _ := e.raise("ola", "a"); aDone <- c }()
	e.waitFor("SELECT count(*) FROM budget_raises")

	// Hold A's final status update on a row lock, without the advisory lock.
	pool, _ := e.h.store.get(context.Background())
	hold, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hold.Exec(context.Background(), "SELECT 1 FROM budget_raises FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	commit := func() { once.Do(func() { _ = hold.Commit(context.Background()) }) }
	t.Cleanup(commit)

	h2.afterList = func() {
		h2.afterList = nil
		close(e.gh.postWait)
		e.waitFor("SELECT count(*) FROM budget_github")
		e.waitFor("SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND wait_event <> 'advisory'") // A at its final step
	}
	h2.midReserve = func() { // reached first only when the final step is unlocked
		h2.midReserve = nil
		commit()
		e.waitFor("SELECT count(*) FROM budget_raises WHERE status = 'applied'")
	}
	bDone, bFinished := make(chan int, 1), make(chan struct{})
	go func() { bDone <- postB("b"); close(bFinished) }()
	go func() { // fixed code: B waits on A's advisory lock instead
		for {
			select {
			case <-bFinished:
				return
			default:
			}
			if e.waiters("wait_event = 'advisory'") > 0 {
				sawLockWait.Store(true)
				commit()
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	a := <-aDone
	b := 0
	select {
	case b = <-bDone:
	case <-time.After(20 * time.Second):
		t.Fatal("B did not finish")
	}
	e.gh.mu.Lock()
	n := len(e.gh.budgets["ola"])
	e.gh.mu.Unlock()
	if n != 1 || e.count("SELECT count(*) FROM budget_github WHERE deleted_at IS NULL") != 1 {
		t.Fatalf("A=%d B=%d: GitHub has %d budgets", a, b, n)
	}
	// The version alone would also save B (it re-lists); prove the lock did.
	if !sawLockWait.Load() {
		t.Fatal("B must wait on A's advisory lock at A's final step")
	}
}

func TestBudgetRaiseFinalUpdateOnlyFromPending(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.gh.postWait = make(chan struct{})
	done := make(chan map[string]any)
	go func() { _, out := e.raise("ola", "a"); done <- out }()
	e.waitFor("SELECT count(*) FROM budget_raises")
	e.exec("UPDATE budget_raises SET status = 'failed'") // another replica reconciled it
	close(e.gh.postWait)
	out := <-done
	if out["raise"].(map[string]any)["status"] != "failed" || e.count("SELECT count(*) FROM budget_raises WHERE status = 'failed'") != 1 {
		t.Fatalf("the final step must not overwrite a reconciliation: %v", out)
	}
}

// #2706 thread: retrying a stuck pending raise with the same key must
// reconcile it, not replay pending forever.
func TestBudgetRaiseSameKeyRetryReconcilesStuck(t *testing.T) {
	for name, tc := range map[string]struct {
		budget ghBudget
		want   string
	}{
		"budget exists": {ghBudget{ID: "lost", BudgetScope: "user", BudgetType: "BundlePricing", SKU: "ai_credits", Amount: 300, ExpiresAt: "2027-01-01"}, "applied"},
		"no budget":     {ghBudget{}, "failed"},
	} {
		t.Run(name, func(t *testing.T) {
			e := newBudgetEnv(t, openBudgetTestDB(t))
			e.profile("ola")
			e.exec(`INSERT INTO budget_raises (github_login, idempotency_key, month, amount_usd, budget_set_usd, category, team_id, expected_value, status, created_at)
				VALUES ('ola', 'k1', '2026-12-01', 150, 300, 'koding', 'annet', 'spare_tid', 'pending', now() - interval '1 hour')`)
			if tc.budget.ID != "" {
				e.gh.budgets["ola"] = []ghBudget{tc.budget}
			}
			code, out := e.raise("ola", "k1")
			if code != 200 || out["replayed"] != true || out["raise"].(map[string]any)["status"] != tc.want {
				t.Fatalf("same-key retry: %d %v", code, out)
			}
			if e.count("SELECT count(*) FROM budget_raises") != 1 || len(e.gh.writes) != 0 {
				t.Errorf("a retry must not start a new raise: writes %v", e.gh.writes)
			}
		})
	}
}

// Without a GitHub list (dry run, no write key) a stuck raise cannot be
// verified: a same-key retry replays pending instead of marking it failed.
func TestBudgetRaiseStuckWithoutListStaysPending(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	e.exec(`INSERT INTO budget_raises (github_login, idempotency_key, month, amount_usd, budget_set_usd, category, team_id, expected_value, status, created_at)
		VALUES ('ola', 'k1', '2026-12-01', 150, 300, 'koding', 'annet', 'spare_tid', 'pending', now() - interval '1 hour')`)
	e.h.writer, e.h.mode = nil, "dry_run"
	if code, out := e.raise("ola", "k1"); code != 200 || out["raise"].(map[string]any)["status"] != "pending" {
		t.Fatalf("same-key retry without a list: %d %v", code, out)
	}
	if e.count("SELECT count(*) FROM budget_raises WHERE status = 'pending'") != 1 {
		t.Error("the stuck raise must stay pending")
	}
}

// A PATCH that GitHub took, followed by a failed bookkeeping step, is an
// unknown outcome (pending), not failed.
func TestBudgetRaisePatchBookkeepingFailureIsUnknown(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	if code, _ := e.raise("ola", "k1"); code != 201 {
		t.Fatal(code)
	}
	e.exec(`CREATE FUNCTION fail_record() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'down'; END $$ LANGUAGE plpgsql`)
	e.exec(`CREATE TRIGGER fail_record BEFORE INSERT OR UPDATE ON budget_github FOR EACH ROW EXECUTE FUNCTION fail_record()`)
	if code, out := e.raise("ola", "k2"); code != 502 || out["raise"].(map[string]any)["status"] != "pending" {
		t.Fatalf("patched but not recorded: %d %v", code, out)
	}
}

// The schema allows at most one pending raise per user, even if two writers
// skip the application lock entirely.
func TestBudgetOnePendingPerUserInSchema(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	pool, err := e.h.store.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ins := `INSERT INTO budget_raises (github_login, idempotency_key, month, amount_usd, category, team_id, expected_value, status)
		VALUES ('ola', $1, '2026-12-01', 150, 'koding', 'annet', 'spare_tid', $2)`
	ctx := context.Background()
	// Two writers, no advisory lock: the second pending insert must fail.
	if _, err := pool.Exec(ctx, ins, "a", "pending"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ins, "b", "pending"); err == nil || !strings.Contains(err.Error(), "budget_raises_one_pending") {
		t.Fatalf("second pending raise must be rejected by the schema: %v", err)
	}
	if _, err := pool.Exec(ctx, ins, "c", "failed"); err != nil {
		t.Errorf("other statuses are not limited: %v", err)
	}
}

// A row count stops being a version once retention deletes rows: A's raise
// (+2) and deleting one old applied row (-2) cancel out. The version does not.
func TestBudgetRaiseVersionSurvivesRetention(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	for _, k := range []string{"old1", "old2"} {
		e.exec(`INSERT INTO budget_raises (github_login, idempotency_key, month, amount_usd, category, team_id, expected_value, status)
			VALUES ('ola', $1, '2025-01-01', 150, 'koding', 'annet', 'spare_tid', 'applied')`, k)
	}
	h2, postB := e.secondReplica()
	h2.afterList = func() { // B's list is taken; now A raises and retention runs
		h2.afterList = nil
		if code, _ := e.raise("ola", "a"); code != 201 {
			t.Errorf("A: %d", code)
		}
		e.exec("DELETE FROM budget_raises WHERE idempotency_key = 'old1'")
	}
	b := postB("b")
	e.gh.mu.Lock()
	defer e.gh.mu.Unlock()
	if b != 201 || len(e.gh.budgets["ola"]) != 1 || e.gh.budgets["ola"][0].Amount != 450 || strings.Join(e.gh.writes, ",") != "POST,PATCH b1" {
		t.Fatalf("B=%d must re-list and PATCH to 450: writes %v, GitHub %+v", b, e.gh.writes, e.gh.budgets["ola"])
	}
}

// A replay reads only: it must not bump the version, or harmless retries
// could make another request's list stale until it gives up with 409.
func TestBudgetRaiseReadOnlyDoesNotBumpVersion(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	e.profile("ola")
	if code, _ := e.raise("ola", "k1"); code != 201 {
		t.Fatal(code)
	}
	v := e.count(raiseFingerprintSQL, "ola")
	for range 3 {
		if code, _ := e.raise("ola", "k1"); code != 200 {
			t.Fatal(code)
		}
	}
	if got := e.count(raiseFingerprintSQL, "ola"); got != v {
		t.Errorf("replays bumped the version from %d to %d", v, got)
	}
}

// #2720: before opening, admin override only works for test users.
func TestBudgetAdminOverrideRespectsNotOpen(t *testing.T) {
	e := newBudgetEnv(t, openBudgetTestDB(t))
	t.Setenv(adminGroupEnv, "admins")
	e.h.open, e.h.testUsers = false, map[string]bool{"tester": true}
	if code, out := e.do("POST", "/api/v1/copilot/budget/admin/override", "boss", "a1", `{"githubLogin":"ola"}`, "admins"); code != 409 || !strings.HasSuffix(out["type"].(string), "not_open") {
		t.Fatalf("override for a non-test user while closed: %d %v", code, out)
	}
	if len(e.gh.writes) != 0 || e.count("SELECT count(*) FROM budget_raises") != 0 {
		t.Fatalf("nothing may be written: %v", e.gh.writes)
	}
	if n := e.count("SELECT count(*) FROM budget_audit WHERE action = 'admin_override' AND actor = 'boss' AND raise_id IS NULL"); n != 1 {
		t.Errorf("refused override audited: %d", n)
	}
	if code, out := e.do("POST", "/api/v1/copilot/budget/admin/override", "boss", "a2", `{"githubLogin":"tester"}`, "admins"); code != 201 {
		t.Fatalf("override for a test user while closed: %d %v", code, out)
	}
}
