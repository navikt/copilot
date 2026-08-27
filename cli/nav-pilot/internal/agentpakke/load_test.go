package agentpakke

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// grillmesterManifest is the reference agentpakke manifest from the PRD's
// worked example, adjusted to the decided design (no `collections` field; the
// manifest supersedes the collection model).
const grillmesterManifest = `{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "A Copilot agent team for clarified software delivery, design, and product work with portable progressive skills.",
  "owner": { "repo": "navikt/grillmester", "team": "Team eSyfo" },
  "clients": {
    "copilot": {
      "primaryAgents": ["grillmester", "barista", "designer", "doctor-who"],
      "compatibility": ">=1.0.79,<2",
      "defaultModel": "gpt-5.6-sol",
      "defaultContext": "full",
      "payloads": {
        "full": { "path": "plugin", "primaryAgents": ["grillmester", "barista", "designer", "doctor-who"] },
        "focused": { "path": "targets/copilot-cli-focused-v1", "primaryAgents": ["barista", "grill-inspektor"] }
      }
    },
    "opencode": {
      "primaryAgents": ["grillmester", "barista", "designer", "doctor-who"],
      "compatibility": ">=1.18.20,<2",
      "defaultModel": "inherit",
      "payloads": {
        "full": { "path": "targets/opencode-v1", "primaryAgents": ["grillmester", "barista", "designer", "doctor-who"] },
        "focused": { "path": "targets/opencode-v1-focused", "primaryAgents": ["barista", "grill-inspektor"] }
      }
    }
  },
  "provenance": {
    "base": { "repo": "navikt/copilot", "digest": "sha256:abc" },
    "overlays": [ { "component": "grillmester", "version": "0.3.0-rc.8" } ]
  },
  "profiles": { "dir": "profiles/opencode", "default": "hybrid" },
  "minNavPilotVersion": "2026.09.01-000000-0000000"
}`

// devVersion stands in for a development build, which is exempt from the
// minNavPilotVersion gate.
const devVersion = "dev"

func TestParseValidManifest(t *testing.T) {
	m, err := parse([]byte(grillmesterManifest), devVersion)
	if err != nil {
		t.Fatalf("parse(grillmester) = %v, want nil", err)
	}
	if m.Name != "grillmester" {
		t.Errorf("Name = %q, want %q", m.Name, "grillmester")
	}
	if got := m.ClientIDs(); len(got) != 2 || got[0] != "copilot" || got[1] != "opencode" {
		t.Errorf("ClientIDs() = %v, want [copilot opencode]", got)
	}
	if got := m.Tier("copilot"); got != TierPayload {
		t.Errorf("Tier(copilot) = %d, want %d", got, TierPayload)
	}
	if !m.IsPrimaryAgent("opencode", "barista") {
		t.Error("IsPrimaryAgent(opencode, barista) = false, want true")
	}
	if m.IsPrimaryAgent("opencode", "kokk") {
		t.Error("IsPrimaryAgent(opencode, kokk) = true, want false")
	}
	if got := m.DefaultModel("opencode"); got != InheritModel {
		t.Errorf("DefaultModel(opencode) = %q, want %q", got, InheritModel)
	}
	// opencode declares no defaultContext, so it falls back to "full".
	if got := m.DefaultContext("opencode"); got != DefaultContext {
		t.Errorf("DefaultContext(opencode) = %q, want %q", got, DefaultContext)
	}
	// The roster is read from the payload, not the client entry: the focused
	// payload ships only barista and grill-inspektor (#437, comment
	// 5437575432), so its default persona is barista, not grillmester.
	if got := m.PayloadPrimaryAgents("opencode", "focused"); !reflect.DeepEqual(got, []string{"barista", "grill-inspektor"}) {
		t.Errorf("PayloadPrimaryAgents(opencode, focused) = %v, want [barista grill-inspektor]", got)
	}
	if got := m.PayloadPrimaryAgents("opencode", "full"); len(got) == 0 || got[0] != "grillmester" {
		t.Errorf("PayloadPrimaryAgents(opencode, full) = %v, want grillmester first", got)
	}
	// No fallback: an undeclared context has no roster at all, even though the
	// client entry carries one.
	if got := m.PayloadPrimaryAgents("opencode", "nonesuch"); got != nil {
		t.Errorf("PayloadPrimaryAgents(opencode, nonesuch) = %v, want nil (no fallback to the client entry)", got)
	}
	if got, ok := m.PayloadManifestPath("opencode", "focused"); !ok || got != "targets/opencode-v1-focused/manifest.json" {
		t.Errorf("PayloadManifestPath(opencode, focused) = %q, %v; want conventional <path>/manifest.json", got, ok)
	}
	if m.Provenance == nil || m.Provenance.Base == nil || m.Provenance.Base.Repo != "navikt/copilot" {
		t.Errorf("Provenance.Base = %+v, want navikt/copilot", m.Provenance)
	}
	if m.Profiles == nil || m.Profiles.Default != "hybrid" {
		t.Errorf("Profiles = %+v, want default hybrid", m.Profiles)
	}
}

func TestParseIgnoresUnknownConstructs(t *testing.T) {
	tests := []struct {
		name  string
		patch func(map[string]any)
		check func(t *testing.T, m *Manifest)
	}{
		{
			name: "unknown client key",
			patch: func(doc map[string]any) {
				clients := doc["clients"].(map[string]any)
				clients["claude-code"] = map[string]any{"primaryAgents": []any{"grillmester"}}
			},
			check: func(t *testing.T, m *Manifest) {
				if _, ok := m.Client("claude-code"); !ok {
					t.Error("unknown client key was dropped, want it retained for forward compatibility")
				}
			},
		},
		{
			name: "unknown context key",
			patch: func(doc map[string]any) {
				clients := doc["clients"].(map[string]any)
				oc := clients["opencode"].(map[string]any)
				oc["payloads"].(map[string]any)["tiny"] = map[string]any{
					"path": "targets/tiny", "primaryAgents": []any{"barista"},
				}
			},
			check: func(t *testing.T, m *Manifest) {
				if _, ok := m.Payload("opencode", "tiny"); !ok {
					t.Error("unknown context key was dropped, want it retained")
				}
			},
		},
		{
			name: "unknown top-level field",
			patch: func(doc map[string]any) {
				doc["experimentalThing"] = map[string]any{"enabled": true}
			},
			check: func(t *testing.T, m *Manifest) {
				if m.Name != "grillmester" {
					t.Errorf("Name = %q after unknown field, want grillmester", m.Name)
				}
			},
		},
		{
			name: "unknown field inside a client entry",
			patch: func(doc map[string]any) {
				clients := doc["clients"].(map[string]any)
				clients["opencode"].(map[string]any)["minCpltVersion"] = "1.2.3"
			},
			check: func(t *testing.T, m *Manifest) {
				if m.Tier("opencode") != TierPayload {
					t.Error("unknown client field changed the derived tier")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := parse(patchManifest(t, grillmesterManifest, tt.patch), devVersion)
			if err != nil {
				t.Fatalf("parse = %v, want nil (unknown constructs must be ignored, not rejected)", err)
			}
			tt.check(t, m)
		})
	}
}

func TestParseRejectsMalformedKnownConstructs(t *testing.T) {
	tests := []struct {
		name     string
		patch    func(map[string]any)
		wantErrs []string
	}{
		{
			name: "unsupported contractVersion",
			patch: func(doc map[string]any) {
				doc["contractVersion"] = "2"
			},
			wantErrs: []string{"contractVersion", "supported"},
		},
		{
			name: "contractVersion of the wrong type",
			patch: func(doc map[string]any) {
				doc["contractVersion"] = 1
			},
			wantErrs: []string{"contractVersion"},
		},
		{
			// A Tier 1 entry (no payloads) is the unit that carries the
			// agents, so its own roster is required.
			name: "tier 1 entry without primaryAgents",
			patch: func(doc map[string]any) {
				doc["clients"].(map[string]any)["opencode"] = map[string]any{}
				doc["layout"] = map[string]any{"agents": "agents", "skills": "skills"}
			},
			wantErrs: []string{"clients.opencode", "primaryAgents", "Tier 1"},
		},
		{
			name: "empty primaryAgents",
			patch: func(doc map[string]any) {
				doc["clients"].(map[string]any)["opencode"] = map[string]any{"primaryAgents": []any{}}
				doc["layout"] = map[string]any{"agents": "agents", "skills": "skills"}
			},
			wantErrs: []string{"primaryAgents"},
		},
		{
			// A Tier 2 payload is the unit that carries the agents, so each
			// payload's roster is required — the reference manifest at
			// 3573b93cc8b7568516117263562d073cae9ee7fc fails exactly here.
			name: "tier 2 payload without primaryAgents",
			patch: func(doc map[string]any) {
				oc := doc["clients"].(map[string]any)["opencode"].(map[string]any)
				delete(oc["payloads"].(map[string]any)["focused"].(map[string]any), "primaryAgents")
			},
			wantErrs: []string{"clients.opencode.payloads.focused", "primaryAgents", "default persona"},
		},
		{
			name: "tier 2 payload with an empty primaryAgents",
			patch: func(doc map[string]any) {
				oc := doc["clients"].(map[string]any)["opencode"].(map[string]any)
				oc["payloads"].(map[string]any)["focused"].(map[string]any)["primaryAgents"] = []any{}
			},
			wantErrs: []string{"clients.opencode.payloads.focused.primaryAgents", "minItems"},
		},
		{
			name: "no clients at all",
			patch: func(doc map[string]any) {
				doc["clients"] = map[string]any{}
			},
			wantErrs: []string{"clients"},
		},
		{
			name: "missing name",
			patch: func(doc map[string]any) {
				delete(doc, "name")
			},
			wantErrs: []string{"name"},
		},
		{
			name: "name is not an identifier",
			patch: func(doc map[string]any) {
				doc["name"] = "Grill Mester"
			},
			wantErrs: []string{"name", "^[a-z]"},
		},
		{
			name: "payload without path",
			patch: func(doc map[string]any) {
				oc := doc["clients"].(map[string]any)["opencode"].(map[string]any)
				oc["payloads"].(map[string]any)["full"] = map[string]any{"primaryAgents": []any{"barista"}}
			},
			wantErrs: []string{"payloads", "path"},
		},
		{
			name: "absolute payload path",
			patch: func(doc map[string]any) {
				oc := doc["clients"].(map[string]any)["opencode"].(map[string]any)
				oc["payloads"].(map[string]any)["full"] = map[string]any{"path": "/etc/passwd", "primaryAgents": []any{"barista"}}
			},
			wantErrs: []string{"clients.opencode.payloads.full.path", "absolute", "relative"},
		},
		{
			name: "escaping payload path",
			patch: func(doc map[string]any) {
				oc := doc["clients"].(map[string]any)["opencode"].(map[string]any)
				oc["payloads"].(map[string]any)["full"] = map[string]any{"path": "../../elsewhere", "primaryAgents": []any{"barista"}}
			},
			wantErrs: []string{"clients.opencode.payloads.full.path", "escapes"},
		},
		{
			name: "escaping payload manifest override",
			patch: func(doc map[string]any) {
				oc := doc["clients"].(map[string]any)["opencode"].(map[string]any)
				oc["payloads"].(map[string]any)["full"] = map[string]any{
					"path":          "targets/opencode-v1",
					"manifest":      "../secrets/manifest.json",
					"primaryAgents": []any{"barista"},
				}
			},
			wantErrs: []string{"clients.opencode.payloads.full.manifest", "escapes"},
		},
		{
			name: "tier 1 client without layout",
			patch: func(doc map[string]any) {
				clients := doc["clients"].(map[string]any)
				clients["copilot"] = map[string]any{"primaryAgents": []any{"grillmester"}}
			},
			wantErrs: []string{"copilot", "Tier 1", "layout"},
		},
		{
			name: "layout without skills",
			patch: func(doc map[string]any) {
				doc["layout"] = map[string]any{"agents": "plugin/agents"}
			},
			wantErrs: []string{"layout", "skills"},
		},
		{
			name: "absolute layout path",
			patch: func(doc map[string]any) {
				doc["layout"] = map[string]any{"agents": "/srv/agents", "skills": "plugin/skills"}
			},
			wantErrs: []string{"layout.agents", "absolute"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(patchManifest(t, grillmesterManifest, tt.patch), devVersion)
			if err == nil {
				t.Fatal("parse = nil, want an error (malformed known constructs must fail closed)")
			}
			for _, want := range tt.wantErrs {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

func TestParseRejectsInvalidJSON(t *testing.T) {
	_, err := parse([]byte("{not json"), devVersion)
	if err == nil {
		t.Fatal("parse(invalid json) = nil, want error")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("error %q should say the manifest is not valid JSON", err)
	}
}

func TestMinNavPilotVersion(t *testing.T) {
	tests := []struct {
		name     string
		running  string
		required string
		wantErr  bool
	}{
		{"running binary is newer", "2026.10.01-120000-abc1234", "2026.09.01-000000-0000000", false},
		{"running binary is exactly the minimum", "2026.09.01-000000-0000000", "2026.09.01-000000-0000000", false},
		{"running binary is older", "2026.08.01-120000-abc1234", "2026.09.01-000000-0000000", true},
		{"same day, older build time", "2026.09.01-000000-abc1234", "2026.09.01-120000-0000000", true},
		{"dev build is exempt", "dev", "2099.01.01-000000-0000000", false},
		{"unset version is exempt", "", "2099.01.01-000000-0000000", false},
		{"no minimum declared", "2020.01.01-000000-abc1234", "", false},
		{"minimum without a build sha still compares", "2026.10.01-120000-abc1234", "2026.09.01-000000", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := patchManifest(t, grillmesterManifest, func(doc map[string]any) {
				if tt.required == "" {
					delete(doc, "minNavPilotVersion")
					return
				}
				doc["minNavPilotVersion"] = tt.required
			})
			_, err := parse(data, tt.running)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parse = nil, want a version error")
				}
				for _, want := range []string{tt.required, tt.running, "nav-pilot update"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not mention %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("parse = %v, want nil", err)
			}
		})
	}
}

// TestMinNavPilotVersionMustBeAReleaseVersion covers the fail-closed half of
// the gate: a minimum nav-pilot cannot compare (a bare date, a word) must be an
// error, not a silently disabled check. Both layers enforce it — the published
// schema, so an agentpakke's own CI lint catches it, and the semantic pass, so
// a manifest reaching this binary another way still fails.
func TestMinNavPilotVersionMustBeAReleaseVersion(t *testing.T) {
	malformed := []string{
		"2026.09.01",              // date only: compares older than every release
		"2026.09.01-",             // truncated
		"whenever",                // not a version at all
		"v2026.09.01-120000-abc1", // tagged form
		" 2026.09.01-120000",      // padded
	}

	for _, required := range malformed {
		t.Run("schema/"+required, func(t *testing.T) {
			data := patchManifest(t, grillmesterManifest, func(doc map[string]any) {
				doc["minNavPilotVersion"] = required
			})
			err := Validate(data)
			if err == nil {
				t.Fatalf("Validate with minNavPilotVersion %q = nil, want a schema violation", required)
			}
			if !strings.Contains(err.Error(), "minNavPilotVersion") {
				t.Errorf("error %q does not name the offending field", err)
			}
		})

		t.Run("semantics/"+required, func(t *testing.T) {
			// Bypass the schema pass: the semantic rule must stand on its own.
			m := &Manifest{Name: "grillmester", MinNavPilotVersion: required}
			err := m.checkMinVersion("2099.01.01-000000-abc1234")
			if err == nil {
				t.Fatalf("checkMinVersion with %q = nil, want a fail-closed error", required)
			}
			if !strings.Contains(err.Error(), "YYYY.MM.DD-HHMMSS") {
				t.Errorf("error %q does not say what the format is", err)
			}
		})

		t.Run("dev-build/"+required, func(t *testing.T) {
			// The dev-build exemption applies to the *running* version, never to
			// a malformed declaration.
			m := &Manifest{Name: "grillmester", MinNavPilotVersion: required}
			if err := m.checkMinVersion(devVersion); err == nil {
				t.Errorf("checkMinVersion(%q) on a dev build = nil, want the format error", required)
			}
		})
	}

	for _, required := range []string{"2026.09.01-120000", "2026.09.01-120000-a1b2c3d", "2026.09.01-120000-a1b2c3d4e5f6"} {
		m := &Manifest{Name: "grillmester", MinNavPilotVersion: required}
		if err := m.checkMinVersion("2099.01.01-000000-abc1234"); err != nil {
			t.Errorf("checkMinVersion with the well-formed %q = %v, want nil", required, err)
		}
	}
}

func TestSetVersion(t *testing.T) {
	original := cliVersion
	t.Cleanup(func() { cliVersion = original })

	SetVersion("2020.01.01-000000-abc1234")
	if err := Validate([]byte(grillmesterManifest)); err == nil {
		t.Fatal("Validate = nil with an old running version, want a minNavPilotVersion error")
	}
	SetVersion("2099.01.01-000000-abc1234")
	if err := Validate([]byte(grillmesterManifest)); err != nil {
		t.Fatalf("Validate = %v with a new running version, want nil", err)
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, grillmesterManifest)

	m, err := Load(root)
	if err != nil {
		t.Fatalf("Load = %v, want nil", err)
	}
	if m.Name != "grillmester" {
		t.Errorf("Name = %q, want grillmester", m.Name)
	}
}

func TestLoadNoManifestFallback(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
	}{
		{
			name:  "empty source checkout",
			setup: func(t *testing.T) string { return t.TempDir() },
		},
		{
			name: "legacy layout without .nav-pilot",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				mkdirAll(t, filepath.Join(root, "agents"))
				mkdirAll(t, filepath.Join(root, "skills"))
				return root
			},
		},
		{
			name: ".nav-pilot dir without a manifest",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				mkdirAll(t, filepath.Join(root, ManifestDir))
				return root
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.setup(t))
			if !errors.Is(err, ErrNoManifest) {
				t.Fatalf("Load = %v, want an error matching ErrNoManifest so callers fall back to the legacy adapter", err)
			}
		})
	}
}

func TestLoadInvalidManifestFailsClosed(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, `{"contractVersion": "9", "name": "x", "description": "d", "clients": {"opencode": {"primaryAgents": ["x"]}}, "layout": {"agents": "a", "skills": "s"}}`)

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load = nil, want an error")
	}
	if errors.Is(err, ErrNoManifest) {
		t.Fatal("an existing but invalid manifest must not fall back to the legacy adapter")
	}
	if !strings.Contains(err.Error(), ManifestFile) {
		t.Errorf("error %q should name the offending file", err)
	}
}

// --- ValidateSource ---

func TestValidateSourceTier2(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, grillmesterManifest)
	for _, dir := range []string{
		"plugin", "targets/copilot-cli-focused-v1",
		"targets/opencode-v1", "targets/opencode-v1-focused",
	} {
		mkdirAll(t, filepath.Join(root, filepath.FromSlash(dir)))
		writeFile(t, filepath.Join(root, filepath.FromSlash(dir), PayloadManifestFile), `{"schemaVersion":1,"files":{}}`)
	}
	mkdirAll(t, filepath.Join(root, "profiles", "opencode"))
	writeFile(t, filepath.Join(root, "profiles", "opencode", "hybrid.json"), `{}`)

	if errs := ValidateSource(root); len(errs) != 0 {
		t.Fatalf("ValidateSource = %v, want no violations", errs)
	}
}

func TestValidateSourceReportsEveryViolation(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, grillmesterManifest)
	// plugin/ exists but ships no payload manifest — grillmester's known A7 gap.
	mkdirAll(t, filepath.Join(root, "plugin"))
	// The other payload trees are missing entirely.

	errs := ValidateSource(root)
	if len(errs) < 2 {
		t.Fatalf("ValidateSource returned %d errors, want every violation reported: %v", len(errs), errs)
	}
	joined := joinErrs(errs)
	for _, want := range []string{
		"clients.copilot.payloads.full",
		"payload manifest",
		"targets/opencode-v1",
		"profiles.dir",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("violations %q do not mention %q", joined, want)
		}
	}
}

func TestValidateSourceTier1(t *testing.T) {
	manifest := `{
  "contractVersion": "1",
  "name": "eksempel",
  "description": "Tier 1 agentpakke",
  "clients": { "opencode": { "primaryAgents": ["eksempel"] } },
  "layout": { "agents": "agents", "skills": "skills" }
}`

	t.Run("conforming", func(t *testing.T) {
		root := t.TempDir()
		writeManifest(t, root, manifest)
		mkdirAll(t, filepath.Join(root, "agents"))
		mkdirAll(t, filepath.Join(root, "skills"))
		writeFile(t, filepath.Join(root, "agents", "eksempel.agent.md"),
			"---\nname: eksempel\ndescription: Et eksempel\n---\n\nBody\n")

		if errs := ValidateSource(root); len(errs) != 0 {
			t.Fatalf("ValidateSource = %v, want no violations", errs)
		}
	})

	t.Run("missing layout dir", func(t *testing.T) {
		root := t.TempDir()
		writeManifest(t, root, manifest)
		mkdirAll(t, filepath.Join(root, "agents"))
		writeFile(t, filepath.Join(root, "agents", "eksempel.agent.md"), "---\nname: eksempel\n---\n")

		errs := ValidateSource(root)
		if joined := joinErrs(errs); !strings.Contains(joined, "layout.skills") {
			t.Fatalf("violations %q should name the missing layout.skills directory", joined)
		}
	})

	t.Run("agent without frontmatter", func(t *testing.T) {
		root := t.TempDir()
		writeManifest(t, root, manifest)
		mkdirAll(t, filepath.Join(root, "agents"))
		mkdirAll(t, filepath.Join(root, "skills"))
		writeFile(t, filepath.Join(root, "agents", "eksempel.agent.md"), "# No frontmatter here\n")

		errs := ValidateSource(root)
		if joined := joinErrs(errs); !strings.Contains(joined, "frontmatter") {
			t.Fatalf("violations %q should flag the missing frontmatter", joined)
		}
	})

	t.Run("agents dir with no agent files", func(t *testing.T) {
		root := t.TempDir()
		writeManifest(t, root, manifest)
		mkdirAll(t, filepath.Join(root, "agents"))
		mkdirAll(t, filepath.Join(root, "skills"))

		errs := ValidateSource(root)
		if joined := joinErrs(errs); !strings.Contains(joined, agentFileSuffix) {
			t.Fatalf("violations %q should flag the empty agents directory", joined)
		}
	})
}

func TestValidateSourceNoManifest(t *testing.T) {
	errs := ValidateSource(t.TempDir())
	if len(errs) != 1 || !errors.Is(errs[0], ErrNoManifest) {
		t.Fatalf("ValidateSource = %v, want a single ErrNoManifest", errs)
	}
}

// --- helpers ---

// patchManifest decodes a manifest, applies a mutation, and re-encodes it, so
// each test case states only what it changes.
func patchManifest(t *testing.T, base string, patch func(map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(base), &doc); err != nil {
		t.Fatalf("decoding base manifest: %v", err)
	}
	patch(doc)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encoding patched manifest: %v", err)
	}
	return data
}

func writeManifest(t *testing.T, root, content string) {
	t.Helper()
	mkdirAll(t, filepath.Join(root, ManifestDir))
	writeFile(t, filepath.Join(root, ManifestDir, ManifestFile), content)
}

func mkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func joinErrs(errs []error) string {
	msgs := make([]string, 0, len(errs))
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "\n")
}
