package main

import (
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
