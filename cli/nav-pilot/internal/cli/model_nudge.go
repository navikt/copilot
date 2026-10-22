package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// askModelNudge is the launch question; a var so tests answer.
var askModelNudge = func(title string, opts []huh.Option[string], choice *string) error {
	return runField(huh.NewSelect[string]().Title(title).Options(opts...).Value(choice))
}

// maybeModelNudge asks, once per change of the primary agent's recommended
// model, a user whose own model = differs from it what to run (#1920). The
// user's model wins over the agent's pin, so without this a new
// recommendation would never reach them.
//
// The value is recorded whatever happens next: a user without a model, or
// with the same one, is not asked about it later, and an answer (Ctrl-C and
// Esc keep the model) is not asked for again. A --model flag for this run is
// the user's choice for it and is not questioned. Without a terminal, or once
// the session's one prompt is taken, it is one line on stderr instead.
func maybeModelNudge(resolved ResolvedConfig, persona, stagedDir string) ResolvedConfig {
	pakke := source.ActivePakke().Name
	rec := providerpkg.RecommendedModel(resolved.Client, providerpkg.PrimaryAgentFile(resolved.Client, persona, stagedDir, resolved.ProjectDir))
	if rec == "" || !artifacts.SeenChanged("model-recommendation-"+pakke+"-"+persona, rec) {
		return resolved
	}
	cfg, _ := readConfig()
	if !providerpkg.UserSetModel(resolved.Client, resolved) || cfg == nil || cfg.Model == nil || *cfg.Model != resolved.Model {
		return resolved
	}
	if user, _ := providerpkg.CopilotModelID(strings.TrimSpace(resolved.Model)); strings.EqualFold(user, rec) {
		return resolved
	}
	if !isInteractive() || !claimSessionPrompt() {
		fmt.Fprintf(os.Stderr, "%s The %s agentpakke now recommends %s; you have model = %s set. Switch: %s  Follow the agentpakke: %s\n",
			dim("ℹ"), pakke, rec, resolved.Model, bold("nav-pilot config set model "+rec), bold("nav-pilot config unset model"))
		return resolved
	}
	choice := "keep"
	title := fmt.Sprintf("The %s agentpakke now recommends %s.\nYou have model = %s set.", pakke, rec, resolved.Model)
	err := askModelNudge(title, []huh.Option[string]{
		huh.NewOption("Keep "+resolved.Model, "keep"),
		huh.NewOption("Switch to "+rec, "switch"),
		huh.NewOption("Follow the agentpakke's recommendation (unset model)", "follow"),
	}, &choice)
	switch {
	case err != nil:
	case choice == "switch":
		if err := cmdConfigSet("model", rec); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", yellow("⚠"), err)
		} else {
			resolved.Model = rec
		}
	case choice == "follow":
		if err := cmdConfigUnset("model"); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", yellow("⚠"), err)
		} else {
			resolved.Model = ""
		}
	}
	return resolved
}
