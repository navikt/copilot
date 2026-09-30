package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// The setup wizard asks two separate things (#1348): how Copilot runs
// commands (nav-pilot's autonomy) and what cplt lets the agent do with git
// (cplt's git_guard.protect_default_branch_only). The network (cplt's
// sandbox.preset) is only asked with --advanced. Merge, push to main and force
// push stay blocked by cplt under every answer.

// autonomyChoice is what the answers settle on.
type autonomyChoice struct {
	Autonomy string // nav-pilot autonomy; "" for a client that does not read it
	Preset   string // cplt sandbox.preset
	Push     bool   // the agent may push branches (protect_default_branch_only)
}

// cpltGitState is the part of the cplt config the presets touch. Preset is ""
// when cplt could not say. Protect is the value in the config file, nil when
// the file does not set it and the preset's baseline applies.
type cpltGitState struct {
	Preset  string
	Protect *bool
}

// readCpltGitState asks cplt for the two keys. `cplt config get` prints the
// default, not the preset's baseline, for a key the file does not set, and
// marks it with a "(default" line on stderr; that is how an unset key is told
// apart.
func readCpltGitState(cliPath string) cpltGitState {
	var s cpltGitState
	if out, _, err := cpltConfigGetDefault(cliPath, "sandbox.preset"); err == nil {
		s.Preset = cpltPresetFromConfigGet(out)
	}
	if out, isDefault, err := cpltConfigGetDefault(cliPath, "git_guard.protect_default_branch_only"); err == nil && !isDefault {
		if v, perr := strconv.ParseBool(out); perr == nil {
			s.Protect = &v
		}
	}
	return s
}

// cpltConfigGetDefault reads one key: the value is the first line of stdout,
// and cplt says "(default, not set in config file)" on stderr when the file
// does not set it. Both streams are searched for the marker, stdout only for
// the value, so a warning on stderr can never pose as the value.
func cpltConfigGetDefault(cliPath, key string) (val string, isDefault bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), cpltCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cliPath, "config", "get", key)
	cmd.WaitDelay = time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", false, err
	}
	first, _, _ := strings.Cut(string(out), "\n")
	isDefault = bytes.Contains(out, []byte("(default")) || bytes.Contains(stderr.Bytes(), []byte("(default"))
	return strings.TrimSpace(first), isDefault, nil
}

// effectivePush is whether cplt lets the agent push a feature branch: the
// file's key if set, else the preset's baseline (only standard allows it).
// An unknown preset reads as cplt's default, standard.
func (s cpltGitState) effectivePush() bool {
	if s.Protect != nil {
		return *s.Protect
	}
	return s.Preset == "" || s.Preset == "standard"
}

// guardedPreset is true for the cplt presets whose git and gh guards are on.
func guardedPreset(p string) bool { return p == "standard" || p == cpltStrictPreset }

// cpltChange is one `cplt config set` the wizard makes.
type cpltChange struct{ Key, From, To string }

// cpltChanges is what it takes to get from s to c, and nothing more: a key
// already at the wanted value is not written.
func cpltChanges(s cpltGitState, c autonomyChoice) []cpltChange {
	var out []cpltChange
	if s.Preset != c.Preset {
		out = append(out, cpltChange{"sandbox.preset", s.Preset, c.Preset})
	}
	// The guard only has a meaning under the two presets that turn it on.
	if !guardedPreset(c.Preset) {
		return out
	}
	after := cpltGitState{Preset: c.Preset, Protect: s.Protect}
	if after.effectivePush() != c.Push {
		from := ""
		if s.Protect != nil {
			from = strconv.FormatBool(*s.Protect)
		}
		out = append(out, cpltChange{"git_guard.protect_default_branch_only", from, strconv.FormatBool(c.Push)})
	}
	return out
}

// askAutonomy asks the wizard's questions. Each starts on cur, today's
// setting, so Enter on every one changes nothing. The client question is
// Copilot's only: the other clients do not read autonomy. advanced adds the
// network question, the one way to cplt's strict preset.
func askAutonomy(client string, cur autonomyChoice, advanced bool) (autonomyChoice, error) {
	c := cur
	if client != "copilot" {
		c.Autonomy = ""
	} else if err := huh.NewSelect[string]().
		Title("How should the agent run commands?").
		Options(
			huh.NewOption("On its own inside the sandbox, and ask you when it needs to (recommended)", "sandbox"),
			huh.NewOption("Ask before each command", "conservative"),
		).
		Value(&c.Autonomy).
		WithTheme(navTheme()).
		Run(); err != nil {
		return autonomyChoice{}, err
	}
	if advanced {
		netOpts := []huh.Option[string]{
			huh.NewOption("Standard: GitHub, Nav and package hosts reachable", "standard"),
			huh.NewOption("Allowlist only (cplt strict): no pushes unless you allow them below", cpltStrictPreset),
		}
		if !guardedPreset(c.Preset) {
			netOpts = append(netOpts, huh.NewOption("Keep cplt preset "+c.Preset, c.Preset))
		}
		if err := huh.NewSelect[string]().
			Title("Network").
			Options(netOpts...).
			Value(&c.Preset).
			WithTheme(navTheme()).
			Run(); err != nil {
			return autonomyChoice{}, err
		}
	}
	// Under permissive and full-trust cplt's git guard is off: there is
	// nothing for the git answer to set.
	if !guardedPreset(c.Preset) {
		return c, nil
	}
	if c.Preset != cur.Preset {
		c.Push = c.Preset == "standard" // a new preset starts on its baseline
	}
	desc := "cplt never lets it push to main or merge, and enforces this whatever the agent is told."
	if client != "copilot" {
		desc += " " + clientLabel[client] + " keeps its own permission settings"
		if client == "opencode" {
			desc += " (the permission key in opencode.json)"
		}
		desc += "."
	}
	// A string select: huh's Select[bool] leaves out the option for true.
	git := strconv.FormatBool(c.Push)
	if err := huh.NewSelect[string]().
		Title("What may the agent do with git?").
		Description(desc).
		Options(
			huh.NewOption("Commit, push branches and open pull requests (recommended)", "true"),
			huh.NewOption("Commit only (no pushes)", "false"),
		).
		Value(&git).
		WithTheme(navTheme()).
		Run(); err != nil {
		return autonomyChoice{}, err
	}
	c.Push = git == "true"
	return c, nil
}

// autonomySummary is the one screen shown before anything is written.
// allowlist is the proxy.allowed_domains that stays set under a preset other
// than strict, or "".
func autonomySummary(c autonomyChoice, changes []cpltChange, cpltFound bool, allowlist string) string {
	var b strings.Builder
	if c.Autonomy != "" {
		fmt.Fprintf(&b, "nav-pilot autonomy = %s\n", c.Autonomy)
	}
	if !cpltFound {
		b.WriteString("cplt is not installed: its git and network rules are left for later.\n")
	}
	for _, ch := range changes {
		from, to := ch.From, ch.To
		if from == "" {
			from = "unset"
		}
		if to == "" {
			to = "unset"
		}
		fmt.Fprintf(&b, "cplt %s: %s → %s\n", ch.Key, from, to)
		if ch.Key == "sandbox.preset" && ch.To == cpltStrictPreset {
			b.WriteString("  also points proxy.allowed_domains at nav-pilot's list of Nav hosts\n")
		}
	}
	if cpltFound && len(changes) == 0 {
		b.WriteString("cplt: no changes\n")
	}
	switch {
	case cpltFound && !guardedPreset(c.Preset):
		fmt.Fprintf(&b, "cplt preset %s: its git and gh guards are off, so nothing is blocked, not even pushing to main or merging.\n", c.Preset)
	case !c.Push:
		b.WriteString("The agent cannot push at all: you push.\n")
	}
	if allowlist != "" {
		fmt.Fprintf(&b, "cplt proxy.allowed_domains stays set: only the hosts in %s are reachable.\n", allowlist)
		if allowlist == navAllowedDomainsPath() {
			b.WriteString("  That is nav-pilot's list from strict. To reach every host, run `cplt config set --global --unset proxy.allowed_domains`.\n")
		}
	}
	fmt.Fprintf(&b, "Your other answers go to %s.", configPath())
	return b.String()
}

// leavingStrictAllowlist is the change that drops nav-pilot's own allowlist
// when the user leaves strict, or nil. A list the user chose is theirs: it
// only gets the summary note.
func leavingStrictAllowlist(from, to, allowlist string) *cpltChange {
	if from != cpltStrictPreset || to == cpltStrictPreset || allowlist != navAllowedDomainsPath() {
		return nil
	}
	return &cpltChange{"proxy.allowed_domains", allowlist, ""}
}

// cpltConfigUnset removes one key from the global cplt config. Setting it to
// "" instead leaves an empty allowlist that cplt refuses to launch with.
func cpltConfigUnset(cliPath, key string) error {
	out, err := exec.Command(cliPath, "config", "set", key, "--unset", "--global").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to unset %s: %v\n%s", key, err, string(out))
	}
	return nil
}

// applyCpltChanges writes the changes. The guard key goes first, so the
// strict preset is never armed with a guard that still allows a push it
// should not; strict itself goes through applyStrictPreset, which seeds the
// Nav hosts before it sets the preset. A change to "" removes the key, and
// removals go last: the allowlist leaves only once cplt reads back the new
// preset, so a failed switch never leaves strict without its Nav hosts.
func applyCpltChanges(cliPath string, changes []cpltChange, allowlistPath, host string) error {
	var unsets, preset []cpltChange
	for _, ch := range changes {
		switch {
		case ch.To == "":
			unsets = append(unsets, ch)
		case ch.Key == "sandbox.preset":
			preset = append(preset, ch)
		default:
			if err := cpltConfigSet(cliPath, ch.Key, ch.To); err != nil {
				return err
			}
			fmt.Printf("%s cplt %s = %s\n", domain.Green("✓"), ch.Key, ch.To)
		}
	}
	for _, ch := range preset {
		if ch.To == cpltStrictPreset {
			return applyStrictPreset(cliPath, allowlistPath, host)
		}
		err := cpltConfigSet(cliPath, ch.Key, ch.To)
		if err == nil && len(unsets) > 0 {
			if got := cpltConfigGet(cliPath, ch.Key); got != ch.To {
				err = fmt.Errorf("cplt reads %s as %q after setting it to %q", ch.Key, got, ch.To)
			}
		}
		if err != nil {
			if len(unsets) > 0 {
				return fmt.Errorf("%w\nproxy.allowed_domains is left in place", err)
			}
			return err
		}
		fmt.Printf("%s cplt %s = %s\n", domain.Green("✓"), ch.Key, ch.To)
	}
	for _, ch := range unsets {
		if err := cpltConfigUnset(cliPath, ch.Key); err != nil {
			return err
		}
		fmt.Printf("%s cplt %s removed\n", domain.Green("✓"), ch.Key)
	}
	return nil
}

// cpltConfigFile is cplt's global config file, found the way cplt finds it
// (navikt/cplt src/config/path.rs): CPLT_CONFIG, else
// ~/.config/cplt/config.toml.
func cpltConfigFile() string {
	if p := os.Getenv("CPLT_CONFIG"); p != "" {
		return expandHome(p)
	}
	return expandHome("~/.config/cplt/config.toml")
}

func expandHome(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, rest)
	}
	return p
}

// strictByOurAdvice reads cplt's config file (no cplt spawn: it runs on the
// launch path) and reports a user on the strict preset the way nav-pilot
// once recommended it: without an allowlist (the doctor tip, #452) or with
// nav-pilot's own (#663). A user with an allowlist of their own chose strict
// themselves. allowlist is proxy.allowed_domains, "" when unset.
func strictByOurAdvice() (allowlist string, ok bool) {
	var cfg struct {
		Sandbox struct {
			Preset string `toml:"preset"`
		} `toml:"sandbox"`
		Proxy struct {
			AllowedDomains string `toml:"allowed_domains"`
		} `toml:"proxy"`
	}
	if _, err := toml.DecodeFile(cpltConfigFile(), &cfg); err != nil || cfg.Sandbox.Preset != cpltStrictPreset {
		return "", false
	}
	allowlist = expandHome(cfg.Proxy.AllowedDomains)
	return allowlist, allowlist == "" || allowlist == navAllowedDomainsPath()
}

// askLeaveStrict is the one-time question. Enter is No. A var so tests answer.
var askLeaveStrict = func(yes *bool) error {
	return huh.NewConfirm().
		Title("You're on cplt's strict preset, which blocks all pushes and some MCP servers. We no longer recommend it. Move to standard?").
		Affirmative("Yes").
		Negative("No").
		Value(yes).
		WithTheme(navTheme()).
		Run()
}

// offerLeaveStrict asks, once per machine and only in a terminal, a user on
// strict by nav-pilot's old advice whether to move to standard. Yes sets the
// preset and, when the allowlist was nav-pilot's own, removes it after cplt
// reads back the new preset (applyCpltChanges). cplt runs only after a yes.
func offerLeaveStrict() {
	if !isInteractive() || sessionPrompted || !cpltInstalled() {
		return
	}
	allowlist, ok := strictByOurAdvice()
	if !ok || !providerpkg.FirstTime("leave-strict-offer") || !claimSessionPrompt() {
		return
	}
	yes := false
	if err := askLeaveStrict(&yes); err != nil || !yes {
		return
	}
	cliPath, err := findCplt()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", domain.Yellow("⚠"), err)
		return
	}
	changes := []cpltChange{{"sandbox.preset", cpltStrictPreset, "standard"}}
	if ch := leavingStrictAllowlist(cpltStrictPreset, "standard", allowlist); ch != nil {
		changes = append(changes, *ch)
	}
	if err := applyCpltChanges(cliPath, changes, "", ""); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not leave strict: %v\n", domain.Yellow("⚠"), err)
	}
}

// ─── GitHub sign-in ──────────────────────────────────────────────────────────

type ghAuth int

const (
	ghAuthUnknown ghAuth = iota // gh ran but did not answer
	ghAuthMissing               // no gh on PATH
	ghAuthNone                  // not signed in, or the token no longer works
	ghAuthOK                    // signed in, token in a file or the environment
	ghAuthKeyring               // signed in, token in the system keyring
)

// ghAuthTimeout bounds `gh auth status`, which asks GitHub about the token.
const ghAuthTimeout = 5 * time.Second

// ghAuthProbe runs `gh auth status`. Setup and doctor only, never a launch.
// A var so tests need no gh.
var ghAuthProbe = func() (ghAuth, string) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return ghAuthMissing, ""
	}
	out, err := runBoundedTimeout(ghAuthTimeout, true, gh, "auth", "status", "--hostname", "github.com")
	return ghAuthFrom(out, err)
}

// ghAuthFrom reads `gh auth status`: exit 0 is signed in, exit 1 is not (or
// a token GitHub rejects), anything else (a kill at the deadline) is unknown.
func ghAuthFrom(out []byte, err error) (ghAuth, string) {
	var ee *exec.ExitError
	switch {
	case err == nil && bytes.Contains(out, []byte("(keyring)")):
		return ghAuthKeyring, ""
	case err == nil:
		return ghAuthOK, ""
	case errors.As(err, &ee) && ee.ExitCode() == 1:
		return ghAuthNone, ""
	default:
		return ghAuthUnknown, err.Error()
	}
}

// keychainBlocked is true where cplt keeps the client from the macOS
// Keychain, which is where gh keeps its token by default (#1348).
func keychainBlocked(client string) bool {
	return runtime.GOOS == "darwin" && (client == "opencode" || client == "pi")
}

// reportGHAuth prints the sign-in result and, when push or PRs would fail,
// the exact command that fixes it.
func reportGHAuth(w io.Writer, indent, client string, st ghAuth, detail string) {
	login := "gh auth login"
	if keychainBlocked(client) {
		login = "gh auth login --insecure-storage"
	}
	switch {
	case st == ghAuthMissing:
		fmt.Fprintf(w, "%s%s gh is not installed; the agent needs it to open PRs. Install it with %s, then run %s\n", indent, yellow("⚠"),
			bold(domain.PkgForInstall().Pick("brew install gh", "sudo apt install gh")), bold(login))
	case st == ghAuthNone:
		fmt.Fprintf(w, "%s%s gh is not signed in to github.com (or its token no longer works), so the agent cannot push or open PRs. Run %s\n", indent, yellow("⚠"), bold(login))
	case st == ghAuthKeyring && keychainBlocked(client):
		fmt.Fprintf(w, "%s%s gh keeps its token in the macOS Keychain, which cplt does not let %s read: push and PRs fail in the sandbox. Run %s\n", indent, yellow("⚠"), clientLabel[client], bold(login))
	case (st == ghAuthOK || st == ghAuthKeyring) && originUsesSSH():
		fmt.Fprintf(w, "%s%s gh is signed in to github.com: the agent can open PRs. This repo's origin uses SSH, which cplt blocks, so pushes fail. Run %s\n", indent, yellow("⚠"),
			bold(`git config --global url."https://github.com/".insteadOf "git@github.com:"`))
	case st == ghAuthOK || st == ghAuthKeyring:
		fmt.Fprintf(w, "%s%s gh is signed in to github.com: the agent can push branches and open PRs\n", indent, green("✓"))
	default:
		fmt.Fprintf(w, "%s%s Could not check the gh sign-in (%s). Run %s to see it.\n", indent, dim("-"), detail, bold("gh auth status"))
	}
}

// originUsesSSH is whether the current repo's origin is an SSH remote. cplt
// denies ~/.ssh and SSH_AUTH_SOCK, so a push over it fails in the sandbox
// whatever gh says. Setup and doctor only. A var so tests need no repo.
var originUsesSSH = func() bool {
	out, err := runBounded("git", "remote", "get-url", "origin")
	if err != nil {
		return false
	}
	u := strings.TrimSpace(string(out))
	return strings.Contains(u, "ssh://") || (!strings.Contains(u, "://") && strings.Contains(u, "@"))
}

// ─── the repo's suggested sandbox rules ──────────────────────────────────────

// offerCpltInit offers `cplt init --write` when the current repository has no
// .cplt.toml and cplt finds tooling to suggest rules for. It never approves
// them: trust is the user's own step.
func offerCpltInit(cliPath string) {
	root := source.FindGitRoot(".")
	if root == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(root, ".cplt.toml")); err == nil {
		return
	}
	preview := exec.Command(cliPath, "init", "-q")
	preview.Dir = root
	out, err := preview.Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return
	}
	detected := ""
	for _, line := range strings.Split(string(out), "\n") {
		if d, ok := strings.CutPrefix(line, "# Detected: "); ok {
			detected = "Detected: " + d + ". "
		}
	}
	yes := true
	if err := huh.NewConfirm().
		Title("Apply the suggested sandbox rules for this repo?").
		Description(detected + "Runs cplt init --write, which writes .cplt.toml. Nothing is approved until you run cplt trust accept.").
		Value(&yes).
		WithTheme(navTheme()).
		Run(); err != nil || !yes {
		return
	}
	write := exec.Command(cliPath, "init", "--write")
	write.Dir = root
	write.Stdout, write.Stderr = os.Stdout, os.Stderr
	if err := write.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s cplt init --write: %v\n", yellow("⚠"), err)
		return
	}
	fmt.Printf("  Review .cplt.toml, then commit it and approve its rules (cplt only trusts a committed file):\n    %s\n    %s\n", bold(`git add :/.cplt.toml && git commit -m "chore: add cplt sandbox rules" -- :/.cplt.toml`), bold("cplt trust accept"))
}
