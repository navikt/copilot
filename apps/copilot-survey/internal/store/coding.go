package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// Codes reads a survey's codes for one codebook version, by HiddenKey.
func (s *Store) Codes(ctx context.Context, surveyID string, version int) (map[string]*survey.TextCodes, error) {
	return readCodes(ctx, s.Pool, surveyID, version)
}

func readCodes(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, surveyID string, version int) (map[string]*survey.TextCodes, error) {
	rows, err := db.Query(ctx, `SELECT field, text_hash, coder, category, onske FROM survey_freetext_codes
		WHERE survey_id = $1 AND codebook_version = $2 ORDER BY category, onske`, surveyID, version)
	if err != nil {
		return nil, err
	}
	out := map[string]*survey.TextCodes{}
	var field, hash, coder, cat string
	var onske bool
	_, err = pgx.ForEachRow(rows, []any{&field, &hash, &coder, &cat, &onske}, func() error {
		k := survey.HiddenKey(field, hash)
		if out[k] == nil {
			out[k] = &survey.TextCodes{}
		}
		c := survey.Code{Category: cat, Onske: onske}
		switch coder {
		case "model":
			out[k].Model = append(out[k].Model, c)
		case "human":
			out[k].Human = append(out[k].Human, c)
		case "final":
			out[k].Final = append(out[k].Final, c)
		}
		return nil
	})
	return out, err
}

// SaveCodes replaces one coder's codes for a text. human is written once
// (errCoded after). final also logs who decided and when, never the text.
// It takes the lock purge takes and refuses a text no row holds any more
// (errGone): a save can then not land between purge counting the codes and
// deleting the rows, where it would be lost.
func (s *Store) SaveCodes(ctx context.Context, surveyID, field, hash, coder string, version int, codes []survey.Code, by string) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('survey_results:' || $1))`, surveyID); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM survey_answers WHERE survey_id = $1
			AND encode(sha256(convert_to(answers->>$2, 'UTF8')), 'hex') = $3)`, surveyID, field, hash).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return survey.ErrGone
		}
		if coder == "human" {
			var seen bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM survey_freetext_codes WHERE survey_id = $1 AND field = $2 AND text_hash = $3 AND coder = 'human' AND codebook_version = $4)`,
				surveyID, field, hash, version).Scan(&seen); err != nil {
				return err
			}
			if seen {
				return survey.ErrCoded
			}
		}
		batch := &pgx.Batch{}
		batch.Queue(`DELETE FROM survey_freetext_codes WHERE survey_id = $1 AND field = $2 AND text_hash = $3 AND coder = $4 AND codebook_version = $5`, surveyID, field, hash, coder, version)
		for _, c := range codes {
			batch.Queue(`INSERT INTO survey_freetext_codes (survey_id, field, text_hash, coder, category, onske, codebook_version) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				surveyID, field, hash, coder, c.Category, c.Onske, version)
		}
		if coder == "final" {
			batch.Queue(`INSERT INTO survey_freetext_decisions (survey_id, field, text_hash, decided_by, decided_at) VALUES ($1, $2, $3, $4, $5)`,
				surveyID, field, hash, by, time.Now().UTC())
		}
		return tx.SendBatch(ctx, batch).Close()
	})
}

// writeCodingResults replaces s's coding counts in survey_results from its
// rows and codes, while every row is there, so the counts outlive the rows.
// Per field, kind freetext, version the codebook's: option "" has count
// coded and n written; option a category or merged pair has its count and
// kappa (mean). Kind freetext_onske has the wish counts.
func writeCodingResults(ctx context.Context, tx pgx.Tx, cb survey.Codebook, s survey.Survey, rows []survey.StoredAnswer) error {
	hidden := map[string]bool{}
	res, err := tx.Query(ctx, `SELECT field, text_hash FROM survey_freetext_hidden WHERE survey_id = $1`, s.ID)
	if err != nil {
		return err
	}
	var f, h string
	if _, err = pgx.ForEachRow(res, []any{&f, &h}, func() error { hidden[survey.HiddenKey(f, h)] = true; return nil }); err != nil {
		return err
	}
	codes, err := readCodes(ctx, tx, s.ID, cb.Version)
	if err != nil {
		return err
	}
	batch := &pgx.Batch{}
	batch.Queue(`DELETE FROM survey_results WHERE survey_id = $1 AND kind IN ('freetext', 'freetext_onske')`, s.ID)
	insert := `INSERT INTO survey_results (survey_id, kind, key, version, option, count, n, mean) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	for field, t := range survey.TallyAll(cb, survey.RespondentCodes(cb, rows, hidden, codes)) {
		batch.Queue(insert, s.ID, "freetext", field, cb.Version, "", t.Coded, t.Written, nil)
		for id, c := range t.Cats {
			batch.Queue(insert, s.ID, "freetext", field, cb.Version, id, c.Count, t.Written, c.Kappa)
			batch.Queue(insert, s.ID, "freetext_onske", field, cb.Version, id, c.Onske, t.Written, nil)
		}
	}
	return tx.SendBatch(ctx, batch).Close()
}

// FinalCoding reads a survey's stored coding counts for one codebook
// version; nil when it has none.
func (s *Store) FinalCoding(ctx context.Context, surveyID string, version int) (map[string]survey.FieldTally, error) {
	rows, err := s.Pool.Query(ctx, `SELECT kind, key, option, count, n, mean FROM survey_results
		WHERE survey_id = $1 AND kind IN ('freetext', 'freetext_onske') AND version = $2`, surveyID, version)
	if err != nil {
		return nil, err
	}
	var out map[string]survey.FieldTally
	var kind, field, opt string
	var count, n int
	var mean *float64
	_, err = pgx.ForEachRow(rows, []any{&kind, &field, &opt, &count, &n, &mean}, func() error {
		if out == nil {
			out = map[string]survey.FieldTally{}
		}
		t := out[field]
		if t.Cats == nil {
			t.Cats = map[string]survey.CatTally{}
		}
		c := t.Cats[opt]
		switch {
		case kind == "freetext_onske":
			c.Onske = count
		case opt == "":
			t.Coded, t.Written = count, n
		default:
			c.Count, c.Kappa = count, mean
		}
		if opt != "" {
			t.Cats[opt] = c
		}
		out[field] = t
		return nil
	})
	return out, err
}
