package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
)

var (
	releasesAPI = "https://api.github.com/repos/navikt/copilot/releases"
	downloadURL = "https://github.com/navikt/copilot/releases/download"

	// cplt ships from its own repo, with its own release train and unprefixed tags.
	cpltReleasesAPI = "https://api.github.com/repos/navikt/cplt/releases"
)

// httpClient is the client used for all HTTP requests. Overridable in tests.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	},
}

type ghRelease struct {
	TagName string `json:"tag_name"`
}

// cmdUpdate checks for a newer version and updates the binary in-place.
// If installed via Homebrew, it tells the user to use brew upgrade instead.
func cmdUpdate() error {
	_, err := doUpdate()
	return err
}

// doUpdate performs the update check and, if a newer version is available,
// downloads and installs it. It returns updated=true only if the binary was
// actually replaced, so callers can distinguish "already up to date" (no-op)
// from "successfully updated" and avoid re-executing when nothing changed.
func doUpdate() (updated bool, err error) {
	if isBrewManaged() {
		// Print first, then check cplt: the cplt lookup can take seconds, and
		// the "managed by Homebrew" line used to be instant.
		fmt.Println("nav-pilot is managed by Homebrew.")
		fmt.Println()
		formulae := "navikt/tap/nav-pilot"
		if cpltBehind() {
			formulae += " navikt/tap/cplt"
		}
		fmt.Printf("  brew upgrade %s\n", formulae)
		return false, nil
	}

	current := Version
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	latest, tag, err := fetchLatestVersion(ctx)
	if err != nil {
		return false, fmt.Errorf("could not check for updates: %w", err)
	}

	if !versionNewer(latest, current) {
		fmt.Printf("✓ nav-pilot is up to date (%s)\n", current)
		return false, nil
	}

	fmt.Printf("Update available: %s → %s\n", current, latest)

	self, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("cannot determine binary path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return false, fmt.Errorf("cannot resolve binary path: %w", err)
	}

	asset := fmt.Sprintf("nav-pilot-%s-%s", runtime.GOOS, runtime.GOARCH)
	assetURL := fmt.Sprintf("%s/%s/%s", downloadURL, tag, asset)
	checksumURL := fmt.Sprintf("%s/%s/SHA256SUMS", downloadURL, tag)

	fmt.Printf("→ Downloading %s...\n", asset)
	bin, err := httpGet(assetURL)
	if err != nil {
		return false, fmt.Errorf("download failed: %w", err)
	}

	if err := verifyChecksum(bin, asset, checksumURL); err != nil {
		return false, err
	}

	// Atomic replace: write temp file next to binary, then rename
	dir := filepath.Dir(self)
	tmp, err := os.CreateTemp(dir, ".nav-pilot-update-*")
	if err != nil {
		return false, fmt.Errorf("cannot create temp file (is %s writable?): %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return false, fmt.Errorf("write failed: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return false, fmt.Errorf("chmod failed: %w", err)
	}

	if err := os.Rename(tmpPath, self); err != nil {
		return false, fmt.Errorf("replace failed: %w", err)
	}

	// Invalidate the staleness cache now that we're on the latest version,
	// so a subsequent process doesn't see a stale "update available" entry
	// (e.g. if this rename raced with a fresh release check elsewhere).
	// The profile rides along: invalidating the version entry must not also
	// drop the published default for a day, which is how long it would be
	// before the fresh LastChecked lets the next check refetch it.
	var models map[string]string
	if prev := artifacts.ReadCache(); prev != nil {
		models = prev.DefaultModels
	}
	artifacts.WriteCache(&artifacts.StalenessCache{
		LastChecked:   time.Now().UTC().Format(time.RFC3339),
		LatestVersion: latest,
		DefaultModels: models,
	})

	fmt.Printf("✓ Updated to nav-pilot %s\n", latest)
	return true, nil
}

// isBrewManaged returns true if the running binary lives inside a Homebrew prefix.
func isBrewManaged() bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return false
	}
	return strings.Contains(self, "/Cellar/") || strings.Contains(self, "/homebrew/")
}

// fetchLatestVersion queries the GitHub releases API for the latest nav-pilot release.
// It filters by the "nav-pilot/" tag prefix to avoid picking up other monorepo releases.
// Returns the raw version (matching the build-injected format, e.g. "2026.04.13-170138-abc1234")
// and the full tag (e.g. "nav-pilot/2026.04.13-170138-abc1234").
func fetchLatestVersion(ctx context.Context) (ver string, tag string, err error) {
	return fetchLatestRelease(ctx, releasesAPI, "nav-pilot/")
}

// fetchLatestRelease returns the newest release from a GitHub releases API whose
// tag carries prefix (empty prefix = the newest release, for repos that do not
// prefix their tags). Returns the version (tag minus prefix) and the full tag.
func fetchLatestRelease(ctx context.Context, api, prefix string) (ver string, tag string, err error) {
	token := os.Getenv("GITHUB_TOKEN")
	resp, err := releasesRequest(ctx, api, token)
	if err != nil {
		return "", "", err
	}

	// A GITHUB_TOKEN scoped for packages (but not for api.github.com) answers
	// 401/403 here and would kill release checks for good. The releases API is
	// public, so retry once anonymously. The authenticated attempt stays first
	// for its higher rate limit.
	if token != "" && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		resp.Body.Close()
		resp, err = releasesRequest(ctx, api, "")
		if err != nil {
			return "", "", err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", "", err
	}

	for _, rel := range releases {
		if strings.HasPrefix(rel.TagName, prefix) {
			tag = rel.TagName
			ver = strings.TrimPrefix(tag, prefix)
			return ver, tag, nil
		}
	}

	return "", "", fmt.Errorf("no release found with tag prefix %q", prefix)
}

// releasesRequest issues one GET against a GitHub releases API, authenticated
// when token is non-empty.
func releasesRequest(ctx context.Context, api, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", api+"?per_page=20", nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return httpClient.Do(req)
}

// latestCpltVersion returns the newest published cplt version. The short
// timeout keeps a slow or unreachable GitHub from stalling doctor or upgrade —
// callers treat an error as "could not check", never as "outdated".
// A var so tests can stub it without a network call.
var latestCpltVersion = func() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ver, _, err := fetchLatestRelease(ctx, cpltReleasesAPI, "")
	return ver, err
}

// cpltCommandTimeout bounds every cplt process spawn. Each spawn gets its own
// deadline: no check may share a wall clock with an unrelated one.
const cpltCommandTimeout = 2 * time.Second

// runBounded runs a command with its own deadline and returns its stdout.
func runBounded(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cpltCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

// runBoundedCombined is runBounded, but keeps stderr — for commands whose
// interesting output is not reliably on stdout.
func runBoundedCombined(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cpltCommandTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// cpltSkew is the three-way outcome of the cplt version-skew check.
type cpltSkew int

const (
	cpltVersionUnknown cpltSkew = iota // could not tell — never report as current
	cpltVersionCurrent
	cpltVersionBehind
)

// classifyCpltSkew compares the installed cplt version against the latest
// release. A failed lookup or a version neither side can parse is unknown, not
// current: a security-relevant check must never go green on missing data.
func classifyCpltSkew(installed, latest string, lookupErr error) cpltSkew {
	if lookupErr != nil || !versionParseable(latest) || !versionParseable(installed) {
		return cpltVersionUnknown
	}
	if versionNewer(latest, installed) {
		return cpltVersionBehind
	}
	return cpltVersionCurrent
}

// cpltVersionSkew reads the installed cplt version and compares it to the
// latest release. Everything uncertain — no cplt, no network, an unparseable
// version — is cpltVersionUnknown.
func cpltVersionSkew() cpltSkew {
	cliPath, err := findCplt()
	if err != nil {
		return cpltVersionUnknown
	}
	out, err := runBounded(cliPath, "--version")
	if err != nil {
		return cpltVersionUnknown
	}
	latest, lerr := latestCpltVersion()
	return classifyCpltSkew(parseCpltVersion(string(out)), latest, lerr)
}

// cpltBehind reports whether the installed cplt is older than the latest cplt
// release. Uncertainty answers false: nav-pilot stays quiet rather than guessing.
func cpltBehind() bool {
	return cpltVersionSkew() == cpltVersionBehind
}

func httpGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// fetchProfile downloads the published nav-pilot profile. The body is capped
// because it is untrusted network input about to be parsed as JSON, and no
// GITHUB_TOKEN is attached: the profile is public and lives on another host.
func fetchProfile(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", artifacts.ProfileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, artifacts.ProfileURL)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<10))
}

// verifyChecksum downloads SHA256SUMS and verifies the binary checksum.
// Fails hard if checksums cannot be fetched or the asset entry is missing.
func verifyChecksum(data []byte, asset, checksumURL string) error {
	fmt.Print("→ Verifying checksum...")
	sums, err := httpGet(checksumURL)
	if err != nil {
		return fmt.Errorf(" failed to download checksums: %w", err)
	}

	var expected string
	for _, line := range strings.Split(string(sums), "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), asset) {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				expected = fields[0]
			}
			break
		}
	}

	if expected == "" {
		return fmt.Errorf(" no checksum entry found for %s", asset)
	}

	actual := sha256sum(data)
	if actual != expected {
		return fmt.Errorf(" checksum mismatch!\n  Expected: %s\n  Got:      %s", expected, actual)
	}

	fmt.Println(" ✓")
	return nil
}

func sha256sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
