package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
)

// TestMain isolates the staleness cache for the whole package: tests here must
// neither read the developer's published profile nor write to their real
// ~/.nav-pilot/cache.json.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cli-cache-")
	if err != nil {
		panic(err)
	}
	artifacts.CacheHome = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// captureNotice runs noticeModelDefaultChange against a throwaway config file
// and returns what it printed plus what it recorded.
func captureNotice(t *testing.T, resolved ResolvedConfig, profileModel string) (string, string) {
	t.Helper()

	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(cfgPath, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatalf("seeding config: %v", err)
	}
	t.Setenv("NAV_PILOT_CONFIG", cfgPath)

	origHome := artifacts.CacheHome
	artifacts.CacheHome = t.TempDir()
	t.Cleanup(func() { artifacts.CacheHome = origHome })
	if profileModel != "" {
		artifacts.WriteCache(&artifacts.StalenessCache{
			DefaultModels: map[string]string{"opencode": profileModel},
		})
	}

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	noticeModelDefaultChange(resolved)
	w.Close()
	os.Stderr = origStderr

	var buf strings.Builder
	io.Copy(&buf, r)

	written, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config back: %v", err)
	}
	return buf.String(), string(written)
}

func TestNoticeModelDefaultChange(t *testing.T) {
	const profileModel = "github-copilot/claude-sonnet-4.6"

	tests := []struct {
		name       string
		resolved   ResolvedConfig
		profile    string
		wantNotice string // substring the notice must contain, "" for silence
		wantRecord string // value model_default_seen must end up holding
	}{
		{
			name:       "first launch after upgrading records the default and says nothing",
			resolved:   ResolvedConfig{Client: "opencode"},
			profile:    profileModel,
			wantRecord: profileModel,
		},
		{
			name:       "a moved default is mentioned once",
			resolved:   ResolvedConfig{Client: "opencode", ModelDefaultSeen: "github-copilot/auto"},
			profile:    profileModel,
			wantNotice: profileModel,
			wantRecord: profileModel,
		},
		{
			name:     "the same default is not mentioned again",
			resolved: ResolvedConfig{Client: "opencode", ModelDefaultSeen: profileModel},
			profile:  profileModel,
		},
		{
			name:       "a user with their own model is told their launches are unchanged",
			resolved:   ResolvedConfig{Client: "opencode", Model: "github-copilot/gpt-5.3", ModelDefaultSeen: "github-copilot/auto"},
			profile:    profileModel,
			wantNotice: "unchanged",
			wantRecord: profileModel,
		},
		{
			name:     "copilot pins no default of its own, so there is nothing to notice",
			resolved: ResolvedConfig{Client: "copilot"},
			profile:  profileModel,
		},
		{
			name:     "an unknown client is not a reason to fail a launch",
			resolved: ResolvedConfig{Client: "nonesuch", ModelDefaultSeen: "github-copilot/auto"},
			profile:  profileModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, cfg := captureNotice(t, tt.resolved, tt.profile)

			if tt.wantNotice == "" {
				if strings.Contains(out, "default model") {
					t.Errorf("expected silence, got %q", out)
				}
			} else if !strings.Contains(out, tt.wantNotice) {
				t.Errorf("notice = %q, want it to mention %q", out, tt.wantNotice)
			}

			if tt.wantRecord == "" {
				if strings.Contains(cfg, "model_default_seen") {
					t.Errorf("config gained model_default_seen when nothing should have been recorded:\n%s", cfg)
				}
				return
			}
			want := `model_default_seen = "` + tt.wantRecord + `"`
			if !strings.Contains(cfg, want) {
				t.Errorf("config = %q, want it to contain %q", cfg, want)
			}
		})
	}
}

// TestNoticeModelDefaultChangeUnwritableConfig covers the fail-soft rule: a
// config that cannot be written warns, it does not stop the launch.
func TestNoticeModelDefaultChangeUnwritableConfig(t *testing.T) {
	// A directory where a file belongs: unreadable and unwritable as config.
	t.Setenv("NAV_PILOT_CONFIG", t.TempDir())

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	noticeModelDefaultChange(ResolvedConfig{Client: "opencode", ModelDefaultSeen: "github-copilot/old"})
	w.Close()
	os.Stderr = origStderr

	var buf strings.Builder
	io.Copy(&buf, r)
	if !strings.Contains(buf.String(), "Warning") {
		t.Errorf("expected a warning about the failed write, got %q", buf.String())
	}
}
