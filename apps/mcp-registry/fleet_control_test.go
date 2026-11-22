package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeFleetControl struct {
	srv         *httptest.Server
	texasStatus int
	status      int
	body        any
	etag        string
	lastINM     string
	requests    atomic.Int32
}

// allowlistEntries returns the first n raw entries of allowlist.json, the form
// fleet-control serves in the registry snapshot.
func allowlistEntries(t *testing.T, n int) []json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile("allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Servers []json.RawMessage `json:"servers"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Servers[:n]
}

func regSnap(gen int, servers []json.RawMessage) map[string]any {
	return map[string]any{"schema_version": 1, "generation": gen, "servers": servers}
}

func newFakeFleetControl(t *testing.T) *fakeFleetControl {
	f := &fakeFleetControl{texasStatus: http.StatusOK, status: http.StatusOK, etag: `"e1"`, body: regSnap(1, allowlistEntries(t, 2))}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		switch r.URL.Path {
		case "/texas":
			var in map[string]string
			_ = json.NewDecoder(r.Body).Decode(&in)
			if f.texasStatus != http.StatusOK || in["target"] != "api://dev-gcp.copilot.fleet-control/.default" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "m2m"})
		case "/internal/v1/snapshots/registry":
			if r.Header.Get("Authorization") != "Bearer m2m" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			f.lastINM = r.Header.Get("If-None-Match")
			w.Header().Set("ETag", f.etag)
			if f.lastINM == f.etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.WriteHeader(f.status)
			_ = json.NewEncoder(w).Encode(f.body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func fleetStore(t *testing.T, f *fakeFleetControl) *Store {
	t.Helper()
	t.Setenv("REGISTRY_SOURCE", "fleet-control")
	t.Setenv("FLEET_CONTROL_URL", f.srv.URL)
	t.Setenv("NAIS_TOKEN_ENDPOINT", f.srv.URL+"/texas")
	t.Setenv("FLEET_CONTROL_AUDIENCE", "api://dev-gcp.copilot.fleet-control/.default")
	fleet, err := fleetControlFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(testConfig(), "allowlist.json", nil)
	s.fleet = fleet
	return s
}

func serverNames(t *testing.T, s *Store) []string {
	t.Helper()
	servers, err := s.GetServers()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, sr := range servers {
		out = append(out, sr.Server.Name)
	}
	return out
}

func TestFleetControlSnapshotThenETag(t *testing.T) {
	f := newFakeFleetControl(t)
	s := fleetStore(t, f)
	if n := len(serverNames(t, s)); n < 3 {
		t.Fatalf("allowlist.json should serve before the first snapshot, got %d servers", n)
	}
	if err := s.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := serverNames(t, s); len(got) != 2 {
		t.Fatalf("servers after snapshot = %v, want 2", got)
	}

	f.body = regSnap(2, nil) // would be rejected if read: proves 304 skips the body
	if err := s.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if f.lastINM != `"e1"` {
		t.Fatalf("If-None-Match = %q, want the stored ETag", f.lastINM)
	}
	if got := serverNames(t, s); len(got) != 2 {
		t.Fatalf("304 changed the servers: %v", got)
	}
}

func TestFleetControlFailsClosed(t *testing.T) {
	invalid := json.RawMessage(`{"name":"io.github.navikt/x","version":"1.0.0"}`)
	cases := map[string]func(t *testing.T, f *fakeFleetControl){
		"empty snapshot":   func(_ *testing.T, f *fakeFleetControl) { f.body = regSnap(2, []json.RawMessage{}) },
		"null servers":     func(_ *testing.T, f *fakeFleetControl) { f.body = regSnap(2, nil) },
		"invalid entry":    func(t *testing.T, f *fakeFleetControl) { f.body = regSnap(2, append(allowlistEntries(t, 1), invalid)) },
		"older generation": func(t *testing.T, f *fakeFleetControl) { f.body = regSnap(0, allowlistEntries(t, 3)) },
		"unknown schema": func(t *testing.T, f *fakeFleetControl) {
			b := regSnap(2, allowlistEntries(t, 3))
			b["schema_version"] = 2
			f.body = b
		},
		"fleet-control down": func(t *testing.T, f *fakeFleetControl) {
			f.status, f.body = http.StatusServiceUnavailable, regSnap(2, allowlistEntries(t, 3))
		},
		"texas down": func(t *testing.T, f *fakeFleetControl) {
			f.texasStatus, f.body = http.StatusInternalServerError, regSnap(2, allowlistEntries(t, 3))
		},
		"malformed json": func(_ *testing.T, f *fakeFleetControl) { f.body = "not an object" },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeFleetControl(t)
			s := fleetStore(t, f)
			if err := s.Reload(t.Context()); err != nil {
				t.Fatal(err)
			}
			good := serverNames(t, s)

			breakIt(t, f)
			f.etag = `"e2"`
			_ = s.Reload(t.Context())
			if got := serverNames(t, s); !slices.Equal(got, good) {
				t.Fatalf("bad snapshot replaced the servers: %v, want %v", got, good)
			}
			if len(s.static) != len(good) {
				t.Fatalf("bad snapshot replaced the static set: %d entries, want %d", len(s.static), len(good))
			}
		})
	}
}

func TestFleetControlDownAtStartupKeepsAllowlist(t *testing.T) {
	f := newFakeFleetControl(t)
	s := fleetStore(t, f)
	file := serverNames(t, s)
	f.srv.Close()
	_ = s.Reload(t.Context())
	if got := serverNames(t, s); !slices.Equal(got, file) {
		t.Fatalf("servers = %v, want allowlist.json %v", got, file)
	}
}

func TestFleetControlFromEnv(t *testing.T) {
	full := map[string]string{ //nolint:gosec // env names, not credentials
		"REGISTRY_SOURCE": "fleet-control", "FLEET_CONTROL_URL": "http://fleet-control",
		"NAIS_TOKEN_ENDPOINT": "http://texas/api/v1/token", "FLEET_CONTROL_AUDIENCE": "api://x/.default",
	}
	cases := []struct {
		name, unset, override, wantErr string
		wantNil                        bool
	}{
		{name: "default is git", unset: "REGISTRY_SOURCE", wantNil: true},
		{name: "git", override: "git", wantNil: true},
		{name: "unknown source", override: "fleetcontrol", wantErr: "unsupported REGISTRY_SOURCE"},
		{name: "no URL", unset: "FLEET_CONTROL_URL", wantErr: "FLEET_CONTROL_URL"},
		{name: "no Texas", unset: "NAIS_TOKEN_ENDPOINT", wantErr: "NAIS_TOKEN_ENDPOINT"},
		{name: "no audience", unset: "FLEET_CONTROL_AUDIENCE", wantErr: "FLEET_CONTROL_AUDIENCE"},
		{name: "full"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range full {
				t.Setenv(k, v)
			}
			if tc.unset != "" {
				t.Setenv(tc.unset, "")
			}
			if tc.override != "" {
				t.Setenv("REGISTRY_SOURCE", tc.override)
			}
			f, err := fleetControlFromEnv()
			switch {
			case tc.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
			case err != nil:
				t.Fatal(err)
			case tc.wantNil != (f == nil):
				t.Fatalf("fleet = %v, wantNil %v", f, tc.wantNil)
			}
		})
	}
}

func TestFlagOffReadsOnlyAllowlist(t *testing.T) {
	f := newFakeFleetControl(t)
	s := NewStore(testConfig(), "allowlist.json", nil)
	before := serverNames(t, s)
	if err := s.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := f.requests.Load(); n != 0 {
		t.Fatalf("git source sent %d requests to fleet-control", n)
	}
	if got := serverNames(t, s); !slices.Equal(got, before) {
		t.Fatalf("git source changed servers: %v", got)
	}
}

// Reloads overlap in production (refresher and post-connect reload). The
// second must not fetch until the first has applied, or an older snapshot
// could land last and stay behind a 304.
func TestFleetControlReloadsDoNotInterleave(t *testing.T) {
	f := newFakeFleetControl(t)
	s := fleetStore(t, f)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	s.loader = func(context.Context) ([]DBEntry, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return nil, nil
	}
	first := make(chan error)
	go func() { first <- s.Reload(t.Context()) }()
	<-entered
	before := f.requests.Load()
	second := make(chan error)
	go func() { second <- s.Reload(t.Context()) }()
	time.Sleep(50 * time.Millisecond) // give an unserialised Reload time to fetch
	if n := f.requests.Load(); n != before {
		t.Fatalf("second reload fetched (%d requests) while the first had not applied", n-before)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
}
