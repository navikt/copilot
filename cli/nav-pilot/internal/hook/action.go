package hook

import (
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"
)

// The action check's first half: which shell commands are worth asking the
// local decide model about at all. It is a small, explicit list on purpose.
// The check runs before the tool call and its budget is half a second, so it
// must say nothing about `ls`, and a rule that fires on ordinary commands
// would bury the few that matter in the log.
//
// The rules read words, not a shell grammar. Quotes and $(…) split a command
// like ; and && do, so `bash -c "rm -rf x"` is still an rm, and so is
// `echo "rm -rf x"`: a false alarm costs one decide call and a log line, a
// missed rm costs more.

// ShellCommand returns a shell tool call's command and the description the
// model gave with it, from either dialect's toolArgs (OpenCode's arrive in
// Copilot's shape via hooks-bridge.js). ok is false for any other tool.
func ShellCommand(tool string, args json.RawMessage) (command, description string, ok bool) {
	if !shellTools[strings.ToLower(tool)] {
		return "", "", false
	}
	var a struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	if json.Unmarshal(args, &a) != nil || strings.TrimSpace(a.Command) == "" {
		return "", "", false
	}
	return a.Command, a.Description, true
}

var (
	segmentSep = regexp.MustCompile("&&|\\|\\||[;|&\n\"'`()]|\\$\\(")
	sqlClient  = regexp.MustCompile(`(?i)\b(psql|mysql|mariadb|sqlite3|sqlcmd)\b`)
	sqlDrop    = regexp.MustCompile(`(?i)\b(drop\s+[a-z]+|truncate)\b`)
)

// wrappers run the command after them; their own flags are skipped, and the
// value of each flag listed here with it (sudo -u root rm: rm, not root).
var wrappers = map[string][]string{
	"sudo":    {"-u", "-g", "-h", "-p", "-C", "-D", "-r", "-t", "-U", "-T"},
	"env":     {"-u", "-C", "-S", "-P"},
	"time":    {"-f", "-o"},
	"nohup":   nil,
	"exec":    {"-a"},
	"command": nil,
	"xargs":   {"-n", "-I", "-L", "-P", "-s", "-d", "-E", "-a"},
}

var mutating = map[string][]string{
	"kubectl":   {"delete", "apply", "create", "replace", "patch", "edit", "scale", "autoscale", "drain", "cordon", "uncordon", "taint", "set", "label", "annotate", "rollout"},
	"nais":      {"create", "delete", "apply", "deploy", "grant", "revoke", "set", "unset", "restart", "scale", "migrate", "prepare", "rollback", "enable", "disable", "update", "reset", "drop"},
	"gcloud":    {"create", "delete", "update", "deploy", "set", "patch", "resize", "reset", "stop", "restart", "import", "undelete", "rollback", "apply", "disable", "enable", "rm", "mv", "add-iam-policy-binding", "remove-iam-policy-binding", "set-iam-policy"},
	"helm":      {"install", "upgrade", "uninstall", "delete", "del", "un", "rollback"},
	"terraform": {"apply", "destroy"},
	"tofu":      {"apply", "destroy"},
}

// RiskyCommand returns the category of the first risky part of a shell
// command, or "" when there is none. The categories are telemetry's enum:
// kubectl, nais, gcloud, helm, terraform, rm, git, disk, sql.
func RiskyCommand(command string) string {
	if sqlClient.MatchString(command) && sqlDrop.MatchString(command) {
		return "sql"
	}
	for _, seg := range segmentSep.Split(command, -1) {
		if c := riskySegment(strings.Fields(seg)); c != "" {
			return c
		}
	}
	return ""
}

func riskySegment(w []string) string {
	// Leading VAR=value assignments and wrappers.
	for len(w) > 0 {
		if strings.Contains(w[0], "=") && !strings.HasPrefix(w[0], "-") {
			w = w[1:]
			continue
		}
		valued, ok := wrappers[w[0]]
		if !ok {
			break
		}
		w = w[1:]
		for len(w) > 0 && strings.HasPrefix(w[0], "-") {
			if slices.Contains(valued, w[0]) && len(w) > 1 {
				w = w[1:]
			}
			w = w[1:]
		}
	}
	if len(w) == 0 {
		return ""
	}
	name, args := path.Base(w[0]), w[1:]
	words := nonFlags(args)
	if dryRun(args) || (name == "gcloud" && len(words) > 0 && words[0] == "config") {
		return ""
	}
	switch name {
	case "kubectl":
		if i := slices.Index(words, "rollout"); i >= 0 && i+1 < len(words) && (words[i+1] == "status" || words[i+1] == "history") {
			return ""
		}
		fallthrough
	case "nais", "gcloud", "helm", "terraform", "tofu":
		for _, v := range words {
			if slices.Contains(mutating[name], v) {
				if name == "tofu" {
					return "terraform"
				}
				return name
			}
		}
	case "rm":
		for _, a := range args {
			if a == "--recursive" || (strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "rR")) {
				return "rm"
			}
		}
	case "dd", "mkfs", "wipefs", "shred":
		return "disk"
	case "git":
		return riskyGit(args)
	default:
		if strings.HasPrefix(name, "mkfs.") {
			return "disk"
		}
	}
	return ""
}

// riskyGit flags git push --force (and -f, +refspec), reset --hard and clean -f.
func riskyGit(args []string) string {
	// Global options before the subcommand; -C and -c take a value.
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if args[0] == "-C" || args[0] == "-c" {
			args = args[1:]
		}
		if len(args) > 0 {
			args = args[1:]
		}
	}
	if len(args) == 0 {
		return ""
	}
	sub, rest := args[0], args[1:]
	for _, a := range rest {
		switch sub {
		case "push":
			if strings.HasPrefix(a, "--force") || a == "-f" || (strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "f")) || strings.HasPrefix(a, "+") {
				return "git"
			}
		case "reset":
			if a == "--hard" {
				return "git"
			}
		case "clean":
			if a == "--force" || (strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "f")) {
				return "git"
			}
		}
	}
	return ""
}

// nonFlags are the words that are not flags. A flag's value counts as a word,
// since which flags take one is each tool's business: `-n delete` would read
// as the delete verb, which is a namespace nobody has.
func nonFlags(args []string) []string {
	var out []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			out = append(out, a)
		}
	}
	return out
}

// dryRun is true for --dry-run and --dry-run=<mode>, except kubectl's
// --dry-run=none, which runs for real.
func dryRun(args []string) bool {
	for _, a := range args {
		if a == "--dry-run" || (strings.HasPrefix(a, "--dry-run=") && a != "--dry-run=none") {
			return true
		}
	}
	return false
}
