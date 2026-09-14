package agentpakke

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// policies.propose: the sandbox configuration an agentpakke asks the installing
// user to consent to (#858, step 2).
//
// The problem it solves is a silent one. A skill that queries Mimir and Loki on
// *.cloud.nais.io hits names that resolve to private addresses over naisdevice,
// and cplt refuses every such host regardless of any allowlist:
// "403 Private target blocked by cplt". The fix is one line of cplt
// configuration, and nothing in the pakke could say so — the 403 arrives in the
// middle of a query, so the human and the model both go looking at the PromQL.
//
// What a proposal is not:
//
//   - It is not configuration. nav-pilot writes nothing into ~/.config/cplt,
//     nothing into a .cplt.toml, and never touches cplt's trust store. An
//     approved proposal becomes --allow-private-domain on the launch line and
//     lives nowhere else, so the flag is derived from the consent record and
//     cannot drift from it.
//   - It is not a permission. proxy.allow_private_domains is cplt's
//     DNS-rebinding guard, and waiving it for one named host opens no port,
//     grants no path and executes nothing. The allowlist and the blocklist
//     still apply. The worst outcome is that the agent reaches an internal
//     service the user's own naisdevice already reaches.
//   - It is not self-applying. It takes effect only after an interactive
//     approval of exactly this content hash, in the scope that approved it.
//
// The block is keyed by tool and written in that tool's own vocabulary, so
// "cplt" here means cplt's key names, not nav-pilot's. Tool keys this binary
// does not implement are ignored per A3/A4, which is what lets pi and opencode
// be defined later without invalidating a manifest that exists today.

// proposableCpltKeys are the keys inside a cplt proposal this binary
// implements. Everything else in the block is inert: reported at install, never
// honoured (invariant 7). The keys a repo must never propose at all — preset,
// allow.exec, repo_dirs, inherit_env, allowed_domains, blocked_domains,
// proxy.forced, the guards — are refused by the schema instead, because those
// are not forward compatibility, they are a pakke asking for the sandbox to be
// taken apart.
var proposableCpltKeys = []string{"reason", "proxy"}

// Propose is policies.propose: proposals keyed by the tool they are written in.
type Propose struct {
	Cplt *CpltProposal `json:"cplt,omitempty"`
}

// CpltProposal is policies.propose.cplt.
type CpltProposal struct {
	// Reason is required and shown verbatim in the consent prompt. It is the
	// only thing the person deciding has to go on.
	Reason string `json:"reason"`

	Proxy CpltProxy `json:"proxy"`

	// raw is the block exactly as it was written. It is what [CpltProposal.Hash]
	// hashes and what [CpltProposal.InertKeys] reads, so a key this binary does
	// not implement still changes the hash — invariant 2, which is the whole
	// reason the raw form is kept rather than re-serialised from the fields.
	raw map[string]any
}

// CpltProxy is the one proposable cplt section in v1.
type CpltProxy struct {
	AllowPrivateDomains []string `json:"allow_private_domains,omitempty"`
}

// UnmarshalJSON decodes the block and keeps a copy of it verbatim.
func (p *CpltProposal) UnmarshalJSON(data []byte) error {
	// A named type, or this recurses into itself.
	type plain CpltProposal
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*p = CpltProposal(decoded)
	p.raw = raw
	return nil
}

// CpltProposal returns the cplt proposal this manifest declares, or nil.
func (m *Manifest) CpltProposal() *CpltProposal {
	if m == nil || m.Policies == nil || m.Policies.Propose == nil {
		return nil
	}
	return m.Policies.Propose.Cplt
}

// Hash is the content hash a consent record is keyed by: arrays sorted,
// canonical JSON, sha256 over the whole block, hex.
//
// Computed the way cplt's own trust store computes its hashes, so a reviewer
// comparing the two is comparing like with like. Over the whole block, not over
// the hosts: a revision that adds a key nav-pilot ignores today still voids the
// previous approval (invariant 2), which is what keeps "inert" from meaning
// "invisible".
func (p *CpltProposal) Hash() string {
	block := p.CanonicalJSON()
	if block == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(block))
	return hex.EncodeToString(sum[:])
}

// CanonicalJSON is the block in the exact form [CpltProposal.Hash] hashes.
//
// A consent record keeps it, so the question a changed revision asks can show
// which field moved rather than only which hosts did. The hash covers the whole
// block on purpose (invariant 2), so the thing that voided the approval can be
// a reason or a key nav-pilot does not implement — and a diff that can only
// compare hosts then has nothing to show for exactly the cases the user most
// needs to see (#861 review).
//
// "" when there is no proposal, or when the block will not re-marshal, which
// cannot happen for a block that came out of json.Unmarshal. Both make the
// hash empty too, so a proposal nav-pilot cannot canonicalise is one no record
// can match: the fail-closed direction.
func (p *CpltProposal) CanonicalJSON() string {
	if p == nil {
		return ""
	}
	data, err := json.Marshal(canonicalJSON(p.raw))
	if err != nil {
		return ""
	}
	return string(data)
}

// canonicalJSON sorts every array and leaves objects to encoding/json, which
// already writes map keys in sorted order.
func canonicalJSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = canonicalJSON(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = canonicalJSON(val)
		}
		sort.Slice(out, func(i, j int) bool {
			a, _ := json.Marshal(out[i])
			b, _ := json.Marshal(out[j])
			return string(a) < string(b)
		})
		return out
	default:
		return v
	}
}

// AllowPrivateDomains returns the hosts the proposal names, sorted and
// deduplicated. Sorted because the launch flags are derived from it and a
// launch vector that reorders between runs is one nobody can diff.
func (p *CpltProposal) AllowPrivateDomains() []string {
	if p == nil {
		return nil
	}
	seen := make(map[string]bool, len(p.Proxy.AllowPrivateDomains))
	out := make([]string, 0, len(p.Proxy.AllowPrivateDomains))
	for _, host := range p.Proxy.AllowPrivateDomains {
		host = strings.TrimSpace(host)
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	sort.Strings(out)
	return out
}

// InertKeys names the keys in the block this binary does not implement, sorted.
// They are reported at install and honoured nowhere (invariant 7).
func (p *CpltProposal) InertKeys() []string {
	if p == nil {
		return nil
	}
	var out []string
	for key := range p.raw {
		if !contains(proposableCpltKeys, key) {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// Summary is the one line the consent prompt and the install log use to say
// what the proposal would change.
func (p *CpltProposal) Summary() string {
	hosts := p.AllowPrivateDomains()
	if len(hosts) == 0 {
		return "nothing this nav-pilot implements"
	}
	return fmt.Sprintf("reach %s, which resolve to private addresses", strings.Join(hosts, ", "))
}

func contains(list []string, want string) bool {
	for _, got := range list {
		if got == want {
			return true
		}
	}
	return false
}
