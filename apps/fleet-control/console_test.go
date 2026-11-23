package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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
			_, _ = w.Write([]byte(`{"errors":[{"message":"user not found"}],"data":null}`))
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
	if _, err := c.IsMember(ctx, "team-a", "nobody@nav.no"); err == nil {
		t.Fatal("unknown user: want error")
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
		_, _ = w.Write([]byte(`{"errors":[{"message":"x"}],"data":{"user":{"teams":{"nodes":[{"team":{"slug":"team-a"}}],"pageInfo":{"hasNextPage":false}}}}}`))
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
	req := httptest.NewRequest(http.MethodGet, "/admin/v1/servers/team-a/x", nil)
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
