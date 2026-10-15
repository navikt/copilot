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
		// Monorepo: eligible projects one and two levels below the repo root,
		// while projects without a lockfile or hidden/vendor dirs are skipped.
		{
			[]string{
				"quotes-frontend/package.json", "quotes-frontend/pnpm-lock.yaml",
				"quotes-go/go.mod", "quotes-go/go.sum",
				"quotes-kotlin/gradlew", "quotes-kotlin/gradle/libs.versions.toml",
				"quotes-dotnet/quotes-dotnet.csproj", "quotes-dotnet/packages.lock.json",
				"quotes-Loadgen/package.json",
				"quotes-migrator/Dockerfile",
				"node_modules/pkg/package.json", "node_modules/pkg/package-lock.json",
				".git/hooks/go.mod", ".git/hooks/go.sum",
			},
			"quotes-dotnet (dotnet), quotes-frontend (npm), quotes-go (go), quotes-kotlin (gradle)",
		},
		{
			[]string{
				"apps/my-copilot/package.json", "apps/my-copilot/pnpm-lock.yaml",
				"cli/nav-pilot/go.mod", "cli/nav-pilot/go.sum",
			},
			"apps/my-copilot (npm), cli/nav-pilot (go)",
		},
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
	got := DepUpgradePolicyText("quotes-frontend (npm), quotes-go (go)")
	for _, want := range []string{"local-worker", "quotes-frontend (npm)", "Dockerfile", ".github/", "postgres", "mise.toml"} {
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

func TestGatePinnedFiles(t *testing.T) {
	worker := func(path string) GateRequest {
		return GateRequest{Session: "w", Agent: WorkerAgent, Tool: "edit", Path: "/repo/" + path, Phase: "worker"}
	}
	workerBash := func(cmd string) GateRequest {
		return GateRequest{Session: "w", Agent: WorkerAgent, Tool: "bash", Command: cmd, Phase: "worker"}
	}
	on := newDispatchGate("", GateRules{Pinned: true})
	for _, p := range []string{"Dockerfile", ".github/workflows/ci.yaml", ".nais/app.yaml", ".tool-versions", "gradle/wrapper/gradle-wrapper.properties"} {
		if key, text := on.verify(worker(p)); key != "deny" || text != GatePinnedText {
			t.Errorf("%s: got %q %q, want the pinned refusal", p, key, text)
		}
	}
	if key, text := on.verify(workerBash("sed -i 's/16/18/' Dockerfile")); key != "deny" || text != GatePinnedText {
		t.Errorf("sed -i Dockerfile: got %q %q, want the pinned refusal", key, text)
	}
	if key, _ := on.verify(workerBash("go test ./...")); key != "" {
		t.Errorf("go test refused: %q", key)
	}
	if key, _ := on.verify(worker("go.mod")); key != "" {
		t.Errorf("go.mod refused: %q", key)
	}
	// local_dep_upgrade off: the worker's writes pass exactly as on main.
	off := newDispatchGate("", GateRules{Multi: true})
	if key, text := off.verify(worker("Dockerfile")); key != "" || text != "" {
		t.Errorf("flag off: got %q %q, want nothing", key, text)
	}
	if key, text := off.verify(workerBash("sed -i 's/16/18/' Dockerfile")); key != "" || text != "" {
		t.Errorf("flag off bash: got %q %q, want nothing", key, text)
	}
	if !off.workerWrote["w"] {
		t.Error("flag off: the worker's write was not recorded")
	}
}

func TestDepUpgradeOffChangesNothing(t *testing.T) {
	SetDepUpgrade(false)
	t.Chdir(t.TempDir())
	os.WriteFile("go.mod", nil, 0o644)
	os.WriteFile("go.sum", nil, 0o644)
	if got := DepUpgradePolicyText(ProjectDepUpgrade()); got != "" {
		t.Errorf("flag off adds policy text: %q", got)
	}
	if (GateRules{}).Any() {
		t.Error("flag off turns the gate on")
	}
}
