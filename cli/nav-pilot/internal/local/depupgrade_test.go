package local

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDepUpgradeRepo(t *testing.T) {
	for _, tc := range []struct {
		files []string
		want  string
	}{
		{[]string{"go.mod", "go.sum"}, "go"},
		{[]string{"go.mod"}, ""}, // no lockfile: nothing checks the versions
		{[]string{"package.json", "pnpm-lock.yaml"}, "npm"},
		{[]string{"package-lock.json"}, ""}, // lockfile, no package.json
		{[]string{"gradlew", "gradle/libs.versions.toml"}, "gradle"},
		{[]string{"app.csproj", "packages.lock.json"}, "dotnet"},
		{[]string{"Dockerfile"}, ""},
	} {
		dir := t.TempDir()
		for _, f := range tc.files {
			p := filepath.Join(dir, f)
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, nil, 0o644)
		}
		if got := DepUpgradeRepo(dir); got != tc.want {
			t.Errorf("%v: got %q, want %q", tc.files, got, tc.want)
		}
	}
}

func TestPinnedFile(t *testing.T) {
	for p, want := range map[string]bool{
		".tool-versions": true, "mise.toml": true, "Dockerfile": true, "app/Dockerfile.prod": true,
		".github/workflows/build.yaml": true, "docker-compose.yml": true, ".nais/app.yaml": true,
		"gradle/wrapper/gradle-wrapper.properties": true, "global.json": true,
		"go.mod": false, "package.json": false, "build.gradle.kts": false, "pnpm-lock.yaml": false,
	} {
		if PinnedFile(p) != want {
			t.Errorf("PinnedFile(%q) = %v, want %v", p, !want, want)
		}
	}
}

func TestDepUpgradePolicyText(t *testing.T) {
	if DepUpgradePolicyText("") != "" {
		t.Fatal("an ineligible repo must add nothing to the policy")
	}
	got := DepUpgradePolicyText("go")
	for _, want := range []string{"local-worker", "Dockerfile", ".github/", "postgres", "mise.toml"} {
		if !strings.Contains(got, want) {
			t.Errorf("policy text lacks %q", want)
		}
	}
	// Off by default: existing setups get byte-identical policy.
	SetDepUpgrade(false)
	if ProjectDepUpgrade() != "" {
		t.Fatal("local_dep_upgrade off must add nothing")
	}
}
