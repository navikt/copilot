package api

import (
	"log/slog"
	"math/rand/v2"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

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
	if len(stored) == 0 && s.Status(a.now()) == "stengt" && a.store != nil {
		t, err := a.store.Final(r.Context(), s.ID)
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
	var codes map[string]*survey.TextCodes
	if coded && s.Series != "" && a.store != nil {
		var err error
		if codes, err = a.store.Codes(r.Context(), s.ID, cb.Version); err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
	}
	rows := make([]survey.ExportRow, len(stored))
	for i, st := range stored {
		rowCodes := map[string]survey.TextCodes{}
		for _, f := range cb.Fields {
			raw, _ := st.Answers[f.ID].(string)
			if c := codes[survey.HiddenKey(f.ID, survey.TextHash(raw))]; raw != "" && c != nil && !hidden[survey.HiddenKey(f.ID, survey.TextHash(raw))] {
				t := *c
				if t.Human == nil {
					t.Model = nil // blind coding: the model's codes wait for the person's
				}
				rowCodes[f.ID] = t
			}
		}
		rows[i] = survey.NewExportRow(st.Answers, st.QuestionVersions, st.Context, func(field, raw string) bool {
			return hidden[survey.HiddenKey(field, survey.TextHash(raw))]
		})
		rows[i].Codes = rowCodes
	}
	rand.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+s.ID+`-svar.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	survey.WriteExportCSV(w, s, rows)
	// Who exported is in my-copilot's access log; never the content here.
	slog.Info("admin survey export", "survey", s.ID, "rows", len(rows))
}
