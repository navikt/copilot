package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

func TestHostGHProbeDecisionBoundary(t *testing.T) {
	isolatedConfig(t)
	dir := t.TempDir()
	path, record := filepath.Join(dir, "trusted-gh"), filepath.Join(dir, "calls")
	t.Setenv("AUTH_TEST_OUT", record)
	t.Setenv(cpltSandboxEnvVar, "")
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN", "COPILOT_GITHUB_TOKEN", "GH_HOST", "GH_REPO"} {
		t.Setenv(name, "polluted")
	}
	script := "#!/bin/sh\nfor name in GH_TOKEN GITHUB_TOKEN COPILOT_GITHUB_TOKEN GH_HOST GH_REPO; do\n eval 'present=${'\"$name\"'+yes}'\n [ -z \"$present\" ] || exit 9\ndone\nprintf '%s|%s\\n' \"$PWD\" \"$*\" > \"$AUTH_TEST_OUT\"\nprintf '%s\\n' 'Logged in to github.com account working (keyring)' 'Failed to log in to github.com account expired (keyring)'\nexit 1\n"
	if err := testhome.WriteExec(path, script); err != nil {
		t.Fatal(err)
	}
	if err := testhome.WriteExec(filepath.Join(dir, "gh"), "#!/bin/sh\nprintf shim > \"$AUTH_TEST_OUT.shim\"\nexit 0\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	binary, probe := hostGHBinary, ghAuthProbe
	t.Cleanup(func() { hostGHBinary, ghAuthProbe = binary, probe })
	ghAuthProbe = probeHostGHAuth
	hostGHBinary = func() string { return path }
	// Exercise the shared setup/doctor decision through the real probe.
	st, detail := ghAuthProbe()
	if st != ghAuthNone {
		t.Fatalf("status = %v, %s", st, detail)
	}
	var out bytes.Buffer
	reportGHAuth(&out, "", "copilot", "auto", st, detail)
	if !strings.Contains(out.String(), "does not start login") {
		t.Fatalf("missing manual recovery: %s", &out)
	}
	if !strings.Contains(out.String(), "Ambient-token authentication and sandbox push/PR authentication remain unverified") || strings.Contains(out.String(), "cannot push or open PRs") {
		t.Fatalf("cleared probe must not classify ambient authentication: %s", &out)
	}
	if !strings.Contains(out.String(), "Other stored accounts may still work") || strings.Contains(out.String(), "No working stored") || !strings.Contains(out.String(), "Native Copilot Keychain login may still work; launch will continue") || !strings.Contains(out.String(), hostGHStorageCheckCommand) {
		t.Fatalf("mixed account validity must not rule out working or native login: %s", &out)
	}
	home, _ := os.UserHomeDir()
	home, _ = filepath.EvalSymlinks(home)
	got, err := os.ReadFile(record)
	if err != nil || string(got) != home+"|auth status --hostname github.com\n" {
		t.Fatalf("host process boundary: %q, %v", got, err)
	}
	if _, err := os.Stat(record + ".shim"); !os.IsNotExist(err) {
		t.Fatalf("ambient project shim was executed: %v", err)
	}
	if os.Getenv("GH_TOKEN") != "polluted" {
		t.Fatal("parent credentials were modified")
	}
}

func TestFindSetupGHRejectsUntrustedSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	path := filepath.Join(outside, "gh")
	if err := testhome.WriteExec(path, "#!/bin/sh\nexit 0\n"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, filepath.Join(root, "gh")); err != nil {
		t.Fatal(err)
	}
	if got := findSetupGH([]string{root}, []string{root}); got != "" {
		t.Fatalf("escaped trusted root: %s", got)
	}
	path, _ = filepath.EvalSymlinks(path)
	if got := findSetupGH([]string{root}, []string{outside}); got != path {
		t.Fatalf("expected allowed symlink target %s, got %s", path, got)
	}
}

func TestHostGHProbeBoundedAndUnavailable(t *testing.T) {
	isolatedConfig(t)
	t.Setenv(cpltSandboxEnvVar, "")
	binary, probe := hostGHBinary, ghAuthProbe
	t.Cleanup(func() { hostGHBinary, ghAuthProbe = binary, probe })
	ghAuthProbe = probeHostGHAuth
	hostGHBinary = func() string { return "" }
	if st, _ := ghAuthProbe(); st != ghAuthMissing {
		t.Fatal("missing trusted binary must not use PATH")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := testhome.WriteExec(path, "#!/bin/sh\nexec /bin/sleep 30\n"); err != nil {
		t.Fatal(err)
	}
	hostGHBinary = func() string { return path }
	start := time.Now()
	if st, _ := ghAuthProbe(); st != ghAuthUnknown {
		t.Fatal("timeout must be unknown")
	}
	if elapsed := time.Since(start); elapsed > ghAuthTimeout+3*time.Second {
		t.Fatalf("recheck took %s", elapsed)
	}
	t.Setenv(cpltSandboxEnvVar, "1")
	hostGHBinary = func() string { t.Fatal("nested sandbox must not resolve host gh"); return "" }
	if st, _ := ghAuthProbe(); st != ghAuthUnknown {
		t.Fatal("nested sandbox must not claim host status")
	}
}

func TestHostGHRecoveryCommands(t *testing.T) {
	isolatedConfig(t)
	dir := t.TempDir()
	script := "#!/bin/sh\nfor name in GH_TOKEN GITHUB_TOKEN COPILOT_GITHUB_TOKEN GH_HOST GH_REPO; do\n eval 'present=${'\"$name\"'+yes}'\n [ -z \"$present\" ] || exit 9\ndone\n"
	for _, name := range []string{"gh", "nav-pilot"} {
		if err := testhome.WriteExec(filepath.Join(dir, name), script); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN", "COPILOT_GITHUB_TOKEN", "GH_HOST", "GH_REPO"} {
		t.Setenv(name, "polluted")
	}
	var out bytes.Buffer
	reportHostGHRecovery(&out, "env_only")
	text := out.String()
	change := strings.Index(text, "nav-pilot config set copilot_auth_mode auto")
	if change < strings.Index(text, hostGHStorageCheckCommand) || change > strings.Index(text, hostGHRestartCommand) {
		t.Fatalf("explicit mode choice must follow storage check and precede restart: %s", text)
	}
	for _, command := range []string{hostGHLoginCommand, hostGHStorageCheckCommand, hostGHRestartCommand} {
		if !strings.Contains(text, command) {
			t.Fatalf("missing command: %s", text)
		}
		if got, err := exec.Command("/bin/sh", "-c", command).CombinedOutput(); err != nil {
			t.Fatalf("command retained overrides: %v, %s", err, got)
		}
	}
	if os.Getenv("GH_TOKEN") != "polluted" {
		t.Fatal("commands changed the parent token")
	}
}
