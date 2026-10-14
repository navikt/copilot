package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// storedAnswer is one survey_answers row as the admin aggregates read it.
type storedAnswer struct {
	Answers          map[string]any `json:"answers"`
	QuestionVersions map[string]int `json:"question_versions"`
	Client           string         `json:"-"` // context.client: web, copilot, forms (import), …
}

type surveySummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Starts    string `json:"starts"`
	Ends      string `json:"ends"`
	Status    string `json:"status"` // kommende, åpen or stengt
	Responses int    `json:"responses"`
	Series    string `json:"series,omitempty"`
}

func (s survey) status(now time.Time) string {
	switch {
	case s.activeOn(now):
		return "åpen"
	case now.UTC().Format(time.DateOnly) < s.Starts:
		return "kommende"
	}
	return "stengt"
}

func (a *surveyAPI) list(w http.ResponseWriter, r *http.Request) {
	out := []surveySummary{}
	for _, s := range a.surveys {
		t, err := a.tally(r, s)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		out = append(out, surveySummary{s.ID, s.Title, s.Starts, s.Ends, s.status(a.now()), t.Responses, s.Series})
	}
	writeJSON(w, map[string]any{"surveys": out})
}

// tally reads a closed survey's final aggregates (survey_results) and falls
// back to its rows while it has none, as an open survey always does.
func (a *surveyAPI) tally(r *http.Request, s survey) (surveyTally, error) {
	if a.final != nil && s.status(a.now()) == "stengt" {
		t, err := a.final(r.Context(), s.ID)
		if err != nil {
			return surveyTally{}, err
		}
		if t != nil {
			return *t, nil
		}
	}
	rows, err := a.read(r, s.ID)
	return tallyRows(s, rows), err
}

var errNoStorage = errors.New("no storage")

func (a *surveyAPI) read(r *http.Request, id string) ([]storedAnswer, error) {
	if a.answers == nil {
		return nil, errNoStorage
	}
	return a.answers(r.Context(), id)
}

// optionCount is one option's count. Suppressed is always false; it stays
// for clients that still read it.
type optionCount struct {
	Value      string `json:"value"`
	Count      *int   `json:"count"`
	Suppressed bool   `json:"suppressed"`
}

type versionResult struct {
	Version int `json:"version"`
	// N is respondents who answered this version (people, also for multi).
	N       *int          `json:"n"`
	Options []optionCount `json:"options"`
}

type itemResult struct {
	ID       string          `json:"id"`
	Text     string          `json:"text"`
	Versions []versionResult `json:"versions"`
}

type questionResult struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Versions []versionResult `json:"versions,omitempty"` // scale, choice, multi
	Items    []itemResult    `json:"items,omitempty"`    // matrix: one per statement
}

type surveyResults struct {
	surveySummary
	// Suppressed: fewer than minSegment responses, and Questions is null.
	Suppressed bool             `json:"suppressed"`
	Questions  []questionResult `json:"questions"`
}

func (a *surveyAPI) results(w http.ResponseWriter, r *http.Request) {
	i := slices.IndexFunc(a.surveys, func(s survey) bool { return s.ID == r.PathValue("id") })
	if i < 0 {
		writeError(w, http.StatusNotFound, "unknown survey")
		return
	}
	t, err := a.tally(r, a.surveys[i])
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return
	}
	writeJSON(w, aggregateTally(a.surveys[i], t, a.now()))
}

// aggregate counts answers per question and question version. Free text is
// never read. Below minSegment respondents for the whole survey nothing is
// shown; otherwise every count is real (owner decision 2026-10-13).
func aggregate(s survey, rows []storedAnswer, now time.Time) surveyResults {
	return aggregateTally(s, tallyRows(s, rows), now)
}

// aggregateTally is aggregate from counts, live or stored (survey_results).
func aggregateTally(s survey, t surveyTally, now time.Time) surveyResults {
	res := surveyResults{surveySummary: surveySummary{s.ID, s.Title, s.Starts, s.Ends, s.status(now), t.Responses, s.Series}}
	if t.Responses < minSegment {
		res.Suppressed = true
		return res
	}
	res.Questions = []questionResult{}
	for _, q := range s.Questions {
		qr := questionResult{ID: q.ID, Type: q.Type, Text: q.Text}
		switch q.Type {
		case "text":
			continue
		case "matrix":
			for _, it := range q.Items {
				item := q
				item.Type, item.ID, item.Version = "scale", it.ID, it.Version
				qr.Items = append(qr.Items, itemResult{it.ID, it.Text, versionsFrom(item, t.Keys[it.ID])})
			}
		default:
			qr.Versions = versionsFrom(q, t.Keys[q.ID])
		}
		res.Questions = append(res.Questions, qr)
	}
	return res
}

func countVersions(q question, rows []storedAnswer) []versionResult {
	return versionsFrom(q, tallyKey(q.ID, rows))
}

// versionsFrom turns one answer key's tally into the API's versions, with
// options as cells lists them.
func versionsFrom(q question, t map[int]*verTally) []versionResult {
	out := []versionResult{}
	for _, ver := range sortedKeys(t) {
		n := t[ver].N
		out = append(out, versionResult{Version: ver, N: &n, Options: cells(q, ver, t[ver].Counts)})
	}
	return out
}

// cells lists the options as defined (current version) or as seen (older
// versions) with their real counts.
func cells(q question, ver int, counts map[string]int) []optionCount {
	var values []string
	if ver == q.Version {
		if q.Type == "scale" {
			for n := q.Min; n <= q.Max; n++ {
				values = append(values, strconv.Itoa(n))
			}
		} else {
			values = q.options()
		}
	}
	for _, v := range sortedKeys(counts) {
		if !slices.Contains(values, v) {
			values = append(values, v)
		}
	}
	out := make([]optionCount, len(values))
	for i, v := range values {
		c := counts[v]
		out[i].Value, out[i].Count = v, &c
	}
	return out
}

func sortedKeys[K int | string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
