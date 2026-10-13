package source

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

func writeCompositionFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func declareComposition(t *testing.T, dir, base string) {
	t.Helper()
	pin := ""
	if !filepath.IsAbs(base) {
		pin = `,"sha":"` + strings.Repeat("a", 40) + `"`
	}
	writeCompositionFile(t, dir, agentpakke.DeclarationPath,
		`{"contractVersion":"1","source":"`+base+`"`+pin+`}`)
}

func compositionSource(t *testing.T, dir string) *Source {
	t.Helper()
	writeCompositionFile(t, dir, agentpakke.ManifestPath,
		`{"contractVersion":"1","name":"test","description":"test","layout":{"agents":"custom-agents"},"clients":{"copilot":{"primaryAgents":["own"]}}}`)
	m, err := agentpakke.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return &Source{Dir: dir, Repo: dir, Pakke: m}
}

func TestComposeResolverNestedLegacyBase(t *testing.T) {
	ownDir, middleDir, bottomDir := t.TempDir(), t.TempDir(), t.TempDir()
	own := compositionSource(t, ownDir)
	compositionSource(t, bottomDir)
	// A manifest-less base still carries nested reuse, as it did in the CLI.
	declareComposition(t, ownDir, middleDir)
	declareComposition(t, middleDir, bottomDir)
	writeCompositionFile(t, ownDir, "custom-agents/shared.agent.md", "own")
	writeAgent(t, middleDir, "shared", "middle")
	writeAgent(t, middleDir, "middle", "middle")
	writeCompositionFile(t, bottomDir, "custom-agents/bottom.agent.md", "bottom")
	r, bases, err := ComposeResolver(NewSourceResolverForLayout(own.Dir, own.Pakke.Layout), own)
	if err != nil {
		t.Fatal(err)
	}
	defer bases.Cleanup()
	if len(bases) != 2 || bases.Nearest().Dir != middleDir || bases[1].Dir != bottomDir {
		t.Fatalf("bases = %v, want middle then bottom", bases)
	}
	if bases[0].Pakke != nil || bases[1].Pakke == nil {
		t.Fatal("base manifests were not loaded correctly")
	}
	for name, root := range map[string]string{"shared": ownDir, "middle": middleDir, "bottom": bottomDir} {
		got, ok := r.Get(KindAgent, name)
		if !ok || !strings.HasPrefix(got.AbsPath, root+string(filepath.Separator)) {
			t.Errorf("Get(%q) = (%v, %v), want content under %s", name, got, ok, root)
		}
	}
	bases.Cleanup()
	for _, dir := range []string{ownDir, middleDir, bottomDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("Cleanup removed local source %s: %v", dir, err)
		}
	}
}

func TestComposeResolverPayloadOnlySkipsDeclaration(t *testing.T) {
	dir := t.TempDir()
	writeCompositionFile(t, dir, agentpakke.ManifestPath,
		`{"contractVersion":"1","name":"payload","description":"test","clients":{"copilot":{"payloads":{"full":{"path":"plugin","primaryAgents":["own"]}}}}}`)
	writeCompositionFile(t, dir, agentpakke.DeclarationPath, "not JSON")
	m, err := agentpakke.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	src := &Source{Dir: dir, Repo: dir, Pakke: m}
	own := NewSourceResolver(dir)
	got, bases, err := ComposeResolver(own, src)
	if err != nil || got != own || bases.Nearest() != nil {
		t.Fatalf("ComposeResolver = (%v, %v, %v), want unchanged payload resolver", got, bases, err)
	}
}

func TestComposeResolverManifestErrorWrapsAndCleans(t *testing.T) {
	src := compositionSource(t, t.TempDir())
	declareComposition(t, src.Dir, "navikt/broken")
	cloneDir := filepath.Join(t.TempDir(), "clone")
	writeCompositionFile(t, cloneDir, agentpakke.ManifestPath, "not JSON")
	orig := CloneRemoteFn
	t.Cleanup(func() { CloneRemoteFn = orig })
	CloneRemoteFn = func(_, _ string) (*Source, error) {
		return &Source{Dir: cloneDir, TempDir: cloneDir}, nil
	}
	got, bases, err := ComposeResolver(NewSourceResolverForLayout(src.Dir, src.Pakke.Layout), src)
	var manifestErr *ReuseManifestError
	if got != nil || bases != nil || !errors.As(err, &manifestErr) {
		t.Fatalf("ComposeResolver = (%v, %v, %v), want manifest error and no owned sources", got, bases, err)
	}
	if manifestErr.Source != "navikt/broken" || manifestErr.Err == nil || !errors.Is(err, manifestErr.Err) {
		t.Fatalf("manifest error does not preserve source and cause: %#v", manifestErr)
	}
	if _, err := os.Stat(cloneDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("failed base checkout remains: %v", err)
	}
}

func TestCompositionSourceIdentity(t *testing.T) {
	dir := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{"Navikt/Example", "navikt/example"},
		{dir, dir + "/"},
		{dir, alias},
		{filepath.Join(dir, "missing"), dir + "/missing/"},
	} {
		if !sameCompositionSource(pair[0], pair[1]) {
			t.Errorf("%q and %q should have the same identity", pair[0], pair[1])
		}
	}
	if sameCompositionSource("navikt/one", "navikt/two") {
		t.Error("different repositories have the same identity")
	}
	// Exercise the public entry point, not only its comparison helper.
	src := compositionSource(t, dir)
	declareComposition(t, dir, alias)
	own := NewSourceResolverForLayout(dir, src.Pakke.Layout)
	got, bases, err := ComposeResolver(own, src)
	if err != nil || got != own || bases.Nearest() != nil {
		t.Fatalf("symlink self-reference composed: (%v, %v, %v)", got, bases, err)
	}
}
