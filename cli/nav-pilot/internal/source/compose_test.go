package source

import (
	"os"
	"path/filepath"
	"testing"
)

// En pakke som gjenbruker en annen arver det den ikke har selv, og skygger det
// den har. Kollisjonsregelen er at nærmest vinner, og den vinner ved at egen
// kilde spørres først (#572).
func writeAgent(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agents", name+".agent.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func composed(t *testing.T) (*SourceResolver, string, string) {
	t.Helper()
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writeAgent(t, baseDir, "felles", "fra basen")
	writeAgent(t, baseDir, "grillmester", "basens grillmester")
	writeAgent(t, ownDir, "grillmester", "egen grillmester")
	writeAgent(t, ownDir, "eget", "bare her")
	return NewSourceResolver(ownDir).WithBase(NewSourceResolver(baseDir)), baseDir, ownDir
}

func TestReusedArtifactIsInherited(t *testing.T) {
	r, _, _ := composed(t)
	got, ok := r.Get(KindAgent, "felles")
	if !ok {
		t.Fatal("artefakt bare basen har ble ikke funnet")
	}
	body, err := os.ReadFile(got.AbsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "fra basen" {
		t.Errorf("innhold = %q, ventet basens", body)
	}
}

func TestOwnArtifactShadowsReused(t *testing.T) {
	r, _, ownDir := composed(t)
	got, ok := r.Get(KindAgent, "grillmester")
	if !ok {
		t.Fatal("grillmester ble ikke funnet")
	}
	body, _ := os.ReadFile(got.AbsPath)
	if string(body) != "egen grillmester" {
		t.Errorf("innhold = %q, ventet pakkens eget", body)
	}
	if filepath.Dir(filepath.Dir(got.AbsPath)) != ownDir {
		t.Errorf("løste til %s, ventet noe under egen kilde %s", got.AbsPath, ownDir)
	}
}

func TestListUnionsBothSourcesOnce(t *testing.T) {
	r, _, _ := composed(t)
	seen := map[string]int{}
	for _, res := range r.List(KindAgent) {
		seen[res.Name]++
	}
	for _, name := range []string{"felles", "grillmester", "eget"} {
		if seen[name] != 1 {
			t.Errorf("%s listet %d ganger, ventet 1 (alle: %v)", name, seen[name], seen)
		}
	}
}

// Uten base skal ingenting endres. Kontrollen finnes fordi hele kjedingen er
// lagt inn i Get og List, som alle installasjoner går gjennom.
func TestNoBaseResolvesAsBefore(t *testing.T) {
	dir := t.TempDir()
	writeAgent(t, dir, "eneste", "x")
	r := NewSourceResolver(dir)
	if _, ok := r.Get(KindAgent, "finnes-ikke"); ok {
		t.Error("fant et artefakt som ikke finnes")
	}
	if got := len(r.List(KindAgent)); got != 1 {
		t.Errorf("listet %d artefakter, ventet 1", got)
	}
}
