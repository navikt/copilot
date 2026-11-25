package survey

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"reflect"
	"slices"
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
				row.Client, _ = c["client"].(string)
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
	if err = st.pool.QueryRow(ctx, `SELECT mean FROM survey_results WHERE survey_id='w1' AND key='overall' LIMIT 1`).Scan(&mean); err != nil || mean < 1 || mean > 5 {
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
		var n int
		if n, err = finalize(ctx, st.pool, st.defs, first, now, true); err != nil || n != 30 {
			t.Fatal(n, err)
		}
	}
	var dup int
	_ = st.pool.QueryRow(ctx, `SELECT count(*) FROM survey_results WHERE survey_id='w1' AND kind='total'`).Scan(&dup)
	if again, _ := st.final(ctx, "w1"); again.Responses != 30 || dup != 1 {
		t.Fatal("replace not idempotent")
	}

	// Pre-existing finalized surveys without kind='channel' fall back to
	// survey_answers while retained, and to kind='context' once expired.
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_results WHERE survey_id='w2' AND kind='channel'`)
	if f, fErr := st.final(ctx, "w2"); fErr != nil || !reflect.DeepEqual(f.Channels, map[string]int{"copilot": 5, "pi": 3}) {
		t.Fatalf("fallback from survey_answers: got %v, err %v", f, fErr)
	}
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers WHERE survey_id='w2'`)
	if f, fErr := st.final(ctx, "w2"); fErr != nil || !reflect.DeepEqual(f.Channels, map[string]int{"copilot": 5, "annet": 3}) {
		t.Fatalf("fallback from context after answers expired: got %v, err %v", f, fErr)
	}

	// Rare client/OS/version combinations merge into "annet" in kind='context'
	// without hiding a channel that has >= 5 total respondents in kind='channel'.
	// Every value is held by 5 rows, every combination by 2 or 3.
	for i := range 10 {
		cj, _ := json.Marshal(map[string]any{"client": []string{"web", "copilot"}[i/5], "os": []string{"linux", "darwin"}[(i/5+i%5/3)%2], "version": "2026.06", "local_models": false})
		if _, err = st.pool.Exec(ctx, `INSERT INTO survey_answers VALUES ('w2', '{"overall":4}', '{"overall":1}', $1, '2026-12-27')`, cj); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = finalize(ctx, st.pool, st.defs, second, now, true); err != nil {
		t.Fatal(err)
	}
	if ctxCount("w2", "client", "web") != 0 || ctxCount("w2", "client", "annet") != 10 {
		t.Fatal("expected rare context segments to merge into annet")
	}
	if f, fErr := st.final(ctx, "w2"); fErr != nil || !reflect.DeepEqual(f.Channels, map[string]int{"web": 5, "copilot": 5}) {
		t.Fatalf("expected channel counts to preserve web=5, copilot=5: got %v, err %v", f, fErr)
	}

	// No rows: nothing written, so a later import still gets aggregated.
	var n int
	if n, err = finalize(ctx, st.pool, st.defs, third, now, true); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if f, _ := st.final(ctx, "w3"); f != nil {
		t.Fatal("aggregates for a survey without rows")
	}

	// Expired rows go only once their survey has aggregates (w3 has none:
	// not closed), or when the survey is no longer defined.
	insert("w3", 2, "2026-01-01")
	insert("gone", 2, "2026-01-01")
	st.retain(ctx, now)
	var w3, gone int
	_ = st.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE survey_id='w3'), count(*) FILTER (WHERE survey_id='gone') FROM survey_answers`).Scan(&w3, &gone)
	if w3 != 2 || gone != 0 {
		t.Fatal("expired rows: w3", w3, "gone", gone)
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

// TestRetainWaitsForSkipCounts (Postgres, COPILOT_SURVEY_TEST_DB_URL):
// expired rows stay until their skip counts are written, and go after, or
// once the rows no longer match the stored total.
func TestRetainWaitsForSkipCounts(t *testing.T) {
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
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results`)
	defs, err := loadSurveys([]byte(compareSurveys))
	if err != nil {
		t.Fatal(err)
	}
	noRule := slices.Clone(defs)
	noRule[1].Questions = slices.Clone(defs[1].Questions)
	noRule[1].Questions[1].SkipIf = nil
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	for id, rows := range waveRows() {
		for _, r := range rows {
			aj, _ := json.Marshal(r.Answers)
			vj, _ := json.Marshal(r.QuestionVersions)
			cj, _ := json.Marshal(r.Context)
			if _, err := st.pool.Exec(ctx, `INSERT INTO survey_answers VALUES ($1, $2, $3, $4, '2026-01-01')`, id, aj, vj, cj); err != nil {
				t.Fatal(err)
			}
		}
	}
	count := func(id string) (n int) {
		_ = st.pool.QueryRow(ctx, `SELECT count(*) FROM survey_answers WHERE survey_id = $1`, id).Scan(&n)
		return n
	}
	block := func(on bool) {
		q := `ALTER TABLE survey_skip_results DROP CONSTRAINT IF EXISTS skip_block`
		if on {
			q = `ALTER TABLE survey_skip_results ADD CONSTRAINT skip_block CHECK (false) NOT VALID`
		}
		if _, err := st.pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	defer block(false)

	// Aggregates written before autumn's skip_if existed: no skip counts.
	st.defs = noRule
	st.closeOut(ctx, now)
	// autumn loses a row: its stored total no longer matches.
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers WHERE ctid = (SELECT ctid FROM survey_answers WHERE survey_id = 'autumn' LIMIT 1)`)

	// The rule is deployed, but its counts cannot be written.
	st.defs = defs
	block(true)
	st.retain(ctx, now)
	if count("spring") != 6 || count("autumn") != 0 {
		t.Fatalf("while skip counts are missing: spring %d (want 6 kept), autumn %d (want 0)", count("spring"), count("autumn"))
	}
	block(false)
	st.retain(ctx, now)
	if count("spring") != 0 {
		t.Fatalf("after skip counts are written: spring %d rows", count("spring"))
	}
	var marker int
	_ = st.pool.QueryRow(ctx, `SELECT count(*) FROM survey_skip_results WHERE survey_id = 'spring' AND version = -1`).Scan(&marker)
	if marker != 2 { // fast and slow
		t.Fatalf("spring skip counts: %d pairs", marker)
	}
}

// TestRetainImportRace (COPILOT_SURVEY_TEST_DB_URL): an import that lands
// between retain deciding a survey's expired rows may go and deleting them
// must not lose its rows. retain pauses at purgeHook; the import runs, or
// waits on the survey's lock; then retain goes on.
func TestRetainImportRace(t *testing.T) {
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
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results`)
	defs, err := loadSurveys([]byte(compareSurveys))
	if err != nil {
		t.Fatal(err)
	}
	st.defs = defs
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	expired := now.AddDate(0, -1, 0)
	var spring []map[string]any
	for _, r := range waveRows()["spring"] {
		spring = append(spring, r.Answers)
	}
	if _, err = writeImport(ctx, url, defs[0], spring, expired); err != nil {
		t.Fatal(err)
	}
	st.closeOut(ctx, now)

	paused, release := make(chan struct{}), make(chan struct{})
	st.purgeHook = func(id string) {
		if id == "spring" {
			close(paused)
			<-release
		}
	}
	retained := make(chan struct{})
	go func() { st.retain(ctx, now); close(retained) }()
	<-paused
	imported := make(chan error, 1)
	go func() { _, ierr := writeImport(ctx, url, defs[0], spring, expired); imported <- ierr }()
	// Until the import has finished, or waits on an advisory lock.
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		var waiting int
		_ = st.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND wait_event = 'advisory'`).Scan(&waiting)
		if waiting > 0 || len(imported) > 0 || time.Now().After(deadline) {
			break
		}
	}
	close(release)
	<-retained
	if err = <-imported; err != nil {
		t.Fatal(err)
	}
	var rows, results int
	_ = st.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM survey_answers WHERE survey_id = 'spring'), (SELECT count(*) FROM survey_results WHERE survey_id = 'spring')`).Scan(&rows, &results)
	if rows != 6 || results != 0 {
		t.Fatalf("after the race: %d rows, %d aggregate rows; want 6 rows waiting for close-out", rows, results)
	}
}
