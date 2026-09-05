// Package agentpakke implements the agentpakke contract: the versioned,
// self-describing manifest an agentpakke repo ships so the nav-pilot binary can
// install, sync, and launch its content without a fork.
//
// The manifest lives at [ManifestPath] inside the agentpakke repo and is
// validated against the published JSON Schema in cli/nav-pilot/schemas (embedded
// here via go:embed, so the schema file is the single source of truth for both
// nav-pilot and an agentpakke repo's own CI lint).
//
// # Manifest is the single internal currency
//
// Every downstream consumer reads its persona, allowlist, model defaults, and
// content layout from a [Manifest] — never from hardcoded constants and never
// from a legacy-vs-manifest branch. Sources that do not ship a manifest are
// adapted up-front by [SynthesizeLegacy] / [Default], which express today's
// navikt/copilot conventions in manifest form. Past the point of load, nothing
// in this package (or in its callers) needs to know whether a Manifest came
// from disk or from synthesis.
//
// # Migration path
//
// The legacy adapter is transitional, and the collection model it represents is
// on its way out:
//
//   - Phase 1 (delivered): the manifest is optional. A source without
//     [ManifestPath] yields [ErrNoManifest] and the caller substitutes
//     [SynthesizeLegacy], so existing installs behave byte-for-byte as before.
//   - Phase 2 (delivered, #468): navikt/copilot ships its own
//     .nav-pilot/agentpakke.json and is an ordinary agentpakke — the default
//     one. Synthesis is no longer exercised for the default source; its five
//     collections are folded into the one pakke, and collection-era state is
//     adopted loudly on sync. The committed manifest is held equal to
//     SynthesizeLegacy("") by test for as long as both exist.
//   - Phase 3: [SynthesizeLegacy] and the collection mechanism are removed,
//     under the deprecation window the contract's compatibility rules (A4)
//     require. The window opened when phase 2 shipped.
//
// Nothing here should grow a second, parallel installation model to serve that
// transition: the adapter exists so there is exactly one.
//
// # Tiers
//
// A client's conformance tier is derived from the shape of its manifest entry,
// never declared separately (so it cannot drift out of sync with the content):
// an entry with payloads is Tier 2 (pre-built, digest-bound trees that nav-pilot
// verifies and stages), an entry without is Tier 1 (markdown content at the
// manifest's layout paths, which nav-pilot materializes). See [Manifest.Tier].
//
// # Ignore-unknown
//
// Client keys, context keys, and additional fields that this binary does not
// recognize are ignored rather than rejected: an agentpakke that adds a client
// or a context stays valid on older binaries, which simply do not offer what
// they cannot name. Fail-closed applies to malformed declarations of *known*
// constructs.
package agentpakke

import (
	"path"
	"sort"
	"strings"
)

const (
	// ManifestDir is the agentpakke-repo directory holding nav-pilot metadata.
	ManifestDir = ".nav-pilot"

	// ManifestFile is the manifest's file name inside [ManifestDir].
	ManifestFile = "agentpakke.json"

	// ManifestPath is the repo-relative path to an agentpakke manifest.
	ManifestPath = ManifestDir + "/" + ManifestFile

	// PayloadManifestFile is the payload manifest name resolved by convention
	// inside a Tier 2 payload directory (A7). A payload's `manifest` field
	// overrides this.
	PayloadManifestFile = "manifest.json"

	// DefaultContext is the payload context launched when a client entry
	// declares no defaultContext (G3).
	DefaultContext = "full"

	// InheritModel is the literal defaultModel value meaning "do not pin a
	// model; inherit the provider's or session's choice" (F3).
	InheritModel = "inherit"
)

// SupportedContractMajors lists the manifest contract major versions this
// binary understands. A manifest outside this set is rejected with an error
// naming the supported versions (A3/A4).
var SupportedContractMajors = []string{"1"}

// KnownClients are the client ids this binary can actually launch. It mirrors
// internal/provider's registry; the list lives here rather than being imported
// because provider will consume this package once the manifest drives its
// personas. Client keys outside this list are ignored, not rejected: that
// client is simply unavailable (A3/A4).
var KnownClients = []string{"copilot", "opencode", "pi"}

// IsKnownClient reports whether this binary can launch a client id.
func IsKnownClient(client string) bool {
	for _, known := range KnownClients {
		if known == client {
			return true
		}
	}
	return false
}

// Manifest is a parsed agentpakke manifest. Unknown JSON fields are tolerated
// (encoding/json ignores them), matching the contract's ignore-unknown rule.
type Manifest struct {
	// ContractVersion is the manifest schema-and-semantics version ("1" or "1.x").
	ContractVersion string `json:"contractVersion"`

	// Name is the agentpakke identity. It is also what an agentpakke install
	// records as its collection: a manifest-bearing source supersedes the
	// legacy collections/<name>/manifest.json model rather than declaring
	// entries in it.
	Name string `json:"name"`

	Description string `json:"description"`

	// Owner is attribution only; the install source is wherever the manifest
	// was cloned from.
	Owner *Owner `json:"owner,omitempty"`

	// Clients holds one entry per supported client, keyed by client id
	// ("copilot", "opencode", "pi", …). Unrecognized keys are ignored, not
	// rejected.
	Clients map[string]ClientEntry `json:"clients"`

	// Layout declares repo-relative content directories. Required whenever any
	// client is Tier 1 (D1).
	Layout *Layout `json:"layout,omitempty"`

	// Provenance records what composed content is made of (H2).
	Provenance *Provenance `json:"provenance,omitempty"`

	// Policies points at optional policy artifacts materialized for Tier 1
	// clients (E1).
	Policies *Policies `json:"policies,omitempty"`

	// Profiles points at optional launch profiles (E2).
	Profiles *Profiles `json:"profiles,omitempty"`

	// MinNavPilotVersion is the minimum running nav-pilot version, in the
	// YYYY.MM.DD-HHMMSS-sha7 release format.
	MinNavPilotVersion string `json:"minNavPilotVersion,omitempty"`
}

// Owner is attribution metadata for an agentpakke.
type Owner struct {
	Repo string `json:"repo,omitempty"`
	Team string `json:"team,omitempty"`
}

// ClientEntry is a manifest's declaration for one client. Its tier is derived
// from shape: a non-empty Payloads makes it Tier 2, otherwise Tier 1.
type ClientEntry struct {
	// PrimaryAgents are the agents selectable/launchable directly in the
	// client (C1, C2). Every other agent materializes as a subagent.
	//
	// It is the Tier 1 roster: the entry's own layout content is what carries
	// the agents there. A Tier 2 entry's roster lives on each [Payload]
	// instead — see [Manifest.PayloadPrimaryAgents] — because different
	// payload trees for the same client ship different agents. The schema
	// requires this field only when the entry declares no payloads, and a
	// Tier 2 entry that carries one anyway is ignored, never a fallback.
	PrimaryAgents []string `json:"primaryAgents"`

	// Compatibility is a client version range (e.g. ">=1.18.20,<2"). Exact
	// pins belong to the local/security-profile tier, not here.
	Compatibility string `json:"compatibility,omitempty"`

	// DefaultModel is a model id, or [InheritModel].
	DefaultModel string `json:"defaultModel,omitempty"`

	// DefaultContext names the payload context launched by default; empty
	// means [DefaultContext].
	DefaultContext string `json:"defaultContext,omitempty"`

	// Payloads maps context id ("full", "focused", …) to a pre-built payload.
	// Its presence is what makes this entry Tier 2.
	Payloads map[string]Payload `json:"payloads,omitempty"`
}

// Payload locates one Tier 2 payload tree for a single client×context.
type Payload struct {
	// Path is the repo-relative directory holding the payload tree.
	Path string `json:"path"`

	// PrimaryAgents are the agents launchable in this context, the first one
	// being the context's default persona. Required by the schema with at
	// least one element, so a loaded Tier 2 manifest cannot leave it absent.
	PrimaryAgents []string `json:"primaryAgents"`

	// Manifest optionally overrides the payload manifest location, which
	// otherwise resolves by convention to <Path>/manifest.json.
	Manifest string `json:"manifest,omitempty"`
}

// Layout declares repo-relative content directories for Tier 1 clients.
type Layout struct {
	Agents       string `json:"agents"`
	Skills       string `json:"skills"`
	Instructions string `json:"instructions,omitempty"`
	Prompts      string `json:"prompts,omitempty"`
	Hooks        string `json:"hooks,omitempty"`
}

// Provenance records the base and overlays a composed agentpakke was built from.
type Provenance struct {
	Base     *ProvenanceBase     `json:"base,omitempty"`
	Overlays []ProvenanceOverlay `json:"overlays,omitempty"`
}

// ProvenanceBase is the pinned base an agentpakke composed against.
type ProvenanceBase struct {
	Repo   string `json:"repo"`
	Digest string `json:"digest"`
}

// ProvenanceOverlay is one composed-in component and its version.
type ProvenanceOverlay struct {
	Component string `json:"component"`
	Version   string `json:"version"`
}

// Policies points at optional policy artifacts.
type Policies struct {
	OpenCodePermissions string `json:"opencodePermissions,omitempty"`
}

// Profiles points at optional launch profiles and names the default one.
type Profiles struct {
	Dir     string `json:"dir,omitempty"`
	Default string `json:"default,omitempty"`
}

// Tier constants returned by [Manifest.Tier].
const (
	// TierUnknown means the manifest declares no entry for that client, so the
	// client is unavailable for this agentpakke.
	TierUnknown = 0

	// TierLayout (Tier 1) means nav-pilot materializes client config from the
	// manifest's layout paths.
	TierLayout = 1

	// TierPayload (Tier 2) means nav-pilot verifies and stages pre-built
	// payloads declared by the client entry.
	TierPayload = 2
)

// Tier returns the conformance tier for a client, derived from the shape of its
// entry. Unknown clients return [TierUnknown].
func (m *Manifest) Tier(client string) int {
	entry, ok := m.Client(client)
	if !ok {
		return TierUnknown
	}
	if len(entry.Payloads) > 0 {
		return TierPayload
	}
	return TierLayout
}

// Client returns the entry for a client id.
func (m *Manifest) Client(client string) (ClientEntry, bool) {
	if m == nil || m.Clients == nil {
		return ClientEntry{}, false
	}
	entry, ok := m.Clients[client]
	return entry, ok
}

// ClientIDs returns the declared client ids in sorted order.
func (m *Manifest) ClientIDs() []string {
	if m == nil {
		return nil
	}
	ids := make([]string, 0, len(m.Clients))
	for id := range m.Clients {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// AvailableClients returns the declared clients this binary can launch, in
// sorted order — the manifest's clients minus the keys it does not recognize.
func (m *Manifest) AvailableClients() []string {
	var ids []string
	for _, id := range m.ClientIDs() {
		if IsKnownClient(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// PrimaryAgents returns the client's primary agents, or nil for an unknown
// client.
func (m *Manifest) PrimaryAgents(client string) []string {
	entry, ok := m.Client(client)
	if !ok {
		return nil
	}
	return entry.PrimaryAgents
}

// IsPrimaryAgent reports whether an agent is primary for a client. Everything
// else materializes as a subagent (C2).
func (m *Manifest) IsPrimaryAgent(client, agent string) bool {
	for _, name := range m.PrimaryAgents(client) {
		if name == agent {
			return true
		}
	}
	return false
}

// PayloadPrimaryAgents returns the agents launchable in one client×context,
// first being that context's default persona. It reads the payload and nothing
// else: a Tier 2 client entry's own primaryAgents describes no payload tree in
// particular, so falling back to it would hand a launch an agent the staged
// tree may not ship. Unknown client or context returns nil.
func (m *Manifest) PayloadPrimaryAgents(client, context string) []string {
	p, ok := m.Payload(client, context)
	if !ok {
		return nil
	}
	return p.PrimaryAgents
}

// DefaultModel returns the client's default model id, or the empty string when
// the client is unknown or pins nothing. The literal [InheritModel] is returned
// as-is: it is a meaningful value, not an absent one.
func (m *Manifest) DefaultModel(client string) string {
	entry, ok := m.Client(client)
	if !ok {
		return ""
	}
	return entry.DefaultModel
}

// DefaultContext returns the payload context a client launches by default,
// falling back to [DefaultContext] when unset (G3).
func (m *Manifest) DefaultContext(client string) string {
	entry, ok := m.Client(client)
	if !ok || strings.TrimSpace(entry.DefaultContext) == "" {
		return DefaultContext
	}
	return entry.DefaultContext
}

// Payload returns the payload declared for a client×context.
func (m *Manifest) Payload(client, context string) (Payload, bool) {
	entry, ok := m.Client(client)
	if !ok || entry.Payloads == nil {
		return Payload{}, false
	}
	p, ok := entry.Payloads[context]
	return p, ok
}

// PayloadManifestPath returns the repo-relative payload manifest path for a
// client×context: the payload's `manifest` override when set, otherwise
// <path>/manifest.json by convention (A7).
func (m *Manifest) PayloadManifestPath(client, context string) (string, bool) {
	p, ok := m.Payload(client, context)
	if !ok {
		return "", false
	}
	return p.ManifestPath(), true
}

// ManifestPath returns the payload manifest location for a payload: its
// override when set, otherwise <Path>/manifest.json.
func (p Payload) ManifestPath() string {
	if override := strings.TrimSpace(p.Manifest); override != "" {
		return override
	}
	return path.Join(p.Path, PayloadManifestFile)
}

// HasTier reports whether any declared client is at the given tier.
func (m *Manifest) HasTier(tier int) bool {
	for _, id := range m.ClientIDs() {
		if m.Tier(id) == tier {
			return true
		}
	}
	return false
}
