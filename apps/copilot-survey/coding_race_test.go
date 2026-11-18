package main

import (
	"errors"
	"os"
	"testing"
	"time"
)

// TestCodingSaveDuringPurge (COPILOT_SURVEY_TEST_DB_URL): a save that comes
// after purge has counted the codes but before it deletes the rows must not
// succeed and then vanish. It waits on purge's lock and then finds the text
// gone.
func TestCodingSaveDuringPurge(t *testing.T) {
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
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results; DELETE FROM survey_freetext_codes; DELETE FROM survey_freetext_decisions`)
	defs, _ := loadSurveys([]byte(testSurveys))
	s := defs[0]
	s.Series, s.Active, s.Starts, s.Ends = "dev", false, "2026-01-01", "2026-01-31"
	st.defs, st.codebooks = []survey{s}, map[string]codebook{"dev": testCodebook}
	for range 5 {
		if _, err = st.pool.Exec(ctx, `INSERT INTO survey_answers (survey_id, answers, question_versions, context, delete_after) VALUES ($1, '{"comment":"rask"}', '{}', '{}', '2026-03-01')`, s.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err = st.saveCodes(ctx, s.ID, "comment", textHash("rask"), "model", 1, []code{{"a", false}}, ""); err != nil {
		t.Fatal(err)
	}
	saved := make(chan error, 1)
	st.purgeHook = func(string) {
		go func() {
			saved <- st.saveCodes(ctx, s.ID, "comment", textHash("rask"), "human", 1, []code{{"a", false}}, "")
		}()
		// Until the save has finished, or waits on an advisory lock.
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			var waiting int
			_ = st.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND wait_event = 'advisory'`).Scan(&waiting)
			if waiting > 0 || len(saved) > 0 || time.Now().After(deadline) {
				return
			}
		}
	}
	st.retain(ctx, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))
	err = <-saved
	var coded int
	_ = st.pool.QueryRow(ctx, `SELECT count FROM survey_results WHERE survey_id = $1 AND kind = 'freetext' AND key = 'comment' AND option = ''`, s.ID).Scan(&coded)
	if err == nil && coded == 0 {
		t.Fatal("the save succeeded after the counts were taken, and is lost")
	}
	if !errors.Is(err, errGone) {
		t.Errorf("save after purge: %v, want errGone", err)
	}
}
