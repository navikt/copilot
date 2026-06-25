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
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	releasesAPI = "https://api.github.com/repos/navikt/copilot/releases"
	downloadURL = "https://github.com/navikt/copilot/releases/download"
)

// httpClient is the client used for all HTTP requests. Overridable in tests.
var httpClient = &http.Client{Timeout: 30 * time.Second}

type ghRelease struct {
	TagName string `json:"tag_name"`
}

// cmdUpdate checks for a newer version and updates the binary in-place.
// If installed via Homebrew, it tells the user to use brew upgrade instead.
func cmdUpdate() error {
	if isBrewManaged() {
		fmt.Println("nav-pilot is managed by Homebrew.")
		fmt.Println()
		fmt.Println("  brew upgrade navikt/tap/nav-pilot")
		return nil
	}

	current := Version
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	latest, tag, err := fetchLatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("could not check for updates: %w", err)
	}

	if !versionNewer(latest, current) {
		fmt.Printf("✓ nav-pilot is up to date (%s)\n", current)
		return nil
	}

	fmt.Printf("Update available: %s → %s\n", current, latest)

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("cannot resolve binary path: %w", err)
	}

	asset := fmt.Sprintf("nav-pilot-%s-%s", runtime.GOOS, runtime.GOARCH)
	assetURL := fmt.Sprintf("%s/%s/%s", downloadURL, tag, asset)
	checksumURL := fmt.Sprintf("%s/%s/SHA256SUMS", downloadURL, tag)

	fmt.Printf("→ Downloading %s...\n", asset)
	bin, err := httpGet(assetURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if err := verifyChecksum(bin, asset, checksumURL); err != nil {
		return err
	}

	// Atomic replace: write temp file next to binary, then rename
	dir := filepath.Dir(self)
	tmp, err := os.CreateTemp(dir, ".nav-pilot-update-*")
	if err != nil {
		return fmt.Errorf("cannot create temp file (is %s writable?): %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return fmt.Errorf("write failed: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	if err := os.Rename(tmpPath, self); err != nil {
		return fmt.Errorf("replace failed: %w", err)
	}

	fmt.Printf("✓ Updated to nav-pilot %s\n", latest)
	return nil
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
	req, err := http.NewRequestWithContext(ctx, "GET", releasesAPI+"?per_page=20", nil)
	if err != nil {
		return "", "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
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
		if strings.HasPrefix(rel.TagName, "nav-pilot/") {
			tag = rel.TagName
			ver = strings.TrimPrefix(tag, "nav-pilot/")
			return ver, tag, nil
		}
	}

	return "", "", fmt.Errorf("no nav-pilot release found")
}

func httpGet(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
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
