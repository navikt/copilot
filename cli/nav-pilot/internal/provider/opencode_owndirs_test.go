package provider

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// openCodeWildcard is OpenCode's util/wildcard.ts match: "*" is ".*", "?"
// is ".", anchored.
func openCodeWildcard(s, pattern string) bool {
	re := regexp.QuoteMeta(pattern)
	re = strings.ReplaceAll(strings.ReplaceAll(re, `\*`, ".*"), `\?`, ".")
	return regexp.MustCompile("^" + re + "$").MatchString(s)
}

// A session may read what nav-pilot installed in opencode's config dir
// without an external_directory request, which `opencode run` rejects
// (#1120), may not edit it, and the user's own choices stay.
func TestApplyOpenCodeOwnDirs(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	if err := os.Mkdir(filepath.Join(project, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(xdg, "opencode")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	instr := filepath.ToSlash(filepath.Join(cfgDir, "instructions"))

	type perms struct {
		Ext  map[string]string `json:"external_directory"`
		Edit map[string]string `json:"edit"`
	}
	parse := func(t *testing.T, env []string) (raw string, p perms) {
		for _, e := range env {
			if v, ok := strings.CutPrefix(e, "OPENCODE_CONFIG_CONTENT="); ok {
				raw = v
			}
		}
		var cfg struct {
			Permission perms `json:"permission"`
		}
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				t.Fatal(err)
			}
		}
		return raw, cfg.Permission
	}

	for _, tc := range []struct {
		name, global, project string
		env                   []string
		unchanged             bool
		extStar, editStar     string
	}{
		{name: "no user rule"},
		{name: "user asks", global: `{"permission": {"external_directory": "ask"}} // jsonc`, extStar: "ask"},
		{name: "user allows", global: `{"permission": {"external_directory": "allow", "edit": "ask"}}`, extStar: "allow", editStar: "ask"},
		{name: "user denies", global: `{"permission": {"external_directory": "deny"}}`, unchanged: true},
		{name: "user denies in object", global: `{"permission": {"external_directory": {"*": "deny", "/data/*": "allow"}}}`, unchanged: true},
		{name: "whole block denies", global: `{"permission": "deny"}`, unchanged: true},
		{name: "block star denies", global: `{"permission": {"*": "deny"}}`, unchanged: true},
		{name: "project denies", project: `{"permission": {"external_directory": "deny"}}`, unchanged: true},
		{name: "whole block asks", global: `{"permission": "ask"}`},
		{name: "later object clears string", global: `{"permission": {"external_directory": "ask"}}`, project: `{"permission": {"external_directory": {"/data/*": "allow"}}}`},
		{name: "staged dir and launch policy", env: []string{"OPENCODE_CONFIG_DIR=/stage/pakke", `OPENCODE_CONFIG_CONTENT={"share":"disabled"}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, f := range []struct{ dir, body string }{{cfgDir, tc.global}, {project, tc.project}} {
				path := filepath.Join(f.dir, "opencode.json")
				os.Remove(path)
				if f.body != "" {
					if err := os.WriteFile(path, []byte(f.body), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			env := applyOpenCodeOwnDirs(tc.env, project)
			raw, p := parse(t, env)
			if tc.unchanged {
				if strings.Contains(raw, "permission") {
					t.Fatalf("added permissions for a user who denies: %s", raw)
				}
				return
			}
			if p.Ext["*"] != tc.extStar || p.Edit["*"] != tc.editStar {
				t.Errorf(`"*" = %q/%q, want %q/%q: %s`, p.Ext["*"], p.Edit["*"], tc.extStar, tc.editStar, raw)
			}
			if tc.extStar != "" && !strings.Contains(raw, `"external_directory":{"*":`) {
				t.Errorf(`"*" is not the first external_directory rule: %s`, raw)
			}
			for _, d := range []string{instr, filepath.ToSlash(filepath.Join(cfgDir, "agents")), filepath.ToSlash(filepath.Join(cfgDir, "skills"))} {
				if p.Ext[d+"/*"] != "allow" {
					t.Errorf("no allow for %s: %s", d, raw)
				}
			}
			// What OpenCode asks: external_directory for the file's dir + "/*",
			// edit for the path relative to the worktree.
			var allowed, denied bool
			for pat, act := range p.Ext {
				if act == "allow" && openCodeWildcard(instr+"/nav/*", pat) {
					allowed = true
				}
			}
			rel, _ := filepath.Rel(project, filepath.Join(cfgDir, "instructions", "nav.md"))
			for pat, act := range p.Edit {
				if act == "deny" && openCodeWildcard(filepath.ToSlash(rel), pat) {
					denied = true
				}
			}
			if !allowed || !denied {
				t.Errorf("read allowed %v, edit of %s denied %v: %s", allowed, rel, denied, raw)
			}
			if strings.Contains(tc.name, "staged") && (p.Ext["/stage/pakke/instructions/*"] != "allow" || !strings.Contains(raw, `"share":"disabled"`)) {
				t.Errorf("staged dir or launch policy missing: %s", raw)
			}
		})
	}
}

// opencode 2's flat permissions list, last match winning across files: a
// user's v2 deny of external_directory keeps nav-pilot's allows out.
func TestUserPermissionReadsV2List(t *testing.T) {
	t.Cleanup(func() { versionCache.Delete("opencode") })
	deny := `{"permissions": [{"action": "external_directory", "resource": "*", "effect": "deny"}]}`
	for _, tc := range []struct {
		name, version string
		docs          []string
		want          bool
	}{
		{"v2 deny", "opencode v2.0.24\n", []string{deny}, true},
		{"v2 wildcard action", "opencode v2.0.24\n", []string{`{"permissions": [{"action": "*", "resource": "*", "effect": "deny"}]}`}, true},
		{"v2 later allow wins", "opencode v2.0.24\n", []string{deny, `{"permissions": [{"action": "external_directory", "resource": "*", "effect": "allow"}]}`}, false},
		{"v2 path rule is not wholesale", "opencode v2.0.24\n", []string{`{"permissions": [{"action": "external_directory", "resource": "/x/*", "effect": "deny"}]}`}, false},
		{"v2 other action", "opencode v2.0.24\n", []string{`{"permissions": [{"action": "edit", "resource": "*", "effect": "deny"}]}`}, false},
		{"v1 ignores the list", "opencode 1.17.0\n", []string{deny}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			versionCache.Store("opencode", versionAnswer{tc.version, nil, time.Hour})
			var docs [][]byte
			for _, d := range tc.docs {
				docs = append(docs, []byte(d))
			}
			if _, got := userPermission(docs, "external_directory"); got != tc.want {
				t.Errorf("deny = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheckOpenCode2Launch(t *testing.T) {
	t.Cleanup(func() { versionCache.Delete("opencode") })
	prevOS := hostOS
	hostOS = "darwin"
	t.Cleanup(func() { hostOS = prevOS })
	for _, c := range []struct {
		version, cplt string
		cpltErr       error
		args          []string
		ok            bool
	}{
		{"opencode v2.0.24\n", "cplt 2026.10.07-103638-7c04fce\n", nil, nil, false},
		{"opencode v2.0.24\n", "cplt 2026.10.08-060626-0000000\n", nil, nil, true},
		{"opencode v2.0.24\n", "cplt 2026.10.07-123313-5326be8\n", nil, []string{"run", "hi"}, true},
		{"opencode v2.0.24\n", "cplt 2026.10.06-120000-0d1d66d\n", nil, nil, false},
		{"opencode v2.0.24\n", "cplt dev\n", nil, nil, false},
		{"opencode v2.0.24\n", "", errors.New("timeout"), nil, false},
		{"opencode v2.0.24\n", okCplt, nil, nil, false},
		{"opencode v2.0.24\n", "cplt 2026.10.07-123313-5326be8\n", nil, []string{"--server", "http://x"}, false},
		{"opencode v2.0.24\n", "cplt 2026.10.07-123313-5326be8\n", nil, []string{"run", "--server=http://x", "hi"}, false},
		{"opencode v2.0.24\n", "cplt 2026.10.07-123313-5326be8\n", nil, []string{"attach", "http://x"}, false},
		{"opencode v2.0.24\n", "cplt 2026.10.07-123313-5326be8\n", nil, []string{"run", "attach"}, true},
		{"opencode v2.0.24\n", "cplt 2026.10.07-123313-5326be8\n", nil, []string{"run", "--", "--server", "x"}, true},
		{"opencode v2.0.24\n", "cplt 2026.10.07-123313-5326be8\n", nil, []string{"--standalone"}, false},
		{"opencode v2.0.24\n", "", errCpltNotFound, nil, true},
		{"opencode 1.17.0\n", "", errors.New("timeout"), []string{"attach", "--server", "x"}, true},
	} {
		versionCache.Store("opencode", versionAnswer{c.version, nil, time.Hour})
		stubProbes(t, c.cplt, c.cpltErr, "", nil)
		if err := checkOpenCode2Launch(c.args); (err == nil) != c.ok {
			t.Errorf("%q cplt %q %v: err = %v, want ok %v", c.version, c.cplt, c.args, err, c.ok)
		}
	}
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24\n", nil, time.Hour})
	stubProbes(t, "cplt 2026.10.06-120000-0d1d66d\n", nil, "", nil)
	if err := checkOpenCode2Launch(nil); !errors.Is(err, errCpltTooOld) {
		t.Errorf("old cplt: err = %v, want errCpltTooOld", err)
	}
	// cplt runs opencode 2 on macOS only; elsewhere nav-pilot says so first.
	stubProbes(t, "cplt 2026.10.07-123313-5326be8\n", nil, "", nil)
	hostOS = "linux"
	if err := checkOpenCode2Launch(nil); err == nil || !strings.Contains(err.Error(), "macOS only") {
		t.Errorf("linux: err = %v, want the macOS-only refusal", err)
	}
	versionCache.Store("opencode", versionAnswer{"1.18.35\n", nil, time.Hour})
	if err := checkOpenCode2Launch(nil); err != nil {
		t.Errorf("linux, opencode 1: err = %v, want nil", err)
	}
}
