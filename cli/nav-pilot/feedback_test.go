package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCollectDiagnostics_NoState(t *testing.T) {
	tmp := t.TempDir()
	diag := collectDiagnostics(tmp)

	if !strings.Contains(diag, "nav-pilot") {
		t.Error("diagnostics should contain 'nav-pilot'")
	}
	if !strings.Contains(diag, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Errorf("diagnostics should contain OS/arch, got:\n%s", diag)
	}
	if !strings.Contains(diag, "(none installed)") {
		t.Error("diagnostics should say '(none installed)' when no collection")
	}
}

func TestCollectDiagnostics_WithState(t *testing.T) {
	tmp := t.TempDir()
	ghDir := filepath.Join(tmp, ".github")
	os.MkdirAll(ghDir, 0o755)

	// Create a test file
	testFile := filepath.Join(tmp, ".github", "test.md")
	os.WriteFile(testFile, []byte("hello"), 0o644)
	hash, _ := fileHash(testFile)

	state := &StateFile{
		Collection: "kotlin-backend",
		Version:    "v2025.07",
		SourceSHA:  "abc1234567890",
		Files:      []InstalledFile{{Path: ".github/test.md", Hash: hash}},
	}
	writeState(tmp, state)

	diag := collectDiagnostics(tmp)

	if !strings.Contains(diag, "kotlin-backend") {
		t.Errorf("diagnostics should contain collection name, got:\n%s", diag)
	}
	if !strings.Contains(diag, "abc1234") {
		t.Errorf("diagnostics should contain short SHA, got:\n%s", diag)
	}
	if !strings.Contains(diag, "1 ok, 0 modified, 0 missing") {
		t.Errorf("diagnostics should contain file integrity, got:\n%s", diag)
	}
}

func TestCollectDiagnostics_WithModifiedFile(t *testing.T) {
	tmp := t.TempDir()
	ghDir := filepath.Join(tmp, ".github")
	os.MkdirAll(ghDir, 0o755)

	testFile := filepath.Join(tmp, ".github", "test.md")
	os.WriteFile(testFile, []byte("hello"), 0o644)

	state := &StateFile{
		Collection: "test",
		Version:    "v1",
		SourceSHA:  "abc1234567890",
		Files:      []InstalledFile{{Path: ".github/test.md", Hash: "wrong-hash"}},
	}
	writeState(tmp, state)

	diag := collectDiagnostics(tmp)

	if !strings.Contains(diag, "0 ok, 1 modified, 0 missing") {
		t.Errorf("expected modified file detected, got:\n%s", diag)
	}
}

func TestBuildFeedbackURL_Bug(t *testing.T) {
	u := buildFeedbackURL(false, "nav-pilot dev\nOS darwin/arm64")

	if !strings.Contains(u, "template=nav-pilot-bug.yml") {
		t.Errorf("bug URL should use bug template, got: %s", u)
	}
	if !strings.Contains(u, "labels=nav-pilot") {
		t.Errorf("bug URL should have nav-pilot label, got: %s", u)
	}
	if strings.Contains(u, "enhancement") {
		t.Error("bug URL should not have enhancement label")
	}
	if !strings.Contains(u, "diagnostics=") {
		t.Error("URL should contain diagnostics param")
	}
}

func TestBuildFeedbackURL_Feature(t *testing.T) {
	u := buildFeedbackURL(true, "nav-pilot dev")

	if !strings.Contains(u, "template=nav-pilot-feature.yml") {
		t.Errorf("feature URL should use feature template, got: %s", u)
	}
	if !strings.Contains(u, "enhancement") {
		t.Errorf("feature URL should have enhancement label, got: %s", u)
	}
}

func TestBuildFeedbackURL_EncodesSpecialChars(t *testing.T) {
	u := buildFeedbackURL(false, "version 1.0 (test)\nOS darwin/arm64")

	// Should not contain raw spaces or parens
	if strings.Contains(u, " ") {
		t.Error("URL should encode spaces")
	}
	// The diagnostics value should be present and encoded
	if !strings.Contains(u, "diagnostics=") {
		t.Error("URL should contain diagnostics")
	}
}

func TestBuildFeedbackURL_EmptyDiagnostics(t *testing.T) {
	u := buildFeedbackURL(false, "")

	// Should still have template and labels but no diagnostics param
	if !strings.Contains(u, "template=nav-pilot-bug.yml") {
		t.Error("URL should have template even with empty diagnostics")
	}
}

func TestShortSHA(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc1234567890", "abc1234"},
		{"short", "short"},
		{"exactly", "exactly"},
		{"", ""},
	}
	for _, tt := range tests {
		got := shortSHA(tt.input)
		if got != tt.want {
			t.Errorf("shortSHA(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCountFileIntegrity(t *testing.T) {
	tmp := t.TempDir()
	ghDir := filepath.Join(tmp, ".github")
	os.MkdirAll(ghDir, 0o755)

	// Create one file
	f1 := filepath.Join(tmp, ".github", "a.md")
	os.WriteFile(f1, []byte("hello"), 0o644)
	hash1, _ := fileHash(f1)

	state := &StateFile{
		Files: []InstalledFile{
			{Path: ".github/a.md", Hash: hash1},       // ok
			{Path: ".github/a.md", Hash: "wrong"},      // modified (same file, wrong hash)
			{Path: ".github/missing.md", Hash: "x"},     // missing
		},
	}

	ok, modified, missing, _ := countFileIntegrity(tmp, state)
	if ok != 1 {
		t.Errorf("ok = %d, want 1", ok)
	}
	if modified != 1 {
		t.Errorf("modified = %d, want 1", modified)
	}
	if missing != 1 {
		t.Errorf("missing = %d, want 1", missing)
	}
}

func TestCmdFeedback_DoesNotError(t *testing.T) {
	// Override browser open to be a no-op
	orig := openBrowserFn
	openBrowserFn = func(url string) error { return nil }
	defer func() { openBrowserFn = orig }()

	tmp := t.TempDir()
	err := cmdFeedback(tmp, false)
	if err != nil {
		t.Errorf("cmdFeedback returned error: %v", err)
	}
}

func TestCmdFeedback_FeatureRequest(t *testing.T) {
	orig := openBrowserFn
	var capturedURL string
	openBrowserFn = func(url string) error {
		capturedURL = url
		return nil
	}
	defer func() { openBrowserFn = orig }()

	tmp := t.TempDir()
	err := cmdFeedback(tmp, true)
	if err != nil {
		t.Errorf("cmdFeedback returned error: %v", err)
	}
	if !strings.Contains(capturedURL, "nav-pilot-feature.yml") {
		t.Errorf("expected feature template in URL, got: %s", capturedURL)
	}
}
