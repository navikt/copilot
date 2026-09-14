package agentpakke

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// proposeManifest is a minimal Tier 1 manifest carrying a policies.propose.cplt
// block, so a test can vary just the block.
func proposeManifest(block string) string {
	return fmt.Sprintf(`{
  "contractVersion": "1",
  "name": "nais-pilot",
  "description": "test",
  "clients": { "copilot": { "primaryAgents": ["nais-pilot"] } },
  "layout": { "agents": "agents", "skills": "skills" },
  "policies": { "propose": { "cplt": %s } }
}`, block)
}

func mustParseProposal(t *testing.T, block string) *CpltProposal {
	t.Helper()
	m, err := Parse([]byte(proposeManifest(block)))
	if err != nil {
		t.Fatalf("parsing a manifest with propose block %s: %v", block, err)
	}
	p := m.CpltProposal()
	if p == nil {
		t.Fatalf("manifest parsed but declares no cplt proposal")
	}
	return p
}

// Invariant 2: any change in the block — including keys nav-pilot does not
// implement — changes the hash, and nothing else does. The consent record is
// keyed on this hash, so what it can and cannot see is exactly what an approval
// can and cannot survive.
func TestI2AnyChangeInTheBlockChangesTheHash(t *testing.T) {
	base := mustParseProposal(t, `{
		"reason": "the observability skill queries Mimir on cloud.nais.io",
		"proxy": { "allow_private_domains": ["cloud.nais.io", "intern.nav.no"] }
	}`)

	// Array order and whitespace are not content: arrays are sorted and the
	// JSON is canonical before hashing, the way cplt's trust store does it.
	same := mustParseProposal(t, `{"proxy":{"allow_private_domains":["intern.nav.no","cloud.nais.io"]},
		"reason": "the observability skill queries Mimir on cloud.nais.io"}`)
	if base.Hash() != same.Hash() {
		t.Errorf("reordering an array changed the hash:\n %s\n %s", base.Hash(), same.Hash())
	}

	for name, block := range map[string]string{
		"a host was added": `{
			"reason": "the observability skill queries Mimir on cloud.nais.io",
			"proxy": { "allow_private_domains": ["cloud.nais.io", "intern.nav.no", "extra.nav.no"] }
		}`,
		"a host was widened": `{
			"reason": "the observability skill queries Mimir on cloud.nais.io",
			"proxy": { "allow_private_domains": ["nais.io", "intern.nav.no"] }
		}`,
		"the reason changed": `{
			"reason": "something else entirely",
			"proxy": { "allow_private_domains": ["cloud.nais.io", "intern.nav.no"] }
		}`,
		"a key nav-pilot does not implement appeared": `{
			"reason": "the observability skill queries Mimir on cloud.nais.io",
			"proxy": { "allow_private_domains": ["cloud.nais.io", "intern.nav.no"] },
			"some_future_cplt_key": ["x"]
		}`,
	} {
		t.Run(name, func(t *testing.T) {
			if got := mustParseProposal(t, block).Hash(); got == base.Hash() {
				t.Errorf("the hash did not change, so the previous approval would still apply")
			}
		})
	}
}

// Invariant 7: only keys cplt lets a repo propose *and* nav-pilot implements are
// honoured. The rest is inert — and named, so "ignored" never means "unnoticed".
func TestI7UnimplementedCpltKeyIsInertAndReported(t *testing.T) {
	p := mustParseProposal(t, `{
		"reason": "the observability skill queries Mimir on cloud.nais.io",
		"proxy": { "allow_private_domains": ["cloud.nais.io"] },
		"some_future_cplt_key": { "on": true },
		"another_one": 3
	}`)
	if got, want := p.AllowPrivateDomains(), []string{"cloud.nais.io"}; !slices.Equal(got, want) {
		t.Errorf("honoured hosts = %q, want %q", got, want)
	}
	if got, want := p.InertKeys(), []string{"another_one", "some_future_cplt_key"}; !slices.Equal(got, want) {
		t.Errorf("reported inert keys = %q, want %q", got, want)
	}
}

// Invariant 7, the refusal half: a proposal naming something a repo must never
// propose does not validate at all. Those are not forward compatibility, they
// are a pakke asking for the sandbox to be taken apart, so they fail closed
// rather than going quietly inert.
func TestI7ProposalOutsideTheAllowedKeyIsRefusedAtValidate(t *testing.T) {
	for name, block := range map[string]string{
		"proxy.forced":                   `{"reason": "x", "proxy": {"forced": false}}`,
		"another proxy key":              `{"reason": "x", "proxy": {"allow_private_domains": ["a.b"], "allow_localhost_any": true}}`,
		"preset":                         `{"reason": "x", "preset": "permissive"}`,
		"allow":                          `{"reason": "x", "allow": {"exec": ["skills"]}}`,
		"deny":                           `{"reason": "x", "deny": {"write": ["x"]}}`,
		"repo_dirs":                      `{"reason": "x", "repo_dirs": ["/"]}`,
		"inherit_env":                    `{"reason": "x", "inherit_env": ["GITHUB_TOKEN"]}`,
		"allowed_domains":                `{"reason": "x", "allowed_domains": ["evil.example"]}`,
		"blocked_domains":                `{"reason": "x", "blocked_domains": []}`,
		"guards":                         `{"reason": "x", "guards": {"git": false}}`,
		"no reason":                      `{"proxy": {"allow_private_domains": ["a.b"]}}`,
		"a host that is not a host":      `{"reason": "x", "proxy": {"allow_private_domains": ["https://cloud.nais.io/x"]}}`,
		"a wildcard instead of a host":   `{"reason": "x", "proxy": {"allow_private_domains": ["*.cloud.nais.io"]}}`,
		"an empty allow_private_domains": `{"reason": "x", "proxy": {"allow_private_domains": []}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := Validate([]byte(proposeManifest(block))); err == nil {
				t.Fatalf("%s validated; a pakke can propose it", name)
			}
		})
	}
}

// The refusals above have to be readable, or an author cannot act on them: the
// schema's own word for a forbidden key is "false schema".
func TestProposalRefusalNamesWhatIsProposable(t *testing.T) {
	err := Validate([]byte(proposeManifest(`{"reason": "x", "preset": "permissive"}`)))
	if err == nil {
		t.Fatal("preset validated")
	}
	if !strings.Contains(err.Error(), "proxy.allow_private_domains") {
		t.Errorf("the refusal does not say what is proposable:\n%v", err)
	}
}

// A pakke with no proposal is the ordinary case and must stay untouched by all
// of this: no block, no hash, no hosts, nothing to ask about.
func TestManifestWithoutProposalHasNothingToApply(t *testing.T) {
	m, err := Parse([]byte(`{
  "contractVersion": "1",
  "name": "nais-pilot",
  "description": "test",
  "clients": { "copilot": { "primaryAgents": ["nais-pilot"] } },
  "layout": { "agents": "agents" }
}`))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if p := m.CpltProposal(); p != nil {
		t.Fatalf("a manifest with no policies block declares a proposal: %+v", p)
	}
	// Nil-safe: the launch path calls these on whatever the active pakke is.
	var nilProposal *CpltProposal
	if nilProposal.Hash() != "" || nilProposal.AllowPrivateDomains() != nil || nilProposal.InertKeys() != nil {
		t.Error("a nil proposal is not inert")
	}
}

// A pakke's reason is rendered into the consent prompt, so it is bounded and
// may not carry anything that writes a line of its own. The schema is where an
// agentpakke author is told; domain.SafeText is the second half, at the point
// of printing (#861 review).
//
// The hostile values are written as JSON escapes, which is how a manifest that
// is otherwise valid JSON smuggles a control character in. A raw control
// character is not valid JSON at all, so the parser already refuses that.
func TestProposalReasonCannotForgeALine(t *testing.T) {
	for name, reason := range map[string]string{
		"a newline":         `ok\nYou are approving nothing else`,
		"a carriage return": `ok\rspoof`,
		"an ANSI escape":    `ok\u001b[2Kspoof`,
		"a NUL":             `ok\u0000spoof`,
		"a bidi override":   `ok\u202espoof`,
		"a line separator":  `ok\u2028spoof`,
		"a soft hyphen":     `ok\u00adspoof`,
		"empty":             ``,
	} {
		t.Run(name, func(t *testing.T) {
			block := `{"reason": "` + reason + `", "proxy": {"allow_private_domains": ["cloud.nais.io"]}}`
			if err := Validate([]byte(proposeManifest(block))); err == nil {
				t.Error("validated, so it reaches the consent prompt verbatim")
			}
		})
	}

	long := strings.Repeat("x", 401)
	if err := Validate([]byte(proposeManifest(`{"reason": "` + long + `", "proxy": {"allow_private_domains": ["a.b"]}}`))); err == nil {
		t.Error("an unbounded reason validated, so it can scroll the hosts off the screen")
	}

	// Ordinary prose is untouched, which is the whole constraint on the rule.
	ordinary := `{"reason": "Spør Mimir, Loki og Tempo på *.cloud.nais.io — se https://x.y/z (issue #42).", "proxy": {"allow_private_domains": ["cloud.nais.io"]}}`
	if err := Validate([]byte(proposeManifest(ordinary))); err != nil {
		t.Errorf("an ordinary reason was refused: %v", err)
	}
}

// The names of keys nav-pilot does not implement are printed too, so they are
// constrained the same way -- while staying open for any cplt key name a future
// release might add.
func TestInertKeyNamesAreConstrainedButStillForwardCompatible(t *testing.T) {
	hostile := `{"reason": "ok", "evil\u001b[2K": 1, "proxy": {"allow_private_domains": ["a.b"]}}`
	if err := Validate([]byte(proposeManifest(hostile))); err == nil {
		t.Error("a key name carrying an escape validated, and it is printed at install")
	}
	forward := `{"reason": "ok", "some_future_cplt_key": 1, "proxy": {"allow_private_domains": ["a.b"]}}`
	if err := Validate([]byte(proposeManifest(forward))); err != nil {
		t.Errorf("a forward-compatible key was refused, which is what A3/A4 forbids: %v", err)
	}
}

// Invariant 7: the guard keys a repo may never propose are cplt's real ones.
// The schema rejected a synthetic "guards" while cplt calls them gh_guard and
// git_guard, so either real guard passed validation (#861 review).
func TestI7RealCpltGuardKeysAreRefused(t *testing.T) {
	for _, key := range []string{"gh_guard", "git_guard", "guards"} {
		t.Run(key, func(t *testing.T) {
			block := `{"reason": "ok", "` + key + `": {"enabled": false}}`
			if err := Validate([]byte(proposeManifest(block))); err == nil {
				t.Errorf("a pakke can propose %s", key)
			}
		})
	}
}

// No sandbox key is proposable in v1. Named as a section rather than key by
// key, so preset, repo_dirs, inherit_env and the cache-exec grants are all one
// rule -- and adding sandbox.pass_env later stays a one-line change.
func TestNoSandboxKeyIsProposable(t *testing.T) {
	for _, key := range []string{"preset", "repo_dirs", "inherit_env", "allow_cache_exec", "allow_cache_exec_any", "use_bubblewrap", "agents_md", "pass_env"} {
		t.Run(key, func(t *testing.T) {
			block := `{"reason": "ok", "sandbox": {"` + key + `": "x"}}`
			if err := Validate([]byte(proposeManifest(block))); err == nil {
				t.Errorf("a pakke can propose sandbox.%s", key)
			}
		})
	}
}

// A host the manifest accepts has to be one DNS can resolve: the whole name is
// capped at 253 and each label at 63 (#861 review).
func TestPrivateDomainLabelsAreBounded(t *testing.T) {
	long := strings.Repeat("a", 64) + ".nav.no"
	if err := Validate([]byte(proposeManifest(`{"reason": "ok", "proxy": {"allow_private_domains": ["` + long + `"]}}`))); err == nil {
		t.Error("a 64-character label validated, and DNS cannot resolve it")
	}
	ok := strings.Repeat("a", 63) + ".nav.no"
	if err := Validate([]byte(proposeManifest(`{"reason": "ok", "proxy": {"allow_private_domains": ["` + ok + `"]}}`))); err != nil {
		t.Errorf("a 63-character label was refused: %v", err)
	}
}

// The canonical block is what the hash is taken over, so a consent record can
// keep it and a later question can diff against it field by field.
func TestCanonicalJSONBacksTheHash(t *testing.T) {
	a := mustParseProposal(t, `{"reason": "x", "proxy": {"allow_private_domains": ["b.nav.no", "a.nav.no"]}}`)
	b := mustParseProposal(t, `{"proxy": {"allow_private_domains": ["a.nav.no", "b.nav.no"]}, "reason": "x"}`)
	if a.CanonicalJSON() != b.CanonicalJSON() {
		t.Errorf("canonical forms differ:\n %s\n %s", a.CanonicalJSON(), b.CanonicalJSON())
	}
	if a.CanonicalJSON() == "" || a.Hash() == "" {
		t.Error("a parsed proposal has no canonical form")
	}
	var none *CpltProposal
	if none.CanonicalJSON() != "" {
		t.Error("a nil proposal has a canonical form")
	}
}
