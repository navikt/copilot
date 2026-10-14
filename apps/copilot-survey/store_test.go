package main

import (
	"os"
	"testing"
	"time"
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

	// One submission is in both tables at once; a duplicate is refused.
	if !submit("s", "a", day(10)) {
		t.Fatal("fresh submission refused")
	}
	if count("survey_answers") != 1 || count("survey_participation") != 1 {
		t.Fatalf("after one submission: %d answers, %d participation", count("survey_answers"), count("survey_participation"))
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

	// A failed write is reported and undone, so the sender can send the same
	// answer again once the database is back.
	exec := func(q string) {
		t.Helper()
		if _, err := s.pool.Exec(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	exec(`ALTER TABLE survey_answers RENAME TO survey_answers_gone`)
	ok, lateErr := s.submit(t.Context(), "s", "late", day(10), r)
	exec(`ALTER TABLE survey_answers_gone RENAME TO survey_answers`)
	if ok || lateErr == nil {
		t.Fatalf("a failed write was acknowledged: ok=%v err=%v", ok, lateErr)
	}
	if count("survey_participation") != 3 || answered("s", "late") {
		t.Fatal("a failed write was partly written or left the sender marked as answered")
	}
	if !submit("s", "late", day(10)) || !answered("s", "late") {
		t.Fatal("the sender could not send again after a failed write")
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
