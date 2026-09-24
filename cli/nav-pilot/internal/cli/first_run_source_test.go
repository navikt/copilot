package cli

import (
	"path/filepath"
	"testing"
)

// TestFirstRunSetupReceivesFlagSource is the other half of #813: the launch
// path runs first-run setup before anything reads the config file, so a first
// run with a custom source can only learn it from the command line. The
// pre-scan in run() is the only parser those arguments ever reach, and it used
// to drop --source entirely — so `nav-pilot --sync --source /custom/pakke`
// seeded the built-in default on a fresh machine.
func TestFirstRunSetupReceivesFlagSource(t *testing.T) {
	// A home and a config path of its own: the wizard runs only when no config
	// exists, and nothing here may touch the developer's real one.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(home, "config.toml"))
	t.Chdir(t.TempDir())

	origInteractive := isInteractive
	isInteractive = func() bool { return true }

	var got string
	origSetup := runConfigSetupFn
	runConfigSetupFn = func(flagSource string) error {
		got = flagSource
		return nil
	}

	// The --sync path launches a client once it has synced. A provider that is
	// not installed is what keeps this test from starting one.
	origProviderFor := providerFor
	providerFor = func(string) (Provider, error) { return failingProvider{unavailable: true}, nil }

	t.Cleanup(func() {
		isInteractive = origInteractive
		runConfigSetupFn = origSetup
		providerFor = origProviderFor
	})

	const flagSource = "/custom/pakke"
	if err := run([]string{"--sync", "--source", flagSource}); err != nil {
		t.Fatalf("run() error: %v", err)
	}
	if got != flagSource {
		t.Errorf("first-run setup got source %q, want the --source flag %q", got, flagSource)
	}
}
