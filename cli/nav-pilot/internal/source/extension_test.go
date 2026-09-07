package source

import "testing"

// TestKindByNameIsDerived pins the change that made adding a kind a one-line
// edit. KindByName used to be a map literal written out beside AllKinds, which
// is how `hook` came to be missing from three separate places after it was
// added (#649, #650, #708).
func TestKindByNameIsDerived(t *testing.T) {
	if len(KindByName) != len(AllKinds) {
		t.Errorf("KindByName has %d entries, AllKinds has %d: the map is not derived", len(KindByName), len(AllKinds))
	}
	for _, kind := range AllKinds {
		got, ok := KindByName[kind.Name]
		if !ok {
			t.Errorf("KindByName has no entry for %q", kind.Name)
			continue
		}
		if got != kind {
			t.Errorf("KindByName[%q] is not the kind from AllKinds", kind.Name)
		}
	}
}

// TestExtensionKind covers the shape #572 asked for: a team's own executable
// code, which had no artifact type at all and therefore could not be
// distributed. watson-developer's .github/extensions/auto-format/extension.mjs
// is the case this is derived from.
func TestExtensionKind(t *testing.T) {
	if !KindExtension.IsDir {
		t.Error("an extension is a directory: an extension is rarely one module")
	}
	if KindExtension.Marker != "extension.mjs" {
		t.Errorf("marker = %q, want extension.mjs: the marker is what makes a directory an extension rather than a stray folder", KindExtension.Marker)
	}
	if KindExtension.Suffix != "" {
		t.Errorf("suffix = %q, want empty for a directory kind", KindExtension.Suffix)
	}

	// Registered, not merely declared. Defining the kind and forgetting to put
	// it in AllKinds gives a type that exists in Go and nowhere else: the
	// resolver never lists it, install-everything never sees it, and nothing
	// fails. Every other assertion here passes in that state.
	if _, ok := KindByName[KindExtension.Name]; !ok {
		t.Error("extension is not in AllKinds, so nothing that iterates kinds will find it")
	}
}

// TestManifestNamesEveryKind pins that the source manifest can carry every kind
// the resolver knows. A kind the manifest cannot name is a kind "install
// everything" silently skips, which is what happened to extensions before #572:
// the resolver would have found them and the install table listed five kinds.
func TestManifestNamesEveryKind(t *testing.T) {
	m := &Manifest{
		Agents:       []string{"a"},
		Skills:       []string{"s"},
		Instructions: []string{"i"},
		Prompts:      []string{"p"},
		Hooks:        []string{"h"},
		Extensions:   []string{"e"},
	}
	for _, kind := range AllKinds {
		names, ok := m.NamesByKind(kind)
		if !ok {
			t.Errorf("the manifest cannot name %q, so install-everything would skip it", kind.Name)
			continue
		}
		if len(names) != 1 {
			t.Errorf("NamesByKind(%q) = %v, want the one fixture entry", kind.Name, names)
		}
	}
}
