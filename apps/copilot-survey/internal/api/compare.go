package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// compare lines up two to four waves question by question
// (GET /api/v1/admin/surveys/compare?ids=q2-2026,q4-2026, oldest first).
//
// Questions follow the last wave's definition; text questions are left out.
// Comparison rule: the last wave's skip_if applies to every wave. If the
// last wave skips a question for some answer, earlier answers from people
// who gave that answer are dropped too, so both waves count the same group
// (q2-2026 came from Forms, which showed the statements to everyone; q4-2026
// skips them for those who do not use KI tools). Excluded reports how many.
//
// A closed wave whose rows are deleted is read from its stored tally
// (survey_skip_results); 409 when that was written before the last wave's
// skip_if existed.
func (a *surveyAPI) compare(w http.ResponseWriter, r *http.Request) {
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(ids) < 2 || len(ids) > 4 {
		writeError(w, http.StatusBadRequest, "ids: two to four survey ids")
		return
	}
	var waves []survey.Survey
	for _, id := range ids {
		i := slices.IndexFunc(a.surveys, func(s survey.Survey) bool { return s.ID == id })
		if i < 0 || slices.ContainsFunc(waves, func(s survey.Survey) bool { return s.ID == id }) {
			writeError(w, http.StatusNotFound, "unknown survey")
			return
		}
		waves = append(waves, a.surveys[i])
	}
	last, sets := waves[len(waves)-1], survey.SkipSets(a.surveys)
	tallies := make([]survey.WaveTally, len(waves))
	for i, s := range waves {
		rs, err := a.read(r, s.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		tallies[i] = survey.RowsTally(s, survey.MergeClients(rs), sets)
		if len(rs) > 0 || a.store == nil || s.Status(a.now()) != "stengt" {
			continue
		}
		stored, err := a.store.FinalCompare(r.Context(), s.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		if stored == nil {
			continue
		}
		for id, set := range survey.SkipSets([]survey.Survey{last}) {
			for _, k := range set.Keys {
				// No answers to k in this wave: nothing to exclude.
				if _, ok := stored.Skipped[id][k]; !ok && len(stored.Keys[k]) > 0 {
					writeError(w, http.StatusConflict, s.ID+": the answers are deleted, and the stored counts were written before "+last.ID+"'s skip_if existed")
					return
				}
			}
		}
		tallies[i] = *stored
	}
	res := survey.CompareTallies(waves, tallies, a.now())
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="sammenligning-`+strings.Join(ids, "-")+`.csv"`)
		w.Header().Set("Cache-Control", "no-store")
		_ = survey.WriteCompareCSV(w, res) // headers are sent; a broken connection has no one to tell
		return
	}
	writeJSON(w, res)
}
