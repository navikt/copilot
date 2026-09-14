package cli

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// allowedCollectionLiterals is every string in this package that may still
// contain the word "collection", and why. Everything else the binary prints
// belongs to a user who never chose a collection and must not be told about
// one (navikt/copilot#878).
//
// Adding an entry is a decision, not a formality: check who reads the string
// before you widen this map.
var allowedCollectionLiterals = map[string]string{
	// State and JSON keys. The key on disk is frozen (see StateFile.Collection);
	// in command output it is the documented alias for "agentpakke".
	"collection":  "state and --json key, frozen for compatibility",
	"collections": "--json key of `list` against a source without a manifest",

	// Paths. A user may read the word in a path without having to act on it.
	"no manifest — %s is absent; read as collections/<name>/manifest.json":                                "validate note, author-facing, names a directory",
	"source ships neither %s nor a collections/ directory, so nav-pilot has nothing to install from it. ": "validate finding, author-facing, names a directory",
	"Add an agentpakke manifest, or ship collections/<name>/manifest.json":                                "validate finding, author-facing, names a directory",
	"collections/%s lists %s %q, which does not exist in %s/":                                             "validate finding, author-facing, names a directory",

	// The mechanism for sources that ship no manifest. navikt/copilot is not
	// one of these, so no nav-pilot user reaches these lines. Retiring the
	// mechanism, and its vocabulary with it, is navikt/copilot#878 fase 4.
	"%q matches both a collection and %s %s.\n  Install the collection: nav-pilot install %s\n  Install the %s: nav-pilot install %s --type %s": "source without a manifest only",
	"Available collections:":     "source without a manifest only",
	"no collections found":       "source without a manifest only",
	"no valid collections found": "source without a manifest only",
	"Choose collection":          "source without a manifest only",
}

func TestNoCollectionVocabulary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil || !strings.Contains(strings.ToLower(value), "collection") {
				return true
			}
			seen[value] = true
			if _, allowed := allowedCollectionLiterals[value]; !allowed {
				t.Errorf("%s: the string %q says \"collection\".\n"+
					"Say what the reader can act on instead, or add it to allowedCollectionLiterals with a reason.",
					fset.Position(lit.Pos()), value)
			}
			return true
		})
	}
	// A stale entry hides the next one behind it.
	for value := range allowedCollectionLiterals {
		if !seen[value] {
			t.Errorf("allowedCollectionLiterals still lists %q, which no longer exists — drop the entry", value)
		}
	}
}

// TestJSONCarriesAgentpakkeAlias pins the alias both ways: "collection" is
// what every existing consumer reads and never disappears, and "agentpakke" is
// the same value under the name the rest of the binary uses.
func TestJSONCarriesAgentpakkeAlias(t *testing.T) {
	dir := t.TempDir()
	agentPath := filepath.Join(dir, ".github", "agents", "test.agent.md")
	if err := os.MkdirAll(filepath.Dir(agentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentPath, []byte("# Agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, _ := fileHash(agentPath)
	if err := writeState(dir, &StateFile{
		Collection:  "nav-pilot",
		Version:     "1.0",
		SourceSHA:   "abc1234",
		InstalledAt: "2025-07-01T12:00:00Z",
		Files:       []InstalledFile{{Path: ".github/agents/test.agent.md", Hash: hash}},
	}); err != nil {
		t.Fatal(err)
	}

	var err error
	out := captureStdoutFor(t, func() { err = cmdListInstalledScoped(ScopeRepo(dir), false, true) })
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("list --installed --json is not JSON: %v\n%s", err, out)
	}
	if doc["collection"] != "nav-pilot" {
		t.Errorf(`"collection" = %v, want "nav-pilot" — the alias is permanent`, doc["collection"])
	}
	if doc["agentpakke"] != doc["collection"] {
		t.Errorf(`"agentpakke" = %v, "collection" = %v — they are the same value`, doc["agentpakke"], doc["collection"])
	}
}
