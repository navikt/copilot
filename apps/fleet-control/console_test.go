package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeConsole serves the user(email).teams query from members (email to
// teams), checks the bearer token and counts calls.
func fakeConsole(t *testing.T, members map[string][]string, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			Variables struct {
				Email string `json:"email"`
			} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		teams, ok := members[req.Variables.Email]
		if !ok {
			_, _ = w.Write([]byte(`{"errors":[{"message":"The specified user was not found.","path":["user"]}],"data":null}`))
			return
		}
		nodes := []map[string]any{}
		for _, s := range teams {
			nodes = append(nodes, map[string]any{"team": map[string]string{"slug": s}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"user": map[string]any{"teams": map[string]any{
			"nodes": nodes, "pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
		}}}})
	}))
}

func tokenFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(p, []byte("tok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConsoleTeams(t *testing.T) {
	ctx := context.Background()
	var calls int
	ts := fakeConsole(t, map[string][]string{"kari.nordmann@nav.no": {"team-a", "copilot"}, "ola@nav.no": {}}, &calls)
	defer ts.Close()
	c := newConsoleTeams(ts.URL, tokenFile(t))

	if ok, err := c.IsMember(ctx, "team-a", "kari.nordmann@nav.no"); err != nil || !ok {
		t.Fatalf("member: got %v, %v", ok, err)
	}
	if ok, err := c.IsMember(ctx, "team-a", "  Kari.Nordmann@NAV.no "); err != nil || !ok {
		t.Fatalf("mixed-case member: got %v, %v", ok, err)
	}
	if ok, err := c.IsMember(ctx, "team-b", "kari.nordmann@nav.no"); err != nil || ok {
		t.Fatalf("other team: got %v, %v", ok, err)
	}
	if ok, err := c.IsMember(ctx, "team-a", "ola@nav.no"); err != nil || ok {
		t.Fatalf("non-member: got %v, %v", ok, err)
	}
	if got, _ := c.Teams(ctx, "kari.nordmann@nav.no"); !slices.Equal(got, []string{"copilot", "team-a"}) {
		t.Fatalf("teams: got %v", got)
	}
	if ok, err := c.IsMember(ctx, "team-a", "nobody@nav.no"); err != nil || ok {
		t.Fatalf("user not in Nais: got %v, %v, want no teams", ok, err)
	}
	if calls != 3 {
		t.Fatalf("cache: got %d calls, want 3", calls)
	}
	c.now = func() time.Time { return time.Now().Add(consoleTeamsTTL) }
	_, _ = c.Teams(ctx, "kari.nordmann@nav.no")
	if calls != 4 {
		t.Fatalf("expired cache: got %d calls, want 4", calls)
	}
}

func TestConsoleTeamsFailClosed(t *testing.T) {
	ctx := context.Background()
	var calls int
	ts := fakeConsole(t, map[string][]string{"kari@nav.no": {"team-a"}}, &calls)
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// A JSON body, so only the status check catches it.
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"data":{"user":{"teams":{"nodes":[{"team":{"slug":"team-a"}}],"pageInfo":{"hasNextPage":false}}}}}`))
	}))
	partial := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"The specified user was not found.","path":["team"]}],"data":{"user":{"teams":{"nodes":[{"team":{"slug":"team-a"}}],"pageInfo":{"hasNextPage":false}}}}}`))
	}))
	defer partial.Close()
	// Answers without checking the token, so only the token-file check catches a missing file.
	open := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"user":{"teams":{"nodes":[{"team":{"slug":"team-a"}}],"pageInfo":{"hasNextPage":false}}}}}`))
	}))
	defer open.Close()
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"user":null}}`))
	}))
	defer ts.Close()
	defer down.Close()
	defer empty.Close()
	badTok := filepath.Join(t.TempDir(), "token")
	_ = os.WriteFile(badTok, []byte("expired"), 0o600)

	for name, c := range map[string]*consoleTeams{
		"api down":           newConsoleTeams(down.URL, tokenFile(t)),
		"api unreachable":    newConsoleTeams("http://127.0.0.1:1", tokenFile(t)),
		"token file missing": newConsoleTeams(open.URL, filepath.Join(t.TempDir(), "missing")),
		"token rejected":     newConsoleTeams(ts.URL, badTok),
		"empty answer":       newConsoleTeams(empty.URL, tokenFile(t)),
		"schema error":       newConsoleTeams(partial.URL, tokenFile(t)),
	} {
		if ok, err := c.IsMember(ctx, "team-a", "kari@nav.no"); err == nil || ok {
			t.Errorf("%s: got %v, %v, want error", name, ok, err)
		}
	}

	// Through the write guard: a failed lookup is 503, not 403 or 2xx.
	srv := newServer()
	h := srv.routes(testAuthz(t), adminID, newConsoleTeams(down.URL, tokenFile(t)), nil)
	req := httptest.NewRequest(http.MethodPut, "/admin/v1/jobs/team-a/kill-switch", strings.NewReader(`{"scope":"team","on":false}`))
	req.Header.Set("Authorization", "Bearer "+userTok("A111111")+":kari@nav.no")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("write guard with API down: got %d, want 503", rec.Code)
	}
}

func TestTeamsFromEnvConsole(t *testing.T) {
	t.Setenv("NAIS_SERVICE_ACCOUNT_TOKEN_PATH", "/var/run/token")
	t.Setenv("FLEET_CONTROL_STATIC_TEAMS", "")
	d, err := teamsFromEnv("prod-gcp")
	if _, ok := d.(*consoleTeams); err != nil || !ok {
		t.Fatalf("got %T, %v", d, err)
	}
	t.Setenv("FLEET_CONTROL_STATIC_TEAMS", `{"team-a":["a@nav.no"]}`)
	if _, err := teamsFromEnv("prod-gcp"); err == nil {
		t.Fatal("static list in prod: want error")
	}
	if d, err := teamsFromEnv("dev-gcp"); err != nil {
		t.Fatal(err)
	} else if _, ok := d.(*consoleTeams); !ok {
		t.Fatalf("dev with both: got %T, want console", d)
	}
}

// pagedConsole answers by the `after` variable: pages[""] is the first page.
func pagedConsole(t *testing.T, pages map[string]string, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		var req struct {
			Variables struct {
				After *string `json:"after"`
			} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		key := ""
		if req.Variables.After != nil {
			key = *req.Variables.After
		}
		body, ok := pages[key]
		if !ok {
			http.Error(w, "unknown cursor", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
}

func page(slug string, next string) string {
	info := `{"hasNextPage":false}`
	if next != "" {
		info = `{"hasNextPage":true,"endCursor":"` + next + `"}`
	}
	return `{"data":{"user":{"teams":{"nodes":[{"team":{"slug":"` + slug + `"}}],"pageInfo":` + info + `}}}}`
}

const notFound = `{"errors":[{"message":"The specified user was not found.","path":["user"]}],"data":null}`

func TestConsoleTeamsPages(t *testing.T) {
	ctx := context.Background()
	run := func(pages map[string]string) ([]string, *consoleTeams, int, error) {
		var calls int
		ts := pagedConsole(t, pages, &calls)
		t.Cleanup(ts.Close)
		c := newConsoleTeams(ts.URL, tokenFile(t))
		got, err := c.Teams(ctx, "kari@nav.no")
		return got, c, calls, err
	}
	got, _, _, err := run(map[string]string{"": page("team-b", "c1"), "c1": page("team-a", "")})
	if err != nil || !slices.Equal(got, []string{"team-a", "team-b"}) {
		t.Errorf("two pages: got %v, %v", got, err)
	}
	if _, _, _, e := run(map[string]string{"": `{"data":{"user":{"teams":{"nodes":[],"pageInfo":{"hasNextPage":true}}}}}`}); e == nil || !strings.Contains(e.Error(), "without cursor") {
		t.Error("next page without cursor: want error")
	}
	if _, _, _, e := run(map[string]string{"": page("Bad_Slug", "")}); e == nil {
		t.Error("invalid slug: want error")
	}
	if _, _, _, e := run(map[string]string{"": page("team-a", "c1"), "c1": page("team-b", "c1")}); e == nil || !strings.Contains(e.Error(), "too many") {
		t.Error("endless pages: want error")
	}
	got, c, calls, err := run(map[string]string{"": page("team-a", "c1"), "c1": notFound})
	if err == nil {
		t.Errorf("not-found on page 2: got %v, want error", got)
	}
	if _, e := c.Teams(ctx, "kari@nav.no"); e == nil || calls < 2 {
		t.Error("not-found on page 2 must not be cached")
	}
	two := `{"errors":[{"message":"The specified user was not found.","path":["user"]},{"message":"x"}],"data":null}`
	if _, _, _, e := run(map[string]string{"": two}); e == nil {
		t.Error("not-found plus another error: want error")
	}
}

func TestNormEmailRefusesNonASCII(t *testing.T) {
	if got := normEmail("Kari@nav.no"); got != "" {
		t.Fatalf("kelvin sign: got %q, want \"\"", got)
	}
	if ok, _ := (staticTeams{"team-a": {"kari@nav.no"}}).IsMember(context.Background(), "team-a", "Kari@nav.no"); ok {
		t.Fatal("kelvin sign matched kari@nav.no")
	}
	if got := normEmail(" Kari@NAV.no "); got != "kari@nav.no" {
		t.Fatalf("ascii: got %q", got)
	}
}

func TestConsoleTeamsCacheBounded(t *testing.T) {
	c := newConsoleTeams("", "")
	base := time.Now()
	c.now = func() time.Time { return base }
	for i := range consoleTeamsMaxCache - 1 {
		c.store(strconv.Itoa(i), nil)
	}
	c.now = func() time.Time { return base.Add(consoleTeamsTTL / 2) }
	c.store("fresh", nil)
	c.now = func() time.Time { return base.Add(consoleTeamsTTL) }
	c.store("new", nil)
	// Expired entries go, the fresh one stays: a sweep, not a clear.
	if _, ok := c.cache["fresh"]; !ok || len(c.cache) != 2 {
		t.Fatalf("sweep: %d left, fresh kept %v", len(c.cache), ok)
	}
	for i := range consoleTeamsMaxCache + 5 {
		c.store(strconv.Itoa(i), nil)
	}
	if len(c.cache) > consoleTeamsMaxCache {
		t.Fatalf("cache unbounded: %d", len(c.cache))
	}
}
