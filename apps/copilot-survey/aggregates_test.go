package main

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestFinalAggregates runs against Postgres when COPILOT_SURVEY_TEST_DB_URL
// is set (see TestSurveyStore), and is skipped otherwise.
func TestFinalAggregates(t *testing.T) {
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
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers; DELETE FROM survey_results`)

	base, err := loadSurveys([]byte(testSurveys))
	if err != nil {
		t.Fatal(err)
	}
	wave := func(id, starts, ends string) survey {
		s := base[0]
		s.ID, s.Series, s.Active, s.Starts, s.Ends = id, "x", false, starts, ends
		return s
	}
	first, second, third := wave("w1", "2026-01-01", "2026-01-31"), wave("w2", "2026-06-01", "2026-06-30"), wave("w3", "2026-12-01", "2026-12-31")
	st.defs = []survey{first, second, third}
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	// Random answers on the fixture's questions, an older item version among
	// them, and contexts where "pi" is held by 3 rows (below the k=5 floor).
	rng := rand.New(rand.NewPCG(1, 2)) // #nosec G404 -- test fixture
	var live []storedAnswer
	insert := func(id string, n int, deleteAfter string) {
		for i := range n {
			a := map[string]any{"overall": rng.IntN(5) + 1, "client": []string{"copilot", "opencode"}[rng.IntN(2)],
				"tools": []string{"a", "c"}[:rng.IntN(3)], "fast": rng.IntN(5) + 1, "comment": "secret text"}
			if rng.IntN(4) == 0 {
				delete(a, "overall")
			}
			v := map[string]int{"overall": 1, "client": 1, "tools": 1, "fast": 1 + rng.IntN(2), "comment": 1}
			c := map[string]any{"client": "copilot", "os": "linux", "version": "2026.06", "local_models": false}
			if i < 3 {
				c["client"] = "pi"
			}
			aj, _ := json.Marshal(a)
			vj, _ := json.Marshal(v)
			cj, _ := json.Marshal(c)
			if _, err = st.pool.Exec(ctx, `INSERT INTO survey_answers VALUES ($1, $2, $3, $4, $5)`, id, aj, vj, cj, deleteAfter); err != nil {
				t.Fatal(err)
			}
			if id == "w1" {
				var row storedAnswer
				_ = json.Unmarshal(aj, &row.Answers)
				row.QuestionVersions = v
				live = append(live, row)
			}
		}
	}
	insert("w1", 40, "2026-03-01")
	insert("w2", 8, "2026-12-27")

	st.closeOut(ctx, now)
	got, err := st.final(ctx, "w1")
	if err != nil || got == nil {
		t.Fatal(got, err)
	}
	want, _ := json.Marshal(aggregate(first, live, now))
	gotJSON, _ := json.Marshal(aggregateTally(first, *got, now))
	if string(want) != string(gotJSON) {
		t.Fatalf("stored aggregates differ from live:\n got %s\nwant %s", gotJSON, want)
	}

	var mean float64
	if err := st.pool.QueryRow(ctx, `SELECT mean FROM survey_results WHERE survey_id='w1' AND key='overall' LIMIT 1`).Scan(&mean); err != nil || mean < 1 || mean > 5 {
		t.Fatal("scale mean", mean, err)
	}
	var texts int
	_ = st.pool.QueryRow(ctx, `SELECT count(*) FROM survey_results WHERE key = 'comment' OR option LIKE '%secret%'`).Scan(&texts)
	if texts != 0 {
		t.Fatal("free text in survey_results")
	}

	// k ≥ 5: pi (3 rows) is merged into annet; copilot (37) stays.
	ctxCount := func(id, key, opt string) (n int) {
		_ = st.pool.QueryRow(ctx, `SELECT coalesce(sum(count), 0) FROM survey_results WHERE survey_id=$1 AND kind='context' AND key=$2 AND option=$3`, id, key, opt).Scan(&n)
		return n
	}
	if ctxCount("w1", "client", "pi") != 0 || ctxCount("w1", "client", "annet") != 3 || ctxCount("w1", "client", "copilot") != 37 {
		t.Fatal("context not merged at k=5")
	}
	// w2 has 8 rows: copilot 5 stays, pi 3 is merged.
	if ctxCount("w2", "client", "annet") != 3 || ctxCount("w2", "client", "copilot") != 5 {
		t.Fatal("w2 context")
	}

	// Retention: w1's rows run to w2's last day (w2 has closed); w2's keep
	// their own date (w3 has not closed); a later date is never shortened.
	deleteAfter := func(id string) (d []string) {
		rows, _ := st.pool.Query(ctx, `SELECT DISTINCT delete_after::text FROM survey_answers WHERE survey_id=$1 ORDER BY 1`, id)
		defer rows.Close()
		for rows.Next() {
			var s string
			_ = rows.Scan(&s)
			d = append(d, s)
		}
		return d
	}
	if d := deleteAfter("w1"); !reflect.DeepEqual(d, []string{"2026-06-30"}) {
		t.Fatal("w1 delete_after", d)
	}
	if d := deleteAfter("w2"); !reflect.DeepEqual(d, []string{"2026-12-27"}) {
		t.Fatal("w2 delete_after", d)
	}
	_, _ = st.pool.Exec(ctx, `UPDATE survey_answers SET delete_after = '2028-04-30' WHERE survey_id='w1'`)
	st.closeOut(ctx, now)
	if d := deleteAfter("w1"); !reflect.DeepEqual(d, []string{"2028-04-30"}) {
		t.Fatal("longer retention shortened", d)
	}

	// Idempotent: close-out again leaves aggregates alone even when rows
	// change (they may be partly deleted by then); finalize with replace
	// twice gives the same rows.
	results := func() (n int) {
		_ = st.pool.QueryRow(ctx, `SELECT count(*) FROM survey_results WHERE survey_id='w1'`).Scan(&n)
		return n
	}
	before := results()
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers WHERE survey_id='w1' AND ctid IN (SELECT ctid FROM survey_answers WHERE survey_id='w1' LIMIT 10)`)
	st.closeOut(ctx, now)
	if again, _ := st.final(ctx, "w1"); again.Responses != 40 || results() != before {
		t.Fatal("close-out rewrote existing aggregates")
	}
	for range 2 {
		if n, err := finalize(ctx, st.pool, st.defs, first, now, true); err != nil || n != 30 {
			t.Fatal(n, err)
		}
	}
	var dup int
	_ = st.pool.QueryRow(ctx, `SELECT count(*) FROM survey_results WHERE survey_id='w1' AND kind='total'`).Scan(&dup)
	if again, _ := st.final(ctx, "w1"); again.Responses != 30 || dup != 1 {
		t.Fatal("replace not idempotent")
	}

	// No rows: nothing written, so a later import still gets aggregated.
	if n, err := finalize(ctx, st.pool, st.defs, third, now, true); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if f, _ := st.final(ctx, "w3"); f != nil {
		t.Fatal("aggregates for a survey without rows")
	}
}

// A closed survey's results and count come from final aggregates when it has
// them; an open survey's always from its rows.
func TestResultsReadFinalForClosed(t *testing.T) {
	rows := []storedAnswer{row(`{"a":1}`, `{"a":1}`)}
	api := adminAPI(t, rows)
	api.final = func(_ context.Context, id string) (*surveyTally, error) {
		if id != "old" {
			return nil, nil
		}
		return &surveyTally{Responses: 6, Keys: map[string]map[int]*verTally{"a": {1: {N: 6, Counts: map[string]int{"2": 6}}}}}, nil
	}
	h := makeRouter(testAuthenticator(t), api)
	rec := do(h, "GET", "/api/v1/admin/surveys/old/results", adminToken, "")
	if !strings.Contains(rec.Body.String(), `"responses":6`) || !strings.Contains(rec.Body.String(), `{"value":"2","count":6,"suppressed":false}`) {
		t.Fatal(rec.Body.String())
	}
	rec = do(h, "GET", "/api/v1/admin/surveys", adminToken, "")
	if !strings.Contains(rec.Body.String(), `"id":"old","title":"Old","starts":"2025-01-01","ends":"2025-01-31","status":"stengt","responses":6`) ||
		!strings.Contains(rec.Body.String(), `"status":"åpen","responses":1`) {
		t.Fatal(rec.Body.String())
	}
}
