package provider

import (
	"os"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
)

// TestMain isolates the staleness cache for the whole package. Without it every
// model-default assertion here would read the developer's real
// ~/.nav-pilot/cache.json and start failing the day someone edits the published
// profile.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "provider-cache-")
	if err != nil {
		panic(err)
	}
	artifacts.CacheHome = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// cacheProfileModel writes a profile straight into the staleness cache, which
// is what a successful release check leaves behind.
func cacheProfileModel(t *testing.T, model string) {
	t.Helper()
	dir := t.TempDir()
	orig := artifacts.CacheHome
	artifacts.CacheHome = dir
	t.Cleanup(func() { artifacts.CacheHome = orig })
	if model != "" {
		artifacts.WriteCache(&artifacts.StalenessCache{
			DefaultModels: map[string]string{"opencode": model},
		})
	}
}

func TestOpenCodeDefaultModelPrecedence(t *testing.T) {
	const profileModel = "github-copilot/claude-sonnet-4.6"
	const pakkeModel = "github-copilot/claude-opus-5"

	tests := []struct {
		name    string
		profile string
		pakke   string // the active agentpakke's opencode defaultModel, "" for the built-in default
		user    string // the user's --model or config model
		want    string
	}{
		{
			name: "no profile, no pakke, no pin: the compiled-in default",
			want: OpenCodeDefaultModel,
		},
		{
			name:    "profile applied",
			profile: profileModel,
			want:    profileModel,
		},
		{
			name:    "an agentpakke that pins its own model outranks the profile",
			profile: profileModel,
			pakke:   pakkeModel,
			want:    pakkeModel,
		},
		{
			name:    "an inherit pakke does not block the profile",
			profile: profileModel,
			pakke:   agentpakke.InheritModel,
			want:    profileModel,
		},
		{
			name:    "an explicit user model outranks the profile",
			profile: profileModel,
			user:    "github-copilot/gpt-5.3",
			want:    "github-copilot/gpt-5.3",
		},
		{
			name:    "a bare user model is still the user's, profile or not",
			profile: profileModel,
			user:    "claude-haiku-4.5",
			want:    "github-copilot/claude-haiku-4.5",
		},
		{
			name:    `"auto" means "follow the Nav default", so the profile applies`,
			profile: profileModel,
			user:    "auto",
			want:    profileModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheProfileModel(t, tt.profile)
			if tt.pakke != "" {
				SetActivePakke(&agentpakke.Manifest{
					Name: "grillmester",
					Clients: map[string]agentpakke.ClientEntry{
						"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: tt.pakke},
					},
				})
				t.Cleanup(func() { SetActivePakke(nil) })
			}

			if got := ToOpenCodeModel(tt.user); got != tt.want {
				t.Errorf("ToOpenCodeModel(%q) = %q, want %q", tt.user, got, tt.want)
			}
		})
	}
}

// TestOpenCodeDefaultModelUnreachableProfile is the no-regression case: a user
// on today's setup, with nothing cached and no network ever reached, sees
// exactly today's behaviour.
func TestOpenCodeDefaultModelUnreachableProfile(t *testing.T) {
	cacheProfileModel(t, "")
	if got := (openCodeProvider{}).DefaultModel(); got != OpenCodeDefaultModel {
		t.Errorf("DefaultModel() with no cached profile = %q, want %q", got, OpenCodeDefaultModel)
	}
}
