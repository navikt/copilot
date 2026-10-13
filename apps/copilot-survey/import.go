package main

import (
	"context"
	"embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// importFiles maps a Forms export of an earlier survey onto its definition:
// which column is which question id, and which answer texts change on the
// way in. Answers themselves are never committed.
//
//go:embed imports
var importFiles embed.FS

type importMapping struct {
	Survey  string            `json:"survey"`
	Columns map[string]string `json:"columns"` // CSV header -> question or matrix item id
	// Values rewrites an answer (or one part of a multi answer) before it is
	// matched against the options; null drops it. Matched without case.
	Values map[string]map[string]*string `json:"values"`
}

// importStats is everything --dry-run shows: counts, never answers.
type importStats struct {
	rows      int
	questions map[string]int // answered, per question or matrix item id
	norms     map[string]int // per normalisation applied
}

// runImport is `copilot-survey import --survey <id> [--dry-run]`: a CSV
// export on in, the survey's answers in survey_answers. It writes no
// participation rows and no hashes, and the export's identity and time
// columns must be gone before it gets here (an unmapped column is refused).
func runImport(args []string, in io.Reader, out io.Writer, dbURL string, now time.Time) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	id := fs.String("survey", "", "survey id, e.g. q2-2026")
	dry := fs.Bool("dry-run", false, "print counts, write nothing")
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
	s := defs[i]
	raw, err := importFiles.ReadFile("imports/" + s.ID + ".json")
	if err != nil {
		return fmt.Errorf("no mapping for %s: %w", s.ID, err)
	}
	var m importMapping
	if err = decodeStrict(raw, &m); err != nil || m.Survey != s.ID {
		return fmt.Errorf("mapping imports/%s.json: invalid or for another survey (%v)", s.ID, err)
	}
	rows, stats, err := convertImport(s, m, in)
	if err != nil {
		return err
	}
	deleteAfter := s.closesOn().Add(retention)
	fmt.Fprintf(out, "survey %s: %d rows, delete_after %s\n", s.ID, stats.rows, deleteAfter.Format(time.DateOnly))
	if !now.Before(deleteAfter.AddDate(0, 0, 1)) {
		fmt.Fprintf(out, "WARNING: delete_after %s has passed: the daily retention job deletes these rows at once\n", deleteAfter.Format(time.DateOnly))
	}
	for _, k := range sortedKeys(stats.questions) {
		fmt.Fprintf(out, "  answered %-24s %d\n", k, stats.questions[k])
	}
	for _, k := range sortedKeys(stats.norms) {
		fmt.Fprintf(out, "  normalised %s: %d\n", k, stats.norms[k])
	}
	if *dry {
		return nil
	}
	if dbURL == "" {
		return errors.New("DB_URL is not set")
	}
	n, err := writeImport(context.Background(), dbURL, s, rows, deleteAfter)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "wrote %d rows to survey_answers for %s (earlier rows replaced)\n", n, s.ID)
	return nil
}

// convertImport turns CSV rows into answers in the form a submission is
// stored: a number per scale or matrix item, the option text for choice, a
// list for multi, free text under <id>.other for an answer not among the
// options. An empty cell is not answered.
func convertImport(s survey, m importMapping, in io.Reader) ([]map[string]any, importStats, error) {
	stats := importStats{questions: map[string]int{}, norms: map[string]int{}}
	qs := map[string]question{}
	for _, q := range s.answerable() {
		qs[q.ID] = q
	}
	r := csv.NewReader(in)
	header, err := r.Read()
	if err != nil {
		return nil, stats, fmt.Errorf("reading header: %w", err)
	}
	cols := make([]question, len(header))
	for i, h := range header {
		qid, ok := m.Columns[strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF"))]
		if !ok {
			return nil, stats, fmt.Errorf("column %d is not in the mapping; drop identity and time columns before import", i+1)
		}
		if cols[i], ok = qs[qid]; !ok {
			return nil, stats, fmt.Errorf("mapping names %q, not a question in %s", qid, s.ID)
		}
	}
	// rewrite applies the mapping's value map; ok false means drop it.
	rewrite := func(q question, v string) (string, bool) {
		for from, to := range m.Values[q.ID] {
			if strings.EqualFold(from, v) {
				if to == nil {
					stats.norms[fmt.Sprintf("%s «%s» dropped", q.ID, v)]++
					return "", false
				}
				stats.norms[fmt.Sprintf("%s «%s» → «%s»", q.ID, v, *to)]++
				return *to, true
			}
		}
		return v, true
	}
	var out []map[string]any
	for line := 2; ; line++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, stats, err
		}
		a := map[string]any{}
		for i, q := range cols {
			v := strings.TrimSpace(rec[i])
			switch q.Type {
			case "scale":
				if v == "" {
					continue
				}
				n := slices.Index(q.Labels, v)
				if n < 0 {
					return nil, stats, fmt.Errorf("line %d, %s: not a scale label", line, q.ID)
				}
				a[q.ID] = q.Min + n
			case "text":
				if v != "" {
					a[q.ID] = v
				}
			case "choice", "multi": // Forms let a choice take several; ";" ends each
				var parts []string
				for _, p := range strings.Split(v, ";") {
					if p = strings.TrimSpace(p); p == "" {
						continue
					}
					if r, ok := rewrite(q, p); ok && !slices.Contains(parts, r) {
						parts = append(parts, r)
					}
				}
				if len(parts) > 0 {
					if err := pick(a, q, parts, stats); err != nil {
						return nil, stats, fmt.Errorf("line %d: %w", line, err)
					}
				}
			}
		}
		for k := range a {
			if _, ok := qs[k]; ok {
				stats.questions[k]++
			}
		}
		stats.rows++
		out = append(out, a)
	}
	return out, stats, nil
}

// pick stores a choice or multi answer; a value not among the options is a
// write-in, stored as q.Other with its text under <id>.other, as a
// submission stores it.
func pick(a map[string]any, q question, vals []string, stats importStats) error {
	var picked, other []string
	for _, v := range vals {
		switch {
		case slices.Contains(q.Options, v):
			picked = append(picked, v)
		case q.Other != "":
			other = append(other, v)
			stats.norms[q.ID+" write-in → «"+q.Other+"»"]++
		default:
			return fmt.Errorf("%s: %q is not an option and the question has no other", q.ID, v)
		}
	}
	if len(other) > 0 {
		picked = append(picked, q.Other)
		a[otherKey(q.ID)] = strings.Join(other, "; ")
		if n := len([]rune(a[otherKey(q.ID)].(string))); n > q.MaxLen {
			stats.norms[fmt.Sprintf("%s write-in longer than max_length %d (kept whole)", q.ID, q.MaxLen)]++
		}
	}
	if q.Type == "choice" {
		if len(picked) != 1 {
			return fmt.Errorf("%s: one answer expected", q.ID)
		}
		a[q.ID] = picked[0]
		return nil
	}
	if q.MaxChoices > 0 && len(picked) > q.MaxChoices {
		stats.norms[fmt.Sprintf("%s more than %d picked (kept)", q.ID, q.MaxChoices)]++
	}
	if q.Exclusive != "" && len(picked) > 1 && slices.Contains(picked, q.Exclusive) {
		stats.norms[fmt.Sprintf("%s «%s» with other options (kept)", q.ID, q.Exclusive)]++
	}
	a[q.ID] = picked
	return nil
}

// writeImport replaces the survey's rows in one transaction, so a rerun
// leaves no duplicates. Rows are shuffled, as submissions are, so their
// order says nothing about when someone answered.
func writeImport(ctx context.Context, dbURL string, s survey, rows []map[string]any, deleteAfter time.Time) (int, error) {
	store, err := openSurveyStore(ctx, dbURL)
	if err != nil {
		return 0, err
	}
	defer store.pool.Close()
	versions := map[string]int{}
	for _, q := range s.answerable() {
		versions[q.ID] = q.Version
	}
	rand.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
	batch := &pgx.Batch{}
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
	err = pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error { return tx.SendBatch(ctx, batch).Close() })
	return len(rows), err
}
