package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// cpltRunner runs a single `cplt config set <key> <val>` invocation and returns
// the combined output and any error. Injected for testability.
type cpltRunner func(ctx context.Context, cliPath, key, val string) ([]byte, error)

// defaultCpltRunner calls cplt via exec.CommandContext with a shared context.
func defaultCpltRunner(ctx context.Context, cliPath, key, val string) ([]byte, error) {
	return exec.CommandContext(ctx, cliPath, "config", "set", key, val).CombinedOutput()
}

// securitySettings returns the ordered list of key/value pairs to apply for the
// given user choices. Extracted for unit testing without I/O.
func securitySettings(enableGuards, enableProxyForced, enableDefaultAllowlist bool, allowedDomainsPath string) [][2]string {
	var settings [][2]string
	if enableGuards {
		settings = append(settings,
			[2]string{"gh_guard.enabled", "true"},
			[2]string{"gh_guard.mode", "block"},
			[2]string{"git_guard.enabled", "true"},
			[2]string{"git_guard.mode", "block"},
		)
	} else {
		settings = append(settings,
			[2]string{"gh_guard.enabled", "false"},
			[2]string{"git_guard.enabled", "false"},
		)
	}
	// Always set proxy.forced to reflect the user's explicit choice so that
	// re-running the wizard can disable a previously enabled setting.
	if enableProxyForced {
		settings = append(settings, [2]string{"proxy.forced", "true"})
	} else {
		settings = append(settings, [2]string{"proxy.forced", "false"})
	}
	if enableDefaultAllowlist {
		settings = append(settings, [2]string{"proxy.default_allowlist", "true"})
		if allowedDomainsPath != "" {
			settings = append(settings, [2]string{"proxy.allowed_domains", allowedDomainsPath})
		}
	} else {
		settings = append(settings, [2]string{"proxy.default_allowlist", "false"})
	}
	return settings
}

func normalizeAllowedDomains(raw string) ([]string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	if len(parts) == 0 {
		return nil, nil
	}

	var out []string
	seen := make(map[string]struct{})
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), "\"'")
		if p == "" {
			continue
		}
		host, err := extractHost(p)
		if err != nil {
			return nil, err
		}
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			return nil, fmt.Errorf("invalid domain input: %q", p)
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	sort.Strings(out)
	return out, nil
}

func extractHost(s string) (string, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", fmt.Errorf("invalid URL %q: %w", s, err)
		}
		return u.Hostname(), nil
	}

	// Accept host, host:port, or host/path by parsing as https URL.
	u, err := url.Parse("https://" + s)
	if err != nil {
		return "", fmt.Errorf("invalid domain %q: %w", s, err)
	}
	return u.Hostname(), nil
}

func writeAllowedDomainsFile(domains []string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "cplt")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("could not create cplt config directory: %w", err)
	}
	path := filepath.Join(dir, "allowed-domains.txt")
	content := strings.Join(domains, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("could not write allowed domains file: %w", err)
	}
	return path, nil
}

// cmdConfigSecurity runs an interactive wizard to configure cplt security guards.
// It sets gh_guard, git_guard, and optionally proxy.forced via cplt config set.
// These are global-only cplt settings and cannot be set in .cplt.toml.
func cmdConfigSecurity() error {
	return cmdConfigSecurityWith(nil)
}

func cmdConfigSecurityWith(runner cpltRunner) error {
	cliPath, cliName := providerpkg.FindCopilotCLI()
	if cliPath == "" || cliName != "cplt" {
		return fmt.Errorf("cplt (Copilot Sandbox) is not available on your PATH. This command requires cplt")
	}
	if runner == nil {
		runner = defaultCpltRunner
	}

	var enableGuards bool
	err := huh.NewConfirm().
		Title("Enable gh guard and git guard?").
		Description("Blocks destructive GitHub operations (gh pr merge, repo delete) and prevents unreviewed git pushes. Recommended for all users.").
		Affirmative("Yes (recommended)").
		Negative("No").
		Value(&enableGuards).
		WithTheme(navTheme()).
		Run()
	if err != nil {
		fmt.Println(dim("  Setup skipped — run 'nav-pilot config security' anytime."))
		return errors.New("setup aborted by user")
	}

	var enableProxyForced bool
	err = huh.NewConfirm().
		Title("Enable proxy.forced?").
		Description("Forces all agent network traffic through cplt's proxy, preventing raw socket bypass of domain filtering. Recommended for sensitive environments.").
		Affirmative("Yes").
		Negative("No (default)").
		Value(&enableProxyForced).
		WithTheme(navTheme()).
		Run()
	if err != nil {
		fmt.Println(dim("  Setup skipped — run 'nav-pilot config security' anytime."))
		return errors.New("setup aborted by user")
	}

	var enableDefaultAllowlist bool
	err = huh.NewConfirm().
		Title("Enable proxy.default_allowlist?").
		Description("Fail-closed network mode: only allowlisted domains can be contacted. Recommended for high-security/autonomous use.").
		Affirmative("Yes (recommended for strict mode)").
		Negative("No (open network)").
		Value(&enableDefaultAllowlist).
		WithTheme(navTheme()).
		Run()
	if err != nil {
		fmt.Println(dim("  Setup skipped — run 'nav-pilot config security' anytime."))
		return errors.New("setup aborted by user")
	}

	allowedDomainsPath := ""
	if enableDefaultAllowlist {
		var rawDomains string
		err = huh.NewInput().
			Title("Allowed domains (optional)").
			Description("Enter hostnames or URLs separated by comma/space/newline. Only hostnames are stored.").
			Placeholder("api.github.com, github.com, openai.com").
			Value(&rawDomains).
			Validate(func(s string) error {
				_, err := normalizeAllowedDomains(s)
				return err
			}).
			WithTheme(navTheme()).
			Run()
		if err != nil {
			fmt.Println(dim("  Setup skipped — run 'nav-pilot config security' anytime."))
			return errors.New("setup aborted by user")
		}
		domains, err := normalizeAllowedDomains(rawDomains)
		if err != nil {
			return err
		}
		path, err := writeAllowedDomainsFile(domains)
		if err != nil {
			return err
		}
		allowedDomainsPath = path
	}

	return applySecuritySettings(cliPath, enableGuards, enableProxyForced, enableDefaultAllowlist, allowedDomainsPath, runner)
}

// applySecuritySettings applies the computed settings via runner and prints a summary.
// Extracted to allow unit testing without interactive prompts.
func applySecuritySettings(cliPath string, enableGuards, enableProxyForced, enableDefaultAllowlist bool, allowedDomainsPath string, runner cpltRunner) error {
	// One shared context for the entire operation — 30s is generous enough for
	// cold-start CI environments (antivirus, IO locks) while still bounding hangs.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, s := range securitySettings(enableGuards, enableProxyForced, enableDefaultAllowlist, allowedDomainsPath) {
		out, err := runner(ctx, cliPath, s[0], s[1])
		if err != nil {
			return fmt.Errorf("failed to set %s: %v\n%s", s[0], err, string(out))
		}
	}

	fmt.Printf("\n%s cplt security guards configured\n", domain.Green("✓"))
	if enableGuards {
		fmt.Printf("  %s  gh_guard: block\n", domain.Green("✓"))
		fmt.Printf("  %s  git_guard: block\n", domain.Green("✓"))
	} else {
		fmt.Printf("  %s  gh_guard: disabled\n", dim("–"))
		fmt.Printf("  %s  git_guard: disabled\n", dim("–"))
		fmt.Printf("  %s  Any previously set gh_guard.mode / git_guard.mode remains in cplt config but has no effect while guards are disabled.\n", dim("i"))
	}
	if enableProxyForced {
		fmt.Printf("  %s  proxy.forced: true\n", domain.Green("✓"))
	} else {
		fmt.Printf("  %s  proxy.forced: false\n", dim("–"))
	}
	if enableDefaultAllowlist {
		fmt.Printf("  %s  proxy.default_allowlist: true\n", domain.Green("✓"))
		if allowedDomainsPath != "" {
			fmt.Printf("  %s  proxy.allowed_domains: %s\n", domain.Green("✓"), allowedDomainsPath)
		}
	} else {
		fmt.Printf("  %s  proxy.default_allowlist: false\n", dim("–"))
	}
	fmt.Println()
	fmt.Println("  Change anytime:")
	fmt.Printf("    %s\n", dim("cplt config set gh_guard.enabled false"))
	fmt.Printf("    %s\n", dim("cplt config set gh_guard.mode audit"))
	fmt.Printf("    %s\n", dim("cplt config set git_guard.enabled false"))
	fmt.Printf("    %s\n", dim("cplt config set git_guard.mode audit"))
	fmt.Printf("    %s\n", dim("cplt config set proxy.forced false"))
	fmt.Printf("    %s\n", dim("cplt config set proxy.default_allowlist false"))
	fmt.Printf("    %s\n", dim("cplt config set proxy.default_allowlist true"))
	fmt.Println()
	return nil
}
