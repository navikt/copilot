package cli

import (
	"fmt"
	"os/exec"

	"github.com/charmbracelet/huh"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// cmdConfigSandbox runs an interactive wizard to configure the cplt sandbox profile.
func cmdConfigSandbox() error {
	cliPath, cliName := providerpkg.FindCopilotCLI()
	if cliPath == "" || cliName != "cplt" {
		return fmt.Errorf("cplt (Copilot Sandbox) is not available on your PATH. This command requires cplt")
	}

	var choices []string
	err := huh.NewMultiSelect[string]().
		Title("Configure cplt sandbox relaxations").
		Description("Select which restrictions to lift for agents running under cplt.").
		Options(
			huh.NewOption("Allow Docker (Colima/OrbStack)", "sandbox.allow_docker"),
			huh.NewOption("Allow any localhost port", "sandbox.allow_localhost_any"),
			huh.NewOption("Allow browser access", "sandbox.allow_browser"),
			huh.NewOption("Allow executing /tmp binaries", "sandbox.allow_tmp_exec"),
		).
		Value(&choices).
		WithTheme(navTheme()).
		Run()

	if err != nil {
		return fmt.Errorf("prompt cancelled: %w", err)
	}

	keys := []string{"sandbox.allow_docker", "sandbox.allow_localhost_any", "sandbox.allow_browser", "sandbox.allow_tmp_exec"}
	for _, key := range keys {
		val := "false"
		for _, c := range choices {
			if c == key {
				val = "true"
				break
			}
		}
		out, err := exec.Command(cliPath, "config", "set", key, val).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set %s: %v\n%s", key, err, string(out))
		}
	}

	fmt.Printf("%s Successfully updated cplt sandbox configuration\n", domain.Green("✓"))
	return nil
}
