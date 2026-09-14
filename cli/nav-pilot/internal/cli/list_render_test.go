package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// naisPakkeManifest is the shape that made the listing unreadable: one
// agentpakke whose description is a full sentence.
const naisPakkeManifest = `{
  "contractVersion": "1",
  "name": "nais-platform",
  "description": "Knowledge and skills for engineers who build the Nais platform: the Nais API, tenants and environment clusters, Fasit features, Terraform, and the Loki/Mimir/Tempo observability stack.",
  "clients": {"copilot": {"primaryAgents": ["grillmester"]}},
  "layout": {"agents": "plugin/agents", "skills": "plugin/skills"}
}`

// plainOutput turns the ANSI codes off so a width assertion counts characters
// the user sees rather than escape sequences.
func plainOutput(t *testing.T) {
	t.Helper()
	orig := domain.UseColor
	t.Cleanup(func() { domain.UseColor = orig })
	domain.UseColor = false
}

func longestLine(out string) (int, string) {
	longest := 0
	worst := ""
	for _, line := range strings.Split(out, "\n") {
		if n := len([]rune(line)); n > longest {
			longest, worst = n, line
		}
	}
	return longest, worst
}

// TestPrintPakkeListingFitsEightyColumns renders the single-agentpakke listing
// at a fixed width: nothing runs off an 80-column terminal, the description is
// wrapped, and the gutter the collections table padded to is gone.
func TestPrintPakkeListingFitsEightyColumns(t *testing.T) {
	plainOutput(t)

	c := collectionInfo{
		Name:        "nais-platform",
		Description: "Knowledge and skills for engineers who build the Nais platform: the Nais API, tenants and environment clusters, Fasit features, Terraform, and the Loki/Mimir/Tempo observability stack.",
		Items:       14,
		agents:      []string{"nais-platform", "nais-review"},
	}
	var buf bytes.Buffer
	printPakkeListing(&buf, "nais/pilot", c, nil, false, 80)
	out := buf.String()

	if n, worst := longestLine(out); n > 80 {
		t.Errorf("a line is %d columns wide, want at most 80:\n%q", n, worst)
	}
	want := []string{
		"Agentpakke in nais/pilot:",
		"  nais-platform (14 items)",
		"  agents: nais-platform, nais-review",
		"Install: nav-pilot install nais-platform",
		"Items:   nav-pilot list --items",
	}
	for _, w := range want {
		if !strings.Contains(out, w+"\n") {
			t.Errorf("the listing is missing the line %q:\n%s", w, out)
		}
	}
	// The description has to arrive whole, across however many lines the width
	// allows, and it has to be more than one of them at 80 columns.
	var body []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  ") && strings.Contains(c.Description, strings.TrimSpace(line)) {
			body = append(body, strings.TrimSpace(line))
		}
	}
	if strings.Join(body, " ") != c.Description {
		t.Errorf("the description was not rendered whole; got %q", strings.Join(body, " "))
	}
	if len(body) < 2 {
		t.Errorf("the description was printed on %d line(s); at 80 columns it has to wrap", len(body))
	}
	// Two suggestion lines, down from three.
	if n := strings.Count(out, "nav-pilot "); n != 2 {
		t.Errorf("the listing suggests %d commands, want 2:\n%s", n, out)
	}
}

// TestListSinglePakkeRendering drives cmdList end to end: with stdout piped the
// width falls back to 80, which is what a user with an 80-column terminal sees.
func TestListSinglePakkeRendering(t *testing.T) {
	plainOutput(t)
	isolatedConfig(t)

	src := &Source{Dir: pakkeSourceTree(t, naisPakkeManifest), SHA: "def5678", Version: "dev", Repo: "nais/pilot"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}
	stubResolveSource(t, src)

	var err error
	out := captureStdoutFor(t, func() { err = cmdList(nil, "", "", false, false) })
	if err != nil {
		t.Fatalf("cmdList = %v", err)
	}
	if n, worst := longestLine(out); n > 80 {
		t.Errorf("the listing prints a line %d columns wide, which runs off an 80-column terminal:\n%q", n, worst)
	}
	if strings.Contains(out, strings.Repeat(" ", 10)) {
		t.Errorf("the listing still pads to the collections table's gutter:\n%s", out)
	}
	if !strings.Contains(out, "nais-platform (2 items)") {
		t.Errorf("the listing lost the name or the item count:\n%s", out)
	}
}

// TestListJSONIsUnchanged: only the human rendering moved, so --json keeps
// every key it had.
func TestListJSONIsUnchanged(t *testing.T) {
	isolatedConfig(t)

	src := &Source{Dir: pakkeSourceTree(t, naisPakkeManifest), SHA: "def5678", Version: "dev", Repo: "nais/pilot"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}
	stubResolveSource(t, src)

	var err error
	out := captureStdoutFor(t, func() { err = cmdList(nil, "", "", false, true) })
	if err != nil {
		t.Fatalf("cmdList --json = %v", err)
	}
	for _, want := range []string{`"name": "nais-platform"`, `"items": 2`, `"description"`, `"source"`} {
		if !strings.Contains(out, want) {
			t.Errorf("--json output lost %s:\n%s", want, out)
		}
	}
}

// TestWrapWords covers the wrap itself, including the word that cannot fit.
func TestWrapWords(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{"empty", "", 10, nil},
		{"fits", "one two", 10, []string{"one two"}},
		{"wraps on the space", "one two three", 7, []string{"one two", "three"}},
		{"a word longer than the width gets its own line", "a loooooooooong word", 6, []string{"a", "loooooooooong", "word"}},
		{"collapses runs of whitespace", "  one \n two  ", 10, []string{"one two"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapWords(tt.text, tt.width)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("wrapWords(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}
		})
	}
}
