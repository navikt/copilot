package main

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// waveMeta is one wave in a comparison: its summary and how it was answered.
type waveMeta struct {
	surveySummary
	// Suppressed: fewer than minSegment responses; this wave has no counts.
	Suppressed bool `json:"suppressed"`
	// Channels counts answers per context.client (forms for the import,
	// web, copilot, opencode, pi …).
	Channels map[string]int `json:"channels"`
}

// waveStat is one version of a question in one wave, with mean and share
// agreeing (the top two steps) for scale and matrix items.
type waveStat struct {
	versionResult
	Mean  *float64 `json:"mean,omitempty"`
	Agree *float64 `json:"agree,omitempty"`
}

// waveCounts is one question or item in one wave, index-aligned with
// compareResult.Surveys. Excluded is how many answers the comparison rule
// dropped (see compare).
type waveCounts struct {
	Excluded int        `json:"excluded"`
	Versions []waveStat `json:"versions"`
}

type compareItem struct {
	ID    string       `json:"id"`
	Text  string       `json:"text"`
	Break bool         `json:"break"` // the waves asked different versions
	Waves []waveCounts `json:"waves"`
}

type compareQuestion struct {
	ID     string        `json:"id"`
	Type   string        `json:"type"`
	Text   string        `json:"text"`
	Min    int           `json:"min,omitempty"`
	Max    int           `json:"max,omitempty"`
	Labels []string      `json:"labels,omitempty"`
	Break  bool          `json:"break,omitempty"`
	Waves  []waveCounts  `json:"waves,omitempty"` // scale, choice, multi
	Items  []compareItem `json:"items,omitempty"` // matrix
}

type compareResult struct {
	Surveys   []waveMeta        `json:"surveys"`
	Questions []compareQuestion `json:"questions"`
}

// compare lines up two to four waves question by question
// (GET /api/v1/admin/surveys/compare?ids=q2-2026,q4-2026, oldest first).
//
// Questions follow the last wave's definition; text questions are left out.
// Comparison rule: the last wave's skip_if applies to every wave. If the
// last wave skips a question for some answer, earlier answers from people
// who gave that answer are dropped too, so both waves count the same group
// (q2-2026 came from Forms, which showed the statements to everyone; q4-2026
// skips them for those who do not use KI tools). Excluded reports how many.
func (a *surveyAPI) compare(w http.ResponseWriter, r *http.Request) {
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(ids) < 2 || len(ids) > 4 {
		writeError(w, http.StatusBadRequest, "ids: two to four survey ids")
		return
	}
	var waves []survey
	var rows [][]storedAnswer
	res := compareResult{Questions: []compareQuestion{}}
	for _, id := range ids {
		i := slices.IndexFunc(a.surveys, func(s survey) bool { return s.ID == id })
		if i < 0 || slices.ContainsFunc(waves, func(s survey) bool { return s.ID == id }) {
			writeError(w, http.StatusNotFound, "unknown survey")
			return
		}
		s := a.surveys[i]
		rs, err := a.read(r, s.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		m := waveMeta{surveySummary{s.ID, s.Title, s.Starts, s.Ends, s.status(a.now()), len(rs), s.Series}, len(rs) < minSegment, map[string]int{}}
		for _, row := range rs {
			m.Channels[row.Client]++
		}
		if m.Suppressed {
			rs, m.Channels = nil, nil
		}
		waves, rows = append(waves, s), append(rows, rs)
		res.Surveys = append(res.Surveys, m)
	}
	for _, q := range waves[len(waves)-1].Questions {
		if q.Type == "text" {
			continue
		}
		cq := compareQuestion{ID: q.ID, Type: q.Type, Text: q.Text, Min: q.Min, Max: q.Max, Labels: q.Labels}
		if q.Type == "matrix" {
			for _, it := range q.Items {
				item := q
				item.Type, item.ID, item.Version = "scale", it.ID, it.Version
				ws, br := compareWaves(item, waves, rows)
				cq.Items = append(cq.Items, compareItem{it.ID, it.Text, br, ws})
			}
		} else {
			cq.Waves, cq.Break = compareWaves(q, waves, rows)
		}
		res.Questions = append(res.Questions, cq)
	}
	writeJSON(w, res)
}

// asked finds the question or matrix item id in s as asked there: its
// version and scale maximum.
func asked(s survey, id string) (version, top int, ok bool) {
	for _, q := range s.Questions {
		if q.ID == id {
			return q.Version, q.Max, true
		}
		for _, it := range q.Items {
			if it.ID == id {
				return it.Version, q.Max, true
			}
		}
	}
	return 0, 0, false
}

// compareWaves counts q in every wave after the skip rule, and reports a
// break when the waves did not all ask, or answer, the same single version.
// A wave's declared version counts even without answers; a wave that did
// not ask the question at all does not break the series.
func compareWaves(q question, waves []survey, rows [][]storedAnswer) ([]waveCounts, bool) {
	out := make([]waveCounts, len(rows))
	seen := map[int]bool{}
	for i, rs := range rows {
		top := q.Max
		if v, m, ok := asked(waves[i], q.ID); ok {
			seen[v], top = true, m
		}
		kept := slices.DeleteFunc(slices.Clone(rs), func(row storedAnswer) bool {
			v, ok := row.Answers[q.ID]
			return ok && v != nil && q.SkipIf.skipped(row.Answers)
		})
		out[i].Excluded = len(rs) - len(kept)
		out[i].Versions = []waveStat{}
		for _, vr := range countVersions(q, kept) {
			ws := waveStat{versionResult: vr}
			if q.Type == "scale" {
				ws.Mean, ws.Agree = scaleStats(vr, top)
			}
			out[i].Versions = append(out[i].Versions, ws)
			seen[vr.Version] = true
		}
	}
	return out, len(seen) > 1
}

// scaleStats is the mean and the share at top or top-1.
func scaleStats(vr versionResult, top int) (*float64, *float64) {
	var sum, n, agree float64
	for _, o := range vr.Options {
		v, err := strconv.Atoi(o.Value)
		if err != nil || o.Count == nil {
			continue
		}
		c := float64(*o.Count)
		sum, n = sum+float64(v)*c, n+c
		if v >= top-1 {
			agree += c
		}
	}
	if n == 0 {
		return nil, nil
	}
	mean, share := sum/n, agree/n
	return &mean, &share
}
