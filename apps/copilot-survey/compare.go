package main

import (
	"cmp"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
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
// dropped (see compare); always set by compare, nil in the published series,
// which has no such rule.
type waveCounts struct {
	Excluded *int       `json:"excluded,omitempty"`
	Versions []waveStat `json:"versions"`
}

type compareItem struct {
	ID    string       `json:"id"`
	Text  string       `json:"text"`
	Break bool         `json:"break"` // the waves asked different versions
	Waves []waveCounts `json:"waves"`
	significance
}

type compareQuestion struct {
	ID     string        `json:"id"`
	Type   string        `json:"type"`
	Text   string        `json:"text"`
	Min    int           `json:"min,omitempty"`
	Max    int           `json:"max,omitempty"`
	Labels []string      `json:"labels,omitempty"`
	Unsure string        `json:"unsure,omitempty"`
	Break  bool          `json:"break,omitempty"`
	Waves  []waveCounts  `json:"waves,omitempty"` // scale, choice, multi
	Items  []compareItem `json:"items,omitempty"` // matrix
	significance
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
	var waves []survey
	for _, id := range ids {
		i := slices.IndexFunc(a.surveys, func(s survey) bool { return s.ID == id })
		if i < 0 || slices.ContainsFunc(waves, func(s survey) bool { return s.ID == id }) {
			writeError(w, http.StatusNotFound, "unknown survey")
			return
		}
		waves = append(waves, a.surveys[i])
	}
	last, sets := waves[len(waves)-1], skipSets(a.surveys)
	tallies := make([]waveTally, len(waves))
	for i, s := range waves {
		rs, err := a.read(r, s.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		tallies[i] = rowsTally(s, mergeClients(rs), sets)
		if len(rs) > 0 || a.finalCompare == nil || s.status(a.now()) != "stengt" {
			continue
		}
		stored, err := a.finalCompare(r.Context(), s.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "survey storage unavailable")
			return
		}
		if stored == nil {
			continue
		}
		for id, set := range skipSets([]survey{last}) {
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
	res := compareTallies(waves, tallies, a.now())
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="sammenligning-`+strings.Join(ids, "-")+`.csv"`)
		w.Header().Set("Cache-Control", "no-store")
		_ = writeCompareCSV(w, res) // headers are sent; a broken connection has no one to tell
		return
	}
	writeJSON(w, res)
}

// mergeClients sets each row's Client to its context.client as the CSV
// export writes it (suppressSmallSegments), so the channels online are the
// ones «copilot-survey analyze» reads from that CSV.
func mergeClients(rs []storedAnswer) []storedAnswer {
	ex := make([]exportRow, len(rs))
	for i, r := range rs {
		ex[i] = newExportRow(map[string]any{}, nil, r.Context, func(string, string) bool { return false })
	}
	suppressSmallSegments(ex)
	out := slices.Clone(rs)
	for i := range out {
		out[i].Client = ex[i].Context["client"]
	}
	return out
}

// waveTally is what a comparison needs from one wave, counted from its rows
// or read from its stored aggregates once the rows are deleted: respondents,
// channels (context.client merged as the export merges them), every answer
// key's counts, and per skip_if rule set the counts among the respondents
// it skips (any rule in the set, each respondent once).
type waveTally struct {
	Responses int
	Channels  map[string]int
	Keys      map[string]map[int]*verTally
	Skipped   map[string]map[string]map[int]*verTally // skipSetID → key → version
}

// skipSet is the skip_if rules on a question, any one of which skips it,
// and the answer keys a definition skips with exactly that set.
type skipSet struct {
	Rules []skipRule
	Keys  []string
}

// skipRulesOf is q's skip_if rules; skipSetID makes their order and
// duplicates irrelevant.
func skipRulesOf(q question) []skipRule { return q.SkipIf }

// skipSetID is the canonical key of a rule set: its rules sorted, without
// duplicates, as JSON. "" for none.
func skipSetID(rules []skipRule) string {
	if len(rules) == 0 {
		return ""
	}
	rules = slices.Clone(rules)
	slices.SortFunc(rules, func(a, b skipRule) int {
		return cmp.Or(cmp.Compare(a.Question, b.Question), cmp.Compare(a.Answer, b.Answer))
	})
	b, _ := json.Marshal(slices.Compact(rules))
	return string(b)
}

// skipSets is every skip_if rule set in defs, by skipSetID, with the
// answer keys a definition skips with it.
func skipSets(defs []survey) map[string]skipSet {
	out := map[string]skipSet{}
	for _, s := range defs {
		for _, q := range s.Questions {
			id := skipSetID(skipRulesOf(q))
			if id == "" || q.Type == "text" {
				continue
			}
			keys := []string{q.ID}
			if q.Type == "matrix" {
				keys = nil
				for _, it := range q.Items {
					keys = append(keys, it.ID)
				}
			}
			set := out[id]
			set.Rules = skipRulesOf(q)
			for _, k := range keys {
				if !slices.Contains(set.Keys, k) {
					set.Keys = append(set.Keys, k)
				}
			}
			out[id] = set
		}
	}
	return out
}

// rowsTally counts s's rows for a comparison under any of sets. Channels
// count row.Client as given, so merge first (mergeClients).
func rowsTally(s survey, rows []storedAnswer, sets map[string]skipSet) waveTally {
	t := waveTally{Responses: len(rows), Channels: map[string]int{}, Keys: tallyRows(s, rows).Keys, Skipped: map[string]map[string]map[int]*verTally{}}
	for _, row := range rows {
		t.Channels[row.Client]++
	}
	for id, set := range sets {
		skipped := slices.DeleteFunc(slices.Clone(rows), func(row storedAnswer) bool { return !skipRules(set.Rules).skipped(row.Answers) })
		t.Skipped[id] = tallyRows(s, skipped).Keys
	}
	return t
}

// compareWavesResult is the comparison of waves (oldest first) from their
// rows. The endpoint and «copilot-survey analyze» both count through
// compareTallies, so the numbers online and from an exported CSV are
// computed by the same code.
func compareWavesResult(waves []survey, rows [][]storedAnswer, now time.Time) compareResult {
	sets := skipSets(waves)
	tallies := make([]waveTally, len(rows))
	for i, rs := range rows {
		tallies[i] = rowsTally(waves[i], rs, sets)
	}
	return compareTallies(waves, tallies, now)
}

// compareTallies is the comparison of waves (oldest first) from their
// tallies, live or stored.
func compareTallies(waves []survey, tallies []waveTally, now time.Time) compareResult {
	res := compareResult{Questions: []compareQuestion{}}
	tallies = slices.Clone(tallies)
	for i, s := range waves {
		t := tallies[i]
		m := waveMeta{surveySummary{s.ID, s.Title, s.Starts, s.Ends, s.status(now), t.Responses, s.Series, s.Published}, t.Responses < minSegment, t.Channels}
		if m.Suppressed {
			tallies[i], m.Channels = waveTally{Responses: t.Responses}, nil
		}
		res.Surveys = append(res.Surveys, m)
	}
	for _, q := range waves[len(waves)-1].Questions {
		if q.Type == "text" {
			continue
		}
		cq := compareQuestion{ID: q.ID, Type: q.Type, Text: q.Text, Min: q.Min, Max: q.Max, Labels: q.Labels, Unsure: q.Unsure}
		if q.Type == "matrix" {
			for _, it := range q.Items {
				item := q
				item.Type, item.ID, item.Version = "scale", it.ID, it.Version
				ws, br := compareWaves(item, waves, tallies)
				cq.Items = append(cq.Items, compareItem{ID: it.ID, Text: it.Text, Break: br, Waves: ws})
			}
		} else {
			cq.Waves, cq.Break = compareWaves(q, waves, tallies)
		}
		res.Questions = append(res.Questions, cq)
	}
	// After the appends, so the pointers stay valid. Holm runs over every
	// tested item: statements for KI users and for everyone alike.
	var tested []*significance
	for qi := range res.Questions {
		cq := &res.Questions[qi]
		if cq.Type == "scale" && testFirstLast(&cq.significance, cq.Waves, cq.Break) {
			tested = append(tested, &cq.significance)
		}
		for ii := range cq.Items {
			if it := &cq.Items[ii]; testFirstLast(&it.significance, it.Waves, it.Break) {
				tested = append(tested, &it.significance)
			}
		}
	}
	ps := make([]float64, len(tested))
	for i, t := range tested {
		ps[i] = *t.P
	}
	for i, h := range holm(ps) {
		tested[i].PHolm = &h
	}
	return res
}

// asked finds the question or matrix item id in s as asked there: its
// version, scale maximum and type (scale for a matrix item).
func asked(s survey, id string) (version, top int, typ string, ok bool) {
	for _, q := range s.Questions {
		if q.ID == id {
			return q.Version, q.Max, q.Type, true
		}
		for _, it := range q.Items {
			if it.ID == id {
				return it.Version, q.Max, "scale", true
			}
		}
	}
	return 0, 0, "", false
}

// compareWaves counts q in every wave after the skip rule, and reports a
// break when the waves did not all ask, or answer, the same single version.
// A wave's declared version counts even without answers; a wave that did
// not ask the question at all does not break the series. A wave that asked
// the id as another type (free text, say) has no counts and breaks it.
func compareWaves(q question, waves []survey, tallies []waveTally) ([]waveCounts, bool) {
	out := make([]waveCounts, len(tallies))
	seen := map[int]bool{}
	broken := false
	for i, t := range tallies {
		top := q.Max
		v, m, typ, ok := asked(waves[i], q.ID)
		if ok {
			seen[v], top = true, m
		}
		if ok && typ != q.Type {
			zero := 0
			out[i].Excluded, out[i].Versions, broken = &zero, []waveStat{}, true
			continue
		}
		kept, excluded := t.Keys[q.ID], 0
		if id := skipSetID(skipRulesOf(q)); id != "" {
			kept, excluded = without(kept, t.Skipped[id][q.ID])
		}
		out[i].Excluded = &excluded
		out[i].Versions = []waveStat{}
		for _, vr := range versionsFrom(q, kept) {
			ws := waveStat{versionResult: vr}
			if q.Type == "scale" {
				ws.Mean, ws.Agree = scaleStats(vr, top)
			}
			out[i].Versions = append(out[i].Versions, ws)
			seen[vr.Version] = true
		}
	}
	return out, broken || len(seen) > 1
}

// without is all minus skipped, as tallyKey would count the rows left, and
// how many answers skipped held.
func without(all, skipped map[int]*verTally) (map[int]*verTally, int) {
	out, excluded := map[int]*verTally{}, 0
	for ver, vt := range all {
		s := skipped[ver]
		if s == nil {
			out[ver] = vt
			continue
		}
		excluded += s.N
		if vt.N == s.N {
			continue
		}
		kept := &verTally{N: vt.N - s.N, Counts: map[string]int{}}
		for o, c := range vt.Counts {
			if c -= s.Counts[o]; c > 0 {
				kept.Counts[o] = c
			}
		}
		out[ver] = kept
	}
	return out, excluded
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
