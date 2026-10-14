package main

import (
	"encoding/csv"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestExportCSV runs against Postgres when COPILOT_SURVEY_TEST_DB_URL is set
// (see TestSurveyStore), and is skipped otherwise.
func TestExportCSV(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	ctx := t.Context()
	st, err := openSurveyStore(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.pool.Close()
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_freetext_hidden`)

	defs, err := loadSurveys([]byte(testSurveys))
	if err != nil {
		t.Fatal(err)
	}
	s := defs[0] // q4-2026, open 2026-10-01 to 2026-11-30
	st.defs = []survey{s}
	insert := func(answers, versions, context string) {
		if _, err = st.pool.Exec(ctx, `INSERT INTO survey_answers VALUES ($1, $2, $3, $4, '2027-06-01')`, s.ID, answers, versions, context); err != nil {
			t.Fatal(err)
		}
	}
	common := `{"client":"copilot","os":"linux","version":"2026.09","local_models":false}`
	insert(`{"overall":3,"client":"copilot","tools":["a","b"],"editor":["vim","Annet"],"editor.other":"ring 98765432","fast":4,"safe":2,"comment":"hans@nav.no"}`,
		`{"overall":1,"client":1,"tools":1,"editor":1,"fast":2,"safe":1,"comment":1}`, common)
	insert(`{"fast":1,"comment":"=SUM(A1)"}`, `{"fast":1,"comment":1}`, common)
	insert(`{"comment":"skjul meg"}`, `{"comment":2}`, common)
	for range 2 {
		insert(`{"overall":5}`, `{"overall":1}`, common)
	}
	insert(`{"overall":4}`, `{"overall":1}`, `{"client":"pi","os":"linux","version":"2026.09","local_models":false}`)
	if err = st.hide(ctx, s.ID, "comment", textHash("skjul meg")); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	api := &surveyAPI{surveys: []survey{s}, answers: st.answers, final: st.final, hidden: st.hidden, hide: st.hide,
		textKey: []byte("k"), now: func() time.Time { return now }}
	h := makeRouter(testAuthenticator(t), api)

	rec := do(h, "GET", "/api/v1/admin/surveys/q4-2026/export.csv", adminToken, "")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for k, want := range map[string]string{"Content-Disposition": `attachment; filename="q4-2026-svar.csv"`, "Cache-Control": "no-store", "Content-Type": "text/csv; charset=utf-8"} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s: %q, want %q", k, got, want)
		}
	}
	body, ok := strings.CutPrefix(rec.Body.String(), "\xef\xbb\xbf")
	if !ok {
		t.Fatal("no BOM")
	}
	recs, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	wantHeader := []string{"overall", "client", "tools=a", "tools=b", "tools=c", "why", "editor=vim", "editor=vscode", "editor=Annet", "editor.other",
		"grid.fast", "grid.fast.version", "grid.safe", "comment", "comment.version", "context.client", "context.os", "context.version", "context.local_models"}
	if !slices.Equal(recs[0], wantHeader) {
		t.Fatalf("header\n%q\nwant\n%q", recs[0], wantHeader)
	}
	if len(recs) != 7 {
		t.Fatalf("%d rows", len(recs)-1)
	}
	find := func(col int, v string) []string {
		for _, r := range recs[1:] {
			if r[col] == v {
				return r
			}
		}
		t.Fatalf("no row with %s=%q in %q", recs[0][col], v, recs)
		return nil
	}
	full := find(0, "3")
	if want := []string{"3", "copilot", "1", "1", "0", "", "1", "0", "1", "ring [telefon]", "4", "2", "2", "[e-post]", "1",
		"copilot", "linux", "2026.09", "false"}; !slices.Equal(full, want) {
		t.Errorf("row\n%q\nwant\n%q", full, want)
	}
	find(13, "'=SUM(A1)")
	// The hidden text's row: empty cell, but its version is kept.
	if r := find(14, "2"); r[13] != "" {
		t.Errorf("hidden text: %q", r[13])
	}
	// The «pi» row: below k=5 on its own, and its combination with it.
	if r := find(0, "4"); !slices.Equal(r[15:], []string{"annet", "annet", "annet", "annet"}) {
		t.Errorf("small segment kept: %q", r[15:])
	}
	if strings.Contains(body, "skjul meg") || strings.Contains(body, "hans@") || strings.Contains(body, "98765432") {
		t.Errorf("hidden or unmasked text in export:\n%s", body)
	}

	// Closed, aggregated, rows deleted: 410.
	now = time.Date(2026, 12, 15, 12, 0, 0, 0, time.UTC)
	st.closeOut(ctx, now)
	if rec := do(h, "GET", "/api/v1/admin/surveys/q4-2026/export.csv", adminToken, ""); rec.Code != 200 {
		t.Fatalf("closed with rows: %d", rec.Code)
	}
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers`)
	if rec := do(h, "GET", "/api/v1/admin/surveys/q4-2026/export.csv", adminToken, ""); rec.Code != 410 {
		t.Fatalf("after deletion: %d %s", rec.Code, rec.Body)
	}
}
