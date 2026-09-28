package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clientTestHome gives the test a config path of its own and a terminal.
func clientTestHome(t *testing.T, interactive bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)
	orig := isInteractive
	isInteractive = func() bool { return interactive }
	t.Cleanup(func() { isInteractive = orig; sessionPrompted = false })
	sessionPrompted = false
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func TestRecordEffectiveClient(t *testing.T) {
	t.Run("no config file", func(t *testing.T) {
		path := clientTestHome(t, true)
		// The wizard writes the file, client included; a skipped wizard
		// must come back next run, so nothing is created here.
		recordEffectiveClient()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("config created: %v", err)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), "seen-client-recorded")); !os.IsNotExist(err) {
			t.Fatal("marker set with no file")
		}
	})

	t.Run("keeps comments and layout, and is done once", func(t *testing.T) {
		path := clientTestHome(t, true)
		orig := "# mine\nversion = 1\nmodel = \"claude-opus-4.8\" # pinned\n\n[extra]\nx = 1\n"
		if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
			t.Fatal(err)
		}
		recordEffectiveClient()
		want := "# mine\nversion = 1\nmodel = \"claude-opus-4.8\" # pinned\nclient = \"copilot\"\n\n[extra]\nx = 1\n"
		if got := readFile(t, path); got != want {
			t.Fatalf("config = %q, want %q", got, want)
		}
		// config unset client afterwards is the user's choice: not undone.
		if err := updateConfigKey("client", ""); err != nil {
			t.Fatal(err)
		}
		recordEffectiveClient()
		if got := readFile(t, path); strings.Contains(got, "client") {
			t.Fatalf("client recorded twice: %q", got)
		}
	})

	t.Run("an explicit client is left alone", func(t *testing.T) {
		path := clientTestHome(t, true)
		orig := "version = 1\nclient = \"opencode\"\n"
		if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
			t.Fatal(err)
		}
		recordEffectiveClient()
		if got := readFile(t, path); got != orig {
			t.Fatalf("config = %q", got)
		}
	})

	t.Run("no terminal writes nothing", func(t *testing.T) {
		path := clientTestHome(t, false)
		recordEffectiveClient()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("config written without a terminal: %v", err)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), "seen-client-recorded")); !os.IsNotExist(err) {
			t.Fatal("marker set without a terminal")
		}
	})
}

func TestClientNudge(t *testing.T) {
	setup := func(t *testing.T, cfg string) {
		t.Helper()
		path := clientTestHome(t, true)
		orig := providerFor
		providerFor = func(string) (Provider, error) { return failingProvider{}, nil }
		t.Cleanup(func() { providerFor = orig })
		t.Setenv("DO_NOT_TRACK", "")
		t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "true")
		if err := os.WriteFile(path, []byte("version = 1\n"+cfg), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	nudge := func(client string) string {
		return captureStderr(func() { maybeClientNudge(client) })
	}

	t.Run("once, for copilot with local models on", func(t *testing.T) {
		setup(t, "client = \"copilot\"\nlocal_enabled = true\n")
		out := nudge("")
		if !strings.Contains(out, "nav-pilot config set client opencode") || !strings.Contains(out, "/nav-pilot/klienter") {
			t.Fatalf("nudge = %q", out)
		}
		if strings.Count(strings.TrimSpace(out), "\n") != 0 {
			t.Fatalf("nudge is more than one line: %q", out)
		}
		sessionPrompted = false // a new session
		if out := nudge(""); out != "" {
			t.Fatalf("shown twice: %q", out)
		}
	})

	t.Run("not after another prompt, and not used up by it", func(t *testing.T) {
		setup(t, "local_enabled = true\n")
		sessionPrompted = true
		if out := nudge(""); out != "" {
			t.Fatalf("second prompt in one session: %q", out)
		}
		sessionPrompted = false
		if out := nudge(""); out == "" {
			t.Fatal("a session that skipped it used it up")
		}
	})

	for name, cfg := range map[string]string{
		"local off":     "",
		"opencode":      "client = \"opencode\"\nlocal_enabled = true\n",
		"surveys false": "local_enabled = true\nsurveys = false\n",
	} {
		t.Run("not with "+name, func(t *testing.T) {
			setup(t, cfg)
			if out := nudge(""); out != "" {
				t.Fatalf("nudge = %q", out)
			}
		})
	}

	t.Run("not for --client opencode", func(t *testing.T) {
		setup(t, "local_enabled = true\n")
		if out := nudge("opencode"); out != "" {
			t.Fatalf("nudge = %q", out)
		}
	})

	t.Run("not without a terminal", func(t *testing.T) {
		setup(t, "local_enabled = true\n")
		isInteractive = func() bool { return false }
		if out := nudge(""); out != "" {
			t.Fatalf("nudge = %q", out)
		}
	})

	t.Run("not without opencode installed", func(t *testing.T) {
		setup(t, "local_enabled = true\n")
		providerFor = func(string) (Provider, error) { return failingProvider{unavailable: true}, nil }
		if out := nudge(""); out != "" {
			t.Fatalf("nudge = %q", out)
		}
	})

	t.Run("not with telemetry opted out", func(t *testing.T) {
		setup(t, "local_enabled = true\n")
		t.Setenv("DO_NOT_TRACK", "1")
		if out := nudge(""); out != "" {
			t.Fatalf("nudge = %q", out)
		}
	})
}
