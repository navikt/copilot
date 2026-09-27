package main

import (
	"os"
	"testing"
	"time"
)

// TestSurveyStore runs the real SQL against Postgres when
// COPILOT_CLI_TEST_DB_URL names one (e.g. docker run -e
// POSTGRES_PASSWORD=x -p 5432:5432 postgres:17), and is skipped otherwise.
func TestSurveyStore(t *testing.T) {
	url := os.Getenv("COPILOT_CLI_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_CLI_TEST_DB_URL not set")
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

	if ok, err := s.participate(t.Context(), "s", "h1", day(10)); !ok || err != nil {
		t.Fatalf("first: %v %v", ok, err)
	}
	if ok, err := s.participate(t.Context(), "s", "h1", day(10)); ok || err != nil {
		t.Fatalf("duplicate: %v %v, want false, nil", ok, err)
	}
	if ok, _ := s.participate(t.Context(), "closed", "h2", day(0)); !ok {
		t.Fatal("closed survey participation not stored")
	}

	r := response{SurveyID: "s", Answers: map[string]any{"q": 1}, QuestionVersions: map[string]int{"q": 1},
		Context: techContext{Version: "1.2", OS: "linux", Arch: "amd64", Client: "pi"}, DeleteAfter: day(100)}
	for range answerBatch - 1 {
		s.record(t.Context(), r)
	}
	if n := count("survey_answers"); n != 0 {
		t.Fatalf("%d answers written before a full batch", n)
	}
	s.record(t.Context(), r)
	if n := count("survey_answers"); n != answerBatch {
		t.Fatalf("answers = %d after a full batch, want %d", n, answerBatch)
	}
	expired := r
	expired.DeleteAfter = day(-1)
	s.record(t.Context(), expired)
	s.retain(t.Context()) // flushes the one queued, then deletes it as expired

	if n := count("survey_answers"); n != answerBatch {
		t.Fatalf("answers = %d after retention, want %d", n, answerBatch)
	}
	if n := count("survey_participation"); n != 1 {
		t.Fatalf("participation = %d, want 1 (the closed survey's deleted)", n)
	}
}
