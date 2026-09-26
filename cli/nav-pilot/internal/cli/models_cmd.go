package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// cmdModels is `nav-pilot models [filter...] [--client <name>] [--json]`. It
// parses its own flags so --client can name the client whose list to show,
// before or after the command, without the launch it means elsewhere.
//
// It does not query the live Copilot catalog (server-side, per-org) — it shows
// the curated list from the provider and explains where restrictions come from.
func cmdModels(args []string, overrides CLIOverrides) error {
	var jsonOutput bool
	var filters []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--json":
			jsonOutput = true
		case "--client":
			if i+1 >= len(args) {
				return fmt.Errorf("--client requires a value")
			}
			i++
			overrides.Client = args[i]
		case "-h", "--help":
			printHelp(os.Stdout, "models")
			return nil
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf("unknown flag %s. Run nav-pilot help models for its flags", a)
			}
			filters = append(filters, strings.ToLower(a))
		}
	}
	if overrides.Client != "" && !containsStr(validProviderIDs, overrides.Client) {
		return fmt.Errorf("--client %q is not valid (allowed: %s)", overrides.Client, strings.Join(validProviderIDs, ", "))
	}

	cfg, err := readConfig()
	if err != nil {
		return err
	}
	resolved := resolve(cfg, overrides)

	p, err := providerFor(resolved.Client)
	if err != nil {
		return err
	}
	all := p.KnownModels()
	var models []domain.ModelChoice
	for _, m := range all {
		if matchesAll(strings.ToLower(m.ID+" "+m.Label), filters) {
			models = append(models, m)
		}
	}
	current := resolved.Model

	if jsonOutput {
		type entry struct {
			ID      string `json:"id"`
			Label   string `json:"label"`
			Current bool   `json:"current,omitempty"`
		}
		out := make([]entry, 0, len(models))
		for _, m := range models {
			out = append(out, entry{m.ID, m.Label, isCurrentModel(m.ID, current)})
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("%s  Known models (%s)\n", bold("📋 nav-pilot"), resolved.Client)
	fmt.Println()
	if len(all) == 0 {
		fmt.Printf("  %s keeps its own model list; nav-pilot has none to show.\n\n", resolved.Client)
		return nil
	}
	if len(models) == 0 {
		fmt.Printf("  No model matches %q. Run nav-pilot models for the whole list.\n\n", strings.Join(filters, " "))
		return nil
	}

	width := 0
	for _, m := range models {
		width = max(width, len(m.ID))
	}
	hasLocal := false
	for _, m := range models {
		mark, label := "  ", dim(m.Label)
		if isCurrentModel(m.ID, current) {
			mark, label = green("* "), label+" "+green("(current)")
		}
		hasLocal = hasLocal || strings.HasSuffix(m.Label, "(local)")
		fmt.Printf("%s%s%s  %s\n", mark, bold(m.ID), strings.Repeat(" ", width-len(m.ID)), label)
	}

	fmt.Println()
	switch {
	case current == "":
		fmt.Println(dim("  No model is set, so the client picks. Set one: nav-pilot config set model <id>"))
	case !slices.ContainsFunc(all, func(m domain.ModelChoice) bool { return isCurrentModel(m.ID, current) }):
		fmt.Println(dim(fmt.Sprintf("  Your model, %s, is not in this list.", current)))
	}
	if hasLocal {
		fmt.Println(dim("  Local entries are chosen with `nav-pilot alpha local use <key>`; `model` is the session model."))
	}
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

// matchesAll reports whether s contains every filter word.
func matchesAll(s string, filters []string) bool {
	for _, f := range filters {
		if !strings.Contains(s, f) {
			return false
		}
	}
	return true
}

// isCurrentModel reports whether a list entry is the configured model. opencode
// lists github-copilot/<id> and runs a bare Copilot id as exactly that.
func isCurrentModel(id, current string) bool {
	return current != "" && (id == current || id == "github-copilot/"+current)
}
