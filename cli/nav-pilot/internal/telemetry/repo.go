package telemetry

import (
	"os"
	"path/filepath"
	"strings"
)

// detectNavRepo resolves the navikt repo slug for the current working
// directory. It is a package-level variable so tests can stub it and stay
// independent of whichever git checkout the tests happen to run inside
// (the telemetry tests are sequential-only, so save/restore is safe).
var detectNavRepo = func() string { return navRepoFromDir("") }

// navRepoFromDir returns the "owner/name" slug of the git origin remote in
// dir (or the process working directory when dir is empty), but only when
// the remote points to the navikt GitHub org. On any other outcome — not a
// git repo, no origin remote, non-navikt remote — it returns "" so the
// caller omits the attribute entirely: we never guess and never leak local
// paths or third-party remotes into telemetry.
//
// It reads .git/config itself rather than run `git remote get-url`: a launch
// with telemetry on may not take longer than one with it off, and starting
// git took 20-40 ms. ponytail: ignores includes and url.insteadOf; such a
// remote is simply not attributed.
func navRepoFromDir(dir string) string {
	if dir == "" {
		var err error
		if dir, err = os.Getwd(); err != nil {
			return ""
		}
	}
	for {
		gitDir := filepath.Join(dir, ".git")
		if b, err := os.ReadFile(gitDir); err == nil {
			// A worktree or submodule: .git names the real directory, and a
			// worktree's config lives in the common one.
			rest, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
			if !ok {
				return ""
			}
			gitDir = strings.TrimSpace(rest)
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(dir, gitDir)
			}
			if c, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
				common := strings.TrimSpace(string(c))
				if !filepath.IsAbs(common) {
					common = filepath.Join(gitDir, common)
				}
				gitDir = common
			}
		}
		if config, err := os.ReadFile(filepath.Join(gitDir, "config")); err == nil {
			return parseNavRepo(originURL(string(config)))
		} else if _, serr := os.Stat(gitDir); serr == nil {
			return "" // a .git without a config: never the parent's remote
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// originURL is the url of [remote "origin"] in a git config file.
func originURL(config string) string {
	in := false
	for line := range strings.Lines(config) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			in = strings.EqualFold(strings.Join(strings.Fields(line), " "), `[remote "origin"]`)
			continue
		}
		if k, v, ok := strings.Cut(line, "="); in && ok && strings.EqualFold(strings.TrimSpace(k), "url") {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
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
