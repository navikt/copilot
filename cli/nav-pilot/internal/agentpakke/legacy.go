package agentpakke

import "regexp"

// DefaultName is the identity of the synthesized manifest that represents
// navikt/copilot's own content.
const DefaultName = "nav-pilot"

// identifierPattern mirrors the schema's identifier definition. Synthesis has
// to honor it: a synthesized manifest must be indistinguishable from a loaded
// one, including passing validation.
var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Default returns the manifest for a source that ships no
// .nav-pilot/agentpakke.json.
//
// It is the transitional legacy adapter, not a hardcoded alternative to the
// manifest mechanism: it expresses today's navikt/copilot conventions —
// personas, opencode primary-agent allowlist, model defaults, and the
// agents//skills/ content layout — in manifest form, so every consumer reads
// one type and no code path branches on "legacy or manifest". It disappears
// once navikt/copilot ships its own manifest and the collection mechanism is
// retired (see the package doc's migration path).
//
// The literals below mirror the values still hardcoded at their call sites
// (internal/provider's CopilotAgentPersona / OpenCodeAgentPersona /
// OpenCodeDefaultModel, and internal/source's openCodePrimaryAgents). Stage 2
// replaces those call sites with reads from the active manifest; until then the
// two must be kept in step, which the package's tests assert.
func Default() *Manifest {
	return SynthesizeLegacy("")
}

// SynthesizeLegacy returns the legacy adapter manifest for a source installed
// under the collection model, naming the manifest after the collection being
// installed. The legacy install flow is collection-parameterized, so the
// synthesized identity has to carry the collection for state to round-trip
// (StateFile.Collection records a manifest's name for agentpakke installs).
//
// An empty collection, or one whose name is not a contract identifier — the
// synthetic "(all)" collection, for instance — falls back to [DefaultName].
func SynthesizeLegacy(collection string) *Manifest {
	name := DefaultName
	if identifierPattern.MatchString(collection) {
		name = collection
	}
	return &Manifest{
		ContractVersion: "1",
		Name:            name,
		Description:     "Nav's default agents, skills, instructions, and prompts",
		Owner: &Owner{
			Repo: "navikt/copilot",
			Team: "nav-pilot maintainers",
		},
		Clients: map[string]ClientEntry{
			// Copilot CLI loads a single Nav persona; every other agent is
			// reachable through it rather than selectable directly.
			"copilot": {
				PrimaryAgents: []string{"nav-pilot"},
			},
			// opencode's picker offers both Nav personas; everything else
			// materializes as a subagent. The first entry is the persona
			// launched by default.
			"opencode": {
				PrimaryAgents: []string{"nav-pilot", "nav-pilot-opus"},
				DefaultModel:  "github-copilot/auto",
			},
			// pi consumes no persona today; the entry exists so client
			// availability is expressible in one place instead of a special
			// case at every call site.
			"pi": {
				PrimaryAgents: []string{"nav-pilot"},
			},
		},
		Layout: &Layout{
			Agents:       "agents",
			Skills:       "skills",
			Instructions: "instructions",
			Prompts:      "prompts",
		},
	}
}
