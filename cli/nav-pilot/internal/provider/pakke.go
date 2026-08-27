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
// The manifest itself is held by internal/source, the lowest package both this
// one and internal/artifacts import — artifacts materializes agent frontmatter
// from the same declarations and cannot import provider.
func SetActivePakke(m *agentpakke.Manifest) { source.SetActivePakke(m) }

// PrimaryAgent returns the persona a client launches by default: the first
// primary agent the active agentpakke declares for it. An agentpakke that
// declares none for the client falls back to the built-in default's, so a
// launch never passes an empty --agent.
func PrimaryAgent(client string) string {
	agents := source.ActivePakke().PrimaryAgents(client)
	if len(agents) == 0 {
		agents = agentpakke.Default().PrimaryAgents(client)
	}
	if len(agents) == 0 {
		return ""
	}
	return agents[0]
}

// openCodeDefaultModel returns the model an opencode launch falls back to when
// the user pins none: the active agentpakke's declaration, or the built-in
// default when it pins nothing.
func openCodeDefaultModel() string {
	if model := source.ActivePakke().DefaultModel("opencode"); model != "" {
		return model
	}
	return OpenCodeDefaultModel
}
