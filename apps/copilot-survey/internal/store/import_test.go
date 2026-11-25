package store

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// syntheticCSV builds an export with the mapped headers, in mapping order.
func syntheticCSV(t *testing.T, rows ...map[string]string) string {
	t.Helper()
	raw, _ := survey.ImportFiles.ReadFile("imports/q2-2026.json")
	var m survey.ImportMapping
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

// TestImportPostgres runs against COPILOT_SURVEY_TEST_DB_URL, as TestSurveyStore.
func TestImportPostgres(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	csv := syntheticCSV(t, importRows...)
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for range 2 { // twice: a rerun replaces, never duplicates
		var out bytes.Buffer
		if err := survey.RunImport([]string{"--survey", "q2-2026"}, strings.NewReader(csv), &out, url, now, WriteImport); err != nil {
			t.Fatal(err)
		}
	}
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var n, parts int
	var ctx, after string
	err = pool.QueryRow(t.Context(), `SELECT count(*), min(context::text), min(delete_after::text),
		(SELECT count(*) FROM survey_participation WHERE survey_id = 'q2-2026')
		FROM survey_answers WHERE survey_id = 'q2-2026'`).Scan(&n, &ctx, &after, &parts)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(importRows) || ctx != `{"client": "forms"}` || after != "2028-04-30" || parts != 0 {
		t.Fatalf("rows %d, context %s, delete_after %s, participation %d", n, ctx, after, parts)
	}
	// No identity: the only keys stored are question ids and their other texts.
	rows, _ := pool.Query(t.Context(), `SELECT DISTINCT jsonb_object_keys(answers) FROM survey_answers WHERE survey_id = 'q2-2026'`)
	defer rows.Close()
	for rows.Next() {
		var k string
		_ = rows.Scan(&k)
		if !slices.Contains(strings.Fields("erfaring verktoy verktoy.other verdi tilfredshet eierskap en-endring en-endring.other opplevelse"), k) {
			t.Errorf("unexpected key %q", k)
		}
	}
}

var importRows = []map[string]string{
	{"erfaring": "11+ 28;", "verktoy": "Copilot CLI (terminal);Cursor, Codex;", "verdi": "Feilsøking;Bruker ikke;",
		"tilfredshet": "Helt enig", "eierskap": "Uenig", "en-endring": "Støtte for flere AI-verktøy eller miljøer", "opplevelse": "fint"},
	{"erfaring": "18+;Option 5;", "verktoy": "Jeg bruker ikke AI-kodeverktøy;", "en-endring": "Noe annet"},
	{"erfaring": "Option 5;"},
}
