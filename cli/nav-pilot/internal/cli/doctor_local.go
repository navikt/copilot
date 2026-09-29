package cli

import (
	"fmt"
	"io"
)

// contextWhy is the one line on why local models need a large context: it is
// what the Copilot session itself sends, not a choice nav-pilot made. Copilot
// CLI refuses to start when its static context is over about 80% of the
// window (mlx-workspace reports/2026-09-23-local-model-evaluation).
const contextWhy = "A Copilot session sends about 22k tokens (system prompt and tool definitions) before your first message, and Copilot will not start unless that fits in 80% of the context, so the server needs at least 30k tokens and 64k leaves room to work"

// contextTooSmall is what to do when that context does not fit.
const contextTooSmall = "If 30k tokens do not fit next to the model, you need a GPU with more memory or a machine with unified memory (such as a Mac with 32 GB or more), or use the cloud models, which need no setup"

// reportLocalModel is doctor's local model section. It only reads the
// config: the probes take minutes and belong to nav-pilot alpha local doctor.
func reportLocalModel(w io.Writer, r ResolvedConfig, managed bool) {
	state := "off"
	if r.LocalEnabled {
		state = "on"
	}
	switch {
	case r.LocalEndpoint != "":
		fmt.Fprintf(w, "    • Own server: %s, model %s (local dispatch %s)\n", r.LocalEndpoint, bold(r.LocalEndpointModel), state)
		fmt.Fprintf(w, "      Check context, tool calls and speed: %s\n", bold("nav-pilot alpha local doctor"))
	case managed:
		fmt.Fprintf(w, "    • Managed MLX server (local dispatch %s)\n", state)
		fmt.Fprintf(w, "      Status: %s\n", bold("nav-pilot alpha local status"))
	case r.LocalEnabled:
		// On, but the MLX environment is gone or pinned to older versions.
		fmt.Fprintf(w, "    • Local dispatch is on, but the MLX environment is missing or out of date. Set it up again: %s\n", bold("nav-pilot alpha local init"))
	default:
		fmt.Fprintf(w, "    • Not configured (optional, alpha). Set up: %s\n", bold("nav-pilot alpha local setup"))
		fmt.Fprintf(w, "      %s %s.\n", dim("Note:"), contextWhy)
		fmt.Fprintf(w, "      %s.\n", contextTooSmall)
	}
}
