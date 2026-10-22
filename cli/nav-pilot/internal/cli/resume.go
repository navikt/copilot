package cli

import (
	"fmt"
	"slices"
	"strings"
)

// sessionClientArgs translates nav-pilot's --resume [id] / --continue into the
// client's own flag, added to the args after "--". nav-pilot keeps no
// session store: with no id, the client's own picker opens. A client with no
// matching flag is refused, never silently dropped.
func sessionClientArgs(client string, s SessionFlag, extra []string) ([]string, error) {
	if !s.Resume && !s.Continue {
		return extra, nil
	}
	if s.Resume && s.Continue {
		return nil, fmt.Errorf("--resume and --continue cannot be used together: pick one")
	}
	var add, clientFlags []string
	switch client {
	case "copilot":
		clientFlags = []string{"--resume", "-r", "--continue"}
		switch {
		case s.Continue:
			add = []string{"--continue"}
		case s.ID != "":
			add = []string{"--resume", s.ID}
		default:
			add = []string{"--resume"}
		}
	case "opencode":
		clientFlags = []string{"--session", "-s", "--continue", "-c"}
		switch {
		case s.Continue:
			add = []string{"--continue"}
		case s.ID != "":
			// OpenCode creates the session when the id does not exist.
			add = []string{"--session", s.ID}
		default:
			// Not mapped to --continue: that resumes the last session, not
			// the one the user would have picked.
			return nil, fmt.Errorf("OpenCode has no session picker: use nav-pilot --continue for the last session, or nav-pilot --resume <id> (ids: opencode session list)")
		}
	default:
		return nil, fmt.Errorf("--resume and --continue are not supported for client %s. Pass the client's own flag after --", client)
	}
	for _, a := range extra {
		if a == "--" {
			break
		}
		name, _, _ := strings.Cut(a, "=")
		if slices.Contains(clientFlags, name) {
			return nil, fmt.Errorf("%s after -- conflicts with nav-pilot's %s: use one of them", a, map[bool]string{true: "--continue", false: "--resume"}[s.Continue])
		}
	}
	// After the client's own args, before a "--" of theirs: an OpenCode
	// subcommand (run …) must stay first.
	at := len(extra)
	if j := slices.Index(extra, "--"); j >= 0 {
		at = j
	}
	return slices.Insert(slices.Clone(extra), at, add...), nil
}
