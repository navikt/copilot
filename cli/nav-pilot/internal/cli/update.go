package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
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

// releasesPage is where a user picks one nav-pilot version by hand.
const releasesPage = "https://github.com/navikt/copilot/releases?q=nav-pilot"

// cmdUpgrade is `nav-pilot upgrade` (and the deprecated `update`). It parses
// its own flags: the shared loop accepts every command's flags everywhere, so
// `upgrade --dry-run` used to parse and then upgrade anyway, and `upgrade
// 2026.09.10-…` installed the latest release without a word about the version.
func cmdUpgrade(command string, args []string) error {
	check := false
	for _, a := range args {
		switch a {
		case "-n", "--dry-run":
			check = true
		case "-y", "--yes":
			// upgrade never asks. Accepted so a script can say it means it.
		case "-h", "--help":
			printHelp(os.Stdout, "upgrade")
			return nil
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf("unknown flag %s. Run nav-pilot help upgrade for its flags", a)
			}
			return fmt.Errorf("upgrade doesn't take a version: it installs the latest release. To pin %s, use your package manager or download it from %s", a, releasesPage)
		}
	}
	if !versionParseable(Version) {
		return fmt.Errorf("can't self-update a development build (%s). Install a release instead: brew install navikt/tap/nav-pilot, or the release script (https://github.com/navikt/copilot/blob/main/docs/README.nav-pilot.md#kom-i-gang)", Version)
	}
	if command == "update" {
		fmt.Fprintf(os.Stderr, "%s %s is deprecated. Use: %s\n\n",
			yellow("⚠"), bold("nav-pilot update"), bold("nav-pilot upgrade"))
	}
	return runWithCommandTelemetry(command, telemetryMode(), "none", func() error {
		if check {
			return checkUpdate()
		}
		_, err := doUpdate(os.Stdout)
		return err
	})
}

// checkUpdate is `upgrade --dry-run`: it says whether a newer release exists
// and changes nothing. Exit 1 when one does, the way sync reports updates.
func checkUpdate() error {
	latest, _, err := latestRelease()
	if err != nil {
		return err
	}
	if !versionNewer(latest, Version) {
		fmt.Printf("✓ nav-pilot is up to date (%s)\n", Version)
		return nil
	}
	how := "nav-pilot upgrade"
	if mgr := packageManager(); mgr.Name != "" {
		how = navPilotUpgradeCmd(mgr)
	}
	fmt.Printf("Update available: %s → %s\nRun %s to install it.\n", Version, latest, bold(how))
	return errUpdatesAvailable
}

// releaseCheckTimeout bounds an explicit upgrade's release lookup. Long enough
// for a slow link, short enough that a GitHub that does not answer is reported
// rather than waited on.
var releaseCheckTimeout = 15 * time.Second

// latestRelease looks up the newest nav-pilot release for an explicit upgrade.
func latestRelease() (ver, tag string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), releaseCheckTimeout)
	defer cancel()
	ver, tag, err = fetchLatestVersion(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return "", "", fmt.Errorf("could not check for updates: GitHub did not answer within %s. Try again later, or download nav-pilot from %s", releaseCheckTimeout, releasesPage)
	}
	if err != nil {
		return "", "", fmt.Errorf("could not check for updates: %w", err)
	}
	return ver, tag, nil
}

// doUpdate performs the update check and, if a newer version is available,
// downloads and installs it. It returns updated=true only if the binary was
// actually replaced, so callers can distinguish "already up to date" (no-op)
// from "successfully updated" and avoid re-executing when nothing changed.
//
// Everything it prints goes to w: stdout for `nav-pilot upgrade`, whose output
// this is, and stderr for the auto-update in front of another command, whose
// stdout (a --json document, say) must stay its own.
func doUpdate(w io.Writer) (updated bool, err error) {
	if mgr := packageManager(); mgr.Name != "" {
		// Up to date says so, rather than sending the user to brew for a
		// no-op. The same cached lookup as the startup nudge (at most one
		// request a day), so the two never disagree; when it knows nothing,
		// the package manager's command is still the answer.
		if a := assessStaleness(Version); a.LatestVersion != "" && !versionNewer(a.LatestVersion, Version) {
			fmt.Fprintf(w, "✓ nav-pilot is up to date (%s)\n", Version)
			return false, nil
		}
		// Print first, then check cplt: the cplt lookup can take seconds, and
		// the "managed by Homebrew" line used to be instant.
		fmt.Fprintf(w, "nav-pilot is managed by %s.\n", mgr.Label)
		fmt.Fprintln(w)
		upgrade := navPilotUpgradeCmd(mgr)
		if cpltBehind() {
			// The apt archive ships cplt too, and apt upgrades both in one go.
			upgrade += mgr.Pick(" navikt/tap/cplt", " cplt")
		}
		fmt.Fprintf(w, "  %s\n", upgrade)
		return false, nil
	}

	current := Version
	latest, tag, err := latestRelease()
	if err != nil {
		return false, err
	}

	if !versionNewer(latest, current) {
		fmt.Fprintf(w, "✓ nav-pilot is up to date (%s)\n", current)
		return false, nil
	}

	fmt.Fprintf(w, "Update available: %s → %s\n", current, latest)

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

	fmt.Fprintf(w, "→ Downloading %s...\n", asset)
	bin, err := httpGet(assetURL)
	if err != nil {
		return false, fmt.Errorf("download failed: %w", err)
	}

	if err := verifyChecksum(w, bin, asset, checksumURL); err != nil {
		return false, err
	}

	// Atomic replace: write temp file next to binary, then rename
	dir := filepath.Dir(self)
	tmp, err := os.CreateTemp(dir, ".nav-pilot-update-*")
	if err != nil {
		return false, fmt.Errorf("can't write to %s, where nav-pilot is installed: %w\nnav-pilot is unchanged. Run the upgrade as the user who owns that directory, or reinstall somewhere you can write: bash install.sh --dir ~/.local/bin (see https://github.com/navikt/copilot/blob/main/docs/README.nav-pilot.md#kom-i-gang)", dir, err)
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
	artifacts.WriteCache(&artifacts.StalenessCache{
		LastChecked:   time.Now().UTC().Format(time.RFC3339),
		LatestVersion: latest,
	})

	if p := autoUpdateFailedPath(); p != "" {
		_ = os.Remove(p) // this one worked, so the next auto-update need not wait
	}
	fmt.Fprintf(w, "✓ Updated to nav-pilot %s\n", latest)
	return true, nil
}

// packageManager reports which package manager owns the running binary, so
// nav-pilot never replaces a file Homebrew or dpkg tracks behind its back. It
// is a variable so a test can assert what a packaged install is told without
// being installed from a package.
var packageManager = domain.PkgSelf

// navPilotUpgradeCmd is the command that upgrades this nav-pilot install.
func navPilotUpgradeCmd(m domain.PkgManager) string {
	return m.Pick("brew upgrade navikt/tap/nav-pilot", "sudo apt upgrade nav-pilot")
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
	resp, err := githubGet(ctx, api+"?per_page=20", "")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", releaseCheckError(resp)
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

// releaseCheckError explains a failed release lookup. A rate limit says so,
// says whether a GITHUB_TOKEN was tried, and where to get nav-pilot meanwhile:
// "GitHub API returned 403" named none of that.
func releaseCheckError(resp *http.Response) error {
	limited := (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) &&
		resp.Header.Get("X-RateLimit-Remaining") == "0"
	if !limited {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	msg := "GitHub's API rate limit for this address is used up"
	if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		msg += fmt.Sprintf(" until %s", time.Unix(reset, 0).Format("15:04"))
	}
	// githubGet retries without the token when GitHub refuses it, so the
	// answer that got here was anonymous whenever a token was set.
	switch {
	case os.Getenv("GITHUB_TOKEN") == "":
		msg += ". Set GITHUB_TOKEN for a higher limit"
	case resp.Request != nil && resp.Request.Header.Get("Authorization") == "":
		msg += ". GitHub refused GITHUB_TOKEN, so nav-pilot retried without it, and that hit the limit"
	}
	return fmt.Errorf("%s. Try again later, or download nav-pilot from %s", msg, releasesPage)
}

// githubGet issues a GET against the GitHub API, authenticated with
// GITHUB_TOKEN when it is set. accept overrides the Accept header when
// non-empty (a release asset needs application/octet-stream).
//
// A GITHUB_TOKEN scoped for packages (but not for api.github.com) answers
// 401/403 and would kill release checks for good. The endpoints read here are
// public, so it retries once anonymously. The authenticated attempt stays first
// for its higher rate limit.
func githubGet(ctx context.Context, url, accept string) (*http.Response, error) {
	token := os.Getenv("GITHUB_TOKEN")
	resp, err := githubRequest(ctx, url, accept, token)
	if err != nil {
		return nil, err
	}
	if token != "" && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		resp.Body.Close()
		return githubRequest(ctx, url, accept, "")
	}
	return resp, nil
}

// githubRequest issues one GET, authenticated when token is non-empty.
func githubRequest(ctx context.Context, url, accept, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
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
	return runBoundedTimeout(cpltCommandTimeout, false, name, args...)
}

// runBoundedCombined is runBounded, but keeps stderr — for commands whose
// interesting output is not reliably on stdout.
func runBoundedCombined(name string, args ...string) ([]byte, error) {
	return runBoundedTimeout(cpltCommandTimeout, true, name, args...)
}

// runBoundedTimeout is the spawn every bounded check goes through, for callers
// whose question needs a different deadline than a local version string does.
//
// The WaitDelay is what makes the deadline real. Cancelling the context kills
// the process nav-pilot started, but Output waits for the read of its stdout to
// finish, and any grandchild still holding that pipe keeps the read open: a
// wedged agent behind a wrapper answered a full 30 seconds late under a 3
// second context. WaitDelay closes the pipes shortly after the kill, so the
// call returns on time whatever the process tree does. Every doctor spawn
// inherits the fix, since every one of them comes through here.
func runBoundedTimeout(timeout time.Duration, combined bool, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	if combined {
		return cmd.CombinedOutput()
	}
	return cmd.Output()
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

// verifyChecksum downloads SHA256SUMS and verifies the binary checksum.
// Fails hard if checksums cannot be fetched or the asset entry is missing.
func verifyChecksum(w io.Writer, data []byte, asset, checksumURL string) (err error) {
	fmt.Fprint(w, "→ Verifying checksum...")
	// End the progress line either way, so what follows starts on its own.
	defer func() {
		if err != nil {
			fmt.Fprintln(w, " ✗")
		}
	}()
	sums, err := httpGet(checksumURL)
	if err != nil {
		return fmt.Errorf("could not download SHA256SUMS to verify the download: %w\nnav-pilot is unchanged", err)
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
		return fmt.Errorf("SHA256SUMS has no checksum for %s, so the download can't be verified.\nnav-pilot is unchanged", asset)
	}

	actual := sha256sum(data)
	if actual != expected {
		return fmt.Errorf("checksum mismatch: the download does not match SHA256SUMS\n  Expected: %s\n  Got:      %s\nnav-pilot is unchanged. Try again later; if it keeps failing, report it with nav-pilot feedback", expected, actual)
	}

	fmt.Fprintln(w, " ✓")
	return nil
}

func sha256sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// autoUpdateBackoff is how long a failed auto-update waits before it tries
// again. Every command runs the update first, so without it one bad release
// (a checksum that does not match, a read-only install dir) failed every
// command until someone found the config key.
const autoUpdateBackoff = 24 * time.Hour

// stateMarker is the path of a marker file in nav-pilot's state directory,
// beside the staleness cache. A marker's mtime is when it was left.
func stateMarker(name string) string {
	p := artifacts.CacheFilePath()
	if p == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(p), name)
}

// markedWithin reports whether the marker was left less than d before now.
func markedWithin(path string, d time.Duration, now time.Time) bool {
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	// A marker from the future (the clock stepped back) holds nothing off.
	age := now.Sub(fi.ModTime())
	return age >= 0 && age < d
}

// autoUpdateFailedPath is the marker a failed auto-update leaves.
func autoUpdateFailedPath() string { return stateMarker("auto-update-failed") }

// autoUpdateBackingOff reports whether an auto-update failed within the backoff.
func autoUpdateBackingOff(now time.Time) bool {
	return markedWithin(autoUpdateFailedPath(), autoUpdateBackoff, now)
}

// updateDeclinedPath is the marker a No at the startup "Upgrade now?" leaves.
// For a day after it, the prompt is a one-line nudge: asking again on every
// command made No mean "not this command" rather than "not now".
func updateDeclinedPath() string { return stateMarker("update-declined") }

// autoUpdateFailed tells the user the update did not happen and the command
// runs on the version they have, and remembers the failure for the backoff.
// Every failure arms it, a network blip included: one lookup a day is the
// cost, and nagging on every command about a GitHub that is down is worse.
func autoUpdateFailed(latest string, err error) {
	if p := autoUpdateFailedPath(); p != "" {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(latest+"\n"), 0o644)
	}
	fmt.Fprintf(os.Stderr, "%s Auto-update to nav-pilot %s failed: %v\n", yellow("⚠"), latest, err)
	fmt.Fprintf(os.Stderr, "  Running %s instead. nav-pilot tries again in 24 hours; %s updates now, %s turns it off.\n\n",
		Version, bold("nav-pilot upgrade"), bold("nav-pilot config set auto_update false"))
}

// e2eSeams is set to "1" by the e2e suite's build (-ldflags -X), and by
// nothing else. Only then does nav-pilot read NAV_PILOT_E2E_GITHUB (a fake
// GitHub serving the releases API and downloads) and NAV_PILOT_E2E_VERSION
// (the version it claims to be), so a journey can drive upgrade without the
// network. A release build has no such variables.
var e2eSeams string

func applyE2ESeams(info *BuildInfo) {
	if e2eSeams != "1" {
		return
	}
	if gh := os.Getenv("NAV_PILOT_E2E_GITHUB"); gh != "" {
		releasesAPI = gh + "/repos/navikt/copilot/releases"
		downloadURL = gh + "/download"
		cpltReleasesAPI = gh + "/repos/navikt/cplt/releases"
	}
	if v := os.Getenv("NAV_PILOT_E2E_VERSION"); v != "" {
		info.Version = v
	}
}
