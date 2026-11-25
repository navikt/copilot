package survey

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// syntheticCSV builds an export with the mapped headers, in mapping order.
func syntheticCSV(t *testing.T, rows ...map[string]string) string {
	t.Helper()
	raw, _ := ImportFiles.ReadFile("imports/q2-2026.json")
	var m ImportMapping
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var hdr []string
	for h := range m.Columns {
		hdr = append(hdr, h)
	}
	var b strings.Builder
	line := func(cells []string) {
		for i, c := range cells {
			cells[i] = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
		}
		b.WriteString(strings.Join(cells, ",") + "\n")
	}
	line(append([]string(nil), hdr...))
	for _, r := range rows {
		cells := make([]string, len(hdr))
		for i, h := range hdr {
			cells[i] = r[m.Columns[h]]
		}
		line(cells)
	}
	return b.String()
}

var importRows = []map[string]string{
	{"erfaring": "11+ 28;", "verktoy": "Copilot CLI (terminal);Cursor, Codex;", "verdi": "Feilsøking;Bruker ikke;",
		"tilfredshet": "Helt enig", "eierskap": "Uenig", "en-endring": "Støtte for flere AI-verktøy eller miljøer", "opplevelse": "fint"},
	{"erfaring": "18+;Option 5;", "verktoy": "Jeg bruker ikke AI-kodeverktøy;", "en-endring": "Noe annet"},
	{"erfaring": "Option 5;"},
}

func TestConvertImport(t *testing.T) {
	defs, _ := LoadSurveyDir(SurveyFiles)
	var s Survey
	for _, d := range defs {
		if d.ID == "q2-2026" {
			s = d
		}
	}
	raw, _ := ImportFiles.ReadFile("imports/q2-2026.json")
	var m ImportMapping
	_ = json.Unmarshal(raw, &m)
	rows, stats, err := convertImport(s, m, strings.NewReader(syntheticCSV(t, importRows...)))
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"erfaring": "11+", "verktoy": []string{"Copilot CLI (terminal)", "Annet"}, "verktoy.other": "Cursor, Codex",
			"verdi": []string{"Feilsøking"}, "tilfredshet": 5, "eierskap": 2,
			"en-endring": "Støtte for flere KI-verktøy eller miljøer", "opplevelse": "fint"},
		{"erfaring": "11+", "verktoy": []string{"Jeg bruker ikke KI-kodeverktøy"}, "en-endring": "Annet", "en-endring.other": "Noe annet"},
		{},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %v\nwant %v", rows, want)
	}
	if stats.rows != 3 || stats.questions["erfaring"] != 2 || stats.norms["erfaring «Option 5» dropped"] != 2 {
		t.Fatalf("stats %+v", stats)
	}

	// An unmapped column (e.g. Email left in) is refused.
	_, _, err = convertImport(s, m, strings.NewReader("Email\nx@nav.no\n"))
	if err == nil || strings.Contains(err.Error(), "x@nav.no") {
		t.Fatalf("unmapped column: %v", err)
	}
}
