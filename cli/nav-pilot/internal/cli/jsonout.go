package cli

import (
	"encoding/json"
	"fmt"
	"os"
)

// jsonStdout is the real stdout saved while progress is suppressed, and nil
// otherwise. It must not be initialised from os.Stdout at package load: tests
// swap os.Stdout to capture output, and a value cached at init would send the
// document to a pipe nobody is reading.
var jsonStdout *os.File

// suppressHumanOutput sends stdout to /dev/null for the duration of a
// JSON-producing command and returns a function that restores it.
//
// The alternative was gating every fmt.Print in the install path on jsonOutput.
// There are over a hundred in internal/cli, and the one that broke `--json`
// last was added without anyone noticing the flag existed (#808). A message
// added after this is quiet by construction instead of by memory.
//
// Only stdout is redirected. Warnings go to stderr and stay visible: a caller
// parsing stdout still wants to know what went wrong.
func suppressHumanOutput(jsonOutput bool) func() {
	// Nesting is how the whole install dispatch stays covered: the command
	// wraps its dispatch once and the installers still wrap themselves, so an
	// inner call must not save /dev/null as the stdout the document goes to.
	if !jsonOutput || jsonStdout != nil {
		return func() {}
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		// Nothing to do about it, and failing a command because /dev/null is
		// unavailable would be worse than a noisy document.
		return func() {}
	}
	prev := os.Stdout
	jsonStdout = prev
	os.Stdout = devnull
	return func() {
		os.Stdout = prev
		jsonStdout = nil
		if err := devnull.Close(); err != nil {
			// Nothing was written to /dev/null worth losing, so this is worth
			// saying and not worth failing a finished command over.
			fmt.Fprintf(os.Stderr, "%s could not close %s: %v\n", yellow("⚠"), os.DevNull, err)
		}
	}
}

// outputJSON writes v to the real stdout, whether or not progress is suppressed.
func outputJSON(v interface{}) error {
	w := jsonStdout
	if w == nil {
		w = os.Stdout
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// withPakkeName writes the installed agentpakke's name into doc under both
// keys it is published as, and returns doc so it can wrap a map literal.
//
// "collection" is what the state file has always been keyed on and what every
// existing consumer reads, so it never disappears; "agentpakke" is the same
// value under the name the rest of the binary uses (navikt/copilot#878).
// Every command that names the pakke in --json goes through here, so the two
// keys cannot drift apart one emit site at a time.
func withPakkeName(doc map[string]interface{}, name string) map[string]interface{} {
	doc["collection"] = name
	doc["agentpakke"] = name
	return doc
}
