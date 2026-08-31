package provider

import (
	"fmt"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
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
// entry, and schemas/agentpakke-v1.json requires a "minItems": 1 primaryAgents
// on every payload of a Tier 2 entry (and on every Tier 1 entry itself),
// checked by agentpakke.Load before a manifest is ever attached to a source.
// That is what lets [PrimaryAgent] and [PrimaryAgentFor] be pure manifest reads
// with no fallback.
//
// The staged path additionally launches a context, so its invariant is that the
// manifest declares the client *and* the launched context's roster — again by
// construction: the context comes from the manifest's own payloads map.
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

// PrimaryAgentFor returns the persona a Tier 2 launch of one client×context
// starts: the first agent that payload declares.
//
// It reads the payload roster only. A Tier 2 client entry's own primaryAgents
// is not consulted, and not a fallback: the payload trees for one client
// differ in which agents they ship (grillmester's focused payloads contain
// only barista and grill-inspektor while the client entry led with
// grillmester), so a client-level answer is wrong for every context but one.
// [PrimaryAgent] keeps serving Tier 1 materialization and the legacy launch,
// where the client entry is the unit that carries the agents.
func PrimaryAgentFor(client, context string) string {
	agents := source.ActivePakke().PayloadPrimaryAgents(client, context)
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

// ResolvedModelNotice returns the one-line launch notice naming the model a
// launch will run on and where it came from, or "" when nothing names one and
// the client picks for itself: pi, which is launched with no model at all,
// copilot with no pin anywhere, and an agentpakke declaring "inherit".
//
// Nothing told a user which model they were about to spend on: the model comes
// from a config file, a flag, or an agentpakke declaration, and the launch said
// none of that out loud. This mirrors the resolution the launch builders
// already do rather than adding a second one, in the same order they use: the
// user's own setting first, then the active agentpakke's declaration.
func ResolvedModelNotice(client string, r domain.ResolvedConfig) string {
	model, origin := resolvedModelOrigin(client, r)
	if model == "" {
		return ""
	}
	return fmt.Sprintf("Model: %s (%s)", model, origin)
}

// resolvedModelOrigin returns the model a launch resolves to and a short phrase
// naming its source, or ("", "") when the launch names no model. The origin
// names the agentpakke that supplied it, so an empty answer is the only honest
// one wherever the launch itself passes no model:
//
//   - a client that forwards no model at all (see [clientForwardsModel]) never
//     runs on the user's setting or on any declaration, whatever they say;
//   - a pakke declaring "inherit" makes the staged launch builders omit --model
//     entirely, and the staged opencode launch points OPENCODE_CONFIG_DIR at
//     the payload, whose own config then picks. Naming the built-in Nav default
//     would announce a model that launch never asks for. The legacy opencode
//     path does substitute the built-in default for an empty model, but it only
//     ever runs under the built-in agentpakke, which declares a model rather
//     than "inherit": only the staged path sets another one.
func resolvedModelOrigin(client string, r domain.ResolvedConfig) (model, origin string) {
	if !clientForwardsModel(client) {
		return "", ""
	}
	if r.Model != "" {
		if client == "opencode" {
			return ToOpenCodeModel(r.Model), "your setting"
		}
		return r.Model, "your setting"
	}
	if declared := pakkeDeclaredModel(client); declared != "" {
		return declared, source.ActivePakke().Name + " default"
	}
	return "", ""
}
