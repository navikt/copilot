package telemetry

import (
	"os"
	"strings"
	"testing"
)

// TestDeviceIDDeterminism verifies that device ID generation is deterministic.
// Same inputs should always produce the same UUID.
func TestDeviceIDDeterminism(t *testing.T) {
	// Generate ID twice
	id1, err := generateDeterministicDeviceID()
	if err != nil {
		t.Fatalf("first generation failed: %v", err)
	}

	id2, err := generateDeterministicDeviceID()
	if err != nil {
		t.Fatalf("second generation failed: %v", err)
	}

	// They should match
	if id1 != id2 {
		t.Errorf("device IDs not deterministic: %q != %q", id1, id2)
	}

	// They should be in the expected format
	if !strings.HasPrefix(id1, "nav-pilot-") {
		t.Errorf("device ID missing prefix: %q", id1)
	}

	if len(id1) != len("nav-pilot-")+12 {
		t.Errorf("device ID wrong length: %q (expected %d chars)", id1, len("nav-pilot-")+12)
	}
}

// TestDeviceIDFormatValid verifies device ID format.
func TestDeviceIDFormatValid(t *testing.T) {
	id, err := generateDeterministicDeviceID()
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Check prefix
	if !strings.HasPrefix(id, "nav-pilot-") {
		t.Errorf("missing 'nav-pilot-' prefix: %q", id)
	}

	// Check length
	expectedLen := len("nav-pilot-") + 12 // prefix + 12 hex chars
	if len(id) != expectedLen {
		t.Errorf("wrong length: got %d, want %d", len(id), expectedLen)
	}

	// Check that hash part is hex only
	hashPart := strings.TrimPrefix(id, "nav-pilot-")
	for _, ch := range hashPart {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			t.Errorf("non-hex character in hash: %c in %q", ch, id)
		}
	}
}

func TestDeviceIDPersistence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Get or create
	id1, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Get again (should read from disk)
	id2, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Should be identical
	if id1 != id2 {
		t.Errorf("device ID not persistent: %q != %q", id1, id2)
	}
}

// TestDeviceIDNoUserInfo verifies that device ID contains no identifiable info.
func TestDeviceIDNoUserInfo(t *testing.T) {
	id, err := generateDeterministicDeviceID()
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	// Should not contain username
	username := os.Getenv("USER")
	if username != "" && strings.Contains(id, username) {
		t.Errorf("device ID contains username: %q in %q", username, id)
	}

	// Should not contain email markers
	if strings.Contains(id, "@") {
		t.Errorf("device ID contains @ (email marker): %q", id)
	}

	// Should not contain common PII patterns
	forbiddenPatterns := []string{"admin", "root", "user", "test", "local"}
	for _, pattern := range forbiddenPatterns {
		if strings.Contains(strings.ToLower(id), pattern) {
			t.Logf("warning: device ID might contain PII pattern %q: %q", pattern, id)
		}
	}
}

// ─── device-id format validation ────────────────────────────────────────────

func TestGetOrCreateDeviceID_ValidStoredIDReturned(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	idFile := dir + "/.nav-pilot/device-id"
	if err := os.MkdirAll(dir+"/.nav-pilot", 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := "nav-pilot-aabbccddeeff"
	if err := os.WriteFile(idFile, []byte(want+"\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetOrCreateDeviceID_InvalidStoredIDRegenerates(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"garbage", "garbage-value"},
		{"uppercase-hex", "nav-pilot-AABBCCDDEEFF"},
		{"too-short", "nav-pilot-aabbcc"},
		{"empty", ""},
		{"spaces-only", "   \n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)

			idFile := dir + "/.nav-pilot/device-id"
			if err := os.MkdirAll(dir+"/.nav-pilot", 0o700); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(idFile, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}

			got, err := GetOrCreateDeviceID()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !deviceIDPattern.MatchString(got) {
				t.Errorf("regenerated id %q does not match expected pattern", got)
			}
			// Must NOT be the invalid stored value.
			if got == strings.TrimSpace(tt.content) {
				t.Errorf("returned the invalid stored value %q unchanged", got)
			}
		})
	}
}
