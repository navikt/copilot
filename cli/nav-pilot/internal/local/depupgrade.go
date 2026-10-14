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

// DepUpgradeRepo reports the first ecosystem in dir with both a lockfile (or
// version catalog) and something to run tests with. "" means the repo gives
// the worker no check, and an upgrade there stays with the cloud.
func DepUpgradeRepo(dir string) string {
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
// local_dep_upgrade is on and the project is eligible. eco is DepUpgradeRepo's
// answer; "" adds nothing.
func DepUpgradePolicyText(eco string) string {
	if eco == "" {
		return ""
	}
	return "\nDependency upgrades (" + eco + "): this project has a lockfile and tests, so a request to upgrade its direct dependencies may go to `local-worker`. " +
		"Send it the upgrade with the repo's own package manager and the command that builds and tests it; accept the result only after that command passes on your side. " +
		"Keep these yourself, or check each new version against the upstream registry before accepting it: tool pins (.tool-versions, mise.toml, .nvmrc, global.json, the Gradle wrapper), " +
		"container image tags in Dockerfiles and compose files, GitHub Actions versions in .github/, and nais manifests. " +
		"Never accept a new major of a database or other infrastructure image (postgres, redis, kafka) unless the user asked for that major.\n"
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
