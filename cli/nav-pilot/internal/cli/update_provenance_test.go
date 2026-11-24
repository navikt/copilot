package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	provenanceTag  = "nav-pilot/2026.11.24-084539-0dda7bc"
	attestedCommit = "0dda7bc9fb5abfbd1f1ae515e1800c0095b68622"
	runningCommit  = "14637c8aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func attestationJSON(repoURI, signer, ref, runner, commit string) string {
	return fmt.Sprintf(`[{"verificationResult":{"signature":{"certificate":{"sourceRepositoryURI":%q,"buildSignerURI":%q,"sourceRepositoryRef":%q,"runnerEnvironment":%q,"sourceRepositoryDigest":%q}}}}]`, repoURI, signer, ref, runner, commit)
}

var validAttestation = attestationJSON(attestRepoURI, attestSigner, attestSourceRef, "github-hosted", attestedCommit)

// fakeGhScript stands in for gh, driven by files beside it. Like gh, it only
// vouches for a file whose digest has an attestation, so a changed binary
// finds none. It refuses any call that lacks a pinned flag, so dropping one
// from the code fails a test.
const fakeGhScript = `#!/bin/sh
d=$(dirname "$0")
has() { case " $ARGS " in *" $1 "*) return 0;; esac; echo "fake gh: missing $1" >&2; exit 3; }
ARGS="$*"
case "$1" in
auth) has "--hostname github.com"; exit $(cat "$d/auth_rc");;
attestation)
  has "--hostname github.com"; has "--repo navikt/copilot"
  has "--cert-identity https://github.com/navikt/copilot/.github/workflows/release-nav-pilot.yaml@refs/heads/main"
  has "--source-ref refs/heads/main"; has "--deny-self-hosted-runners"; has "--format json"
  [ -f "$d/attest.json" ] && [ "$(shasum -a 256 "$3" | cut -c1-64)" = "$(cat "$d/digest")" ] || { echo "no attestations found" >&2; exit 1; }
  cat "$d/attest.json";;
api)
  has "--hostname github.com"
  [ -f "$d/compare" ] || { echo "HTTP 502: Bad Gateway" >&2; exit 1; }
  case "$*" in *"repos/navikt/copilot/compare/$(cat "$d/base")...$(cat "$d/head")"*) cat "$d/compare";; *) echo "HTTP 404" >&2; exit 1;; esac;;
*) exit 2;;
esac
`

type fakeGh struct {
	dir string
}

// installFakeGh puts the fake first (and alone) on PATH, logged in.
func installFakeGh(t *testing.T) fakeGh {
	t.Helper()
	f := fakeGh{dir: t.TempDir()}
	f.write(t, "gh", fakeGhScript)
	if err := os.Chmod(filepath.Join(f.dir, "gh"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.write(t, "auth_rc", "0")
	t.Setenv("PATH", f.dir+string(os.PathListSeparator)+"/usr/bin:/bin")
	return f
}

func (f fakeGh) write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// release attests the binary at path and answers the compare call.
func (f fakeGh) release(t *testing.T, path, attest, status string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f.write(t, "digest", sha256sum(data))
	f.write(t, "attest.json", attest)
	f.write(t, "base", runningCommit)
	f.write(t, "head", attestedCommit)
	if status != "" {
		f.write(t, "compare", status+"\n")
	}
}

func setRunningCommit(t *testing.T, commit string) {
	t.Helper()
	orig := buildInfo
	t.Cleanup(func() { buildInfo = orig })
	buildInfo.Commit = commit
}

func writeBinary(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nav-pilot")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVerifyProvenance(t *testing.T) {
	tests := []struct {
		name    string
		attest  string // "" means the binary has no attestation
		status  string // the compare status; "" means the API call fails
		tamper  bool
		running string
		wantErr string
	}{
		{name: "valid and newer", attest: validAttestation, status: "ahead"},
		{name: "missing attestation (older release)", status: "ahead", wantErr: "no attestations found"},
		{name: "empty answer", attest: "[]", status: "ahead", wantErr: "no attestation found"},
		{name: "wrong repo", attest: attestationJSON("https://github.com/evil/copilot", attestSigner, attestSourceRef, "github-hosted", attestedCommit), status: "ahead", wantErr: "not https://github.com/navikt/copilot"},
		{name: "wrong workflow", attest: attestationJSON(attestRepoURI, "https://github.com/navikt/copilot/.github/workflows/other.yaml@refs/heads/main", attestSourceRef, "github-hosted", attestedCommit), status: "ahead", wantErr: "not the release workflow"},
		{name: "wrong branch", attest: attestationJSON(attestRepoURI, attestSigner, "refs/heads/evil", "github-hosted", attestedCommit), status: "ahead", wantErr: "not refs/heads/main"},
		{name: "self-hosted runner", attest: attestationJSON(attestRepoURI, attestSigner, attestSourceRef, "self-hosted", attestedCommit), status: "ahead", wantErr: "not a GitHub-hosted one"},
		{name: "tampered binary", attest: validAttestation, status: "ahead", tamper: true, wantErr: "no attestations found"},
		{name: "downgrade: older attested commit", attest: validAttestation, status: "behind", wantErr: `"behind"`},
		{name: "diverged commit", attest: validAttestation, status: "diverged", wantErr: `"diverged"`},
		{name: "same commit", attest: validAttestation, status: "identical", wantErr: `"identical"`},
		{name: "compare API error", attest: validAttestation, wantErr: "HTTP 502"},
		{name: "dev build without commit", attest: validAttestation, status: "ahead", running: "none", wantErr: "names no commit"},
		{name: "garbage answer", attest: "not json", status: "ahead", wantErr: "unreadable answer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolatedConfig(t)
			gh := installFakeGh(t)
			path := writeBinary(t, "release binary")
			running := tt.running
			if running == "" {
				running = runningCommit
			}
			setRunningCommit(t, running)
			if tt.attest != "" {
				gh.release(t, path, tt.attest, tt.status)
			} else if tt.status != "" {
				gh.write(t, "compare", tt.status)
			}
			if tt.tamper {
				if err := os.WriteFile(path, []byte("release binary, changed"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			var out strings.Builder
			err := verifyProvenance(&out, path, provenanceTag)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("verifyProvenance = %v, want nil", err)
				}
				if !strings.Contains(out.String(), "provenance... ✓") {
					t.Errorf("no ✓ for a verified release. Output:\n%s", out.String())
				}
				return
			}
			if err == nil {
				t.Fatalf("verifyProvenance accepted it, want an error containing %q", tt.wantErr)
			}
			for _, want := range []string{tt.wantErr, "nav-pilot is unchanged", "Update by hand: download nav-pilot from " + releasesPage} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
			if strings.Contains(out.String(), "SHA256 only") {
				t.Errorf("a gh that ran fell back to SHA256. Output:\n%s", out.String())
			}
		})
	}
}

// TestVerifyProvenanceFallback: only a missing or logged-out gh falls back to
// the SHA256 check. Logged out says so every time, missing once per release.
func TestVerifyProvenanceFallback(t *testing.T) {
	interactive := func(t *testing.T) {
		orig := isInteractive
		t.Cleanup(func() { isInteractive = orig })
		isInteractive = func() bool { return true }
	}
	t.Run("no gh", func(t *testing.T) {
		isolatedConfig(t)
		interactive(t)
		t.Setenv("PATH", t.TempDir())
		path := writeBinary(t, "release binary")
		var first, second strings.Builder
		if err := verifyProvenance(&first, path, provenanceTag); err != nil {
			t.Fatalf("verifyProvenance = %v, want the SHA256-only fallback", err)
		}
		if err := verifyProvenance(&second, path, provenanceTag); err != nil {
			t.Fatalf("second verifyProvenance = %v", err)
		}
		if !strings.Contains(first.String(), "gh auth login") || strings.Contains(second.String(), "gh auth login") {
			t.Errorf("want the hint once per release. First:\n%s\nSecond:\n%s", first.String(), second.String())
		}
	})
	t.Run("gh not logged in", func(t *testing.T) {
		isolatedConfig(t)
		gh := installFakeGh(t)
		gh.write(t, "auth_rc", "1")
		path := writeBinary(t, "release binary")
		for i := range 2 {
			var out strings.Builder
			if err := verifyProvenance(&out, path, provenanceTag); err != nil {
				t.Fatalf("run %d: verifyProvenance = %v, want the SHA256-only fallback", i, err)
			}
			if !strings.Contains(out.String(), "without a provenance check") || !strings.Contains(out.String(), "gh auth login") {
				t.Errorf("run %d: no warning naming gh auth login. Output:\n%s", i, out.String())
			}
		}
	})
}

// TestVerifyProvenanceNoFallbackOnReject: a logged-in gh that rejects never
// drops into the SHA256 fallback.
func TestVerifyProvenanceNoFallbackOnReject(t *testing.T) {
	isolatedConfig(t)
	installFakeGh(t) // logged in, no attestation for anything
	setRunningCommit(t, runningCommit)
	err := verifyProvenance(io.Discard, writeBinary(t, "release binary"), provenanceTag)
	if err == nil || !strings.Contains(err.Error(), "no attestations found") {
		t.Fatalf("verifyProvenance = %v, want a refusal", err)
	}
}
