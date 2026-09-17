package agentpakke

import (
	"encoding/json"
	"regexp"
)

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
			//
			// DefaultModel is [InheritModel] rather than a model id, and that
			// is a decision, not a placeholder. Copilot's routing comment in
			// internal/provider (OpenCodeDefaultModel) prefers letting Copilot
			// Auto follow the current cost/quality frontier over pinning a
			// model, and a Tier 1 copilot launch has always emitted no --model
			// at all when the user pinned none. "inherit" is the value that
			// says exactly that: the declaration point now exists for both
			// tiers and for both clients, and no launch argument changes.
			// Picking a concrete id belongs to whoever owns the routing
			// decision, in a commit that is about the routing decision.
			"copilot": {
				PrimaryAgents: []string{"nav-pilot"},
				DefaultModel:  InheritModel,
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
			Hooks:        "hooks",
		},
		Policies: &Policies{Propose: &Propose{Cplt: defaultCpltProposal()}},
	}
}

// defaultCpltProposalJSON is the sandbox proposal navikt/copilot's own pakke
// makes, byte-for-byte as .nav-pilot/agentpakke.json spells it.
//
// The four hosts are the ones skills/observability-debugging curls, and they
// are named in full rather than as the suffix `cloud.nais.io`, which
// [is_domain_match] in cplt would also accept. The suffix would waive the
// DNS-rebinding guard for every host under every Nais tenant; these four are
// what the skill actually queries, so they are what the pakke asks for. A fifth
// one is a decision somebody makes on purpose, here, rather than something a
// new hostname inherits.
//
// Only Mimir, Loki and Tempo. grafana.nav.cloud.nais.io and
// console.nav.cloud.nais.io appear in the same artifacts but resolve publicly
// and are handed to a human to open in a browser, never fetched, so neither
// needs a waiver. collector-internet.nav.cloud.nais.io, which nav-pilot itself
// posts telemetry to, resolves publicly too — it needs the allowlist entry it
// already has in internal/cli/config_sandbox.go and nothing more.
//
// The allowlist and this are different gates and both have to pass: cplt
// refuses a host outside `proxy.allowed_domains` before DNS
// ("Domain not in allowlist"), and refuses a host that resolves to a private IP
// after DNS unless `proxy.allow_private_domains` covers it ("Resolved to a
// private IP"). navOwnDomains carries these four for the first gate; this block
// asks the user for the second. TestObservabilityHostsMatchTheProposal holds
// the two lists together.
const defaultCpltProposalJSON = `{
  "reason": "The observability-debugging skill queries Mimir, Loki and Tempo with curl. Those four hosts resolve to private addresses over naisdevice, so cplt refuses every one of them before the request leaves the sandbox. Without this waiver each metrics, logs and trace query in that skill returns a 403 mid-query, where it reads as a broken PromQL rather than as a sandbox rule.",
  "proxy": {
    "allow_private_domains": [
      "mimir.nav.cloud.nais.io",
      "loki.nav.cloud.nais.io",
      "tempo.dev-gcp.nav.cloud.nais.io",
      "tempo.prod-gcp.nav.cloud.nais.io"
    ]
  }
}`

// defaultCpltProposal decodes [defaultCpltProposalJSON] through the same
// UnmarshalJSON a loaded manifest goes through, so the synthesized proposal
// carries the verbatim block a hash is taken over and compares equal to the
// committed manifest's.
func defaultCpltProposal() *CpltProposal {
	var p CpltProposal
	if err := json.Unmarshal([]byte(defaultCpltProposalJSON), &p); err != nil {
		// Unreachable: the literal is a constant in this file and the test
		// suite decodes it on every run.
		panic("agentpakke: default cplt proposal does not decode: " + err.Error())
	}
	return &p
}

// legacyCollections are the five curated collections navikt/copilot shipped
// before it declared its own agentpakke — folded into the single "nav-pilot"
// pakke by #468. The source no longer knows these names, so the binary has to:
// they are what an existing install's state may still say, and what a user's
// muscle memory still types.
var legacyCollections = map[string]bool{
	"frontend":        true,
	"fullstack":       true,
	"kotlin-backend":  true,
	"nextjs-frontend": true,
	"platform":        true,
}

// IsLegacyCollection reports whether name is one of the five retired
// navikt/copilot collections, so install can answer `nav-pilot install
// frontend` with the fold instead of a bare "not found".
func IsLegacyCollection(name string) bool {
	return legacyCollections[name]
}

// IsIdentifier reports whether name is a contract identifier — the shape every
// real collection name had, and the synthetic "(all)"/"(à la carte)" labels do
// not. It is the test [SynthesizeLegacy] applies before naming a manifest
// after a collection.
func IsIdentifier(name string) bool {
	return identifierPattern.MatchString(name)
}
