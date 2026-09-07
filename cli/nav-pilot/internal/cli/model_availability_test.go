package cli

import "testing"

func TestUnavailableSuffix(t *testing.T) {
	reset := func(ids map[string]bool) {
		availabilityOnce.Do(func() {}) // burn the once so the probe never runs
		availableIDs = ids             // and inject the answer
	}

	t.Run("unknown availability annotates nothing", func(t *testing.T) {
		reset(nil)
		// The important case. An offline developer, or one on a client that
		// stops writing the catalogue, must get the plain list they had before,
		// never a list where every entry is marked unavailable.
		if got := unavailableSuffix("claude-sonnet-4.6"); got != "" {
			t.Errorf("unavailableSuffix with no catalogue = %q, want empty", got)
		}
	})

	t.Run("marks what the account cannot launch", func(t *testing.T) {
		reset(map[string]bool{"claude-sonnet-5": true})
		if got := unavailableSuffix("claude-sonnet-4.6"); got == "" {
			t.Error("a model outside the catalogue was not marked")
		}
		if got := unavailableSuffix("claude-sonnet-5"); got != "" {
			t.Errorf("an available model was marked: %q", got)
		}
	})

	t.Run("opencode ids carry a provider prefix", func(t *testing.T) {
		reset(map[string]bool{"claude-sonnet-5": true})
		if got := unavailableSuffix("github-copilot/claude-sonnet-5"); got != "" {
			t.Errorf("prefixed id was marked unavailable: %q; the prefix must be stripped before comparing", got)
		}
	})

	t.Run("auto is never marked", func(t *testing.T) {
		reset(map[string]bool{"claude-sonnet-5": true})
		// "auto" is a pseudo-model the client resolves itself; it is not in the
		// catalogue and never will be.
		if got := unavailableSuffix("auto"); got != "" {
			t.Errorf("auto was marked unavailable: %q", got)
		}
	})
}
