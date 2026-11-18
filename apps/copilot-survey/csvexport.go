package main

import (
	"encoding/csv"
	"fmt"
	"io"
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
		t, err := a.final(r.Context(), s.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		if t != nil {
			writeError(w, http.StatusGone, "the answers to this survey are deleted; only the aggregates remain")
			return
		}
	}
	cb, coded := a.codebooks[s.Series]
	var codes map[string]*textCodes
	if coded && s.Series != "" && a.codes != nil {
		var err error
		if codes, err = a.codes(r.Context(), s.ID, cb.Version); err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
	}
	rows := make([]exportRow, len(stored))
	for i, st := range stored {
		rowCodes := map[string]textCodes{}
		for _, f := range cb.Fields {
			raw, _ := st.Answers[f.ID].(string)
			if c := codes[hiddenKey(f.ID, textHash(raw))]; raw != "" && c != nil && !hidden[hiddenKey(f.ID, textHash(raw))] {
				t := *c
				if t.Human == nil {
					t.Model = nil // blind coding: the model's codes wait for the person's
				}
				rowCodes[f.ID] = t
			}
		}
		rows[i] = newExportRow(st.Answers, st.QuestionVersions, st.Context, func(field, raw string) bool {
			return hidden[hiddenKey(field, textHash(raw))]
		})
		rows[i].Codes = rowCodes
	}
	rand.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+s.ID+`-svar.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	writeExportCSV(w, s, rows)
	// Who exported is in my-copilot's access log; never the content here.
	slog.Info("admin survey export", "survey", s.ID, "rows", len(rows))
}

// writeExportCSV writes rows as the columns described at exportCSV.
// readExportCSV is its inverse. Context under minSegment is merged here,
// so every export, and the test of the round trip, goes through it.
func writeExportCSV(w io.Writer, s survey, rows []exportRow) {
	suppressSmallSegments(rows)
	// A column is a header and how a row fills it.
	type column struct {
		name string
		cell func(exportRow) string
	}
	var cols []column
	answer := func(name, key string) {
		cols = append(cols, column{name, func(r exportRow) string { return csvValue(r.Answers[key]) }})
	}
	// Versions come from question_versions, not the answers: a hidden text
	// is gone from those but was still answered.
	// Written whenever a row's version is not the definition's, so
	// readExportCSV can take the definition's version when the column is absent.
	version := func(name, key string, def int) {
		seen := map[int]bool{}
		for _, r := range rows {
			if v, ok := r.QuestionVersions[key]; ok {
				seen[v] = true
			}
		}
		if len(seen) > 1 || len(seen) == 1 && !seen[def] {
			cols = append(cols, column{name + ".version", func(r exportRow) string {
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
		version(q.ID, q.ID, q.Version)
	}
	for _, k := range exportCSVContext {
		cols = append(cols, column{"context." + k, func(r exportRow) string { return r.Context[k] }})
	}
	for _, f := range freetextFields(s) {
		if !slices.ContainsFunc(rows, func(r exportRow) bool { _, ok := r.Codes[f.ID]; return ok }) {
			continue
		}
		for _, coder := range codeColumns {
			cols = append(cols, column{"codes." + f.ID + "." + coder, func(r exportRow) string { return codesCell(r.Codes[f.ID], coder) }})
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
func codesCell(t textCodes, coder string) string {
	codes := map[string][]code{"final": t.final(), "model": t.Model, "human": t.Human}[coder]
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
func parseCodesCell(cell string) []code {
	if cell == "" {
		return nil
	}
	var out []code
	for _, p := range strings.Split(cell, ";") {
		cat, wish := strings.CutSuffix(p, ":onske")
		out = append(out, code{cat, wish})
	}
	return out
}

// multiColumns are q's options, then any value a row picked that is no
// longer an option (an older version's), sorted, so nothing picked is lost.
func multiColumns(q question, rows []exportRow) []string {
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
