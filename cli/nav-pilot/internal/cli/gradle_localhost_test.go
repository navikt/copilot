package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type gradleFixture struct {
	root   string
	calls  []string
	stored bool
	json   string // answer to `get --json`; "" means older cplt (exit 2)
	answer bool
	asked  int
}

var prevRunCpltConfig = runCpltConfig

func newGradleFixture(t *testing.T, files map[string]string) *gradleFixture {
	t.Helper()
	isolatedConfig(t)
	f := &gradleFixture{root: t.TempDir()}
	if err := exec.Command("git", "init", "-q", f.root).Run(); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(f.root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prevI, prevC, prevA, prevR, prevS := isInteractive, cpltInstalled, askTrustReview, runCpltConfig, cpltStampForGradle
	t.Cleanup(func() {
		isInteractive, cpltInstalled, askTrustReview, runCpltConfig, cpltStampForGradle = prevI, prevC, prevA, prevR, prevS
		sessionPrompted = false
	})
	sessionPrompted = false
	isInteractive = func() bool { return true }
	cpltInstalled = func() bool { return true }
	cpltStampForGradle = func() string { return cpltLocalFloor + "-abc" }
	askTrustReview = func(p string) bool { f.asked++; return f.answer }
	runCpltConfig = func(dir string, args ...string) (string, error) {
		f.calls = append(f.calls, strings.Join(args, " "))
		if args[0] == "get" && args[1] == "--json" {
			if f.json == "" {
				return "error: unexpected argument '--json'\n", errors.New("exit status 2")
			}
			return f.json, nil
		}
		if args[0] == "get" && !f.stored {
			return "false\n[cplt] (default, not set in config file)\n", nil
		}
		return "false\n[cplt] (local, this project only)\n", nil
	}
	return f
}

func TestIsGradleRepo(t *testing.T) {
	for _, m := range gradleMarkers {
		d := t.TempDir()
		os.WriteFile(filepath.Join(d, m), nil, 0o644)
		if !isGradleRepo(d) {
			t.Errorf("%s not detected", m)
		}
	}
	if isGradleRepo(t.TempDir()) {
		t.Error("empty dir detected as Gradle")
	}
}

func TestGradleLocalhostStoresAnswer(t *testing.T) {
	for _, tc := range []struct {
		answer bool
		want   string
	}{{true, "true"}, {false, "false"}} {
		f := newGradleFixture(t, map[string]string{"build.gradle.kts": ""})
		f.answer = tc.answer
		maybeGradleLocalhost(f.root)
		if f.asked != 1 || len(f.calls) != 3 || f.calls[2] != "set --local sandbox.allow_localhost_any "+tc.want {
			t.Errorf("answer %v: asked %d, calls %q", tc.answer, f.asked, f.calls)
		}
	}
}

func TestGradleLocalhostSkips(t *testing.T) {
	for name, setup := range map[string]func(*gradleFixture){
		"no tty":     func(*gradleFixture) { isInteractive = func() bool { return false } },
		"stored":     func(f *gradleFixture) { f.stored = true },
		"old cplt":   func(*gradleFixture) { cpltStampForGradle = func() string { return "2026.11.01-000000" } },
		"no cplt":    func(*gradleFixture) { cpltInstalled = func() bool { return false } },
		"not gradle": func(f *gradleFixture) { os.Remove(filepath.Join(f.root, "gradlew")) },
		"propose": func(f *gradleFixture) {
			os.WriteFile(filepath.Join(f.root, ".cplt.toml"), []byte("[propose]\nallow_localhost_any = true\n"), 0o644)
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newGradleFixture(t, map[string]string{"gradlew": ""})
			setup(f)
			maybeGradleLocalhost(f.root)
			if f.asked != 0 {
				t.Errorf("asked %d", f.asked)
			}
			for _, c := range f.calls {
				if strings.HasPrefix(c, "set") {
					t.Errorf("changed config: %q", c)
				}
			}
		})
	}
}

func TestGradleLocalhostJSON(t *testing.T) {
	const k = `"key":"sandbox.allow_localhost_any"`
	for _, tc := range []struct {
		name, json string
		stored     bool // the text fallback's answer
		ask        bool
		calls      int // cplt calls: JSON probe, text fallback if any, write if asked
	}{
		{"unset", `{"schema":1,` + k + `,"value":false,"source":"default","set":false}`, true, true, 2},
		{"set local", `{"schema":1,` + k + `,"value":false,"source":"local","set":true}`, false, false, 1},
		{"set by preset", `{"schema":1,` + k + `,"value":false,"source":"user","set":true,"preset":"strict"}`, false, false, 1},
		{"older cplt, text says default", "", false, true, 3},
		{"older cplt, text says stored", "", true, false, 2},
		{"schema 2 falls back", `{"schema":2,` + k + `,"set":false}`, true, false, 2},
		{"other key falls back", `{"schema":1,"key":"sandbox.quiet","set":false}`, true, false, 2},
		{"no set field falls back", `{"schema":1,` + k + `}`, true, false, 2},
		{"garbage falls back", `false`, true, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newGradleFixture(t, map[string]string{"gradlew": ""})
			f.json, f.stored = tc.json, tc.stored
			maybeGradleLocalhost(f.root)
			if (f.asked == 1) != tc.ask || len(f.calls) != tc.calls {
				t.Errorf("asked %d, want ask=%v; calls %q, want %d", f.asked, tc.ask, f.calls, tc.calls)
			}
		})
	}
}

// Crosses the exec seam with a fake cplt script: a non-zero exit, even with
// JSON on stdout, must keep the text check, and a JSON "set":true must stop the prompt.
func TestGradleLocalhostFakeCpltScript(t *testing.T) {
	for _, tc := range []struct {
		name, jsonBranch string
		ask              bool
	}{
		{"exit 3 keeps text check", `exit 3`, true},
		{"non-zero exit ignores JSON", `echo '{"schema":1,"key":"sandbox.allow_localhost_any","value":true,"source":"local","set":true}'; exit 1`, true},
		{"stderr warning, json on stdout", `echo '[cplt] warning: something' >&2; echo '{"schema":1,"key":"sandbox.allow_localhost_any","value":true,"source":"local","set":true}'`, false},
		{"json set", `echo '{"schema":1,"key":"sandbox.allow_localhost_any","value":true,"source":"local","set":true}'`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newGradleFixture(t, map[string]string{"gradlew": ""})
			runCpltConfig = prevRunCpltConfig
			dir := t.TempDir()
			log := filepath.Join(dir, "set.log")
			script := "#!/bin/sh\n" +
				"if [ \"$1 $2 $3\" = \"config get --json\" ]; then " + tc.jsonBranch + "; exit 0; fi\n" +
				"if [ \"$1 $2\" = \"config get\" ]; then echo false; echo '[cplt] (default, not set in config file)' >&2; exit 0; fi\n" +
				"echo \"$*\" >> " + log + "\n"
			if err := os.WriteFile(filepath.Join(dir, "cplt"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			maybeGradleLocalhost(f.root)
			if (f.asked == 1) != tc.ask {
				t.Fatalf("asked %d, want ask=%v", f.asked, tc.ask)
			}
			got, _ := os.ReadFile(log)
			if tc.ask != strings.Contains(string(got), "config set --local sandbox.allow_localhost_any false") {
				t.Errorf("set log %q", got)
			}
		})
	}
}
