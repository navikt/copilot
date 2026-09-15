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
//     approved proposal becomes --allow-private-domain and --allow-read on the
//     launch line and lives nowhere else, so the flags are derived from the
//     consent record and cannot drift from it.
//   - It is not a permission. proxy.allow_private_domains is cplt's
//     DNS-rebinding guard, and waiving it for one named host opens no port,
//     grants no path and executes nothing. The allowlist and the blocklist
//     still apply. The worst outcome is that the agent reaches an internal
//     service the user's own naisdevice already reaches. allow.read is
//     narrower still: one read-only rule per named file, no write, no bind, no
//     outbound, and a path a pakke may propose at all is one neither cplt's
//     deny lists nor nav-pilot's own rules cover (#885).
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
var proposableCpltKeys = []string{"allow", "reason", "proxy"}

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

	Allow CpltAllow `json:"allow"`

	// raw is the block exactly as it was written. It is what [CpltProposal.Hash]
	// hashes and what [CpltProposal.InertKeys] reads, so a key this binary does
	// not implement still changes the hash — invariant 2, which is the whole
	// reason the raw form is kept rather than re-serialised from the fields.
	raw map[string]any
}

// CpltProxy is the proposable cplt proxy section.
type CpltProxy struct {
	AllowPrivateDomains []string `json:"allow_private_domains,omitempty"`
}

// CpltAllow is policies.propose.cplt.allow. Read is the only grant a pakke may
// ask for: cplt emits exactly one rule per entry, `(allow file-read* (subpath
// "<p>"))` on macOS and AccessFs::ReadFile|ReadDir on Linux. No write, no bind,
// no outbound, and ResolveUnix is granted only alongside a write, so a read
// grant cannot become a connect right. allow.write, allow.exec and
// allow.socket stay refused by the schema.
type CpltAllow struct {
	Read []string `json:"read,omitempty"`
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
	return dedupeSorted(p.Proxy.AllowPrivateDomains)
}

// dedupeSorted trims, drops empties and duplicates, and sorts.
func dedupeSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// AllowRead returns the files the proposal asks to read, in the "~/"-relative
// form the manifest declares, sorted and deduplicated. Sorted for the same
// reason the hosts are: the launch flags are derived from it.
//
// Still "~/"-relative here. Expansion happens at launch, against the home the
// launch actually runs under, so the consent record keeps what the person read
// on screen and the absolute path is never stored anywhere.
func (p *CpltProposal) AllowRead() []string {
	if p == nil {
		return nil
	}
	return dedupeSorted(p.Allow.Read)
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
	var parts []string
	if hosts := p.AllowPrivateDomains(); len(hosts) > 0 {
		parts = append(parts, fmt.Sprintf("reach %s, which resolve to private addresses", strings.Join(hosts, ", ")))
	}
	if reads := p.AllowRead(); len(reads) > 0 {
		parts = append(parts, fmt.Sprintf("read %s", strings.Join(reads, ", ")))
	}
	if len(parts) == 0 {
		return "nothing this nav-pilot implements"
	}
	return strings.Join(parts, "; and ")
}

func contains(list []string, want string) bool {
	for _, got := range list {
		if got == want {
			return true
		}
	}
	return false
}

// cpltDeniedHomePaths are the "~/"-relative paths a pakke may never propose a
// read grant on, mirrored from cplt's own lists in src/sandbox_policy.rs:
// DENIED_DOTFILES, DENIED_FILES and DENIED_HOME_SUBPATHS.
//
// Mirrored rather than deferred to, because cplt refuses less than this on
// purpose and the difference is the whole point. cplt's grant_is_refused stops
// a grant on ~/.ssh itself, but a grant on a file *inside* it is a supported
// override on both backends — that is how a developer with a private registry
// opens ~/.npmrc, and how anyone opens one named key. A person choosing that
// for themselves is not the same as a pakke asking for it and getting a yes
// out of the install prompt, so a proposal is held to the stricter rule: at or
// under any of the three lists, refused at validate.
//
// ponytail: a copied list, so it ages if cplt adds an entry. The upgrade path
// is cplt publishing them, not nav-pilot growing a probe — and an entry this
// list lacks is still refused for grants cplt itself refuses.
var cpltDeniedHomePaths = []string{
	// DENIED_DOTFILES
	".ssh", ".gnupg", ".aws", ".azure", ".kube", ".docker", ".nais",
	".password-store", ".config/gcloud", ".config/op", ".terraform.d",
	".config/cplt", ".nav-pilot",
	// DENIED_FILES
	".netrc", ".pypirc", ".gem/credentials", ".vault-token",
	// DENIED_HOME_SUBPATHS
	".m2/settings.xml", ".m2/settings-security.xml", ".gradle/gradle.properties",
	".cargo/credentials", ".cargo/credentials.toml", ".nuget/NuGet.Config", ".npmrc",
}

// checkPropose runs the read-grant rules the schema cannot express: what the
// path resolves to, and what it must never resolve to.
//
// The schema owns the shape — leading "~/", no backslash, no control rune, and
// a final component with a name and an extension, which is how a directory and
// a trailing separator are refused before anything looks at the value. This
// owns the two that need a list or a walk: traversal, and the denied paths.
func (m *Manifest) checkPropose() error {
	proposal := m.CpltProposal()
	if proposal == nil {
		return nil
	}
	for _, declared := range proposal.AllowRead() {
		if err := checkReadGrant(declared); err != nil {
			return fmt.Errorf("policies.propose.cplt.allow.read: %w", err)
		}
	}
	return nil
}

// checkReadGrant refuses one proposed read grant, or returns nil.
func checkReadGrant(declared string) error {
	rest, ok := strings.CutPrefix(declared, "~/")
	if !ok {
		return fmt.Errorf("%q is not a path under the home directory: write it as \"~/<path>\", "+
			"which nav-pilot expands at launch", declared)
	}
	for _, segment := range strings.Split(rest, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("%q does not name a file: it has an empty, \".\" or \"..\" component, "+
				"and a grant that can climb out of what it names is not one anybody can read off the prompt", declared)
		}
	}
	for _, denied := range cpltDeniedHomePaths {
		if rest == denied || strings.HasPrefix(rest, denied+"/") {
			return fmt.Errorf("%q is at or under ~/%s, which cplt denies or protects. "+
				"A pakke may not propose it, and a user who needs it grants it themselves with "+
				"`cplt config set allow.read`", declared, denied)
		}
	}
	return nil
}
