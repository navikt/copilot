package survey

import (
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

var exportCSVContext = []string{"client", "os", "version", "local_models"}

// WriteExportCSV writes rows as the columns described at exportCSV.
// ReadExportCSV is its inverse. Context under MinSegment is merged here,
// so every export, and the test of the round trip, goes through it.
func WriteExportCSV(w io.Writer, s Survey, rows []ExportRow) {
	SuppressSmallSegments(rows)
	type column struct {
		name string
		cell func(ExportRow) string
	}
	var cols []column
	answer := func(name, key string) {
		cols = append(cols, column{name, func(r ExportRow) string { return csvValue(r.Answers[key]) }})
	}
	// Versions come from question_versions, not the answers: a hidden text
	// is gone from those but was still answered.
	// Written whenever a row's version is not the definition's, so
	// ReadExportCSV can take the definition's version when the column is absent.
	version := func(name, key string, def int) {
		seen := map[int]bool{}
		for _, r := range rows {
			if v, ok := r.QuestionVersions[key]; ok {
				seen[v] = true
			}
		}
		if len(seen) > 1 || len(seen) == 1 && !seen[def] {
			cols = append(cols, column{name + ".version", func(r ExportRow) string {
				if v, ok := r.QuestionVersions[key]; ok {
					return strconv.Itoa(v)
				}
				return ""
			}})
		}
	}
	for _, q := range s.Questions {
		switch q.Type {
		case "matrix":
			for _, it := range q.Items {
				answer(q.ID+"."+it.ID, it.ID)
				version(q.ID+"."+it.ID, it.ID, it.Version)
			}
			continue
		case "multi":
			for _, o := range multiColumns(q, rows) {
				cols = append(cols, column{q.ID + "=" + o, func(r ExportRow) string {
					picked, ok := r.Answers[q.ID].([]any)
					switch {
					case !ok:
						return ""
					case slices.Contains(picked, any(o)):
						return "1"
					}
					return "0"
				}})
			}
		default:
			answer(q.ID, q.ID)
		}
		if q.Other != "" {
			answer(otherKey(q.ID), otherKey(q.ID))
		}
		version(q.ID, q.ID, q.Version)
	}
	for _, k := range exportCSVContext {
		cols = append(cols, column{"context." + k, func(r ExportRow) string { return r.Context[k] }})
	}
	for _, f := range FreetextFields(s) {
		if !slices.ContainsFunc(rows, func(r ExportRow) bool { _, ok := r.Codes[f.ID]; return ok }) {
			continue
		}
		for _, coder := range codeColumns {
			cols = append(cols, column{"codes." + f.ID + "." + coder, func(r ExportRow) string { return codesCell(r.Codes[f.ID], coder) }})
		}
	}

	_, _ = w.Write([]byte("\xef\xbb\xbf")) // BOM: Excel then reads æøå as UTF-8
	cw := csv.NewWriter(w)
	line := make([]string, len(cols))
	for i, c := range cols {
		line[i] = c.name
	}
	_ = cw.Write(line)
	for _, r := range rows {
		for i, c := range cols {
			line[i] = c.cell(r)
		}
		_ = cw.Write(line)
	}
	cw.Flush()
}

// codeColumns are the coders whose codes follow, per coded field, as
// «codes.<field>.<coder>»: final codes first, then each coder's.
var codeColumns = []string{"final", "model", "human"}

// codesCell writes codes as «category» or «category:onske», «;» between;
// empty when that coder has not coded the text.
func codesCell(t TextCodes, coder string) string {
	codes := map[string][]Code{"final": t.FinalCodes(), "model": t.Model, "human": t.Human}[coder]
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = c.Category
		if c.Onske {
			parts[i] += ":onske"
		}
	}
	return strings.Join(parts, ";")
}

// parseCodesCell is codesCell's inverse; nil for an empty cell.
func parseCodesCell(cell string) []Code {
	if cell == "" {
		return nil
	}
	var out []Code
	for _, p := range strings.Split(cell, ";") {
		cat, wish := strings.CutSuffix(p, ":onske")
		out = append(out, Code{cat, wish})
	}
	return out
}

// multiColumns are q's options, then any value a row picked that is no
// longer an option (an older version's), sorted, so nothing picked is lost.
func multiColumns(q Question, rows []ExportRow) []string {
	cols := q.options()
	var extra []string
	for _, r := range rows {
		picked, _ := r.Answers[q.ID].([]any)
		for _, p := range picked {
			if v := fmt.Sprint(p); !slices.Contains(cols, v) && !slices.Contains(extra, v) {
				extra = append(extra, v)
			}
		}
	}
	slices.Sort(extra)
	return append(slices.Clip(cols), extra...)
}

// csvValue writes an answer as a cell. Text a spreadsheet would read as a
// formula gets a leading apostrophe, and so does text that already starts
// with one, so unformula can undo it without guessing.
func csvValue(v any) string {
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case string:
		if t != "" && strings.ContainsRune("=+-@\t\r'", rune(t[0])) {
			return "'" + t
		}
		return t
	}
	return ""
}
