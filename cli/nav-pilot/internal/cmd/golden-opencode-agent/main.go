// Command golden-opencode-agent prints an agent file as nav-pilot installs it
// for OpenCode: rebuilt frontmatter and the tools allowlist as permissions.
// Used only by scripts/nav-pilot-golden.sh --client opencode.
//
//	go run ./internal/cmd/golden-opencode-agent <agent.md> <name>
package main

import (
	"fmt"
	"os"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: golden-opencode-agent <agent.md> <name>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(artifacts.OpenCodePrimaryAgent(data, os.Args[2]))
}
