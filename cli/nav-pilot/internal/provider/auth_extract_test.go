package provider

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// fakeCmd returns an *exec.Cmd that prints output and exits with the given code.
func fakeCmd(t *testing.T, output string, exitCode int) func(context.Context) *exec.Cmd {
	t.Helper()
	code := strconv.Itoa(exitCode)

	return func(ctx context.Context) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestFakeCmdHelperProcess", "--", output, code)
		cmd.Env = append(os.Environ(), "GO_WANT_FAKECMD_HELPER=1")
		return cmd
	}
}

func TestFakeCmdHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_FAKECMD_HELPER") != "1" {
		return
	}

	sep := -1
	for i, a := range os.Args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep < 0 || len(os.Args) < sep+3 {
		os.Exit(2)
	}

	output := os.Args[sep+1]
	code, err := strconv.Atoi(os.Args[sep+2])
	if err != nil {
		os.Exit(2)
	}

	if output != "" {
		_, _ = os.Stdout.WriteString(output)
	}
	os.Exit(code)
}

// ─── extractGHEnvToken ────────────────────────────────────────────────────────

func TestExtractGHEnvToken_GHToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghu_testtoken")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("COPILOT_GITHUB_TOKEN", "")

	tok, ok := extractGHEnvToken()
	if !ok {
		t.Fatal("expected token from GH_TOKEN")
	}
	if tok != "ghu_testtoken" {
		t.Errorf("got %q, want %q", tok, "ghu_testtoken")
	}
}

func TestExtractGHEnvToken_GitHubToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "ghp_fallback")
	t.Setenv("COPILOT_GITHUB_TOKEN", "")

	tok, ok := extractGHEnvToken()
	if !ok {
		t.Fatal("expected token from GITHUB_TOKEN")
	}
	if tok != "ghp_fallback" {
		t.Errorf("got %q, want %q", tok, "ghp_fallback")
	}
}

func TestExtractGHEnvToken_CopilotGitHubToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("COPILOT_GITHUB_TOKEN", "gho_copilot")

	tok, ok := extractGHEnvToken()
	if !ok {
		t.Fatal("expected token from COPILOT_GITHUB_TOKEN")
	}
	if tok != "gho_copilot" {
		t.Errorf("got %q, want %q", tok, "gho_copilot")
	}
}

func TestExtractGHEnvToken_NotSet(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("COPILOT_GITHUB_TOKEN", "")

	_, ok := extractGHEnvToken()
	if ok {
		t.Fatal("expected no token when both vars are empty")
	}
}

func TestExtractGHEnvToken_GHTokenTakesPrecedence(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghu_primary")
	t.Setenv("GITHUB_TOKEN", "ghp_secondary")
	t.Setenv("COPILOT_GITHUB_TOKEN", "gho_tertiary")

	tok, ok := extractGHEnvToken()
	if !ok {
		t.Fatal("expected token")
	}
	if tok != "ghu_primary" {
		t.Errorf("GH_TOKEN should take precedence; got %q", tok)
	}
}

func TestExtractGHEnvToken_GitHubTokenTakesPrecedenceOverCopilotToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "ghp_secondary")
	t.Setenv("COPILOT_GITHUB_TOKEN", "gho_tertiary")

	tok, ok := extractGHEnvToken()
	if !ok {
		t.Fatal("expected token")
	}
	if tok != "ghp_secondary" {
		t.Errorf("GITHUB_TOKEN should take precedence over COPILOT_GITHUB_TOKEN; got %q", tok)
	}
}

// ─── extractGHCLIToken ────────────────────────────────────────────────────────

func TestExtractGHCLIToken_Success(t *testing.T) {
	orig := ghCLITokenCmd
	ghCLITokenCmd = fakeCmd(t, "ghu_fromghcli", 0)
	defer func() { ghCLITokenCmd = orig }()

	tok, err := extractGHCLIToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "ghu_fromghcli" {
		t.Errorf("got %q, want %q", tok, "ghu_fromghcli")
	}
}

func TestExtractGHCLIToken_Failure(t *testing.T) {
	orig := ghCLITokenCmd
	ghCLITokenCmd = fakeCmd(t, "", 1)
	defer func() { ghCLITokenCmd = orig }()

	_, err := extractGHCLIToken()
	if err == nil {
		t.Fatal("expected error on non-zero exit")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected wrapped *exec.ExitError, got: %T (%v)", err, err)
	}
	if !strings.Contains(err.Error(), "gh auth token failed") {
		t.Fatalf("expected gh auth context in error, got: %v", err)
	}
}

func TestExtractGHCLIToken_Failure_DoesNotLeakAmbientTokenValue(t *testing.T) {
	secret1 := "ghu_secret_from_parent_env"
	secret2 := "ghp_secret_from_parent_env"
	t.Setenv("GH_TOKEN", secret1)
	t.Setenv("GITHUB_TOKEN", secret2)
	orig := ghCLITokenCmd
	ghCLITokenCmd = fakeCmd(t, "", 1)
	defer func() { ghCLITokenCmd = orig }()

	_, err := extractGHCLIToken()
	if err == nil {
		t.Fatal("expected error on non-zero exit")
	}
	msg := err.Error()
	if strings.Contains(msg, secret1) || strings.Contains(msg, secret2) {
		t.Fatalf("error leaked token value: %q", msg)
	}
}

func TestExtractGHCLIToken_EmptyOutput(t *testing.T) {
	orig := ghCLITokenCmd
	ghCLITokenCmd = fakeCmd(t, "", 0)
	defer func() { ghCLITokenCmd = orig }()

	_, err := extractGHCLIToken()
	if err == nil {
		t.Fatal("expected error on empty output")
	}
}

func TestGHCLITokenCmd_PinsGitHubHostname(t *testing.T) {
	cmd := ghCLITokenCmd(context.Background())
	want := []string{"gh", "auth", "token", "--hostname", "github.com"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("args length = %d, want %d (%v)", len(cmd.Args), len(want), cmd.Args)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("arg[%d] = %q, want %q (all args: %v)", i, cmd.Args[i], want[i], cmd.Args)
		}
	}
}

// ─── ExtractCopilotToken ──────────────────────────────────────────────────────

func withEnvToken(t *testing.T, token string) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("COPILOT_GITHUB_TOKEN", "")
}

func withNoEnvToken(t *testing.T) {
	t.Helper()
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("COPILOT_GITHUB_TOKEN", "")
}

func withFakeGHCLI(t *testing.T, output string, exitCode int) func() {
	orig := ghCLITokenCmd
	ghCLITokenCmd = fakeCmd(t, output, exitCode)
	return func() { ghCLITokenCmd = orig }
}

func TestExtractCopilotToken_EnvMode_Success(t *testing.T) {
	withEnvToken(t, "ghu_env")

	et, err := ExtractCopilotToken("env_only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if et.Source != TokenSourceEnv {
		t.Errorf("source = %q, want env", et.Source)
	}
	if et.Token != "ghu_env" {
		t.Errorf("token = %q, want ghu_env", et.Token)
	}
}

func TestExtractCopilotToken_EnvMode_Missing(t *testing.T) {
	withNoEnvToken(t)

	_, err := ExtractCopilotToken("env_only")
	if err == nil {
		t.Fatal("expected error when env token missing in env mode")
	}
	if !strings.Contains(err.Error(), "GH_TOKEN/GITHUB_TOKEN/COPILOT_GITHUB_TOKEN not set") {
		t.Fatalf("expected missing-env context in error, got: %v", err)
	}
}

func TestExtractCopilotToken_EnvMode_CopilotGitHubTokenSuccess(t *testing.T) {
	withNoEnvToken(t)
	t.Setenv("COPILOT_GITHUB_TOKEN", "gho_env")

	et, err := ExtractCopilotToken("env_only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if et.Source != TokenSourceEnv {
		t.Errorf("source = %q, want env", et.Source)
	}
	if et.Token != "gho_env" {
		t.Errorf("token = %q, want gho_env", et.Token)
	}
}

func TestExtractCopilotToken_GHOnly_GHCLISuccess(t *testing.T) {
	withNoEnvToken(t)
	defer withFakeGHCLI(t, "ghu_ghcli", 0)()

	et, err := ExtractCopilotToken("gh_only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if et.Source != TokenSourceGHCLI {
		t.Errorf("source = %q, want gh-cli", et.Source)
	}
}

func TestExtractCopilotToken_GHOnly_FailsClosedWhenGHCLIFails(t *testing.T) {
	withNoEnvToken(t)
	defer withFakeGHCLI(t, "", 1)()

	_, err := ExtractCopilotToken("gh_only")
	if err == nil {
		t.Fatal("expected error when gh-cli extraction fails")
	}
}

func TestExtractCopilotToken_Auto_EnvFirst(t *testing.T) {
	withEnvToken(t, "ghu_auto_env")
	defer withFakeGHCLI(t, "ghu_ghcli_should_not_be_used", 0)()

	et, err := ExtractCopilotToken("auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if et.Source != TokenSourceEnv {
		t.Errorf("auto mode should prefer env, got source=%q", et.Source)
	}
}

func TestExtractCopilotToken_Auto_GHCLIFallback(t *testing.T) {
	withNoEnvToken(t)
	defer withFakeGHCLI(t, "ghu_from_ghcli", 0)()

	et, err := ExtractCopilotToken("auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if et.Source != TokenSourceGHCLI {
		t.Errorf("auto mode should fall back to gh-cli, got source=%q", et.Source)
	}
}

func TestExtractCopilotToken_Auto_GHCLIFailure_PreservesCause(t *testing.T) {
	withNoEnvToken(t)
	orig := ghCLITokenCmd
	ghCLITokenCmd = fakeCmd(t, "", 1)
	defer func() { ghCLITokenCmd = orig }()

	_, err := ExtractCopilotToken("auto")
	if err == nil {
		t.Fatal("expected error when env tokens are missing and gh-cli fails")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected wrapped *exec.ExitError from gh command, got: %T (%v)", err, err)
	}

	msg := err.Error()
	if !strings.Contains(msg, "could not extract Copilot token") {
		t.Fatalf("expected top-level extraction context, got: %v", err)
	}
	if !strings.Contains(msg, "GH_TOKEN/GITHUB_TOKEN/COPILOT_GITHUB_TOKEN not set") {
		t.Fatalf("expected missing-env wording in error, got: %v", err)
	}
	if !strings.Contains(msg, "gh auth token failed") {
		t.Fatalf("expected gh extraction context, got: %v", err)
	}
	if strings.Contains(msg, "ghu_secret_token") || strings.Contains(msg, "ghp_secret_token") {
		t.Fatalf("error leaked token-like value: %q", msg)
	}
}

func TestExtractCopilotToken_UnknownMode_Error(t *testing.T) {
	withNoEnvToken(t)
	defer withFakeGHCLI(t, "ghu_ghcli_should_not_be_used", 0)()

	_, err := ExtractCopilotToken("unknown_mode")
	if err == nil {
		t.Fatal("expected error for unknown auth mode")
	}
	if !strings.Contains(err.Error(), "unknown copilot_auth_mode") {
		t.Fatalf("expected unknown-mode validation error, got: %v", err)
	}
}

type envScenario struct {
	name               string
	ghToken            string
	githubToken        string
	copilotGitHubToken string
}

func setEnvScenario(t *testing.T, sc envScenario) {
	t.Helper()
	t.Setenv("GH_TOKEN", sc.ghToken)
	t.Setenv("GITHUB_TOKEN", sc.githubToken)
	t.Setenv("COPILOT_GITHUB_TOKEN", sc.copilotGitHubToken)
}

func TestExtractCopilotToken_Matrix_AutoEnvOnlyGHOnly(t *testing.T) {
	envCases := []envScenario{
		{name: "gh-token-set", ghToken: "ghu_primary", githubToken: "", copilotGitHubToken: ""},
		{name: "github-token-set", ghToken: "", githubToken: "ghu_secondary", copilotGitHubToken: ""},
		{name: "copilot-github-token-set", ghToken: "", githubToken: "", copilotGitHubToken: "gho_tertiary"},
		{name: "all-set", ghToken: "ghu_primary", githubToken: "ghu_secondary", copilotGitHubToken: "gho_tertiary"},
		{name: "neither-set", ghToken: "", githubToken: "", copilotGitHubToken: ""},
	}

	for _, sc := range envCases {
		for _, ghSucceeds := range []bool{true, false} {
			t.Run("auto/"+sc.name+"/gh="+strconv.FormatBool(ghSucceeds), func(t *testing.T) {
				setEnvScenario(t, sc)
				ghCalls := 0
				orig := ghCLITokenCmd
				exitCode := 1
				if ghSucceeds {
					exitCode = 0
				}
				ghCLITokenCmd = func(ctx context.Context) *exec.Cmd {
					ghCalls++
					return fakeCmd(t, "ghu_from_ghcli", exitCode)(ctx)
				}
				defer func() { ghCLITokenCmd = orig }()

				et, err := ExtractCopilotToken("auto")
				hasEnvToken := strings.TrimSpace(sc.ghToken) != "" ||
					strings.TrimSpace(sc.githubToken) != "" ||
					strings.TrimSpace(sc.copilotGitHubToken) != ""
				if hasEnvToken {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if et.Source != TokenSourceEnv {
						t.Fatalf("source = %q, want env", et.Source)
					}
					want := strings.TrimSpace(sc.ghToken)
					if want == "" {
						want = strings.TrimSpace(sc.githubToken)
					}
					if want == "" {
						want = strings.TrimSpace(sc.copilotGitHubToken)
					}
					if et.Token != want {
						t.Fatalf("token = %q, want %q", et.Token, want)
					}
					if ghCalls != 0 {
						t.Fatalf("expected gh-cli to be skipped when env token exists, calls=%d", ghCalls)
					}
					return
				}
				if ghSucceeds {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if et.Source != TokenSourceGHCLI || et.Token != "ghu_from_ghcli" {
						t.Fatalf("got (%q,%q), want (gh-cli,ghu_from_ghcli)", et.Source, et.Token)
					}
					if ghCalls != 1 {
						t.Fatalf("expected exactly one gh-cli call, got %d", ghCalls)
					}
					return
				}
				if err == nil {
					t.Fatal("expected error when neither env token exists and gh-cli fails")
				}
				if ghCalls != 1 {
					t.Fatalf("expected exactly one gh-cli call, got %d", ghCalls)
				}
			})

			t.Run("env_only/"+sc.name+"/gh="+strconv.FormatBool(ghSucceeds), func(t *testing.T) {
				setEnvScenario(t, sc)
				ghCalls := 0
				orig := ghCLITokenCmd
				exitCode := 1
				if ghSucceeds {
					exitCode = 0
				}
				ghCLITokenCmd = func(ctx context.Context) *exec.Cmd {
					ghCalls++
					return fakeCmd(t, "ghu_from_ghcli", exitCode)(ctx)
				}
				defer func() { ghCLITokenCmd = orig }()

				et, err := ExtractCopilotToken("env_only")
				hasEnvToken := strings.TrimSpace(sc.ghToken) != "" ||
					strings.TrimSpace(sc.githubToken) != "" ||
					strings.TrimSpace(sc.copilotGitHubToken) != ""
				if hasEnvToken {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if et.Source != TokenSourceEnv {
						t.Fatalf("source = %q, want env", et.Source)
					}
					want := strings.TrimSpace(sc.ghToken)
					if want == "" {
						want = strings.TrimSpace(sc.githubToken)
					}
					if want == "" {
						want = strings.TrimSpace(sc.copilotGitHubToken)
					}
					if et.Token != want {
						t.Fatalf("token = %q, want %q", et.Token, want)
					}
				} else if err == nil {
					t.Fatal("expected error when env tokens are missing")
				}
				if ghCalls != 0 {
					t.Fatalf("env mode must never invoke gh-cli, calls=%d", ghCalls)
				}
			})

			t.Run("gh_only/"+sc.name+"/gh="+strconv.FormatBool(ghSucceeds), func(t *testing.T) {
				setEnvScenario(t, sc)
				exitCode := 1
				if ghSucceeds {
					exitCode = 0
				}
				defer withFakeGHCLI(t, "ghu_from_ghcli", exitCode)()

				et, err := ExtractCopilotToken("gh_only")
				if ghSucceeds {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if et.Source != TokenSourceGHCLI || et.Token != "ghu_from_ghcli" {
						t.Fatalf("got (%q,%q), want (gh-cli,ghu_from_ghcli)", et.Source, et.Token)
					}
					return
				}
				if err == nil {
					t.Fatal("expected gh_only to fail closed when gh-cli fails")
				}
			})
		}
	}
}

// ─── Config validation ────────────────────────────────────────────────────────

func TestCopilotAuthModeValidation(t *testing.T) {
	valid := []string{"auto", "env_only", "gh_only"}
	for _, mode := range valid {
		found := false
		for _, v := range domain.ValidCopilotAuthModes {
			if v == mode {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q to be a valid auth mode", mode)
		}
	}

	for _, v := range domain.ValidCopilotAuthModes {
		if v == "bogus" {
			t.Error("expected 'bogus' to be invalid")
		}
	}
}

// ─── injectPreExtractedToken ─────────────────────────────────────────────────

func TestInjectPreExtractedToken_InjectsGHToken(t *testing.T) {
	withEnvToken(t, "") // start with empty tokens, then set GH_TOKEN for this test case
	t.Setenv("GH_TOKEN", "ghu_inject")
	// env is a fresh slice without GH_TOKEN
	env := []string{"HOME=/tmp"}

	result, err := injectPreExtractedToken(env, "env_only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, e := range result {
		if e == "GH_TOKEN=ghu_inject" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected GH_TOKEN=ghu_inject in env, got: %v", result)
	}
}

func TestInjectPreExtractedToken_DoesNotOverwriteExisting(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghu_new")
	env := []string{"GH_TOKEN=ghu_existing", "HOME=/tmp"}

	result, err := injectPreExtractedToken(env, "env_only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count := 0
	for _, e := range result {
		if len(e) >= 9 && e[:9] == "GH_TOKEN=" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 GH_TOKEN entry, got %d in %v", count, result)
	}
	if result[0] != "GH_TOKEN=ghu_existing" {
		t.Errorf("existing GH_TOKEN should not be overwritten, got: %v", result)
	}
}

func TestInjectPreExtractedToken_SkipsExtractionWhenTokenAlreadyPresent(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghu_existing")
	orig := ghCLITokenCmd
	ghCLITokenCmd = func(context.Context) *exec.Cmd {
		t.Fatal("expected injectPreExtractedToken to skip extraction when GH_TOKEN is already present")
		return nil
	}
	defer func() { ghCLITokenCmd = orig }()

	env := []string{"GH_TOKEN=ghu_existing", "HOME=/tmp"}
	result, err := injectPreExtractedToken(env, "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != len(env) {
		t.Fatalf("expected env to remain unchanged, got %v", result)
	}
	for i := range env {
		if result[i] != env[i] {
			t.Fatalf("env[%d] changed: got %q want %q", i, result[i], env[i])
		}
	}
}

func TestInjectPreExtractedToken_AutoModeStripsBlankAuthEntriesBeforeHonoringEnv(t *testing.T) {
	orig := ghCLITokenCmd
	ghCLITokenCmd = func(context.Context) *exec.Cmd {
		t.Fatal("expected injectPreExtractedToken to skip extraction when non-empty child token is present")
		return nil
	}
	defer func() { ghCLITokenCmd = orig }()

	env := []string{"GH_TOKEN=   ", "COPILOT_GITHUB_TOKEN=   ", "GITHUB_TOKEN=ghu_existing", "HOME=/tmp"}
	result, err := injectPreExtractedToken(env, "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, e := range result {
		if e == "GH_TOKEN=   " {
			t.Fatalf("expected blank GH_TOKEN entry to be stripped, got %v", result)
		}
	}
	if len(result) != 2 {
		t.Fatalf("expected blank auth entries to be removed while keeping non-empty token, got %v", result)
	}
}

func TestInjectPreExtractedToken_IgnoresBlankChildToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghu_inject")
	env := []string{"GH_TOKEN=", "HOME=/tmp"}

	result, err := injectPreExtractedToken(env, "env_only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	ghTokenCount := 0
	for _, e := range result {
		if strings.HasPrefix(e, "GH_TOKEN=") {
			ghTokenCount++
		}
		if e == "GH_TOKEN=ghu_inject" {
			found = true
		}
		if e == "GH_TOKEN=" {
			t.Fatalf("expected blank GH_TOKEN entry to be removed, got %v", result)
		}
	}
	if !found {
		t.Fatalf("expected injected GH_TOKEN to replace blank child token, got %v", result)
	}
	if ghTokenCount != 1 {
		t.Fatalf("expected exactly one GH_TOKEN entry, got %d in %v", ghTokenCount, result)
	}
}

func TestInjectPreExtractedToken_RestrictiveModeReturnsError(t *testing.T) {
	withNoEnvToken(t)
	defer withFakeGHCLI(t, "", 1)()

	env := []string{"HOME=/tmp"}
	result, err := injectPreExtractedToken(env, "env_only")
	if err == nil {
		t.Fatal("expected restrictive auth mode to return an error")
	}
	if len(result) != len(env) {
		t.Errorf("on failure, env should be unchanged; got len=%d want %d", len(result), len(env))
	}
}

func TestInjectPreExtractedToken_AutoModeCanContinueOnFailure(t *testing.T) {
	withNoEnvToken(t)
	defer withFakeGHCLI(t, "", 1)()

	env := []string{"HOME=/tmp"}
	result, err := injectPreExtractedToken(env, "auto")
	if err != nil {
		t.Fatalf("expected permissive auto mode to continue, got error: %v", err)
	}
	if len(result) != len(env) {
		t.Errorf("on failure, env should be unchanged; got len=%d want %d", len(result), len(env))
	}
}

func TestInjectPreExtractedToken_AutoModeFailureStripsBlankAuthEntries(t *testing.T) {
	withNoEnvToken(t)
	defer withFakeGHCLI(t, "", 1)()

	env := []string{"GH_TOKEN=", "GITHUB_TOKEN=   ", "COPILOT_GITHUB_TOKEN=  ", "HOME=/tmp"}
	result, err := injectPreExtractedToken(env, "auto")
	if err != nil {
		t.Fatalf("expected permissive auto mode to continue, got error: %v", err)
	}
	for _, e := range result {
		if strings.HasPrefix(e, "GH_TOKEN=") || strings.HasPrefix(e, "GITHUB_TOKEN=") || strings.HasPrefix(e, "COPILOT_GITHUB_TOKEN=") {
			t.Fatalf("expected blank auth entries to be stripped before fallback return, got %v", result)
		}
	}
	if len(result) != 1 || result[0] != "HOME=/tmp" {
		t.Fatalf("expected only non-auth env entries after stripping blanks, got %v", result)
	}
}

func TestInjectPreExtractedToken_GHOnlyDoesNotSilentlyFallback(t *testing.T) {
	withNoEnvToken(t)
	defer withFakeGHCLI(t, "", 1)()

	env := []string{"HOME=/tmp"}
	result, err := injectPreExtractedToken(env, "gh_only")
	if err == nil {
		t.Fatal("expected gh_only failure to return an error")
	}
	if len(result) != len(env) {
		t.Errorf("on failure, env should be unchanged; got len=%d want %d", len(result), len(env))
	}
}

func TestInjectPreExtractedToken_GHOnlyIgnoresInheritedToken(t *testing.T) {
	withNoEnvToken(t)
	defer withFakeGHCLI(t, "ghu_broker", 0)()

	env := []string{"GH_TOKEN=ghu_child", "COPILOT_GITHUB_TOKEN=gho_child", "HOME=/tmp"}
	result, err := injectPreExtractedToken(env, "gh_only")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, e := range result {
		if e == "GH_TOKEN=ghu_broker" {
			found = true
		}
		if e == "GH_TOKEN=ghu_child" {
			t.Fatalf("expected inherited GH_TOKEN to be stripped, got %v", result)
		}
		if e == "COPILOT_GITHUB_TOKEN=gho_child" {
			t.Fatalf("expected inherited COPILOT_GITHUB_TOKEN to be stripped, got %v", result)
		}
	}
	if !found {
		t.Fatalf("expected gh-only token to be injected, got %v", result)
	}
}

func TestInjectPreExtractedToken_NormalizesInheritedGithubTokenToGHToken(t *testing.T) {
	orig := ghCLITokenCmd
	ghCLITokenCmd = func(context.Context) *exec.Cmd {
		t.Fatal("expected injectPreExtractedToken to honor the inherited token without extraction")
		return nil
	}
	defer func() { ghCLITokenCmd = orig }()

	env := []string{"GITHUB_TOKEN=ghu_child", "HOME=/tmp"}
	result, err := injectPreExtractedToken(env, "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, e := range result {
		if e == "GH_TOKEN=ghu_child" {
			found = true
		}
		if strings.HasPrefix(e, "GITHUB_TOKEN=") {
			t.Fatalf("expected GITHUB_TOKEN to be normalized away, got %v", result)
		}
	}
	if !found {
		t.Fatalf("expected inherited GITHUB_TOKEN to be normalized to GH_TOKEN, got %v", result)
	}
}

func TestInjectPreExtractedToken_HonorsTokenPrecedenceRegardlessOfEnvOrder(t *testing.T) {
	orig := ghCLITokenCmd
	ghCLITokenCmd = func(context.Context) *exec.Cmd {
		t.Fatal("expected injectPreExtractedToken to honor the inherited token without extraction")
		return nil
	}
	defer func() { ghCLITokenCmd = orig }()

	// GITHUB_TOKEN appears before GH_TOKEN, but GH_TOKEN wins per precedence.
	env := []string{"GITHUB_TOKEN=ghu_secondary", "GH_TOKEN=ghu_primary", "HOME=/tmp"}
	result, err := injectPreExtractedToken(env, "auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range result {
		if e != "GITHUB_TOKEN=ghu_secondary" && e != "GH_TOKEN=ghu_primary" && e != "HOME=/tmp" {
			t.Fatalf("unexpected env mutation: %v", result)
		}
	}
	if len(result) != len(env) {
		t.Fatalf("expected env unchanged when GH_TOKEN already present, got %v", result)
	}
}

func TestStripChildEnvTokens_RemovesBlankEntries(t *testing.T) {
	env := []string{"GH_TOKEN=", "GITHUB_TOKEN=   ", "COPILOT_GITHUB_TOKEN=  ", "HOME=/tmp", "PATH=/usr/bin"}

	stripped := stripChildEnvTokens(env)
	for _, e := range stripped {
		if strings.HasPrefix(e, "GH_TOKEN=") || strings.HasPrefix(e, "GITHUB_TOKEN=") || strings.HasPrefix(e, "COPILOT_GITHUB_TOKEN=") {
			t.Fatalf("expected auth env keys to be stripped, got %v", stripped)
		}
	}
	if len(stripped) != 2 {
		t.Fatalf("expected only non-auth keys to remain, got %v", stripped)
	}
}

func TestStripBlankChildEnvTokens_RemovesBlankAndTrimsValues(t *testing.T) {
	env := []string{"GH_TOKEN=   ", "GITHUB_TOKEN=  ghu_x  ", "COPILOT_GITHUB_TOKEN=  gho_x  ", "HOME=/tmp"}

	stripped := stripBlankChildEnvTokens(env)
	if len(stripped) != 3 {
		t.Fatalf("expected 3 entries, got %v", stripped)
	}
	if stripped[0] != "GITHUB_TOKEN=ghu_x" {
		t.Fatalf("expected trimmed GITHUB_TOKEN value, got %v", stripped)
	}
	if stripped[1] != "COPILOT_GITHUB_TOKEN=gho_x" {
		t.Fatalf("expected trimmed COPILOT_GITHUB_TOKEN value, got %v", stripped)
	}
	if stripped[2] != "HOME=/tmp" {
		t.Fatalf("expected HOME to remain, got %v", stripped)
	}
}

// ─── stripAuthEnv ─────────────────────────────────────────────────────────────

func TestStripAuthEnv_RemovesAuthVars(t *testing.T) {
	env := []string{"GH_TOKEN=tok", "GITHUB_TOKEN=tok2", "COPILOT_GITHUB_TOKEN=tok3", "HOME=/tmp", "PATH=/usr/bin"}
	got := stripAuthEnv(env)
	for _, e := range got {
		if strings.HasPrefix(e, "GH_TOKEN=") || strings.HasPrefix(e, "GITHUB_TOKEN=") || strings.HasPrefix(e, "COPILOT_GITHUB_TOKEN=") {
			t.Errorf("stripAuthEnv should have removed %q", e)
		}
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %v", got)
	}
}

func TestStripAuthEnv_NoOp_WhenAbsent(t *testing.T) {
	env := []string{"HOME=/tmp", "PATH=/usr/bin"}
	got := stripAuthEnv(env)
	if len(got) != len(env) {
		t.Errorf("expected env unchanged, got %v", got)
	}
}

// TestStripAuthEnv_CaseSensitivityFollowsOS pins that a lower/mixed-case token
// var is left alone on POSIX (a genuinely different variable) but stripped when
// the OS treats names case-insensitively, as Windows does — otherwise gh_only
// would still hand ambient env auth to gh there.
func TestStripAuthEnv_CaseSensitivityFollowsOS(t *testing.T) {
	env := []string{"gh_token=tok", "GitHub_Token=tok2", "HOME=/tmp"}

	orig := envNamesCaseInsensitive
	t.Cleanup(func() { envNamesCaseInsensitive = orig })

	envNamesCaseInsensitive = false
	if got := stripAuthEnv(env); len(got) != 3 {
		t.Errorf("POSIX: lower-case names are distinct vars and must be kept, got %v", got)
	}

	envNamesCaseInsensitive = true
	got := stripAuthEnv(env)
	if len(got) != 1 || got[0] != "HOME=/tmp" {
		t.Errorf("case-insensitive OS: mixed-case token vars must be stripped, got %v", got)
	}
}

// ─── gh subprocess env isolation ─────────────────────────────────────────────

// TestExtractGHCLIToken_EnvIsolation verifies that GH_TOKEN, GITHUB_TOKEN, and
// COPILOT_GITHUB_TOKEN are NOT forwarded to the gh subprocess even when set in
// the parent process.
func TestExtractGHCLIToken_EnvIsolation(t *testing.T) {
	t.Setenv("GH_TOKEN", "ghu_parent_secret")
	t.Setenv("GITHUB_TOKEN", "ghp_parent_secret")
	t.Setenv("COPILOT_GITHUB_TOKEN", "gho_parent_secret")

	var capturedCmd *exec.Cmd
	orig := ghCLITokenCmd
	ghCLITokenCmd = func(ctx context.Context) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestFakeCmdHelperProcess", "--", "ghu_fromghcli", "0")
		cmd.Env = append(os.Environ(), "GO_WANT_FAKECMD_HELPER=1")
		capturedCmd = cmd
		return cmd
	}
	defer func() { ghCLITokenCmd = orig }()

	_, err := extractGHCLIToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCmd == nil {
		t.Fatal("expected cmd to be captured")
	}
	for _, e := range capturedCmd.Env {
		if e == "GH_TOKEN=ghu_parent_secret" {
			t.Errorf("GH_TOKEN leaked into gh subprocess env: %q", e)
		}
		if e == "GITHUB_TOKEN=ghp_parent_secret" {
			t.Errorf("GITHUB_TOKEN leaked into gh subprocess env: %q", e)
		}
		if e == "COPILOT_GITHUB_TOKEN=gho_parent_secret" {
			t.Errorf("COPILOT_GITHUB_TOKEN leaked into gh subprocess env: %q", e)
		}
	}
}

// ─── os.Setenv helper (Go 1.17+: t.Setenv) ──────────────────────────────────

func init() {
	// Ensure auth vars don't leak from the developer's shell into tests.
	_ = os.Unsetenv("GH_TOKEN")
	_ = os.Unsetenv("GITHUB_TOKEN")
	_ = os.Unsetenv("COPILOT_GITHUB_TOKEN")
}
