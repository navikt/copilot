package telemetry

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// navRepoGitTimeout bounds the git invocation used for repo detection so a
// hung git (e.g. slow credential helper or network filesystem) can never
// delay a session launch noticeably.
const navRepoGitTimeout = 2 * time.Second

// detectNavRepo resolves the navikt repo slug for the current working
// directory. It is a package-level variable so tests can stub it and stay
// independent of whichever git checkout the tests happen to run inside
// (the telemetry tests are sequential-only, so save/restore is safe).
var detectNavRepo = func() string { return navRepoFromDir("") }

// navRepoFromDir returns the "owner/name" slug of the git origin remote in
// dir (or the process working directory when dir is empty), but only when
// the remote points to the navikt GitHub org. On any other outcome — not a
// git repo, no origin remote, git missing, timeout, non-navikt remote — it
// returns "" so the caller omits the attribute entirely: we never guess and
// never leak local paths or third-party remotes into telemetry.
func navRepoFromDir(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), navRepoGitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return parseNavRepo(string(out))
}

// parseNavRepo extracts a "navikt/<name>" slug from a git remote URL,
// accepting the common ssh (git@github.com:navikt/foo.git), ssh-URL
// (ssh://git@github.com/navikt/foo.git) and https
// (https://github.com/navikt/foo, with or without .git) forms. Any remote
// outside the navikt GitHub org yields "".
func parseNavRepo(remote string) string {
	remote = strings.TrimSpace(remote)

	var path string
	switch {
	case strings.HasPrefix(remote, "git@github.com:"):
		path = strings.TrimPrefix(remote, "git@github.com:")
	case strings.HasPrefix(remote, "ssh://git@github.com/"):
		path = strings.TrimPrefix(remote, "ssh://git@github.com/")
	case strings.HasPrefix(remote, "https://github.com/"):
		path = strings.TrimPrefix(remote, "https://github.com/")
	default:
		return ""
	}

	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] == "" {
		return ""
	}
	if !strings.EqualFold(parts[0], "navikt") {
		return ""
	}
	return "navikt/" + parts[1]
}
