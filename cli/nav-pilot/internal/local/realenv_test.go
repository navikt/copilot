//go:build realenv

package local

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestEnsureEnvAgainstTheRealToolchain provisions the environment for real: it
// downloads the pinned uv, creates the venv on the pinned interpreter, and
// installs the pinned mlx-lm and mlx.
//
// It exists because the rest of this package's tests deliberately run with no
// uv, no Python and no network, so they prove the logic and nothing about the
// platform. What actually breaks for a developer is pin rot: mlx ships macOS
// arm64 wheels for cp310 to cp312 only, and the three pinned versions are
// strings in source. When one stops resolving, EnsureEnv fails on a laptop and
// nowhere else.
//
// Guarded by a build tag so it never runs in the normal suite. It needs Apple
// Silicon, a network, and a few minutes. It does not download weights or start
// a server.
func TestEnsureEnvAgainstTheRealToolchain(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if err := EnsureEnv(ctx); err != nil {
		// A network failure and pin rot look nothing alike and need different
		// action, so say which this is. The first run of this test on a laptop
		// behind a firewall reported "check the pinned versions" for what was a
		// blocked connection, which is the wrong week of work.
		if isNetworkFailure(err) {
			t.Fatalf("EnsureEnv could not reach the network: %v\n\n"+
				"This says nothing about the pinned versions. Check connectivity, a "+
				"proxy, or a firewall prompt waiting for someone to approve it, then "+
				"run this again.", err)
		}
		t.Fatalf("EnsureEnv against the real toolchain: %v\n\n"+
			"One of the pinned versions no longer resolves, or mlx has dropped a wheel "+
			"for the pinned interpreter. Check uvVersion, pythonVersion, mlxLMVersion "+
			"and mlxVersion in runtime.go.", err)
	}

	// Provisioning twice must be cheap and must not fail: init is documented as
	// safe to re-run, and a developer who runs it again should not pay for it.
	if err := EnsureEnv(ctx); err != nil {
		t.Fatalf("EnsureEnv is not idempotent against a real environment: %v", err)
	}
}

// isNetworkFailure reports a failure to reach the network rather than a failure of
// what was fetched. Matched on the error text because the transport wraps these
// several layers down and the concrete types are not worth reaching for in a test.
func isNetworkFailure(err error) bool {
	text := err.Error()
	for _, s := range []string{
		"i/o timeout", "no such host", "connection refused",
		"network is unreachable", "TLS handshake timeout", "context deadline exceeded",
	} {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}
