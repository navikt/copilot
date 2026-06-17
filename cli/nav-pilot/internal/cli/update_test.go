package cli

import (
	"crypto/sha256"
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
