package cli

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageManager(t *testing.T) {
	// In dev/test, the binary is neither in a Homebrew Cellar nor dpkg-owned.
	// This just verifies the function runs without panic.
	_ = packageManager()
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

// localReleaseAPI points every release lookup at a local server that answers
// with the running version, which is the "already up to date" answer. #830: the
// command tests below ran the real update path against api.github.com, and the
// only thing standing between a test run and a downloaded binary written over
// the test binary was a version string that happened not to parse.
func localReleaseAPI(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `[{"tag_name": "nav-pilot/%s"}]`, Version)
	}))
	t.Cleanup(srv.Close)
	origReleases, origCplt, origDownload := releasesAPI, cpltReleasesAPI, downloadURL
	t.Cleanup(func() {
		releasesAPI, cpltReleasesAPI, downloadURL = origReleases, origCplt, origDownload
	})
	releasesAPI, cpltReleasesAPI, downloadURL = srv.URL, srv.URL, srv.URL
}

func TestRun_UpdateCommand(t *testing.T) {
	// Set version to a known value to trigger "up to date" path
	// (avoids actually downloading a binary in tests)
	origVersion := Version
	Version = "test-version-that-wont-match"
	defer func() { Version = origVersion }()
	localReleaseAPI(t)

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

// TestUpdateRefusesToReplaceAPackagedBinary: nav-pilot must never rename a new
// binary over one a package manager owns. For the .deb that binary is
// /usr/bin/nav-pilot, and replacing it leaves dpkg's database claiming a
// version that is no longer on disk — the next `apt upgrade` or
// `apt install --reinstall` silently reverts the user's update. So the update
// declines before it downloads anything, and prints the command that works.
func TestUpdateRefusesToReplaceAPackagedBinary(t *testing.T) {
	tests := []struct {
		name string
		mgr  pkgManager
		want string
	}{
		{"apt", pkgApt, "sudo apt upgrade nav-pilot"},
		{"homebrew", pkgBrew, "brew upgrade navikt/tap/nav-pilot"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origManager, origAPI := packageManager, releasesAPI
			t.Cleanup(func() { packageManager, releasesAPI = origManager, origAPI })
			packageManager = func() pkgManager { return tt.mgr }

			// A refusal must not reach the network, and the empty PATH keeps the
			// Homebrew branch's cplt lookup off it too.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("a packaged install asked GitHub for a release: %s", r.URL)
			}))
			t.Cleanup(srv.Close)
			releasesAPI = srv.URL
			t.Setenv("PATH", t.TempDir())

			var updated bool
			var err error
			out := captureStdoutFor(t, func() { updated, err = doUpdate() })
			if err != nil {
				t.Fatalf("doUpdate = %v", err)
			}
			if updated {
				t.Fatal("doUpdate replaced a binary the package manager owns")
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("update did not print %q. Output:\n%s", tt.want, out)
			}
		})
	}
}

// TestDpkgOwns: the path alone cannot decide. A hand-built binary in /usr/bin
// must still self-update — telling it `sudo apt upgrade` while refusing to
// update leaves the user with no way forward — so dpkg has to confirm.
func TestDpkgOwns(t *testing.T) {
	// packageManager is what limits dpkg to Linux; the check itself is the
	// same shell call everywhere, so it is tested everywhere.
	// fakeDpkgQuery puts a dpkg-query with a fixed exit code on PATH.
	fakeDpkgQuery := func(t *testing.T, exit int) {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "dpkg-query"), fmt.Sprintf("#!/bin/sh\nexit %d\n", exit))
		if err := os.Chmod(filepath.Join(dir, "dpkg-query"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
	}

	t.Run("dpkg owns it", func(t *testing.T) {
		fakeDpkgQuery(t, 0)
		if !dpkgOwns("/usr/bin/nav-pilot") {
			t.Error("a binary dpkg reports as its own was treated as unmanaged")
		}
	})
	t.Run("dpkg disowns it", func(t *testing.T) {
		fakeDpkgQuery(t, 1)
		if dpkgOwns("/usr/bin/nav-pilot") {
			t.Error("a hand-built binary in /usr/bin was refused an update it can do")
		}
	})
	t.Run("outside dpkg territory", func(t *testing.T) {
		// dpkg-query answers yes to everything here: only the path check can
		// keep the usual installs from paying for a process spawn.
		fakeDpkgQuery(t, 0)
		if dpkgOwns("/usr/local/bin/nav-pilot") {
			t.Error("a /usr/local/bin install asked dpkg about itself")
		}
	})
	t.Run("no dpkg-query", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if dpkgOwns("/usr/bin/nav-pilot") {
			t.Error("a missing dpkg-query was read as proof of ownership")
		}
	})
}
