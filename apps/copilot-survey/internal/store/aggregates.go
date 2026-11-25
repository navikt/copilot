package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// finalize writes s's aggregates from its rows in survey_answers, replacing
// any earlier ones, in one transaction that also extends the rows' retention.
// Unless replace is set, a survey that already has aggregates is left alone:
// its rows may be partly deleted by then. No rows: nothing is written, so an
// import that comes later still gets aggregated. Returns respondents.
func finalize(ctx context.Context, pool *pgxpool.Pool, defs []survey.Survey, s survey.Survey, now time.Time, replace bool) (int, error) {
	var n int
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// Serialises close-out and the aggregate command for this survey.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('survey_results:' || $1))`, s.ID); err != nil {
			return err
		}
		rows, ctxRows, err := readRows(ctx, tx, s.ID)
		if err != nil || len(rows) == 0 {
			return err
		}
		if !replace {
			var total int
			err := tx.QueryRow(ctx, `SELECT count FROM survey_results WHERE survey_id = $1 AND kind = 'total'`, s.ID).Scan(&total)
			if err == nil {
				// Aggregated already: redo only the skip counts, for skip_if
				// defined since, and only while every row is still there.
				if total != len(rows) {
					return nil
				}
				return writeSkipped(ctx, tx, survey.SkipSets(defs), s, rows)
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		n = len(rows)
		batch := &pgx.Batch{}
		batch.Queue(`DELETE FROM survey_results WHERE survey_id = $1 AND kind NOT IN ('freetext', 'freetext_onske')`, s.ID)
		insert := `INSERT INTO survey_results (survey_id, kind, key, version, option, count, n, mean) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
		batch.Queue(insert, s.ID, "total", "", 0, "", n, n, nil)
		t := survey.TallyRows(s, rows)
		for cl, c := range t.Channels {
			batch.Queue(insert, s.ID, "channel", "", 0, cl, c, n, nil)
		}
		scale := survey.AnswerKeys(s)
		for k, vers := range t.Keys {
			for ver, vt := range vers {
				var mean *float64
				if scale[k] {
					sum, cnt := 0.0, 0
					for o, c := range vt.Counts {
						if f, err := strconv.ParseFloat(o, 64); err == nil { // unsure is not a step
							sum, cnt = sum+f*float64(c), cnt+c
						}
					}
					if cnt > 0 {
						m := sum / float64(cnt)
						mean = &m
					}
				}
				for o, c := range vt.Counts {
					batch.Queue(insert, s.ID, "answer", k, ver, o, c, vt.N, mean)
				}
			}
		}
		survey.SuppressSmallSegments(ctxRows)
		ctxCounts := map[[2]string]int{}
		for _, r := range ctxRows {
			for k, v := range r.Context {
				ctxCounts[[2]string{k, v}]++
			}
		}
		for kv, c := range ctxCounts {
			batch.Queue(insert, s.ID, "context", kv[0], 0, kv[1], c, n, nil)
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return err
		}
		if err := writeSkipped(ctx, tx, survey.SkipSets(defs), s, rows); err != nil {
			return err
		}
		return extendRetention(ctx, tx, defs, s, now)
	})
	return n, err
}

// writeSkipped replaces s's survey_skip_results: for every skip_if rule
// set and every answer key a definition skips with it, the counts among the
// respondents the set skips (any of its rules, each respondent once), as
// compare subtracts them (#2227). With survey_results that is the cross
// table compare already shows as excluded and kept counts; nothing per row.
// Version -1, option ”: the pair is counted, count = respondents it skips
// who answered the key.
func writeSkipped(ctx context.Context, tx pgx.Tx, sets map[string]survey.SkipSet, s survey.Survey, rows []survey.StoredAnswer) error {
	batch := &pgx.Batch{}
	batch.Queue(`DELETE FROM survey_skip_results WHERE survey_id = $1`, s.ID)
	insert := `INSERT INTO survey_skip_results (survey_id, skip_rules, key, version, option, count, n) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	t := survey.RowsTally(s, rows, sets)
	for id, set := range sets {
		for _, k := range set.Keys {
			excluded := 0
			for ver, vt := range t.Skipped[id][k] {
				excluded += vt.N
				for o, c := range vt.Counts {
					batch.Queue(insert, s.ID, id, k, ver, o, c, vt.N)
				}
			}
			batch.Queue(insert, s.ID, id, k, -1, "", excluded, 0)
		}
	}
	return tx.SendBatch(ctx, batch).Close()
}

func readRows(ctx context.Context, tx pgx.Tx, id string) ([]survey.StoredAnswer, []survey.ExportRow, error) {
	res, err := tx.Query(ctx, `SELECT answers, question_versions, context FROM survey_answers WHERE survey_id = $1`, id)
	if err != nil {
		return nil, nil, err
	}
	defer res.Close()
	var rows []survey.StoredAnswer
	var ctxRows []survey.ExportRow
	for res.Next() {
		var a survey.StoredAnswer
		var c map[string]any
		if err := res.Scan(&a.Answers, &a.QuestionVersions, &c); err != nil {
			return nil, nil, err
		}
		a.Client, _ = c["client"].(string)
		rows = append(rows, a)
		ctxRows = append(ctxRows, survey.ExportRow{Context: survey.ContextStrings(c)})
	}
	return rows, ctxRows, res.Err()
}

// extendRetention keeps an aggregated survey's rows until the next wave has
// closed (owner, 2026-10-13): delete_after becomes the next wave's last day,
// but never earlier than it is (retention_days, e.g. q2-2026's 730, wins
// when it is longer). Before the next wave closes delete_after is unchanged.
func extendRetention(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, defs []survey.Survey, s survey.Survey, now time.Time) error {
	end, ok := survey.NextRoundEnd(defs, s, now)
	if !ok {
		return nil
	}
	_, err := db.Exec(ctx, `UPDATE survey_answers SET delete_after = $2 WHERE survey_id = $1 AND delete_after < $2
		AND EXISTS (SELECT 1 FROM survey_results WHERE survey_id = $1)`, s.ID, end)
	return err
}

// CloseOut aggregates every closed survey without aggregates and extends
// the retention of those with them. Runs in retain, before rows expire.
func (s *Store) CloseOut(ctx context.Context, now time.Time) {
	for _, d := range s.Defs {
		if d.ClosesOn().After(now) {
			continue
		}
		if n, err := finalize(ctx, s.Pool, s.Defs, d, now, false); err != nil {
			slog.Error("survey: writing final aggregates failed", "survey", d.ID, "error_type", fmt.Sprintf("%T", err))
		} else if n > 0 {
			slog.Info("survey: final aggregates written", "survey", d.ID, "responses", n)
		}
		if err := extendRetention(ctx, s.Pool, s.Defs, d, now); err != nil {
			slog.Error("survey: extending retention failed", "survey", d.ID, "error_type", fmt.Sprintf("%T", err))
		}
	}
}

// Final reads a survey's aggregates; nil when it has none.
func (s *Store) Final(ctx context.Context, id string) (*survey.Tally, error) {
	rows, err := s.Pool.Query(ctx, `SELECT kind, key, version, option, count, n FROM survey_results WHERE survey_id = $1 AND kind IN ('total', 'answer', 'channel', 'context')`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var t *survey.Tally
	channels := map[string]int{}
	ctxClients := map[string]int{}
	for rows.Next() {
		var kind, key, opt string
		var ver, count, n int
		if err = rows.Scan(&kind, &key, &ver, &opt, &count, &n); err != nil {
			return nil, err
		}
		if t == nil {
			t = &survey.Tally{Keys: map[string]map[int]*survey.VerTally{}, Channels: map[string]int{}}
		}
		switch kind {
		case "total":
			t.Responses = count
		case "channel":
			if opt != "" {
				channels[opt] = count
			}
		case "context":
			if key == "client" && opt != "" {
				ctxClients[opt] = count
			}
		case "answer":
			if t.Keys[key] == nil {
				t.Keys[key] = map[int]*survey.VerTally{}
			}
			if t.Keys[key][ver] == nil {
				t.Keys[key][ver] = &survey.VerTally{N: n, Counts: map[string]int{}}
			}
			t.Keys[key][ver].Counts[opt] = count
		}
	}
	if err = rows.Err(); err != nil || t == nil {
		return t, err
	}
	if len(channels) > 0 {
		t.Channels = channels
		return t, nil
	}
	// Surveys finalized before kind='channel' existed: reconstruct from
	// survey_answers if all rows are still present; otherwise fall back to
	// the suppressed context counts (which include «annet» for merged rows).
	cRows, err := s.Pool.Query(ctx, `SELECT coalesce(context->>'client', ''), count(*) FROM survey_answers WHERE survey_id = $1 GROUP BY 1`, id)
	if err != nil {
		return nil, err
	}
	defer cRows.Close()
	liveClients := map[string]int{}
	liveTotal := 0
	for cRows.Next() {
		var cl string
		var c int
		if err = cRows.Scan(&cl, &c); err != nil {
			return nil, err
		}
		liveTotal += c
		if cl != "" {
			liveClients[cl] = c
		}
	}
	if err = cRows.Err(); err != nil {
		return nil, err
	}
	if liveTotal == t.Responses && len(liveClients) > 0 {
		t.Channels = liveClients
	} else {
		t.Channels = ctxClients
	}
	return t, nil
}

// FinalCompare reads what compare needs from a survey's aggregates; nil
// when it has none. Channels are the stored context.client counts, merged as
// the export merges them, with the rest under "" as MergeClients gives them.
func (s *Store) FinalCompare(ctx context.Context, id string) (*survey.WaveTally, error) {
	f, err := s.Final(ctx, id)
	if err != nil || f == nil {
		return nil, err
	}
	t := &survey.WaveTally{Responses: f.Responses, Channels: map[string]int{}, Keys: f.Keys, Skipped: map[string]map[string]map[int]*survey.VerTally{}}
	var set, key, opt string
	var ver, count, n int
	rows, err := s.Pool.Query(ctx, `SELECT option, count FROM survey_results WHERE survey_id = $1 AND kind = 'context' AND key = 'client'`, id)
	if err != nil {
		return nil, err
	}
	rest := t.Responses
	if _, err = pgx.ForEachRow(rows, []any{&opt, &count}, func() error {
		t.Channels[opt] += count
		rest -= count
		return nil
	}); err != nil {
		return nil, err
	}
	if rest > 0 {
		t.Channels[""] += rest
	}
	rows, err = s.Pool.Query(ctx, `SELECT skip_rules, key, version, option, count, n FROM survey_skip_results WHERE survey_id = $1`, id)
	if err != nil {
		return nil, err
	}
	_, err = pgx.ForEachRow(rows, []any{&set, &key, &ver, &opt, &count, &n}, func() error {
		if t.Skipped[set] == nil {
			t.Skipped[set] = map[string]map[int]*survey.VerTally{}
		}
		keys := t.Skipped[set]
		if keys[key] == nil {
			keys[key] = map[int]*survey.VerTally{}
		}
		if ver == -1 {
			return nil
		}
		if keys[key][ver] == nil {
			keys[key][ver] = &survey.VerTally{N: n, Counts: map[string]int{}}
		}
		keys[key][ver].Counts[opt] = count
		return nil
	})
	return t, err
}

// Aggregate writes a closed survey's final aggregates for the aggregate
// command (survey.RunAggregate).
func Aggregate(ctx context.Context, dbURL string, defs []survey.Survey, s survey.Survey, now time.Time) (int, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, schema); err != nil {
		return 0, err
	}
	return finalize(ctx, pool, defs, s, now, true)
}
