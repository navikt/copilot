package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// storedAnswer is one survey_answers row as the admin aggregates read it.
type storedAnswer struct {
	Answers          map[string]any `json:"answers"`
	QuestionVersions map[string]int `json:"question_versions"`
}

type surveySummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Starts    string `json:"starts"`
	Ends      string `json:"ends"`
	Status    string `json:"status"` // kommende, åpen or stengt
	Responses int    `json:"responses"`
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
		rows, err := a.read(r, s.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		out = append(out, surveySummary{s.ID, s.Title, s.Starts, s.Ends, s.status(a.now()), len(rows)})
	}
	writeJSON(w, map[string]any{"surveys": out})
}

func (a *surveyAPI) read(r *http.Request, id string) ([]storedAnswer, error) {
	if a.answers == nil {
		return nil, errors.New("no storage")
	}
	return a.answers(r.Context(), id)
}

// optionCount is one option's count. Count is null and Suppressed true when
// it would show a group of 1 to minSegment-1.
type optionCount struct {
	Value      string `json:"value"`
	Count      *int   `json:"count"`
	Suppressed bool   `json:"suppressed"`
}

type versionResult struct {
	Version int `json:"version"`
	// N is respondents who answered this version (people, also for multi).
	// Both N and Options are null when fewer than minSegment did.
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
	rows, err := a.read(r, a.surveys[i].ID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
		return
	}
	writeJSON(w, aggregate(a.surveys[i], rows, a.now()))
}

// aggregate counts answers per question and question version. Free text is
// never read. Below minSegment respondents nothing is shown.
func aggregate(s survey, rows []storedAnswer, now time.Time) surveyResults {
	res := surveyResults{surveySummary: surveySummary{s.ID, s.Title, s.Starts, s.Ends, s.status(now), len(rows)}}
	if len(rows) < minSegment {
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
				qr.Items = append(qr.Items, itemResult{it.ID, it.Text, countVersions(item, rows)})
			}
		default:
			qr.Versions = countVersions(q, rows)
		}
		res.Questions = append(res.Questions, qr)
	}
	// A skipped question's n is the source's n minus the skip answer's count,
	// so that count shows through it, hidden or not. Hide the dependent
	// question unless the source shows the count itself.
	for _, q := range s.Questions {
		if q.SkipIf == nil || q.Type == "text" {
			continue
		}
		j := slices.IndexFunc(res.Questions, func(r questionResult) bool { return r.ID == q.SkipIf.Question })
		if j < 0 || !shows(res.Questions[j], q.SkipIf.Answer) {
			k := slices.IndexFunc(res.Questions, func(r questionResult) bool { return r.ID == q.ID })
			hide(&res.Questions[k])
		}
	}
	return res
}

// shows reports whether every version of a source question shows the count
// of value. An option never chosen on an older version is a shown zero.
func shows(src questionResult, value string) bool {
	for _, v := range src.Versions {
		if v.N == nil {
			return false
		}
		i := slices.IndexFunc(v.Options, func(o optionCount) bool { return o.Value == value })
		if i >= 0 && v.Options[i].Suppressed {
			return false
		}
	}
	return true
}

// hide suppresses every version of a question: n and options null.
func hide(qr *questionResult) {
	for i := range qr.Versions {
		qr.Versions[i].N, qr.Versions[i].Options = nil, nil
	}
	for _, it := range qr.Items {
		for i := range it.Versions {
			it.Versions[i].N, it.Versions[i].Options = nil, nil
		}
	}
}

func countVersions(q question, rows []storedAnswer) []versionResult {
	counts := map[int]map[string]int{}
	ns := map[int]int{}
	for _, row := range rows {
		v, ok := row.Answers[q.ID]
		if !ok || v == nil {
			continue
		}
		ver := row.QuestionVersions[q.ID]
		if counts[ver] == nil {
			counts[ver] = map[string]int{}
		}
		ns[ver]++
		switch v := v.(type) {
		case []any:
			for _, p := range v {
				counts[ver][fmt.Sprint(p)]++
			}
		case float64:
			counts[ver][strconv.FormatFloat(v, 'f', -1, 64)]++
		default:
			counts[ver][fmt.Sprint(v)]++
		}
	}
	out := []versionResult{}
	for _, ver := range sortedKeys(ns) {
		vr := versionResult{Version: ver}
		if n := ns[ver]; n >= minSegment {
			if opts := cells(q, ver, counts[ver]); opts != nil {
				vr.N, vr.Options = &n, opts
			}
		}
		out = append(out, vr)
	}
	return out
}

// cells lists the options as defined (current version) or as seen (older
// versions) and suppresses small counts.
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
	hidden, hiddenSum := 0, 0
	for i, v := range values {
		c := counts[v]
		out[i].Value = v
		if c > 0 && c < minSegment {
			out[i].Suppressed = true
			hidden++
			hiddenSum += c
		} else {
			out[i].Count = &c
		}
	}
	// One answer per person (all but a multi with free choice): the hidden
	// cells sum to n minus the shown ones. That sum gives them away when there
	// is one hidden cell, or when they must all be 1 or all be minSegment-1,
	// so hide the smallest shown non-zero cell as well. With no such cell
	// (every answer is in a small cell, e.g. 4+4 or five 1s) nothing can be
	// shown: nil, and the caller hides n too.
	single := q.Type != "multi" || q.MaxChoices == 1
	if single && hidden > 0 && (hidden == 1 || hiddenSum == hidden || hiddenSum == hidden*(minSegment-1)) {
		j := -1
		for i, o := range out {
			if o.Count != nil && *o.Count > 0 && (j < 0 || *o.Count < *out[j].Count) {
				j = i
			}
		}
		if j < 0 {
			return nil
		}
		out[j].Count, out[j].Suppressed = nil, true
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
