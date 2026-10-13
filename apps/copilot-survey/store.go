package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
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
// Neither is written one submission at a time. Each survey queues
// submissions in memory and writes answerBatch of them at once: their
// participation rows and their answers, each shuffled, in one transaction.
// Commit time, transaction id and row order then place an answer among
// answerBatch participants and no fewer, until the last, smaller batch when
// the survey closes, after which the participation rows are deleted.
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
)`

// answerBatch is k: the fewest participants any written answer can be
// narrowed down to while a survey is open.
const answerBatch = 10

type surveyStore struct {
	pool *pgxpool.Pool

	mu     sync.Mutex
	queues map[string]*surveyQueue
}

// surveyQueue is one survey's submissions not written yet.
//
// ponytail: in memory, in one pod (replicas max 1: a second pod would have
// its own queue and dedup). A graceful shutdown writes them as a smaller
// batch (flush); only a crash or a failed write loses them, and their
// senders may then answer again.
type surveyQueue struct {
	closesOn time.Time
	hashes   map[string]bool
	answers  []response
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
	return &surveyStore{pool: pool, queues: map[string]*surveyQueue{}}, nil
}

// submit queues one submission. false means hash already answered, written
// or queued. A failed batch write stays queued for the next try and is not
// the caller's problem.
func (s *surveyStore) submit(ctx context.Context, surveyID, hash string, closesOn time.Time, r response) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queues[surveyID]
	if q == nil {
		q = &surveyQueue{closesOn: closesOn, hashes: map[string]bool{}}
		s.queues[surveyID] = q
	}
	if q.hashes[hash] {
		return false, nil
	}
	var seen bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM survey_participation WHERE survey_id = $1 AND participant_hash = $2)`,
		surveyID, hash).Scan(&seen)
	if err != nil || seen {
		return false, err
	}
	q.hashes[hash] = true
	q.answers = append(q.answers, r)
	if len(q.answers) >= answerBatch {
		if err := s.writeLocked(ctx, surveyID, q); err != nil {
			slog.Error("survey: writing a batch failed, kept queued", "survey", surveyID, "error_type", fmt.Sprintf("%T", err))
		}
	}
	return true, nil
}

func (s *surveyStore) writeLocked(ctx context.Context, surveyID string, q *surveyQueue) error {
	hashes := make([]string, 0, len(q.hashes))
	for h := range q.hashes {
		hashes = append(hashes, h)
	}
	rand.Shuffle(len(hashes), func(i, j int) { hashes[i], hashes[j] = hashes[j], hashes[i] })
	rand.Shuffle(len(q.answers), func(i, j int) { q.answers[i], q.answers[j] = q.answers[j], q.answers[i] })

	batch := &pgx.Batch{}
	for _, h := range hashes {
		batch.Queue(`INSERT INTO survey_participation (survey_id, participant_hash, closes_on) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			surveyID, h, q.closesOn)
	}
	for _, r := range q.answers {
		answers, err1 := json.Marshal(r.Answers)
		versions, err2 := json.Marshal(r.QuestionVersions)
		techCtx, err3 := json.Marshal(r.Context)
		if err1 != nil || err2 != nil || err3 != nil {
			return fmt.Errorf("encoding answers")
		}
		batch.Queue(`INSERT INTO survey_answers (survey_id, answers, question_versions, context, delete_after) VALUES ($1, $2, $3, $4, $5)`,
			surveyID, answers, versions, techCtx, r.DeleteAfter)
	}
	if err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return tx.SendBatch(ctx, batch).Close() }); err != nil {
		return err
	}
	q.hashes, q.answers = map[string]bool{}, nil
	return nil
}

// flush writes every queued submission at shutdown, also batches smaller
// than answerBatch. Losing answers on every deploy is the larger harm; the
// data is down-classified (owner, 2026-10-13). It returns how many answers
// could not be written.
func (s *surveyStore) flush(ctx context.Context) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	lost := 0
	for id, q := range s.queues {
		if len(q.answers) == 0 {
			continue
		}
		if err := s.writeLocked(ctx, id, q); err != nil {
			slog.Error("survey: writing queued submissions at shutdown failed", "survey", id, "error_type", fmt.Sprintf("%T", err))
			lost += len(q.answers)
		}
	}
	return lost
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

// retain closes out surveys that no longer take answers: writes their last,
// smaller batch and then deletes their participation rows (their keys are
// deleted by hand, see README). It also deletes answers past retention.
func (s *surveyStore) retain(parent context.Context, now time.Time) {
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	s.mu.Lock()
	for id, q := range s.queues {
		if !now.Before(q.closesOn) {
			if err := s.writeLocked(ctx, id, q); err != nil {
				slog.Error("survey: writing the last batch failed", "survey", id, "error_type", fmt.Sprintf("%T", err))
				continue
			}
			delete(s.queues, id)
		}
	}
	s.mu.Unlock()
	today := now.UTC().Truncate(24 * time.Hour)
	for _, q := range []string{
		`DELETE FROM survey_participation WHERE closes_on <= $1`,
		`DELETE FROM survey_answers WHERE delete_after < $1`,
	} {
		tag, err := s.pool.Exec(ctx, q, today)
		if err != nil {
			slog.Error("survey retention step failed", "error_type", fmt.Sprintf("%T", err))
		} else if tag.RowsAffected() > 0 {
			slog.Info("survey retention step", "rows", tag.RowsAffected())
		}
	}
}

// answers returns a survey's written rows: answers and question versions,
// nothing else. Submissions still queued are not included.
func (s *surveyStore) answers(ctx context.Context, surveyID string) ([]storedAnswer, error) {
	rows, err := s.pool.Query(ctx, `SELECT answers, question_versions FROM survey_answers WHERE survey_id = $1`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storedAnswer
	for rows.Next() {
		var a storedAnswer
		if err := rows.Scan(&a.Answers, &a.QuestionVersions); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
