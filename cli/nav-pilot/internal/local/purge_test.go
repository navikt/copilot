package local

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRemovablesFindsBothHalvesAndNamesTheirSize: a developer leaving the alpha
// gets back the toolchain and the weights, and the weights are the 23 GB.
//
// They are listed separately because they are removed for different reasons. The
// toolchain is nav-pilot's alone; the weights live in the shared Hugging Face
// cache, where another MLX tool on the machine may be using the same download.
func TestRemovablesFindsBothHalvesAndNamesTheirSize(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HF_HOME", filepath.Join(home, ".cache", "huggingface"))

	const model = "mlx-community/Some-Model-4bit"
	write := func(path string, size int) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(home, ".nav-pilot", "local", "venv", "lib", "pkg"), 4096)
	write(filepath.Join(home, ".cache", "huggingface", "hub",
		"models--mlx-community--Some-Model-4bit", "blobs", "weights"), 65536)

	items := Removables(model)
	if len(items) != 2 {
		t.Fatalf("Removables() returned %d entries, want the toolchain and the weights: %+v", len(items), items)
	}
	// Largest first: the weights are the number that decides whether a developer
	// bothers, and burying them under a 300 MB venv reads as "not worth it".
	if items[0].Bytes < items[1].Bytes {
		t.Errorf("Removables() is not largest-first: %d then %d", items[0].Bytes, items[1].Bytes)
	}
	if items[0].Bytes != 65536 {
		t.Errorf("weights measured %d bytes, want 65536", items[0].Bytes)
	}

	// A model nobody has downloaded contributes nothing, so purge on a machine
	// that never provisioned has nothing to offer to delete.
	if got := Removables("mlx-community/Never-Downloaded"); len(got) != 1 {
		t.Errorf("Removables() for an absent model returned %d entries, want only the toolchain", len(got))
	}
}
