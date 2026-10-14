package main

import (
	"encoding/csv"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// exportCSVContext are the context columns, in order.
var exportCSVContext = []string{"client", "os", "version", "local_models"}

// exportCSV is GET /api/v1/admin/surveys/{id}/export.csv: one row per
// response, in random order, masked and k ≥ 5 merged as in runExport, hidden
// texts left out. No row id, hash or timestamp.
//
// Columns, in definition order: a scale, choice or text question under its
// id; a multi question as one 0/1 column per option, «<id>=<option>»
// (empty when not answered), so a column sums in a spreadsheet; a matrix
// item as «<question>.<item>»; «<id>.other» after a question with an other
// option. «<id>.version» follows any question or item whose answered
// version differs between rows. Then «context.client», «context.os»,
// «context.version» (year.month) and «context.local_models».
func (a *surveyAPI) exportCSV(w http.ResponseWriter, r *http.Request) {
	s, stored, hidden, ok := a.freetextRows(w, r)
	if !ok {
		return
	}
	if len(stored) == 0 && s.status(a.now()) == "stengt" && a.final != nil {
		if t, err := a.final(r.Context(), s.ID); err == nil && t != nil {
			writeError(w, http.StatusGone, "the answers to this survey are deleted; only the aggregates remain")
			return
		}
	}
	rows := make([]exportRow, len(stored))
	for i, st := range stored {
		rows[i] = newExportRow(st.Answers, st.QuestionVersions, st.Context, func(field, raw string) bool {
			return hidden[hiddenKey(field, textHash(raw))]
		})
	}
	suppressSmallSegments(rows)
	rand.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })

	// A column is a header and how a row fills it.
	type column struct {
		name string
		cell func(exportRow) string
	}
	var cols []column
	answer := func(name, key string) {
		cols = append(cols, column{name, func(r exportRow) string { return csvValue(r.Answers[key]) }})
	}
	answered := func(r exportRow, key string) bool { _, ok := r.Answers[key]; return ok }
	version := func(name, key string) {
		seen := map[int]bool{}
		for _, r := range rows {
			if answered(r, key) {
				seen[r.QuestionVersions[key]] = true
			}
		}
		if len(seen) > 1 {
			cols = append(cols, column{name + ".version", func(r exportRow) string {
				if !answered(r, key) {
					return ""
				}
				return strconv.Itoa(r.QuestionVersions[key])
			}})
		}
	}
	for _, q := range s.Questions {
		switch q.Type {
		case "matrix":
			for _, it := range q.Items {
				answer(q.ID+"."+it.ID, it.ID)
				version(q.ID+"."+it.ID, it.ID)
			}
			continue
		case "multi":
			for _, o := range q.options() {
				cols = append(cols, column{q.ID + "=" + o, func(r exportRow) string {
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
		version(q.ID, q.ID)
	}
	for _, k := range exportCSVContext {
		cols = append(cols, column{"context." + k, func(r exportRow) string { return r.Context[k] }})
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+s.ID+`-svar.csv"`)
	w.Header().Set("Cache-Control", "no-store")
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
	// Who exported is in my-copilot's access log; never the content here.
	slog.Info("admin survey export", "survey", s.ID, "rows", len(rows))
}

// csvValue writes an answer as a cell. Text a spreadsheet would read as a
// formula gets a leading apostrophe.
func csvValue(v any) string {
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case string:
		if t != "" && strings.ContainsRune("=+-@\t\r", rune(t[0])) {
			return "'" + t
		}
		return t
	}
	return ""
}
