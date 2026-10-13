package main

import (
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestSurveyStore runs the real SQL against Postgres when
// COPILOT_SURVEY_TEST_DB_URL names one (e.g. docker run -e
// POSTGRES_PASSWORD=x -p 5432:5432 postgres:17), and is skipped otherwise.
func TestSurveyStore(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	s, err := openSurveyStore(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.pool.Close()
	_, _ = s.pool.Exec(t.Context(), `DELETE FROM survey_participation; DELETE FROM survey_answers`)
	count := func(table string) (n int) {
		_ = s.pool.QueryRow(t.Context(), `SELECT count(*) FROM `+table).Scan(&n)
		return n
	}
	day := func(d int) time.Time { return time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, d) }
	r := response{SurveyID: "s", Answers: map[string]any{"q": 1}, QuestionVersions: map[string]int{"q": 1},
		Context: techContext{Version: "1.2", OS: "linux", Client: "pi"}, DeleteAfter: day(100)}
	submit := func(survey, hash string, closes time.Time) bool {
		t.Helper()
		ok, err := s.submit(t.Context(), survey, hash, closes, r)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}

	answered := func(survey, hash string) bool {
		t.Helper()
		ok, err := s.answered(t.Context(), survey, hash)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if answered("s", "a") {
		t.Fatal("answered before submitting")
	}
	queued := func(survey string) float64 { return testutil.ToFloat64(queuedAnswers.WithLabelValues(survey)) }
	// Control: a stale value the store must overwrite, so a missing gauge
	// update fails the checks below instead of reading a default 0.
	queuedAnswers.WithLabelValues("s").Set(42)
	if queued("s") != 42 {
		t.Fatal("gauge read does not reflect Set")
	}

	// One submission is in both tables at once; a duplicate is refused.
	if !submit("s", "a", day(10)) {
		t.Fatal("fresh submission refused")
	}
	if count("survey_answers") != 1 || count("survey_participation") != 1 {
		t.Fatalf("after one submission: %d answers, %d participation", count("survey_answers"), count("survey_participation"))
	}
	if queued("s") != 0 {
		t.Fatalf("queued gauge after a written submission = %v, want 0", queued("s"))
	}
	if submit("s", "a", day(10)) {
		t.Fatal("a written duplicate was accepted")
	}
	if !answered("s", "a") || answered("s", "zz") || answered("other", "a") {
		t.Fatal("answered wrong for written submissions")
	}
	submit("s", "b", day(10))
	submit("other", "a", day(1))
	if count("survey_answers") != 3 || count("survey_participation") != 3 {
		t.Fatal("submissions not written at once")
	}

	// A failed write keeps the submission queued (and refuses a duplicate),
	// and flush writes it at shutdown.
	exec := func(q string) {
		t.Helper()
		if _, err := s.pool.Exec(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	exec(`ALTER TABLE survey_answers RENAME TO survey_answers_gone`)
	ok := submit("s", "late", day(10))
	exec(`ALTER TABLE survey_answers_gone RENAME TO survey_answers`)
	if !ok || count("survey_participation") != 3 {
		t.Fatal("a failed write was refused or partly written")
	}
	if !answered("s", "late") || submit("s", "late", day(10)) {
		t.Fatal("a queued submission was not seen as answered")
	}
	if queued("s") != 1 {
		t.Fatalf("queued gauge after a failed write = %v, want 1", queued("s"))
	}
	before := count("survey_answers")
	if queued("s") != 1 {
		t.Fatalf("queued gauge = %v, want 1", queued("s"))
	}
	if lost := s.flush(t.Context()); lost != 0 {
		t.Fatalf("flush lost %d", lost)
	}
	if count("survey_answers") != before+1 || !answered("s", "late") {
		t.Fatal("flush did not write the queued answer")
	}
	if queued("s") != 0 {
		t.Fatalf("queued gauge after flush = %v, want 0", queued("s"))
	}
	if submit("s", "late", day(10)) {
		t.Fatal("a flushed sender could answer again")
	}

	// Close-out: participation of closed surveys goes, answers stay.
	s.retain(t.Context(), day(2))
	if count("survey_answers") != 4 {
		t.Fatalf("answers after close-out = %d, want 4", count("survey_answers"))
	}
	if n := count("survey_participation"); n != 3 {
		t.Fatalf("participation = %d; the rows of s (closes in 10 days) stay until it closes", n)
	}
}
