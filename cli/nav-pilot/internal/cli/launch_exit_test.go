package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// failingProvider launches by returning launchErr. Only the methods the launch
// path calls are implemented; the embedded nil interface panics on any other.
type failingProvider struct {
	Provider
	launchErr   error
	unavailable bool
}

func (failingProvider) ID() string          { return "pi" }
func (failingProvider) DisplayName() string { return "Pi" }
func (f failingProvider) Available() bool   { return !f.unavailable }
func (f failingProvider) Launch(ResolvedConfig) error {
	return f.launchErr
}

// A launch that fails must reach the exit code. offerLaunchCopilot printed the
// error and returned nothing, so `nav-pilot` exited 0 when the client never
// started or exited non-zero.
func TestOfferLaunchCopilotReturnsLaunchFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	origInteractive, origProviderFor := isInteractive, providerFor
	t.Cleanup(func() { isInteractive, providerFor = origInteractive, origProviderFor })
	isInteractive = func() bool { return true }

	clientExit := exec.Command("sh", "-c", "exit 7").Run()
	if clientExit == nil {
		t.Fatal("sh -c 'exit 7' did not fail")
	}

	for _, tc := range []struct {
		name      string
		launchErr error
		wantCode  int
	}{
		{"nav-pilot side failure exits 1", errors.New("the recorded local server exited because its generation thread died"), ExitError},
		{"client exit status passes through", clientExit, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			providerFor = func(string) (Provider, error) { return failingProvider{launchErr: tc.launchErr}, nil }
			err := offerLaunchCopilot(ResolvedConfig{Client: "pi", AutoLaunch: true})
			if !errors.Is(err, tc.launchErr) {
				t.Fatalf("offerLaunchCopilot = %v, want it to wrap %v", err, tc.launchErr)
			}
			if code := exitCodeFor(err); code != tc.wantCode {
				t.Errorf("exitCodeFor = %d, want %d", code, tc.wantCode)
			}
		})
	}

	// A client nav-pilot cannot resolve never launched, so it is a failure too.
	providerFor = func(string) (Provider, error) { return nil, errors.New(`unknown client "nope"`) }
	if code := exitCodeFor(offerLaunchCopilot(ResolvedConfig{Client: "nope", AutoLaunch: true})); code != ExitError {
		t.Errorf("unknown client: exitCodeFor = %d, want %d", code, ExitError)
	}

	// Skipping the launch is not a failure.
	providerFor = func(string) (Provider, error) { return failingProvider{launchErr: errors.New("launched")}, nil }
	if err := offerLaunchCopilot(ResolvedConfig{Client: "pi", AutoLaunch: false}); err != nil {
		t.Errorf("auto_launch = false: offerLaunchCopilot = %v, want nil", err)
	}
}

func TestExitCodeForSignalledChild(t *testing.T) {
	err := exec.Command("sh", "-c", "kill -TERM $$").Run()
	if err == nil {
		t.Fatal("sh did not report the signal")
	}
	if code := exitCodeFor(err); code != 128+15 {
		t.Errorf("exitCodeFor(SIGTERM child) = %d, want 143", code)
	}
}

// A prompt after "--" in a terminal, with the client missing, never ran: exit
// non-zero. Bare nav-pilot with the client missing only warns.
func TestOfferLaunchMissingClientWithPromptFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	origInteractive, origProviderFor := isInteractive, providerFor
	t.Cleanup(func() { isInteractive, providerFor = origInteractive, origProviderFor })
	isInteractive = func() bool { return true }
	providerFor = func(string) (Provider, error) { return failingProvider{unavailable: true}, nil }

	if code := exitCodeFor(offerLaunch(ResolvedConfig{Client: "pi", AutoLaunch: true, ExtraArgs: []string{"-p", "x"}, DashDash: true}, false)); code != ExitError {
		t.Errorf("with prompt: exitCodeFor = %d, want %d", code, ExitError)
	}
	if err := offerLaunch(ResolvedConfig{Client: "pi", AutoLaunch: true}, false); err != nil {
		t.Errorf("bare: offerLaunch = %v, want nil", err)
	}
}

// --project-dir names the repo a launch runs in, so its repo-scope install
// counts even when nav-pilot is started from somewhere else.
func TestNothingInstalledHonoursProjectDir(t *testing.T) {
	isolatedConfig(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !nothingInstalled(repo) {
		t.Fatal("nothingInstalled = false before any install")
	}
	state := ScopeRepo(repo).StatePath()
	if err := os.MkdirAll(filepath.Dir(state), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if nothingInstalled(repo) {
		t.Error("nothingInstalled = true with a repo-scope install in --project-dir")
	}
}
