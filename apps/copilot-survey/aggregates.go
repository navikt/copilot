package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Final aggregates (#1774). When a survey closes, its counts go to
// survey_results, one row per (kind, key, version, option):
//
//   - total:   key "", option "", count = respondents.
//   - answer:  key = question id or matrix item id; count per option, n =
//     respondents for that version, mean for scale and matrix items. Free
//     text is never read.
//   - context: key = client, os, version, local_models; option = value,
//     with values held by fewer than minSegment rows merged into «annet»
//     (the export's rule, suppressSmallSegments).
//
// The admin results read these for a closed survey, so the individual
// answers can go. They are kept until the next wave of the same series has
// closed, or the survey's own retention if that is later (see
// extendRetention).

type verTally struct {
	N      int
	Counts map[string]int
}

type surveyTally struct {
	Responses int
	Keys      map[string]map[int]*verTally // answer key → version
}

func tallyKey(id string, rows []storedAnswer) map[int]*verTally {
	t := map[int]*verTally{}
	for _, row := range rows {
		v, ok := row.Answers[id]
		if !ok || v == nil {
			continue
		}
		ver := row.QuestionVersions[id]
		if t[ver] == nil {
			t[ver] = &verTally{Counts: map[string]int{}}
		}
		t[ver].N++
		switch v := v.(type) {
		case []any:
			for _, p := range v {
				t[ver].Counts[fmt.Sprint(p)]++
			}
		case float64:
			t[ver].Counts[strconv.FormatFloat(v, 'f', -1, 64)]++
		default:
			t[ver].Counts[fmt.Sprint(v)]++
		}
	}
	return t
}

// answerKeys are the keys counted for s, with whether each is on a scale.
func answerKeys(s survey) map[string]bool {
	keys := map[string]bool{}
	for _, q := range s.Questions {
		switch q.Type {
		case "text":
		case "matrix":
			for _, it := range q.Items {
				keys[it.ID] = true
			}
		default:
			keys[q.ID] = q.Type == "scale"
		}
	}
	return keys
}

func tallyRows(s survey, rows []storedAnswer) surveyTally {
	t := surveyTally{Responses: len(rows), Keys: map[string]map[int]*verTally{}}
	for k := range answerKeys(s) {
		t.Keys[k] = tallyKey(k, rows)
	}
	return t
}

// finalize writes s's aggregates from its rows in survey_answers, replacing
// any earlier ones, in one transaction that also extends the rows' retention.
// Unless replace is set, a survey that already has aggregates is left alone:
// its rows may be partly deleted by then. No rows: nothing is written, so an
// import that comes later still gets aggregated. Returns respondents.
func finalize(ctx context.Context, pool *pgxpool.Pool, defs []survey, s survey, now time.Time, replace bool) (int, error) {
	var n int
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// Serialises close-out and the aggregate command for this survey.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('survey_results:' || $1))`, s.ID); err != nil {
			return err
		}
		if !replace {
			var done bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM survey_results WHERE survey_id = $1)`, s.ID).Scan(&done); err != nil || done {
				return err
			}
		}
		rows, ctxRows, err := readRows(ctx, tx, s.ID)
		if err != nil || len(rows) == 0 {
			return err
		}
		n = len(rows)
		batch := &pgx.Batch{}
		batch.Queue(`DELETE FROM survey_results WHERE survey_id = $1`, s.ID)
		insert := `INSERT INTO survey_results (survey_id, kind, key, version, option, count, n, mean) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
		batch.Queue(insert, s.ID, "total", "", 0, "", n, n, nil)
		t := tallyRows(s, rows)
		scale := answerKeys(s)
		for k, vers := range t.Keys {
			for ver, vt := range vers {
				var mean *float64
				if scale[k] && vt.N > 0 {
					sum := 0.0
					for o, c := range vt.Counts {
						f, _ := strconv.ParseFloat(o, 64)
						sum += f * float64(c)
					}
					m := sum / float64(vt.N)
					mean = &m
				}
				for o, c := range vt.Counts {
					batch.Queue(insert, s.ID, "answer", k, ver, o, c, vt.N, mean)
				}
			}
		}
		suppressSmallSegments(ctxRows)
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
		return extendRetention(ctx, tx, defs, s, now)
	})
	return n, err
}

func readRows(ctx context.Context, tx pgx.Tx, id string) ([]storedAnswer, []exportRow, error) {
	res, err := tx.Query(ctx, `SELECT answers, question_versions, context FROM survey_answers WHERE survey_id = $1`, id)
	if err != nil {
		return nil, nil, err
	}
	defer res.Close()
	var rows []storedAnswer
	var ctxRows []exportRow
	for res.Next() {
		var a storedAnswer
		var c map[string]any
		if err := res.Scan(&a.Answers, &a.QuestionVersions, &c); err != nil {
			return nil, nil, err
		}
		rows = append(rows, a)
		ctxRows = append(ctxRows, exportRow{Context: contextStrings(c)})
	}
	return rows, ctxRows, res.Err()
}

func contextStrings(c map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range c {
		if b, ok := v.(bool); ok {
			out[k] = strconv.FormatBool(b)
		} else {
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

// nextRoundEnd is the last day of the next wave of s's series (by start
// date) that has closed. false: none has closed yet, or s has no series.
func nextRoundEnd(defs []survey, s survey, now time.Time) (time.Time, bool) {
	var next *survey
	for i, d := range defs {
		if s.Series != "" && d.Series == s.Series && d.Starts > s.Starts && (next == nil || d.Starts < next.Starts) {
			next = &defs[i]
		}
	}
	if next == nil || next.status(now) != "stengt" {
		return time.Time{}, false
	}
	end, err := time.Parse(time.DateOnly, next.Ends)
	return end, err == nil
}

// extendRetention keeps an aggregated survey's rows until the next wave has
// closed (owner, 2026-10-13): delete_after becomes the next wave's last day,
// but never earlier than it is (retention_days, e.g. q2-2026's 730, wins
// when it is longer). Before the next wave closes delete_after is unchanged.
func extendRetention(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, defs []survey, s survey, now time.Time) error {
	end, ok := nextRoundEnd(defs, s, now)
	if !ok {
		return nil
	}
	_, err := db.Exec(ctx, `UPDATE survey_answers SET delete_after = $2 WHERE survey_id = $1 AND delete_after < $2
		AND EXISTS (SELECT 1 FROM survey_results WHERE survey_id = $1)`, s.ID, end)
	return err
}

// closeOut aggregates every closed survey without aggregates and extends
// the retention of those with them. Runs in retain, before rows expire.
func (s *surveyStore) closeOut(ctx context.Context, now time.Time) {
	for _, d := range s.defs {
		if d.status(now) != "stengt" {
			continue
		}
		if n, err := finalize(ctx, s.pool, s.defs, d, now, false); err != nil {
			slog.Error("survey: writing final aggregates failed", "survey", d.ID, "error_type", fmt.Sprintf("%T", err))
		} else if n > 0 {
			slog.Info("survey: final aggregates written", "survey", d.ID, "responses", n)
		}
		if err := extendRetention(ctx, s.pool, s.defs, d, now); err != nil {
			slog.Error("survey: extending retention failed", "survey", d.ID, "error_type", fmt.Sprintf("%T", err))
		}
	}
}

// final reads a survey's aggregates; nil when it has none.
func (s *surveyStore) final(ctx context.Context, id string) (*surveyTally, error) {
	rows, err := s.pool.Query(ctx, `SELECT kind, key, version, option, count, n FROM survey_results WHERE survey_id = $1 AND kind IN ('total', 'answer')`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var t *surveyTally
	for rows.Next() {
		var kind, key, opt string
		var ver, count, n int
		if err := rows.Scan(&kind, &key, &ver, &opt, &count, &n); err != nil {
			return nil, err
		}
		if t == nil {
			t = &surveyTally{Keys: map[string]map[int]*verTally{}}
		}
		if kind == "total" {
			t.Responses = count
			continue
		}
		if t.Keys[key] == nil {
			t.Keys[key] = map[int]*verTally{}
		}
		if t.Keys[key][ver] == nil {
			t.Keys[key][ver] = &verTally{N: n, Counts: map[string]int{}}
		}
		t.Keys[key][ver].Counts[opt] = count
	}
	return t, rows.Err()
}

// runAggregate is `copilot-survey aggregate --survey <id>`: (re)writes a
// closed survey's aggregates from its rows, for a survey that had no
// close-out in this service (an import such as q2-2026), or to redo one.
func runAggregate(args []string, out io.Writer, dbURL string, now time.Time) error {
	fs := flag.NewFlagSet("aggregate", flag.ContinueOnError)
	id := fs.String("survey", "", "survey id, e.g. q2-2026")
	if err := fs.Parse(args); err != nil {
		return err
	}
	defs, err := loadSurveyDir(surveyFiles)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(defs, func(s survey) bool { return s.ID == *id })
	if i < 0 {
		return fmt.Errorf("no survey %q", *id)
	}
	if defs[i].status(now) != "stengt" {
		return fmt.Errorf("%s has not closed", *id)
	}
	if dbURL == "" {
		return errors.New("DB_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, schema); err != nil {
		return err
	}
	n, err := finalize(ctx, pool, defs, defs[i], now, true)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s has no rows in survey_answers: nothing written", *id)
	}
	fmt.Fprintf(out, "wrote aggregates for %s: %d responses\n", *id, n)
	return nil
}
