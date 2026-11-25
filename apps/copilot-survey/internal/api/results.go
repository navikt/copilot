package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

func (a *surveyAPI) list(w http.ResponseWriter, r *http.Request) {
	out := []survey.Summary{}
	for _, s := range a.surveys {
		t, err := a.tally(r, s)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		out = append(out, s.Summary(a.now(), t.Responses))
	}
	writeJSON(w, map[string]any{"surveys": out})
}

// publishedList lists closed, published surveys with at least MinSegment
// responses for signed-in Nav users (#1775).
func (a *surveyAPI) publishedList(w http.ResponseWriter, r *http.Request) {
	now := a.now()
	out := []survey.Summary{}
	for _, s := range a.surveys {
		if !s.Published || s.Status(now) != "stengt" {
			continue
		}
		t, err := a.tally(r, s)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		if t.Responses >= survey.MinSegment {
			out = append(out, s.Summary(now, t.Responses))
		}
	}
	writeJSON(w, map[string]any{"surveys": out})
}

// tally reads a closed survey's final aggregates (survey_results) and falls
// back to its rows while it has none, as an open survey always does.
func (a *surveyAPI) tally(r *http.Request, s survey.Survey) (survey.Tally, error) {
	if a.store != nil && s.Status(a.now()) == "stengt" {
		t, err := a.store.Final(r.Context(), s.ID)
		if err != nil {
			return survey.Tally{}, err
		}
		if t != nil {
			return *t, nil
		}
	}
	rows, err := a.read(r, s.ID)
	return survey.TallyRows(s, rows), err
}

var errNoStorage = errors.New("no storage")

func (a *surveyAPI) read(r *http.Request, id string) ([]survey.StoredAnswer, error) {
	if a.store == nil {
		return nil, errNoStorage
	}
	return a.store.Answers(r.Context(), id)
}

func (a *surveyAPI) results(w http.ResponseWriter, r *http.Request) {
	i := slices.IndexFunc(a.surveys, func(s survey.Survey) bool { return s.ID == r.PathValue("id") })
	if i < 0 {
		writeError(w, http.StatusNotFound, "unknown survey")
		return
	}
	t, err := a.tally(r, a.surveys[i])
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return
	}
	writeJSON(w, survey.AggregateTally(a.surveys[i], t, a.now()))
}

// publishedResults serves aggregated results for a closed, published survey
// with at least MinSegment responses (#1775). Open, unpublished, or small
// surveys return 404.
func (a *surveyAPI) publishedResults(w http.ResponseWriter, r *http.Request) {
	now := a.now()
	i := slices.IndexFunc(a.surveys, func(s survey.Survey) bool { return s.ID == r.PathValue("id") })
	if i < 0 || !a.surveys[i].Published || a.surveys[i].Status(now) != "stengt" {
		writeError(w, http.StatusNotFound, "unknown survey")
		return
	}
	t, err := a.tally(r, a.surveys[i])
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return
	}
	if t.Responses < survey.MinSegment {
		writeError(w, http.StatusNotFound, "unknown survey")
		return
	}
	res := survey.AggregateTally(a.surveys[i], t, now)
	res.Channels = survey.MergeSmallChannels(res.Channels)
	survey.HideSmallVersions(&res)
	writeJSON(w, res)
}

// publishedCompare compares the last four closed, published waves of a
// series that have at least MinSegment rows, oldest first (#2229). Channels
// are left out, and so are versions under MinSegment (HideSmallWaveVersions).
// Coded free text comes along per wave as publishedCoding gives it.
// Unlike compare, the last wave's skip_if is not applied to earlier waves:
// each wave counts its own answers as its /results does, so no difference
// between the two can single out a small group (owner decision, #2271). The
// test then compares groups that are not quite alike. An unknown series, or
// fewer than two such waves, is 404. Each wave is read as its /results is
// (tally): from its aggregates once closed, so it stays after its rows are
// deleted.
func (a *surveyAPI) publishedCompare(w http.ResponseWriter, r *http.Request) {
	now, series := a.now(), r.URL.Query().Get("series")
	candidates := slices.Clone(a.surveys)
	slices.SortFunc(candidates, func(x, y survey.Survey) int { return strings.Compare(y.Starts, x.Starts) }) // newest first
	var waves []survey.Survey
	var tallies []survey.WaveTally
	for _, s := range candidates {
		if len(waves) == 4 {
			break
		}
		if series == "" || s.Series != series || !s.Published || s.Status(now) != "stengt" {
			continue
		}
		t, err := a.tally(r, s)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		if t.Responses >= survey.MinSegment {
			waves = append([]survey.Survey{s}, waves...)
			tallies = append([]survey.WaveTally{{Responses: t.Responses, Keys: t.Keys}}, tallies...)
		}
	}
	if len(waves) < 2 {
		writeError(w, http.StatusNotFound, "unknown series")
		return
	}
	last := &waves[len(waves)-1]
	last.Questions = slices.Clone(last.Questions)
	for i := range last.Questions {
		last.Questions[i].SkipIf = nil
	}
	res := survey.CompareTallies(waves, tallies, now)
	for i := range res.Surveys {
		res.Surveys[i].Channels = nil
	}
	survey.HideSmallWaveVersions(&res)
	for _, s := range waves {
		wf, err := a.publishedCoding(r.Context(), s)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		if wf != nil {
			res.Freetext = append(res.Freetext, *wf)
		}
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="sammenligning-`+series+`.csv"`)
		w.Header().Set("Cache-Control", "no-store")
		_ = survey.WriteCompareCSV(w, res)
		return
	}
	writeJSON(w, res)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
