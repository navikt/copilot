package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TokenSource describes how a Copilot token was obtained.
type TokenSource string

const (
	TokenSourceEnv   TokenSource = "env"    // GH_TOKEN / GITHUB_TOKEN / COPILOT_GITHUB_TOKEN in environment
	TokenSourceGHCLI TokenSource = "gh-cli" // extracted via `gh auth token`
)

// ExtractedToken holds a Copilot/GitHub token and records where it came from.
type ExtractedToken struct {
	Token  string
	Source TokenSource
}

// ghCLITokenCmd is the command used to extract a token via the gh CLI.
// Replaced in tests with a fake binary.
var ghCLITokenCmd = func(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", "github.com")
}

// extractGHEnvToken returns the token from GH_TOKEN, GITHUB_TOKEN, or
// COPILOT_GITHUB_TOKEN if non-empty.
func extractGHEnvToken() (string, bool) {
	if t := strings.TrimSpace(os.Getenv("GH_TOKEN")); t != "" {
		return t, true
	}
	if t := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); t != "" {
		return t, true
	}
	if t := strings.TrimSpace(os.Getenv("COPILOT_GITHUB_TOKEN")); t != "" {
		return t, true
	}
	return "", false
}

// stripAuthEnv returns a copy of env with GH_TOKEN, GITHUB_TOKEN, and
// COPILOT_GITHUB_TOKEN removed.
// Used to prevent the gh subprocess from silently inheriting ambient env tokens
// when the caller has chosen a non-env extraction mode (gh_only).
func stripAuthEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if hasEnvKey(e, "GH_TOKEN") ||
			hasEnvKey(e, "GITHUB_TOKEN") ||
			hasEnvKey(e, "COPILOT_GITHUB_TOKEN") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// extractGHCLIToken runs `gh auth token` and returns the token on success.
// This runs in the unsandboxed parent process so `gh` here is the real binary,
// not the cplt gh-wrapper.
//
// GH_TOKEN, GITHUB_TOKEN, and COPILOT_GITHUB_TOKEN are stripped from the
// subprocess environment so that gh reads from its credential store rather than
// silently falling back to an ambient env token. This matters for gh_only
// callers where the env-var path must not be used.
func extractGHCLIToken() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := ghCLITokenCmd(ctx)
	// Strip GH_TOKEN/GITHUB_TOKEN/COPILOT_GITHUB_TOKEN from the subprocess env
	// so gh reads from its credential store rather than silently picking up an
	// ambient env token.
	// If the factory left cmd.Env nil (real usage), fall back to os.Environ first.
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = stripAuthEnv(cmd.Env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh auth token failed: %w", err)
	}

	token := strings.TrimSpace(stdout.String())
	if token == "" {
		return "", errors.New("gh auth token returned empty output")
	}
	return token, nil
}

// ExtractCopilotToken attempts to obtain a GitHub/Copilot token using the
// strategy determined by authMode:
//
//   - "auto":          env → gh-cli (`gh auth token`)
//   - "env_only":      env only; error if not set
//   - "gh_only":       gh-cli only; error if extraction fails
//   - any other value: error
//
// Note: there is no separate direct Keychain reader; gh-cli extraction is the
// only non-env path.
//
// Returns the token and the source that succeeded, or an error if all attempts fail.
func ExtractCopilotToken(authMode string) (ExtractedToken, error) {
	switch authMode {
	case "env_only":
		if t, ok := extractGHEnvToken(); ok {
			return ExtractedToken{Token: t, Source: TokenSourceEnv}, nil
		}
		return ExtractedToken{}, errors.New("copilot_auth_mode=env_only: GH_TOKEN/GITHUB_TOKEN/COPILOT_GITHUB_TOKEN not set")

	case "gh_only":
		t, err := extractGHCLIToken()
		if err != nil {
			return ExtractedToken{}, fmt.Errorf("copilot_auth_mode=gh_only: gh-cli extraction failed: %w", err)
		}
		return ExtractedToken{Token: t, Source: TokenSourceGHCLI}, nil

	case "auto":
		if t, ok := extractGHEnvToken(); ok {
			return ExtractedToken{Token: t, Source: TokenSourceEnv}, nil
		}
		// gh auth token reads the macOS Keychain internally, so a single call
		// covers both gh-cli and Keychain sources on all platforms.
		t, err := extractGHCLIToken()
		if err == nil {
			return ExtractedToken{Token: t, Source: TokenSourceGHCLI}, nil
		}
		return ExtractedToken{}, fmt.Errorf("could not extract Copilot token: GH_TOKEN/GITHUB_TOKEN/COPILOT_GITHUB_TOKEN not set and gh auth token failed: %w", err)

	default:
		return ExtractedToken{}, fmt.Errorf("unknown copilot_auth_mode %q (allowed: auto, env_only, gh_only)", authMode)
	}
}
