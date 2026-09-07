package cli

import (
	"os/exec"
	"strings"
	"sync"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// The picker is generated from models.dev, which is global, while what a person
// can actually launch depends on their account and plan. So the list offers
// models the client refuses: on the machine this was written on, 14 of them,
// including one the generator's PINNED list still described as "delisted but
// still launches" (#717).
//
// Marking beats pruning. A model missing from one account may be present for
// another, so removing entries would take away something that works for a
// colleague on a different plan. An annotated entry tells the truth for the
// person reading it and no lies about anyone else.

var (
	// A pointer so a test can install a fresh one. sync.Once cannot be reset,
	// and a test that seeds the cache would otherwise leak into whatever runs
	// after it in this package.
	availabilityOnce = &sync.Once{}
	availableIDs     map[string]bool
)

// availableModelIDs returns the ids this account can launch, or nil when the
// question could not be answered.
//
// Asked once per process. The probe costs a client start (~2s) and no AI
// credits, but a picker that pays it per keystroke would be unusable, and the
// answer cannot change while a picker is open.
//
// nil is not "nothing is available": it is "unknown", and every caller must
// treat it as a reason to annotate nothing. An offline developer gets the plain
// list they had before, never a list where everything is marked unavailable.
func availableModelIDs() map[string]bool {
	availabilityOnce.Do(func() {
		copilotPath, err := exec.LookPath("copilot")
		if err != nil {
			return
		}
		ids, ok := clientChatModels(copilotPath)
		if !ok {
			return
		}
		availableIDs = make(map[string]bool, len(ids))
		for _, id := range ids {
			availableIDs[strings.ToLower(id)] = true
		}
	})
	return availableIDs
}

// unavailableSuffix is what the picker appends to a model the account cannot
// launch, or "" when it can, or when availability is unknown.
//
// have is the answer from [availableModelIDs]: nil for "not known", which is
// the only value that means unknown. An empty non-nil map is a catalogue that
// was read and holds nothing, and every model is then genuinely unavailable;
// conflating the two would hide a real answer behind the offline case.
func unavailableSuffix(have map[string]bool, modelID string) string {
	if have == nil {
		return ""
	}
	id := strings.ToLower(strings.TrimPrefix(modelID, domain.OpenCodeProviderPrefix))
	if id == "" || id == "auto" || have[id] {
		return ""
	}
	return "  (not available to this account)"
}
