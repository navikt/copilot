package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// writePakke lays down a minimal conforming agentpakke with the named agents.
func writePakke(t *testing.T, dir, name string, agents ...string) {
	t.Helper()
	for _, sub := range []string{".nav-pilot", "agents", "skills"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"contractVersion":"1","name":"` + name + `","description":"` + name +
		`","layout":{"agents":"agents","skills":"skills"},"clients":{"copilot":{"primaryAgents":["` + agents[0] + `"]}}}`
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		body := "---\nname: " + a + "\ndescription: fra " + name + "\n---\n" + name + "\n"
		if err := os.WriteFile(filepath.Join(dir, "agents", a+".agent.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func declareReuse(t *testing.T, dir, base string) {
	t.Helper()
	body := `{"contractVersion":"1","source":"` + base + `"}`
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot", "agentpakke.lock.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadSource(t *testing.T, dir string) *Source {
	t.Helper()
	src := &Source{Dir: dir, Repo: dir}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	return src
}

// En pakke som gjenbruker en annen installerer begges innhold, og sitt eget
// vinner der navnene kolliderer (#572).
func TestComposedInstallInheritsAndShadows(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writePakke(t, baseDir, "basepakke", "felles", "grillmester")
	writePakke(t, ownDir, "egenpakke", "eget", "grillmester")
	declareReuse(t, ownDir, baseDir)

	src := loadSource(t, ownDir)
	resolver, reused, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err != nil {
		t.Fatal(err)
	}
	if reused == nil || reused.Dir != baseDir {
		t.Fatalf("gjenbrukt kilde = %v, ventet %s", reused, baseDir)
	}
	manifest, err := pakkeContents(resolver, src)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(manifest.Agents, ","); got != "eget,felles,grillmester" {
		t.Errorf("agenter = %q, ventet eget,felles,grillmester", got)
	}
	got, ok := resolver.Get(KindAgent, "grillmester")
	if !ok {
		t.Fatal("grillmester ble ikke løst")
	}
	if !strings.HasPrefix(got.AbsPath, ownDir) {
		t.Errorf("grillmester løste til %s, ventet pakkens eget under %s", got.AbsPath, ownDir)
	}
}

// To pakker som gjenbruker hverandre har ingen rekkefølge å løses i. Uten
// vakta henter composeResolver dem vekselvis til stacken tar slutt.
func TestReuseCycleIsRefused(t *testing.T) {
	aDir, bDir := t.TempDir(), t.TempDir()
	writePakke(t, aDir, "a", "en")
	writePakke(t, bDir, "b", "to")
	declareReuse(t, aDir, bDir)
	declareReuse(t, bDir, aDir)

	src := loadSource(t, aDir)
	_, _, err := composeResolver(resolverFor(src.Dir, src.Pakke), src)
	if err == nil {
		t.Fatal("en gjenbrukssyklus ble godtatt")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("feilmeldinga nevner ikke syklusen: %v", err)
	}
}

// En pakke uten erklæring skal løses nøyaktig som før kjedinga fantes.
func TestPakkeWithoutDeclarationIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	writePakke(t, dir, "alene", "en")
	src := loadSource(t, dir)
	own := resolverFor(src.Dir, src.Pakke)
	composed, reused, err := composeResolver(own, src)
	if err != nil {
		t.Fatal(err)
	}
	if reused != nil {
		t.Errorf("fant en gjenbrukt kilde der ingen er erklært: %v", reused)
	}
	if composed.Base() != nil {
		t.Error("resolveren fikk en base uten at noe er erklært")
	}
	if _, err := agentpakke.LoadDeclaration(dir); err == nil {
		t.Error("testpakka har en erklæring, da tester dette noe annet enn det står")
	}
}
