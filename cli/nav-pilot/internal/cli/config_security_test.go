package cli

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// ─── securitySettings ────────────────────────────────────────────────────────

func TestSecuritySettings_GuardsOn_ProxyOff(t *testing.T) {
	got := securitySettings(true, false, false, "")
	want := [][2]string{
		{"gh_guard.enabled", "true"},
		{"gh_guard.mode", "block"},
		{"git_guard.enabled", "true"},
		{"git_guard.mode", "block"},
		{"proxy.forced", "false"},
		{"proxy.default_allowlist", "false"},
	}
	assertSettings(t, got, want)
}

func TestSecuritySettings_GuardsOn_ProxyOn(t *testing.T) {
	got := securitySettings(true, true, false, "")
	want := [][2]string{
		{"gh_guard.enabled", "true"},
		{"gh_guard.mode", "block"},
		{"git_guard.enabled", "true"},
		{"git_guard.mode", "block"},
		{"proxy.forced", "true"},
		{"proxy.default_allowlist", "false"},
	}
	assertSettings(t, got, want)
}

func TestSecuritySettings_GuardsOff_ProxyOff(t *testing.T) {
	got := securitySettings(false, false, false, "")
	want := [][2]string{
		{"gh_guard.enabled", "false"},
		{"git_guard.enabled", "false"},
		{"proxy.forced", "false"},
		{"proxy.default_allowlist", "false"},
	}
	assertSettings(t, got, want)
}

func TestSecuritySettings_GuardsOff_ProxyOn(t *testing.T) {
	got := securitySettings(false, true, false, "")
	want := [][2]string{
		{"gh_guard.enabled", "false"},
		{"git_guard.enabled", "false"},
		{"proxy.forced", "true"},
		{"proxy.default_allowlist", "false"},
	}
	assertSettings(t, got, want)
}

func TestSecuritySettings_DefaultAllowlistOnWithPath(t *testing.T) {
	got := securitySettings(true, true, true, "/tmp/allowed-domains.txt")
	want := [][2]string{
		{"gh_guard.enabled", "true"},
		{"gh_guard.mode", "block"},
		{"git_guard.enabled", "true"},
		{"git_guard.mode", "block"},
		{"proxy.forced", "true"},
		{"proxy.default_allowlist", "true"},
		{"proxy.allowed_domains", "/tmp/allowed-domains.txt"},
	}
	assertSettings(t, got, want)
}

func TestNormalizeAllowedDomains(t *testing.T) {
	got, err := normalizeAllowedDomains("https://api.github.com/repos/navikt/copilot?x=1, openai.com/path localhost:3000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"api.github.com", "localhost", "openai.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("domains mismatch: got %v, want %v", got, want)
	}
}

// ─── applySecuritySettings ───────────────────────────────────────────────────

func TestApplySecuritySettings_CallsRunnerForEachSetting(t *testing.T) {
	var called [][2]string
	runner := func(_ context.Context, _ string, key, val string) ([]byte, error) {
		called = append(called, [2]string{key, val})
		return nil, nil
	}

	if err := applySecuritySettings("cplt", true, true, false, "", runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := securitySettings(true, true, false, "")
	assertSettings(t, called, want)
}

func TestApplySecuritySettings_GuardsOff_DoesNotSetMode(t *testing.T) {
	var calledKeys []string
	runner := func(_ context.Context, _ string, key, _ string) ([]byte, error) {
		calledKeys = append(calledKeys, key)
		return nil, nil
	}

	if err := applySecuritySettings("cplt", false, false, false, "", runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, k := range calledKeys {
		if strings.Contains(k, "mode") {
			t.Errorf("disable path should not set mode keys, but got %q", k)
		}
	}
}

func TestApplySecuritySettings_ErrorSurfacesKeyAndOutput(t *testing.T) {
	runner := func(_ context.Context, _ string, key, _ string) ([]byte, error) {
		if key == "gh_guard.mode" {
			return []byte("permission denied"), errors.New("exit status 1")
		}
		return nil, nil
	}

	err := applySecuritySettings("cplt", true, false, false, "", runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "gh_guard.mode") {
		t.Errorf("error should contain failing key, got: %v", err)
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error should contain captured output, got: %v", err)
	}
}

func TestApplySecuritySettings_FirstErrorStopsExecution(t *testing.T) {
	var calledKeys []string
	runner := func(_ context.Context, _ string, key, _ string) ([]byte, error) {
		calledKeys = append(calledKeys, key)
		if key == "gh_guard.enabled" {
			return []byte("fail"), errors.New("exit status 1")
		}
		return nil, nil
	}

	err := applySecuritySettings("cplt", true, true, false, "", runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(calledKeys) != 1 {
		t.Errorf("expected execution to stop after first error, but runner was called %d times", len(calledKeys))
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func assertSettings(t *testing.T, got, want [][2]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("settings length: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("settings[%d]: got %v, want %v", i, got[i], w)
		}
	}
}
