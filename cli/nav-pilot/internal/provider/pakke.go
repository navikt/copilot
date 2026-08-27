package provider

import (
	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// SetActivePakke sets the agentpakke that launch and materialization read
// their persona, primary-agent allowlist, and model defaults from. It mirrors
// [SetVersion]: a process-wide seam set once, at startup or before a launch.
// A nil manifest restores the built-in default.
//
// Invariant callers must uphold: only set a manifest that declares the client
// about to be launched or materialized. WP3's single call site (the staged
// launch branch in internal/cli) satisfies it by construction — it sets the
// pakke only after Tier(client) == TierPayload, which requires a declared
// entry, and schemas/agentpakke-v1.json requires primaryAgents (line 49) with
// "minItems": 1 (line 56) on every declared client entry, checked by
// agentpakke.Load before a manifest is ever attached to a source. That is what lets
// [PrimaryAgent] be a pure manifest read with no fallback.
//
// The manifest itself is held by internal/source, the lowest package both this
// one and internal/artifacts import — artifacts materializes agent frontmatter
// from the same declarations and cannot import provider.
func SetActivePakke(m *agentpakke.Manifest) { source.SetActivePakke(m) }

// PrimaryAgent returns the persona a client launches by default: the first
// primary agent the active agentpakke declares for it.
//
// It returns "" exactly when the active agentpakke declares no entry for the
// client — there is no fallback to the built-in default's persona, which would
// inject Nav's nav-pilot persona into a foreign agentpakke's launch while
// materialization (artifacts/export.go:317) correctly demoted it to a
// subagent. Launch paths never observe the empty string: the built-in default
// declares every known client, and the staged path only sets a pakke that
// declares the client it is launching (see [SetActivePakke]).
func PrimaryAgent(client string) string {
	agents := source.ActivePakke().PrimaryAgents(client)
	if len(agents) == 0 {
		return ""
	}
	return agents[0]
}

// openCodeDefaultModel returns the model an opencode launch falls back to when
// the user pins none: the active agentpakke's declaration, or the built-in
// default when it pins nothing.
//
// [agentpakke.InheritModel] counts as pinning nothing: consumers of this
// function (ToOpenCodeModel, the setup label) need a concrete model id, and
// "inherit" means "whatever the client would use anyway". The staged launch
// path does not call this at all — it reads the declaration directly and omits
// --model entirely for inherit. Cosmetic residue: `config setup` run with an
// inherit-pakke active labels the built-in id "Nav default"; no M2 flow sets a
// pakke before setup, so nothing reaches it today.
func openCodeDefaultModel() string {
	model := source.ActivePakke().DefaultModel("opencode")
	if model != "" && model != agentpakke.InheritModel {
		return model
	}
	return OpenCodeDefaultModel
}
