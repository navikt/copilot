package main

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

// TestDeviceIDPersistence verifies that device ID is saved to disk.
func TestDeviceIDPersistence(t *testing.T) {
	// Get or create
	id1, err := getOrCreateDeviceID()
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Get again (should read from disk)
	id2, err := getOrCreateDeviceID()
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
