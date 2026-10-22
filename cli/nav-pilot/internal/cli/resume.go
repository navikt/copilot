package cli

import (
	"errors"
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
		return nil, errResumeAndContinue
	}
	var add, clientFlags []string
	var valueFlags map[string]bool
	switch client {
	case "copilot":
		clientFlags = []string{"--resume", "-r", "--continue"}
		valueFlags = copilotValueFlags
		switch {
		case s.Continue:
			add = []string{"--continue"}
		case s.ID != "":
			add = []string{"--resume=" + s.ID} // one word: Copilot's value is optional
		default:
			add = []string{"--resume"}
		}
	case "opencode":
		clientFlags = []string{"--session", "-s", "--continue", "-c"}
		valueFlags = openCodeValueFlags
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
	for i := 0; i < len(extra); i++ {
		a := extra[i]
		if a == "--" {
			break
		}
		if valueFlags[a] {
			i++ // its value, which may look like a flag: -p '--resume'
			continue
		}
		name, _, _ := strings.Cut(a, "=")
		// -r<id> and -s<id>: a short flag with its value attached.
		if len(name) > 2 && name[1] != '-' {
			name = name[:2]
		}
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

var errResumeAndContinue = errors.New("--resume and --continue cannot be used together: pick one")

// sessionIDArg reads the optional id of the --resume at args[i]. ok says
// args[i+1] is that id (and may be "", from --resume=); a word that is a flag,
// "--" or a nav-pilot command is not one, and the client's picker opens.
func sessionIDArg(args []string, i int) (id string, ok bool) {
	if i+1 >= len(args) {
		return "", false
	}
	next := args[i+1]
	if strings.HasPrefix(next, "-") || isKnownCommand(next) {
		return "", false
	}
	return next, true
}

// openCodeValueFlags are OpenCode's options that take a value, from
// `opencode --help` and `opencode run --help` of v2.1.3 (2026-10-23).
// -s/--session is left out: it is a conflict, read above.
var openCodeValueFlags = setOf("--server", "--prompt", "-m", "--model", "--agent",
	"--format", "-f", "--file", "--title", "--completions", "--log-level")
