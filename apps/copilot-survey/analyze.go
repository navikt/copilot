package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// readExportCSV is writeExportCSV's inverse: rows as storedAnswer, for the
// definition s the CSV was exported from. It refuses a CSV whose columns are
// not exactly what s exports, so an edited or mismatched file fails rather
// than giving other numbers.
//
// What the CSV cannot give back: hidden or redacted free text (never
// counted), multi picks outside the current options, and context values
// under minSegment, which the export merged into «annet». The answer counts
// therefore match the admin page exactly; the channels match only at the
// merged level.
func readExportCSV(s survey, r io.Reader) ([]storedAnswer, error) {
	cr := csv.NewReader(r)
	head, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	if len(head) > 0 {
		head[0] = strings.TrimPrefix(head[0], "\ufeff")
	}
	col := map[string]int{}
	for i, h := range head {
		col[h] = i
	}
	var want []string
	// The writer's order: answer, «.other», «.version» when present.
	versioned := func(name string) {
		if _, ok := col[name+".version"]; ok {
			want = append(want, name+".version")
		}
	}
	for _, q := range s.Questions {
		switch q.Type {
		case "matrix":
			for _, it := range q.Items {
				want = append(want, q.ID+"."+it.ID)
				versioned(q.ID + "." + it.ID)
			}
			continue
		case "multi":
			for _, o := range multiOptions(q, head) {
				want = append(want, q.ID+"="+o)
			}
		default:
			want = append(want, q.ID)
		}
		if q.Other != "" {
			want = append(want, otherKey(q.ID))
		}
		versioned(q.ID)
	}
	for _, k := range exportCSVContext {
		want = append(want, "context."+k)
	}
	if !slices.Equal(head, want) {
		return nil, fmt.Errorf("columns do not match survey %s:\n got  %s\n want %s", s.ID, strings.Join(head, ","), strings.Join(want, ","))
	}

	var rows []storedAnswer
	for line := 2; ; line++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		row := storedAnswer{Answers: map[string]any{}, QuestionVersions: map[string]int{}, Context: map[string]any{}}
		put := func(key, name string, def int, v any) error {
			row.Answers[key] = v
			row.QuestionVersions[key] = def
			if i, ok := col[name+".version"]; ok {
				n, aerr := strconv.Atoi(rec[i])
				if aerr != nil {
					return fmt.Errorf("line %d: %s.version %q", line, name, rec[i])
				}
				row.QuestionVersions[key] = n
			}
			return nil
		}
		number := func(key, name string, def int, unsure string) error {
			cell := rec[col[name]]
			if cell == "" {
				return nil
			}
			f, perr := strconv.ParseFloat(cell, 64)
			if perr == nil {
				return put(key, name, def, f)
			}
			// Not a number: the unsure label, or for an older version any
			// label it had (#2258): what the export writes must read back.
			label := unformula(cell)
			if perr := put(key, name, def, label); perr != nil {
				return perr
			}
			if label != unsure && row.QuestionVersions[key] == def {
				return fmt.Errorf("line %d: %s %q", line, name, cell)
			}
			return nil
		}
		for _, q := range s.Questions {
			switch q.Type {
			case "matrix":
				for _, it := range q.Items {
					err = number(it.ID, q.ID+"."+it.ID, it.Version, q.Unsure)
					if err != nil {
						return nil, err
					}
				}
				continue
			case "scale":
				err = number(q.ID, q.ID, q.Version, q.Unsure)
			case "multi":
				picked, answered := []any{}, false
				for _, o := range multiOptions(q, head) {
					switch rec[col[q.ID+"="+o]] {
					case "1":
						picked, answered = append(picked, o), true
					case "0":
						answered = true
					case "":
					default:
						return nil, fmt.Errorf("line %d: %s=%s %q", line, q.ID, o, rec[col[q.ID+"="+o]])
					}
				}
				if answered {
					err = put(q.ID, q.ID, q.Version, picked)
				}
			default:
				if cell := rec[col[q.ID]]; cell != "" {
					err = put(q.ID, q.ID, q.Version, unformula(cell))
				}
			}
			if err != nil {
				return nil, err
			}
			if cell := rec[col[otherKey(q.ID)]]; q.Other != "" && cell != "" {
				row.Answers[otherKey(q.ID)] = unformula(cell)
			}
		}
		for _, k := range exportCSVContext {
			if v := rec[col["context."+k]]; v != "" {
				row.Context[k] = v
			}
		}
		row.Client, _ = row.Context["client"].(string)
		rows = append(rows, row)
	}
}

// unformula undoes csvValue's leading apostrophe.
func unformula(s string) string {
	return strings.TrimPrefix(s, "'")
}

// multiOptions are q's options and the older ones the header carries
// (multiColumns), in header order after the current ones.
func multiOptions(q question, head []string) []string {
	opts := q.options()
	for _, h := range head {
		if v, ok := strings.CutPrefix(h, q.ID+"="); ok && !slices.Contains(opts, v) {
			opts = append(slices.Clip(opts), v)
		}
	}
	return opts
}

// runAnalyze is «copilot-survey analyze [--csv] id=file.csv …»: the admin
// results for one survey, or the comparison of two to four (oldest first),
// computed by the same functions as the endpoints, from exported CSVs.
func runAnalyze(args []string, out io.Writer, now time.Time) error {
	asCSV := len(args) > 0 && args[0] == "--csv"
	if asCSV {
		args = args[1:]
	}
	if len(args) < 1 || len(args) > 4 {
		return errors.New("usage: copilot-survey analyze [--csv] <survey-id>=<file.csv> … (one survey, or two to four oldest first)")
	}
	defs, err := loadSurveyDir(surveyFiles)
	if err != nil {
		return err
	}
	var waves []survey
	var rows [][]storedAnswer
	for _, a := range args {
		id, path, ok := strings.Cut(a, "=")
		i := slices.IndexFunc(defs, func(s survey) bool { return s.ID == id })
		if !ok || i < 0 {
			return fmt.Errorf("%q: want <survey-id>=<file.csv> with a known survey id", a)
		}
		if slices.ContainsFunc(waves, func(s survey) bool { return s.ID == id }) {
			return fmt.Errorf("%q: survey %s given twice", a, id) // as /compare refuses it
		}
		f, err := os.Open(path) //nolint:gosec // G304: the path is the operator's own argument to a local command
		if err != nil {
			return err
		}
		rs, err := readExportCSV(defs[i], f)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		waves, rows = append(waves, defs[i]), append(rows, rs)
	}
	if len(waves) == 1 {
		if asCSV {
			return errors.New("--csv is for comparisons")
		}
		return encodeJSON(out, aggregate(waves[0], rows[0], now))
	}
	res := compareWavesResult(waves, rows, now)
	if asCSV {
		return writeCompareCSV(out, res)
	}
	return encodeJSON(out, res)
}

func encodeJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
