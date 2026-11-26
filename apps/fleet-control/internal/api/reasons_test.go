package api

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestReasonsPinned keeps reasons.go and reasons.json in step: a renamed or
// removed reason breaks clients that switch on it (my-copilot, #3087).
func TestReasonsPinned(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "reasons.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	ast.Inspect(f, func(n ast.Node) bool {
		if v, ok := n.(*ast.ValueSpec); ok {
			s, _ := strconv.Unquote(v.Values[0].(*ast.BasicLit).Value)
			if v.Names[0].Name != "Reason"+s {
				t.Errorf("%s = %q: the name must be Reason + value", v.Names[0].Name, s)
			}
			got = append(got, s)
		}
		return true
	})
	want := pinnedReasons(t)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("reasons.go and reasons.json differ:\n go:   %v\n json: %v", got, want)
	}
}

func pinnedReasons(t testing.TB) []string {
	t.Helper()
	raw, err := os.ReadFile("reasons.json")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	return want
}

// TestNoLiteralReasons makes every refusal use a constant from reasons.go.
func TestNoLiteralReasons(t *testing.T) {
	lit := regexp.MustCompile(`stErr\(http\.\w+, "|refuse\(http\.\w+, "|conflict\("[^"]*", "|reason: "`)
	files, _ := filepath.Glob("*.go")
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name) //nolint:gosec // package files from Glob
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range lit.FindAll(src, -1) {
			t.Errorf("%s: reason literal in %q; add a constant to reasons.go", name, m)
		}
	}
}
