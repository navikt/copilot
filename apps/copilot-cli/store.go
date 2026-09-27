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
//     time. Rows are written in shuffled batches (answerBuffer), so neither
//     row order nor the time a row reached the database matches the order or
//     time of participation.
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

// answerBatch is how many answers are written together. Timing and order
// then narrow an answer down to one of at least this many participants.
const answerBatch = 10

type surveyStore struct {
	pool *pgxpool.Pool

	mu      sync.Mutex
	pending []response
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

func (s *surveyStore) participate(ctx context.Context, surveyID, hash string, closesOn time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO survey_participation (survey_id, participant_hash, closes_on) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		surveyID, hash, closesOn)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// record queues answers and writes them once answerBatch are waiting.
//
// ponytail: the queue is in memory. A crash loses up to answerBatch-1 answers
// whose participation is already recorded (those people cannot answer again).
// A graceful stop writes them (flush); a persistent queue would bring back the
// ordering this avoids.
// A failed write keeps the answers queued for the next try.
func (s *surveyStore) record(ctx context.Context, r response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = append(s.pending, r)
	if len(s.pending) < answerBatch {
		return
	}
	if err := s.flushLocked(ctx); err != nil {
		slog.Error("survey: writing queued answers failed, kept queued", "queued", len(s.pending), "error_type", fmt.Sprintf("%T", err))
	}
}

// flush writes whatever is queued, however few: at shutdown and daily.
func (s *surveyStore) flush(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.flushLocked(ctx); err != nil {
		slog.Error("survey: writing queued answers failed", "queued", len(s.pending), "error_type", fmt.Sprintf("%T", err))
	}
}

func (s *surveyStore) flushLocked(ctx context.Context) error {
	if len(s.pending) == 0 {
		return nil
	}
	rand.Shuffle(len(s.pending), func(i, j int) { s.pending[i], s.pending[j] = s.pending[j], s.pending[i] })
	batch := &pgx.Batch{}
	for _, r := range s.pending {
		answers, err1 := json.Marshal(r.Answers)
		versions, err2 := json.Marshal(r.QuestionVersions)
		techCtx, err3 := json.Marshal(r.Context)
		if err1 != nil || err2 != nil || err3 != nil {
			return fmt.Errorf("encoding answers")
		}
		batch.Queue(`INSERT INTO survey_answers (survey_id, answers, question_versions, context, delete_after) VALUES ($1, $2, $3, $4, $5)`,
			r.SurveyID, answers, versions, techCtx, r.DeleteAfter)
	}
	// One transaction: the rows commit together, with one commit time.
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return tx.SendBatch(ctx, batch).Close() })
	if err == nil {
		s.pending = nil
	}
	return err
}

// purgeExpired runs retain once at start and then daily.
func (s *surveyStore) purgeExpired(ctx context.Context) {
	for {
		s.retain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
		}
	}
}

// retain writes queued answers, deletes the participation of closed surveys
// (their keys are deleted by hand at the same time, see README), and deletes
// answers past retention.
func (s *surveyStore) retain(ctx context.Context) {
	s.flush(ctx)
	for _, q := range []string{
		`DELETE FROM survey_participation WHERE closes_on <= current_date`,
		`DELETE FROM survey_answers WHERE delete_after < current_date`,
	} {
		tag, err := s.pool.Exec(ctx, q)
		if err != nil {
			slog.Error("survey retention step failed", "error_type", fmt.Sprintf("%T", err))
		} else if tag.RowsAffected() > 0 {
			slog.Info("survey retention step", "rows", tag.RowsAffected())
		}
	}
}
