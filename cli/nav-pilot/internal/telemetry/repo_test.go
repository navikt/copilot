package telemetry

import (
	"os/exec"
	"testing"
)

func TestParseNavRepo(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "ssh form", in: "git@github.com:navikt/foo.git", want: "navikt/foo"},
		{name: "ssh form without .git", in: "git@github.com:navikt/foo", want: "navikt/foo"},
		{name: "ssh url form", in: "ssh://git@github.com/navikt/foo.git", want: "navikt/foo"},
		{name: "https form with .git", in: "https://github.com/navikt/foo.git", want: "navikt/foo"},
		{name: "https form without .git", in: "https://github.com/navikt/foo", want: "navikt/foo"},
		{name: "https form with trailing slash", in: "https://github.com/navikt/foo/", want: "navikt/foo"},
		{name: "owner casing normalized", in: "git@github.com:NAVIKT/foo.git", want: "navikt/foo"},
		{name: "surrounding whitespace trimmed", in: "  git@github.com:navikt/foo.git\n", want: "navikt/foo"},
		{name: "non-navikt org omitted", in: "git@github.com:otherorg/foo.git", want: ""},
		{name: "non-navikt https omitted", in: "https://github.com/otherorg/foo.git", want: ""},
		{name: "non-github host omitted", in: "git@gitlab.com:navikt/foo.git", want: ""},
		{name: "local path omitted", in: "/home/user/repos/foo", want: ""},
		{name: "missing repo name omitted", in: "https://github.com/navikt", want: ""},
		{name: "extra path segments omitted", in: "https://github.com/navikt/foo/bar", want: ""},
		{name: "empty omitted", in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseNavRepo(tt.in); got != tt.want {
				t.Fatalf("parseNavRepo(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNavRepoFromDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	// initGitRepo creates a git repository in a temp dir, optionally with an
	// origin remote, and returns the dir.
	initGitRepo := func(t *testing.T, originURL string) string {
		t.Helper()
		dir := t.TempDir()
		if out, err := exec.Command("git", "-C", dir, "init", "--quiet").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		if originURL != "" {
			if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", originURL).CombinedOutput(); err != nil {
				t.Fatalf("git remote add: %v\n%s", err, out)
			}
		}
		return dir
	}

	t.Run("navikt ssh remote resolves to slug", func(t *testing.T) {
		dir := initGitRepo(t, "git@github.com:navikt/foo.git")
		if got := navRepoFromDir(dir); got != "navikt/foo" {
			t.Fatalf("navRepoFromDir() = %q, want navikt/foo", got)
		}
	})

	t.Run("navikt https remote resolves to slug", func(t *testing.T) {
		dir := initGitRepo(t, "https://github.com/navikt/foo")
		if got := navRepoFromDir(dir); got != "navikt/foo" {
			t.Fatalf("navRepoFromDir() = %q, want navikt/foo", got)
		}
	})

	t.Run("non-navikt remote yields empty", func(t *testing.T) {
		dir := initGitRepo(t, "git@github.com:otherorg/foo.git")
		if got := navRepoFromDir(dir); got != "" {
			t.Fatalf("navRepoFromDir() = %q, want empty", got)
		}
	})

	t.Run("repo without origin remote yields empty", func(t *testing.T) {
		dir := initGitRepo(t, "")
		if got := navRepoFromDir(dir); got != "" {
			t.Fatalf("navRepoFromDir() = %q, want empty", got)
		}
	})

	t.Run("not a git repo yields empty", func(t *testing.T) {
		if got := navRepoFromDir(t.TempDir()); got != "" {
			t.Fatalf("navRepoFromDir() = %q, want empty", got)
		}
	})
}
