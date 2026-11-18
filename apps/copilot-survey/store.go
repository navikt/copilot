package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The data model: two tables with nothing in common but the survey id.
//
//   - survey_participation says who has answered (a per-survey keyed hash of
//     the Nav e-mail) and exists only to refuse a second answer. Deleted when
//     the survey closes.
//   - survey_answers holds the answers: no identifier, no hash, no row id, no
//     time.
//
// Each submission is written at once in one transaction: its participation row
// and its answer. While the survey is open, commit time and transaction id link
// an answer to its participation row; the participation rows are deleted when
// it closes. A failed write is undone and reported, so the sender can send the
// answer again.
const schema = `
CREATE TABLE IF NOT EXISTS survey_participation (
	survey_id        text NOT NULL,
	participant_hash text NOT NULL,
	closes_on        date NOT NULL,
	PRIMARY KEY (survey_id, participant_hash)
);
CREATE TABLE IF NOT EXISTS survey_answers (
	survey_id         text  NOT NULL,
	answers           jsonb NOT NULL,
	question_versions jsonb NOT NULL,
	context           jsonb NOT NULL,
	delete_after      date  NOT NULL
);
CREATE TABLE IF NOT EXISTS survey_results (
	survey_id text    NOT NULL,
	kind      text    NOT NULL,
	key       text    NOT NULL,
	version   integer NOT NULL,
	option    text    NOT NULL,
	count     integer NOT NULL,
	n         integer NOT NULL,
	mean      double precision,
	PRIMARY KEY (survey_id, kind, key, version, option)
);
-- Per skip_if rule set (skip_rules: skipSetID, canonical JSON) and the
-- answer keys it skips, survey_results' answer counts among the respondents
-- it skips, so compare works after the rows are deleted (writeSkipped).
CREATE TABLE IF NOT EXISTS survey_skip_results (
	survey_id     text    NOT NULL,
	skip_rules    text    NOT NULL,
	key           text    NOT NULL,
	version       integer NOT NULL,
	option        text    NOT NULL,
	count         integer NOT NULL,
	n             integer NOT NULL,
	PRIMARY KEY (survey_id, skip_rules, key, version, option)
);
CREATE TABLE IF NOT EXISTS survey_freetext_hidden (
	survey_id text NOT NULL,
	field     text NOT NULL,
	text_hash text NOT NULL, -- sha256 of the raw text, see freetext.go
	PRIMARY KEY (survey_id, field, text_hash)
)`

type surveyStore struct {
	pool *pgxpool.Pool
	defs []survey // for closeOut: final aggregates and next-round retention
	// purgeHook, in tests only, runs after purge decides that a survey's
	// expired rows may go and before it deletes them.
	purgeHook func(surveyID string)
}

func openSurveyStore(ctx context.Context, dbURL string) (*surveyStore, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}
	return &surveyStore{pool: pool}, nil
}

// submit writes one submission in a single transaction. false means hash
// already answered. When the write fails, the transaction rolls back and the
// error is returned so the sender can send the answer again.
func (s *surveyStore) submit(ctx context.Context, surveyID, hash string, closesOn time.Time, r response) (bool, error) {
	answers, err1 := json.Marshal(r.Answers)
	versions, err2 := json.Marshal(r.QuestionVersions)
	techCtx, err3 := json.Marshal(r.Context)
	if err1 != nil || err2 != nil || err3 != nil {
		return false, fmt.Errorf("encoding answers")
	}
	fresh := false
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`INSERT INTO survey_participation (survey_id, participant_hash, closes_on) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			surveyID, hash, closesOn)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO survey_answers (survey_id, answers, question_versions, context, delete_after) VALUES ($1, $2, $3, $4, $5)`,
			surveyID, answers, versions, techCtx, r.DeleteAfter)
		if err != nil {
			return err
		}
		fresh = true
		return nil
	})
	return fresh, err
}

// answered: hash has answered surveyID.
func (s *surveyStore) answered(ctx context.Context, surveyID, hash string) (bool, error) {
	var seen bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM survey_participation WHERE survey_id = $1 AND participant_hash = $2)`,
		surveyID, hash).Scan(&seen)
	return seen, err
}

// purgeExpired runs retain once at start and then daily.
func (s *surveyStore) purgeExpired(ctx context.Context, now func() time.Time) {
	for {
		s.retain(ctx, now())
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
		}
	}
}

// retain closes out surveys that no longer take answers: writes their final
// aggregates (closeOut) and then deletes their participation rows (their keys
// are deleted by hand, see README). It also deletes answers past retention.
func (s *surveyStore) retain(parent context.Context, now time.Time) {
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	s.closeOut(ctx, now)
	today := now.UTC().Truncate(24 * time.Hour)
	ids := make([]string, len(s.defs))
	for i, d := range s.defs {
		ids[i] = d.ID
		n, err := s.purge(ctx, d, today)
		if err != nil {
			slog.Error("survey retention step failed", "survey", d.ID, "error_type", fmt.Sprintf("%T", err))
		} else if n > 0 {
			slog.Info("survey retention step", "survey", d.ID, "rows", n)
		}
	}
	// A survey no longer defined has nothing to aggregate with and expires
	// as before; import needs a definition, so nothing races this.
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM survey_participation WHERE closes_on <= $1`, []any{today}},
		{`DELETE FROM survey_answers WHERE delete_after < $1 AND NOT survey_id = ANY($2)`, []any{today, ids}},
	} {
		tag, err := s.pool.Exec(ctx, q.sql, q.args...)
		if err != nil {
			slog.Error("survey retention step failed", "error_type", fmt.Sprintf("%T", err))
		} else if tag.RowsAffected() > 0 {
			slog.Info("survey retention step", "rows", tag.RowsAffected())
		}
	}
}

// purge deletes d's expired answers once it has aggregates and skip counts
// (deletable), so a failed close-out is retried the next day instead of
// losing the counts. Decision and delete run in one transaction under the
// lock finalize and writeImport take, so an import cannot replace the rows
// in between. Each of the three takes only this one lock: no deadlock.
func (s *surveyStore) purge(ctx context.Context, d survey, today time.Time) (int64, error) {
	var n int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('survey_results:' || $1))`, d.ID); err != nil {
			return err
		}
		ok, err := s.deletable(ctx, tx, d)
		if err != nil || !ok {
			return err
		}
		if s.purgeHook != nil {
			s.purgeHook(d.ID)
		}
		tag, err := tx.Exec(ctx, `DELETE FROM survey_answers WHERE survey_id = $1 AND delete_after < $2`, d.ID, today)
		n = tag.RowsAffected()
		return err
	})
	return n, err
}

// deletable: d's expired rows may go. It has aggregates, and its skip
// counts cover every skip_if in the definitions, or its rows no longer
// match its total (partly deleted: the counts can no longer be written).
// Otherwise it logs that the rows are kept.
func (s *surveyStore) deletable(ctx context.Context, tx pgx.Tx, d survey) (bool, error) {
	var total *int
	var rows int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count FROM survey_results WHERE survey_id = $1 AND kind = 'total'),
		(SELECT count(*) FROM survey_answers WHERE survey_id = $1)`, d.ID).Scan(&total, &rows); err != nil || total == nil {
		return false, err
	}
	if *total != rows {
		return true, nil
	}
	have := map[[2]string]bool{}
	res, err := tx.Query(ctx, `SELECT skip_rules, key FROM survey_skip_results WHERE survey_id = $1 AND version = -1`, d.ID)
	if err != nil {
		return false, err
	}
	var set, k string
	if _, err := pgx.ForEachRow(res, []any{&set, &k}, func() error { have[[2]string{set, k}] = true; return nil }); err != nil {
		return false, err
	}
	for id, set := range skipSets(s.defs) {
		for _, k := range set.Keys {
			if !have[[2]string{id, k}] {
				slog.Warn("survey: rows kept until skip counts are written", "survey", d.ID)
				return false, nil
			}
		}
	}
	return true, nil
}

// answers returns a survey's written rows: answers, question versions and
// context, nothing else. Submissions still queued are not included.
func (s *surveyStore) answers(ctx context.Context, surveyID string) ([]storedAnswer, error) {
	rows, err := s.pool.Query(ctx, `SELECT answers, question_versions, context FROM survey_answers WHERE survey_id = $1`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storedAnswer
	for rows.Next() {
		var a storedAnswer
		if err := rows.Scan(&a.Answers, &a.QuestionVersions, &a.Context); err != nil {
			return nil, err
		}
		a.Client, _ = a.Context["client"].(string)
		out = append(out, a)
	}
	return out, rows.Err()
}
