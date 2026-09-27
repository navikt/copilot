package cli

import "fmt"

// cmdAuth dispatches `nav-pilot auth <subcommand>`.
func cmdAuth(args []string, jsonOutput bool) error {
	if len(args) == 0 {
		return fmt.Errorf("auth requires a subcommand.\n\nUsage: nav-pilot auth <subcommand>\n\nSubcommands:\n  login    Authenticate via GitHub device flow, storing the token in the OS keychain\n  status   Show current authentication status\n  logout   Remove the stored token")
	}

	switch args[0] {
	case "login":
		return cmdAuthLogin()
	case "status":
		return cmdAuthStatus(jsonOutput)
	case "logout":
		return cmdAuthLogout()
	default:
		return fmt.Errorf("unknown auth subcommand: %s\n\nUsage: nav-pilot auth <login|status|logout>", args[0])
	}
}
