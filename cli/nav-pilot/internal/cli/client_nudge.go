package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// sessionPrompted is set by the first thing a run brings up on its own: the
// survey hint, the survey question, the client nudge (and news, #1024, when
// it comes). Each of them checks it first, so a session gets at most one.
var sessionPrompted bool

// claimSessionPrompt reports whether this run may bring something up, and
// takes the one slot if so.
func claimSessionPrompt() bool {
	if sessionPrompted {
		return false
	}
	sessionPrompted = true
	return true
}

// recordEffectiveClient writes the client a user runs today into config.toml
// when the file has no client key, once (#1029). A later change of the
// built-in default then reaches new installs only: nobody is moved to another
// client by an upgrade. Only in a terminal, where the run already writes the
// config (the first-run wizard); CI and a launch with client args after --
// write nothing. No file is left to the wizard, which writes the client
// itself: creating one here would keep a skipped wizard from coming back. A
// client key removed after this (config unset client) stays removed, because
// the marker says it was done.
func recordEffectiveClient() {
	if !isInteractive() {
		return
	}
	dir, err := telemetrypkg.GetConfigDir()
	if err != nil {
		return
	}
	if _, err := os.Stat(filepath.Join(dir, "seen-client-recorded")); err == nil {
		return
	}
	data, err := os.ReadFile(configPath())
	if err != nil {
		return
	}
	raw := map[string]any{}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return // config validate is where a broken file is reported
	}
	// agent is the retired name of client: a choice all the same.
	_, hasClient := raw["client"]
	_, hasAgent := raw["agent"]
	if !hasClient && !hasAgent {
		// No client key: the effective client is the built-in default.
		if updateConfigKey("client", tomlString(findKeyDef("client").defaultVal)) != nil {
			return // try again next run
		}
	}
	providerpkg.FirstTime("client-recorded")
}

// maybeClientNudge offers OpenCode, once, to a Copilot CLI user it would give
// something and that has it installed: with local models on, only OpenCode runs a cloud model with the
// local worker beside it, and only OpenCode has the dispatch gate (#1022).
// One line on stderr as the session starts; the same gates as the survey
// hint (terminal, not CI, not a launch after --, surveys = false and the
// telemetry opt-out turn it off), and never in a session that already brought
// something up.
func maybeClientNudge(client string) {
	cfg, _ := readConfig()
	r := resolve(cfg, CLIOverrides{Client: client})
	if r.Client != "copilot" || !r.LocalEnabled || !surveysAllowed(r) || sessionPrompted {
		return
	}
	// Only when opencode is installed: switching to a client that is not there
	// would end the next launch in "opencode not found".
	if p, err := providerFor("opencode"); err != nil || !p.Available() {
		return
	}
	if !providerpkg.FirstTime("opencode-nudge") || !claimSessionPrompt() {
		return
	}
	fmt.Fprintf(os.Stderr, "%s Du har lokale modeller på, men bare opencode lar en skymodell sende oppgaver til en lokal modell. Bytt klient: %s. Se https://ki-utvikling.nav.no/nav-pilot/klienter\n\n",
		dim("ℹ"), bold("nav-pilot config set client opencode"))
}
