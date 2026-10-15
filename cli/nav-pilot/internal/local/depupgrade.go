package local

import (
	"os"
	"path/filepath"
	"strings"
)

// Dependency upgrades as a routing class (alpha, #1821). Design and evidence:
// docs/local-dispatch.md, "Bounded dependency upgrades".
//
// The nais/examples run (navikt/mlx-workspace reports/2026-10-14-nais-
// examples-upgrade) split cleanly in two. Upgrading direct dependencies in a
// repo with a lockfile and a test command, then running the tests, the local
// worker finished every Go, npm, Gradle and .NET app with tests green. The
// open-ended part failed: tool pins, image tags and CI action versions got
// invented versions, a package manager that no longer matched the lockfile,
// and a postgres major bump. The difference is the check: a lockfile and a
// test suite catch a wrong dependency version, nothing catches a wrong tag.

var depUpgrade bool

// SetDepUpgrade records the local_dep_upgrade config key. Off by default.
func SetDepUpgrade(on bool) { depUpgrade = on }

// DepUpgrade reports whether bounded dependency upgrades may go to the worker.
func DepUpgrade() bool { return depUpgrade }

// depEcosystems is, per ecosystem, the lockfiles that make an upgrade
// checkable and the file whose presence means there is a test command.
// ponytail: a test command is inferred from the build file, not read from it;
// read package.json "scripts.test" etc. if false positives show up.
var depEcosystems = []struct {
	name      string
	lockfiles []string
	testFile  []string
}{
	{"go", []string{"go.sum"}, []string{"go.mod"}},
	{"npm", []string{"package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"}, []string{"package.json"}},
	{"gradle", []string{"gradle.lockfile", "gradle/libs.versions.toml"}, []string{"gradlew"}},
	{"dotnet", []string{"packages.lock.json"}, []string{"*.sln", "*.csproj"}},
}

// depUpgradeDir reports the first ecosystem in dir with both a lockfile (or
// version catalog) and something to run tests with, or "".
func depUpgradeDir(dir string) string {
	has := func(pattern string) bool {
		m, _ := filepath.Glob(filepath.Join(dir, pattern))
		return len(m) > 0
	}
	for _, e := range depEcosystems {
		lock, test := false, false
		for _, p := range e.lockfiles {
			lock = lock || has(p)
		}
		for _, p := range e.testFile {
			test = test || has(p)
		}
		if lock && test {
			return e.name
		}
	}
	return ""
}

func skipDepScanDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "dist", "build", "target", "bin", "obj", "out", "testdata":
		return true
	}
	return false
}

// DepUpgradeRepo reports the eligible project(s) in dir: the ecosystem name when
// only dir itself has both a lockfile (or version catalog) and a test file, or a
// comma-separated "<subdir> (<ecosystem>)" list when subdirectories up to two
// levels down qualify. "" means nothing in the repo gives the worker a check,
// and an upgrade there stays with the cloud.
func DepUpgradeRepo(dir string) string {
	rootEco := depUpgradeDir(dir)
	var subs []string
	var scan func(rel string, depth int)
	scan = func(rel string, depth int) {
		entries, err := os.ReadDir(filepath.Join(dir, rel))
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || skipDepScanDir(e.Name()) {
				continue
			}
			sub := filepath.ToSlash(filepath.Join(rel, e.Name()))
			if eco := depUpgradeDir(filepath.Join(dir, sub)); eco != "" {
				subs = append(subs, sub+" ("+eco+")")
			}
			if depth < 2 {
				scan(sub, depth+1)
			}
		}
	}
	scan("", 1)
	if len(subs) == 0 {
		return rootEco
	}
	if rootEco != "" {
		subs = append([]string{". (" + rootEco + ")"}, subs...)
	}
	return strings.Join(subs, ", ")
}

// PinnedFile reports whether path is one the worker must not bump: a tool
// pin, a container image tag or a CI action version. Nothing in the repo
// checks those, so a wrong version there only shows up later, in CI or prod.
func PinnedFile(path string) bool {
	p := filepath.ToSlash(path)
	base := filepath.Base(p)
	switch {
	case base == ".tool-versions", base == "mise.toml", base == ".mise.toml",
		base == ".nvmrc", base == ".node-version", base == ".python-version",
		base == "global.json", base == "gradle-wrapper.properties":
		return true
	case strings.HasPrefix(strings.ToLower(base), "dockerfile"), strings.HasSuffix(base, ".dockerfile"),
		base == "docker-compose.yml", base == "docker-compose.yaml", base == "compose.yml", base == "compose.yaml":
		return true
	case strings.Contains(p, ".github/workflows/"), strings.Contains(p, ".github/actions/"):
		return true
	case base == "nais.yaml", base == "nais.yml", strings.Contains(p, ".nais/"):
		return true
	}
	return false
}

// DepUpgradePolicyText is the paragraph added to the dispatch policy when
// local_dep_upgrade is on and the repo has at least one eligible project. eco
// is DepUpgradeRepo's answer; "" adds nothing.
func DepUpgradePolicyText(eco string) string {
	if eco == "" {
		return ""
	}
	return "\nDependency upgrades (" + eco + "): send direct dependency upgrades in the listed lockfile-and-test project(s) to `local-worker` instead of doing them yourself. " +
		"Send one task per project with its directory, the repo's own pinned package manager and the command that builds and tests it; accept the result only after that command passes on your side. " +
		"Projects without both a lockfile and a test command stay with you. " +
		"Do not send pinned files to `local-worker` (nav-pilot refuses its edits to them); keep these yourself and check each new version against the upstream registry before writing it: tool pins (.tool-versions, mise.toml, .nvmrc, global.json, the Gradle wrapper), " +
		"container image tags in Dockerfiles and compose files, GitHub Actions versions in .github/, and nais manifests. " +
		"Never upgrade a database or other infrastructure image or driver (postgres, redis, kafka) to a new major unless the user asked for that major.\n"
}

// ProjectDepUpgrade is DepUpgradeRepo for the working directory, or "" when
// the key is off. It is what the policy writer calls.
func ProjectDepUpgrade() string {
	if !depUpgrade {
		return ""
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return DepUpgradeRepo(wd)
}
