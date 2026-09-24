package cli

import (
	"errors"
	"os/exec"
	"testing"
)

// failingProvider launches by returning launchErr. Only the methods the launch
// path calls are implemented; the embedded nil interface panics on any other.
type failingProvider struct {
	Provider
	launchErr error
}

func (failingProvider) ID() string          { return "pi" }
func (failingProvider) DisplayName() string { return "Pi" }
func (failingProvider) Available() bool     { return true }
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
