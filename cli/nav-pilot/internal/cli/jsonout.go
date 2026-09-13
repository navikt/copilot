package cli

import (
	"encoding/json"
	"os"
)

// jsonStdout is where outputJSON writes. It is captured before any suppression
// so the document still reaches the real stdout while progress is silenced.
var jsonStdout *os.File = os.Stdout

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
	if !jsonOutput {
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
		devnull.Close()
	}
}

// outputJSON writes v to the real stdout, whether or not progress is suppressed.
func outputJSON(v interface{}) error {
	enc := json.NewEncoder(jsonStdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
