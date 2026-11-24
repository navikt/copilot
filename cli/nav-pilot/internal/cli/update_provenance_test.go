package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const provenanceTag = "nav-pilot/2026.11.24-084539-0dda7bc"

func attestationJSON(repoURI, signer, ref, commit string) string {
	return fmt.Sprintf(`[{"verificationResult":{"signature":{"certificate":{"sourceRepositoryURI":%q,"buildSignerURI":%q,"sourceRepositoryRef":%q,"sourceRepositoryDigest":%q}}}}]`, repoURI, signer, ref, commit)
}

// stubAttestations stands in for gh: like gh, it answers only for a file whose
// digest has an attestation, so a changed binary finds none.
func stubAttestations(t *testing.T, byDigest map[string]string) {
	t.Helper()
	orig := runAttestationVerify
	t.Cleanup(func() { runAttestationVerify = orig })
	runAttestationVerify = func(path string) ([]byte, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		if out, ok := byDigest[hex.EncodeToString(sum[:])]; ok {
			return []byte(out), nil
		}
		return nil, errors.New("gh attestation verify: no attestations found")
	}
}

func writeBinary(t *testing.T, content string) (path, digest string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "nav-pilot")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return path, hex.EncodeToString(sum[:])
}

func TestVerifyProvenance(t *testing.T) {
	const commit = "0dda7bc9fb5abfbd1f1ae515e1800c0095b68622"
	valid := attestationJSON(attestRepoURI, attestSigner, attestSourceRef, commit)
	tests := []struct {
		name    string
		attest  string // "" means the binary has no attestation
		tamper  bool
		tag     string
		wantErr string
	}{
		{name: "valid", attest: valid},
		{name: "missing attestation (older release)", wantErr: "no attestations found"},
		{name: "empty answer", attest: "[]", wantErr: "no attestation found"},
		{name: "wrong repo", attest: attestationJSON("https://github.com/evil/copilot", attestSigner, attestSourceRef, commit), wantErr: "not https://github.com/navikt/copilot"},
		{name: "wrong workflow", attest: attestationJSON(attestRepoURI, "https://github.com/navikt/copilot/.github/workflows/other.yaml@refs/heads/main", attestSourceRef, commit), wantErr: "not the release workflow"},
		{name: "wrong branch", attest: attestationJSON(attestRepoURI, attestSigner, "refs/heads/evil", commit), wantErr: "not refs/heads/main"},
		{name: "tampered binary", attest: valid, tamper: true, wantErr: "no attestations found"},
		{name: "downgrade: old build under a new tag", attest: attestationJSON(attestRepoURI, attestSigner, attestSourceRef, "14637c8aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), wantErr: "names 0dda7bc"},
		{name: "tag without commit", attest: valid, tag: "nav-pilot/latest", wantErr: "names no commit"},
		{name: "garbage answer", attest: "not json", wantErr: "unreadable answer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, digest := writeBinary(t, "release binary")
			stubs := map[string]string{}
			if tt.attest != "" {
				stubs[digest] = tt.attest
			}
			stubAttestations(t, stubs)
			if tt.tamper {
				if err := os.WriteFile(path, []byte("release binary, changed"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			tag := tt.tag
			if tag == "" {
				tag = provenanceTag
			}
			err := verifyProvenance(io.Discard, path, tag)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("verifyProvenance = %v, want nil", err)
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
		})
	}
}

// fakeGhOnPath puts a gh script first on PATH: auth says whether it is logged
// in, and attestation verify always rejects, as for a tampered binary.
func fakeGhOnPath(t *testing.T, loggedIn bool) {
	t.Helper()
	dir := t.TempDir()
	auth := "1"
	if loggedIn {
		auth = "0"
	}
	script := "#!/bin/sh\ncase \"$1\" in\nauth) exit " + auth + ";;\nattestation) echo 'no attestations found' >&2; exit 1;;\nesac\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestVerifyProvenanceGhAvailability(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		wantErr string // "" means the SHA256-only fallback
	}{
		{name: "no gh falls back", setup: func(t *testing.T) { t.Setenv("PATH", t.TempDir()) }},
		{name: "gh not logged in falls back", setup: func(t *testing.T) { fakeGhOnPath(t, false) }},
		{name: "gh logged in but rejecting refuses", setup: func(t *testing.T) { fakeGhOnPath(t, true) }, wantErr: "no attestations found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolatedConfig(t)
			orig := isInteractive
			t.Cleanup(func() { isInteractive = orig })
			isInteractive = func() bool { return true }
			tt.setup(t)
			path, _ := writeBinary(t, "release binary")
			var out strings.Builder
			err := verifyProvenance(&out, path, provenanceTag)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("verifyProvenance = %v, want a refusal containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("verifyProvenance = %v, want the SHA256-only fallback", err)
			}
			if !strings.Contains(out.String(), "gh auth login") {
				t.Errorf("no hint about gh. Output:\n%s", out.String())
			}
			out.Reset()
			if err := verifyProvenance(&out, path, provenanceTag); err != nil || strings.Contains(out.String(), "gh auth login") {
				t.Errorf("second run for the same release: err %v, output %q; want no hint", err, out.String())
			}
		})
	}
}
