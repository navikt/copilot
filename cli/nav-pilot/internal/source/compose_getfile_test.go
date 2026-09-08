package source

import (
	"path/filepath"
	"testing"
)

// MapLocalPath må kunne navngi kildestien til et arvet artefakt. Klarte den
// ikke det, falt den tilbake på den lokale stien, og sync lette etter den i
// feil repo og meldte at fila var slettet oppstrøms. Første sync --apply etter
// install av en gjenbrukende pakke slettet da alt basen bidro med (#572).
func TestGetFileFindsInheritedArtifact(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writeAgent(t, baseDir, "felles", "fra basen")
	writeAgent(t, ownDir, "eget", "eget")
	r := NewSourceResolver(ownDir).WithBase(NewSourceResolver(baseDir))

	abs, rel, ok := r.GetFile(KindAgent.Dir, "felles.agent.md")
	if !ok {
		t.Fatal("GetFile fant ikke det arvede artefaktet")
	}
	if rel != filepath.Join("agents", "felles.agent.md") {
		t.Errorf("rel = %q, ventet agents/felles.agent.md", rel)
	}
	if want := filepath.Join(baseDir, "agents", "felles.agent.md"); abs != want {
		t.Errorf("abs = %q, ventet %q", abs, want)
	}
}

// Sync spør hvilken kilde som faktisk har fila. Uten kjeding svarte den nei
// for alt basen bidro med.
func TestSourceRootForWalksTheChain(t *testing.T) {
	baseDir, ownDir := t.TempDir(), t.TempDir()
	writeAgent(t, baseDir, "felles", "fra basen")
	writeAgent(t, ownDir, "eget", "eget")
	r := NewSourceResolver(ownDir).WithBase(NewSourceResolver(baseDir))

	for _, tc := range []struct{ rel, want string }{
		{filepath.Join("agents", "eget.agent.md"), ownDir},
		{filepath.Join("agents", "felles.agent.md"), baseDir},
	} {
		got, ok := r.SourceRootFor(tc.rel)
		if !ok {
			t.Errorf("%s ble ikke funnet i noen kilde", tc.rel)
			continue
		}
		if got != tc.want {
			t.Errorf("%s løste til %s, ventet %s", tc.rel, got, tc.want)
		}
	}
	if _, ok := r.SourceRootFor(filepath.Join("agents", "finnes-ikke.agent.md")); ok {
		t.Error("en fil ingen av kildene har ble funnet")
	}
}
