package cli

import (
	"strings"
	"testing"
)

func TestUnavailableSuffix(t *testing.T) {
	t.Run("unknown availability annotates nothing", func(t *testing.T) {
		// The important case. An offline developer, or one on a client that
		// stops writing the catalogue, must get the plain list they had before,
		// never a list where every entry is marked unavailable.
		if got := unavailableSuffix(nil, "claude-sonnet-4.6"); got != "" {
			t.Errorf("unavailableSuffix with no catalogue = %q, want empty", got)
		}
	})

	t.Run("an empty catalogue is an answer, not a shrug", func(t *testing.T) {
		// Distinct from nil on purpose: a catalogue that was read and holds
		// nothing means nothing is launchable, and saying so beats pretending
		// the question was never answered.
		if got := unavailableSuffix(map[string]bool{}, "claude-sonnet-5"); got == "" {
			t.Error("an empty non-nil catalogue annotated nothing; that is the offline case, not this one")
		}
	})

	t.Run("marks what the account cannot launch", func(t *testing.T) {
		have := map[string]bool{"claude-sonnet-5": true}
		if got := unavailableSuffix(have, "claude-sonnet-4.6"); got == "" {
			t.Error("a model outside the catalogue was not marked")
		}
		if got := unavailableSuffix(have, "claude-sonnet-5"); got != "" {
			t.Errorf("an available model was marked: %q", got)
		}
	})

	t.Run("opencode ids carry a provider prefix", func(t *testing.T) {
		have := map[string]bool{"claude-sonnet-5": true}
		if got := unavailableSuffix(have, "github-copilot/claude-sonnet-5"); got != "" {
			t.Errorf("prefixed id was marked unavailable: %q; the prefix must be stripped before comparing", got)
		}
	})

	t.Run("auto is never marked", func(t *testing.T) {
		// A pseudo-model the client resolves itself. It is not in the catalogue
		// and never will be.
		if got := unavailableSuffix(map[string]bool{"claude-sonnet-5": true}, "auto"); got != "" {
			t.Errorf("auto was marked unavailable: %q", got)
		}
	})
}

// TestModelPickerLabelsCarryNoStrayEscapes pins the bug review found: dim("")
// is not the empty string, it is an empty pair of escape codes, so appending
// the suffix unconditionally decorated every label with \033[2m\033[0m.
func TestModelPickerLabelsCarryNoStrayEscapes(t *testing.T) {
	p, err := providerFor("copilot")
	if err != nil || p == nil || len(p.KnownModels()) == 0 {
		t.Skip("no copilot provider in this build")
	}
	// Availability known, and everything in it available: no option should be
	// annotated, so no option should carry an escape sequence around nothing.
	have := map[string]bool{}
	for _, m := range p.KnownModels() {
		have[strings.ToLower(m.ID)] = true
	}

	for _, opt := range modelPickerOptions(p, have) {
		if strings.Contains(opt.Key, "\033[2m\033[0m") {
			t.Errorf("option %q carries an empty escape pair", opt.Key)
		}
	}
}

// Models the account cannot use sort last, greyed; the empty choice reads
// "Nav default (recommended)" (#1962 F11). The control is nil availability,
// which keeps the catalogue order.
func TestModelPickerSortsUnavailableLast(t *testing.T) {
	p, err := providerFor("copilot")
	if err != nil || p == nil || len(p.KnownModels()) < 2 {
		t.Skip("no copilot provider in this build")
	}
	known := p.KnownModels()
	// Only the last catalogue entry is available.
	last := known[len(known)-1].ID
	have := map[string]bool{strings.ToLower(last): true}
	opts := modelPickerOptions(p, have)
	seenUnavailable := false
	for i, o := range opts[2:] {
		off := strings.Contains(o.Key, "not available to this account")
		if seenUnavailable && !off {
			t.Errorf("option %d %q, usable, sorts after an unavailable one", i+2, o.Value)
		}
		seenUnavailable = seenUnavailable || off
	}
	if !seenUnavailable || strings.Contains(opts[len(opts)-1].Key, last) {
		t.Errorf("the unavailable models are not last: %q", opts[len(opts)-1].Key)
	}
	if p.DefaultModel() != "" && !strings.HasPrefix(opts[0].Key, "Nav default (recommended)") {
		t.Errorf("unset reads %q", opts[0].Key)
	}
	if ctl := modelPickerOptions(p, nil); ctl[2].Value != known[0].ID {
		t.Errorf("unknown availability reordered the list: %q", ctl[2].Value)
	}
}
