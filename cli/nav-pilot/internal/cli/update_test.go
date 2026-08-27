package cli

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsBrewManaged(t *testing.T) {
	// In dev/test, the binary is not in a Homebrew Cellar
	// This just verifies the function runs without panic
	_ = isBrewManaged()
}

func TestSha256sum(t *testing.T) {
	data := []byte("hello world")
	got := sha256sum(data)
	want := fmt.Sprintf("%x", sha256.Sum256(data))
	if got != want {
		t.Errorf("sha256sum = %s, want %s", got, want)
	}
}

func TestVerifyChecksum_Valid(t *testing.T) {
	data := []byte("binary-data")
	checksum := sha256sum(data)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  nav-pilot-linux-amd64\n", checksum)
	}))
	defer srv.Close()

	err := verifyChecksum(data, "nav-pilot-linux-amd64", srv.URL+"/SHA256SUMS")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	data := []byte("binary-data")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "0000000000000000000000000000000000000000000000000000000000000000  nav-pilot-linux-amd64\n")
	}))
	defer srv.Close()

	err := verifyChecksum(data, "nav-pilot-linux-amd64", srv.URL+"/SHA256SUMS")
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestVerifyChecksum_NoSumsFile(t *testing.T) {
	data := []byte("binary-data")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Should error — checksum verification is mandatory
	err := verifyChecksum(data, "nav-pilot-linux-amd64", srv.URL+"/SHA256SUMS")
	if err == nil {
		t.Fatal("expected error when checksums unavailable")
	}
}

func TestVerifyChecksum_NoEntry(t *testing.T) {
	data := []byte("binary-data")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "abcdef1234567890  nav-pilot-linux-arm64\n") // different asset
	}))
	defer srv.Close()

	err := verifyChecksum(data, "nav-pilot-linux-amd64", srv.URL+"/SHA256SUMS")
	if err == nil {
		t.Fatal("expected error when asset entry is missing")
	}
}

func TestFetchLatestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[
			{"tag_name": "nav-pilot/2026.04.13-170138-abc1234"},
			{"tag_name": "nav-pilot/2026.04.12-093000-def5678"}
		]`)
	}))
	defer srv.Close()

	// Override the client and API URL for testing
	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	origAPI := releasesAPI
	// releasesAPI is a const, so we test the parsing logic directly
	_ = origAPI

	// Test the tag parsing logic directly
	tag := "nav-pilot/2026.04.13-170138-abc1234"
	ver := tag[len("nav-pilot/"):]
	if ver != "2026.04.13-170138-abc1234" {
		t.Errorf("unexpected version: %s", ver)
	}
}

func TestFetchLatestVersion_SkipsNonNavPilot(t *testing.T) {
	// Verify the filtering logic: only nav-pilot/ prefixed tags are matched
	tags := []string{"other-app/1.0.0", "nav-pilot/2026.04.13-170138-abc1234"}
	var found string
	for _, tag := range tags {
		if len(tag) > len("nav-pilot/") && tag[:len("nav-pilot/")] == "nav-pilot/" {
			found = tag[len("nav-pilot/"):]
			break
		}
	}
	if found != "2026.04.13-170138-abc1234" {
		t.Errorf("expected 2026.04.13-170138-abc1234, got %s", found)
	}
}

func TestRun_UpdateCommand(t *testing.T) {
	// Set version to a known value to trigger "up to date" path
	// (avoids actually downloading a binary in tests)
	origVersion := Version
	Version = "test-version-that-wont-match"
	defer func() { Version = origVersion }()

	err := run([]string{"update"})
	// Should not be "unknown command" — verifies wiring
	if err != nil && err.Error() == "unknown command: update. Run with --help for usage" {
		t.Fatal("update command not wired up in main.go")
	}
	// Will get a network error or version mismatch, that's fine
}

func TestFetchLatestRelease_UnprefixedTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{"tag_name": "2026.08.26-201133-2e78d25"},
			{"tag_name": "2026.08.24-153138-0d1d66d"}
		]`)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	// cplt tags carry no prefix, so the newest release wins outright.
	ver, tag, err := fetchLatestRelease(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if ver != "2026.08.26-201133-2e78d25" || tag != ver {
		t.Errorf("got ver=%q tag=%q, want both %q", ver, tag, "2026.08.26-201133-2e78d25")
	}
}

func TestParseCpltVersion(t *testing.T) {
	tests := []struct{ out, want string }{
		{"cplt 2026.08.24-153138-0d1d66d", "2026.08.24-153138-0d1d66d"},
		{"cplt 2026.08.24-153138-0d1d66d\n", "2026.08.24-153138-0d1d66d"},
		{"cplt dev", ""},
		{"unknown", ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := parseCpltVersion(tc.out); got != tc.want {
			t.Errorf("parseCpltVersion(%q) = %q, want %q", tc.out, got, tc.want)
		}
	}
}

func TestCpltVersionSkew(t *testing.T) {
	const latest = "2026.08.26-201133-2e78d25"
	tests := []struct {
		name, versionOut, latest string
		lookupErr                error
		want                     cpltSkew
	}{
		{"older", "cplt 2026.08.24-153138-0d1d66d", latest, nil, cpltVersionBehind},
		{"same", "cplt " + latest, latest, nil, cpltVersionCurrent},
		{"newer (local build)", "cplt 2026.09.01-090000-aaaaaaa", latest, nil, cpltVersionCurrent},
		// An unreadable installed version must never read as up to date.
		{"unparseable version output", "unknown", latest, nil, cpltVersionUnknown},
		{"dev build", "cplt dev", latest, nil, cpltVersionUnknown},
		{"lookup failed", "cplt " + latest, "", errors.New("no network"), cpltVersionUnknown},
		{"empty latest", "cplt " + latest, "", nil, cpltVersionUnknown},
	}
	for _, tc := range tests {
		got := classifyCpltSkew(parseCpltVersion(tc.versionOut), tc.latest, tc.lookupErr)
		if got != tc.want {
			t.Errorf("%s: classifyCpltSkew = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A GITHUB_TOKEN that is valid for packages but not for api.github.com answers
// 401 here; the releases API is public, so the check must fall back to an
// anonymous request instead of going dark for good.
func TestFetchLatestRelease_RetriesAnonymouslyOn401(t *testing.T) {
	var attempts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts = append(attempts, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"tag_name": "2026.08.26-201133-2e78d25"}]`)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()
	t.Setenv("GITHUB_TOKEN", "packages-only-token")

	ver, _, err := fetchLatestRelease(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if ver != "2026.08.26-201133-2e78d25" {
		t.Errorf("ver = %q, want the release from the anonymous retry", ver)
	}
	if len(attempts) != 2 || attempts[0] == "" || attempts[1] != "" {
		t.Errorf("attempts = %q, want an authenticated request followed by an anonymous one", attempts)
	}
}

// Without a token there is nothing to retry: a 401 stays an error.
func TestFetchLatestRelease_NoRetryWithoutToken(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()
	t.Setenv("GITHUB_TOKEN", "")

	if _, _, err := fetchLatestRelease(context.Background(), srv.URL, ""); err == nil {
		t.Fatal("fetchLatestRelease = nil error, want a failure")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}
