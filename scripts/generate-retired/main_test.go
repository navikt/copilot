package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A hash only the committed record knows (its blob lived on a deleted branch)
// must survive regeneration; a re-added path must not.
func TestUnionKeepsCommittedHashes(t *testing.T) {
	doc := &document{Paths: map[string][]string{"agents/a.agent.md": {"bbb"}}}
	committed := &document{
		Paths: map[string][]string{
			"agents/a.agent.md":    {"aaa", "bbb"},
			"agents/gone.agent.md": {"ccc"},
			"agents/back.agent.md": {"ddd"},
		},
		Replacements: map[string]string{"agents/gone.agent.md": "new"},
	}
	union(doc, committed, func(p string) bool { return p == "agents/back.agent.md" })
	want := map[string][]string{
		"agents/a.agent.md":    {"aaa", "bbb"},
		"agents/gone.agent.md": {"ccc"},
	}
	if !reflect.DeepEqual(doc.Paths, want) {
		t.Errorf("paths = %v, want %v", doc.Paths, want)
	}
	if doc.Replacements["agents/gone.agent.md"] != "new" {
		t.Errorf("replacements = %v, want the committed one kept", doc.Replacements)
	}
}

// Control: with an empty record, history alone decides.
func TestUnionWithEmptyRecord(t *testing.T) {
	doc := &document{Paths: map[string][]string{"agents/a.agent.md": {"bbb"}}}
	union(doc, &document{}, func(string) bool { return false })
	if want := map[string][]string{"agents/a.agent.md": {"bbb"}}; !reflect.DeepEqual(doc.Paths, want) || doc.Replacements != nil {
		t.Errorf("doc = %+v, want history unchanged", doc)
	}
}

// Only a missing record is skipped. An unreadable one must abort, or the
// generator writes history alone over it and loses the squash-lost hashes.
func TestUnionWithFileAbortsOnReadError(t *testing.T) {
	doc := &document{Paths: map[string][]string{}}
	dir := t.TempDir()
	if err := unionWithFile(doc, filepath.Join(dir, "missing.json")); err != nil {
		t.Errorf("missing record: err = %v, want nil", err)
	}
	if err := unionWithFile(doc, dir); err == nil { // a directory cannot be read as a file
		t.Error("unreadable record: err = nil, want an error")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := unionWithFile(doc, bad); err == nil {
		t.Error("malformed record: err = nil, want an error")
	}
}
