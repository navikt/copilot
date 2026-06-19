package cli

import (
	"fmt"
	"strings"
)

// cmdModels prints the nav-pilot curated model list for the current client,
// with org-restriction guidance and fallback instructions.
// It does not query the live Copilot catalog (server-side, per-org) — it shows
// the curated list from the provider and explains where restrictions come from.
func cmdModels(jsonOutput bool) error {
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	resolved := resolve(cfg, CLIOverrides{})

	p, err := providerFor(resolved.Client)
	if err != nil {
		return err
	}
	models := p.KnownModels()

	if jsonOutput {
		fmt.Print("[\n")
		for i, m := range models {
			comma := ","
			if i == len(models)-1 {
				comma = ""
			}
			fmt.Printf("  {\"id\": %q, \"label\": %q}%s\n", m.ID, m.Label, comma)
		}
		fmt.Print("]\n")
		return nil
	}

	fmt.Printf("%s  Known models (%s)\n", bold("📋 nav-pilot"), resolved.Client)
	fmt.Println()

	maxLen := 0
	for _, m := range models {
		if len(m.ID) > maxLen {
			maxLen = len(m.ID)
		}
	}
	for _, m := range models {
		padding := strings.Repeat(" ", maxLen-len(m.ID))
		fmt.Printf("  %s%s  %s\n", bold(m.ID), padding, dim(m.Label))
	}

	fmt.Println()
	fmt.Println(dim("  Availability depends on your GitHub Copilot plan and organization policy."))
	fmt.Println(dim("  Org admins manage model access at: github.com/<org>/settings/copilot/policies"))
	fmt.Println()
	fmt.Printf("  %s If Copilot rejects a model and falls back unexpectedly:\n", yellow("⚠"))
	fmt.Printf("    1. Switch to auto:   %s\n", bold("nav-pilot config set model auto"))
	fmt.Printf("    2. Check your org:   %s\n", dim("github.com/<your-org>/settings/copilot/policies"))
	fmt.Printf("    3. See model config: %s\n", bold("nav-pilot config explain model"))
	fmt.Println()

	return nil
}
