package store

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// WriteImport replaces the survey's rows in one transaction, so a rerun
// leaves no duplicates. Its aggregates go too, under finalize's lock, so
// close-out counts the new rows instead of mixing old totals with new skip
// counts. Rows are shuffled, as submissions are, so their order says nothing
// about when someone answered.
func WriteImport(ctx context.Context, dbURL string, s survey.Survey, rows []map[string]any, deleteAfter time.Time) (int, error) {
	store, err := Open(ctx, dbURL)
	if err != nil {
		return 0, err
	}
	defer store.Pool.Close()
	versions := map[string]int{}
	for _, q := range s.Answerable() {
		versions[q.ID] = q.Version
	}
	rand.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
	batch := &pgx.Batch{}
	batch.Queue(`SELECT pg_advisory_xact_lock(hashtext('survey_results:' || $1))`, s.ID)
	batch.Queue(`DELETE FROM survey_results WHERE survey_id = $1`, s.ID)
	batch.Queue(`DELETE FROM survey_skip_results WHERE survey_id = $1`, s.ID)
	batch.Queue(`DELETE FROM survey_answers WHERE survey_id = $1`, s.ID)
	for _, a := range rows {
		v := map[string]int{}
		for k := range a {
			if n, ok := versions[k]; ok {
				v[k] = n
			}
		}
		answers, _ := json.Marshal(a)
		vj, _ := json.Marshal(v)
		batch.Queue(`INSERT INTO survey_answers (survey_id, answers, question_versions, context, delete_after) VALUES ($1, $2, $3, $4, $5)`,
			s.ID, answers, vj, `{"client":"forms"}`, deleteAfter)
	}
	err = pgx.BeginFunc(ctx, store.Pool, func(tx pgx.Tx) error { return tx.SendBatch(ctx, batch).Close() })
	return len(rows), err
}
