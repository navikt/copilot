package cli

import (
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
	answer bool
	asked  int
}

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
		if f.asked != 1 || len(f.calls) != 2 || f.calls[1] != "set --local sandbox.allow_localhost_any "+tc.want {
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
