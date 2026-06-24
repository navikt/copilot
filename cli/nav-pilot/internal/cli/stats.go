package cli

import (
	"fmt"
	"os"
	"os/exec"
)

var rtkLookPath = exec.LookPath

func cmdStats(discover, jsonOutput bool) error {
	if discover && jsonOutput {
		return fmt.Errorf("--json is not supported together with --discover")
	}

	rtkPath, err := rtkLookPath("rtk")
	if err != nil || rtkPath == "" {
		return fmt.Errorf("rtk is not installed or not on PATH — install it with your package manager and ensure the rtk binary is available on PATH")
	}
	if !jsonOutput {
		fmt.Println("🧭 nav-pilot stats")
		fmt.Println()
		if discover {
			fmt.Println("→ Looking for new RTK savings opportunities...")
		} else {
			fmt.Println("→ Fetching RTK savings...")
		}
	}

	args := []string{"gain"}
	if discover {
		args = []string{"discover"}
	}
	if jsonOutput {
		args = []string{"gain", "--all", "--format", "json"}
	}

	cmd := exec.Command(rtkPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return err
	}
	if !jsonOutput && !discover {
		fmt.Println()
		fmt.Println("Tips:")
		fmt.Println("  nav-pilot stats --discover")
		fmt.Println("  nav-pilot stats --json")
	}
	return nil
}
