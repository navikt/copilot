package survey

import (
	"cmp"
	"encoding/json"
	"slices"
	"strconv"
	"time"
)

// waveMeta is one wave in a comparison: its summary and how it was answered.
type waveMeta struct {
	Summary
	// Suppressed: fewer than MinSegment responses; this wave has no counts.
	Suppressed bool `json:"suppressed"`
	// Channels counts answers per context.client (forms for the import,
	// web, copilot, opencode, pi …).
	Channels map[string]int `json:"channels"`
}

// waveStat is one version of a question in one wave, with mean and share
// agreeing (the top two steps) for scale and matrix items.
type waveStat struct {
	VersionResult
	Mean  *float64 `json:"mean,omitempty"`
	Agree *float64 `json:"agree,omitempty"`
}

// WaveCounts is one question or item in one wave, index-aligned with
// CompareResult.Surveys. Excluded is how many answers the comparison rule
// dropped (see compare); always set by compare, nil in the published series,
// which has no such rule.
type WaveCounts struct {
	Excluded *int       `json:"excluded,omitempty"`
	Versions []waveStat `json:"versions"`
}

// CompareItem is one matrix item across the waves.
type CompareItem struct {
	ID    string       `json:"id"`
	Text  string       `json:"text"`
	Break bool         `json:"break"` // the waves asked different versions
	Waves []WaveCounts `json:"waves"`
	Significance
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
	Waves  []WaveCounts  `json:"waves,omitempty"` // scale, choice, multi
	Items  []CompareItem `json:"items,omitempty"` // matrix
	Significance
}

// CompareResult is a comparison of waves, question by question.
type CompareResult struct {
	Surveys   []waveMeta        `json:"surveys"`
	Questions []compareQuestion `json:"questions"`
	// Freetext: the published coding per wave, published series only.
	Freetext []WaveFreetext `json:"freetext,omitempty"`
}

// MergeClients sets each row's Client to its context.client as the CSV
// export writes it (SuppressSmallSegments), so the channels online are the
// ones «copilot-survey analyze» reads from that CSV.
func MergeClients(rs []StoredAnswer) []StoredAnswer {
	ex := make([]ExportRow, len(rs))
	for i, r := range rs {
		ex[i] = NewExportRow(map[string]any{}, nil, r.Context, func(string, string) bool { return false })
	}
	SuppressSmallSegments(ex)
	out := slices.Clone(rs)
	for i := range out {
		out[i].Client = ex[i].Context["client"]
	}
	return out
}

// WaveTally is what a comparison needs from one wave, counted from its rows
// or read from its stored aggregates once the rows are deleted: respondents,
// channels (context.client merged as the export merges them), every answer
// key's counts, and per skip_if rule set the counts among the respondents
// it skips (any rule in the set, each respondent once).
type WaveTally struct {
	Responses int
	Channels  map[string]int
	Keys      map[string]map[int]*VerTally
	Skipped   map[string]map[string]map[int]*VerTally // skipSetID → key → version
}

// SkipSet is the skip_if rules on a question, any one of which skips it,
// and the answer keys a definition skips with exactly that set.
type SkipSet struct {
	Rules []SkipRule
	Keys  []string
}

// skipRulesOf is q's skip_if rules; SkipSetID makes their order and
// duplicates irrelevant.
func skipRulesOf(q Question) []SkipRule { return q.SkipIf }

// SkipSetID is the canonical key of a rule set: its rules sorted, without
// duplicates, as JSON. "" for none.
func SkipSetID(rules []SkipRule) string {
	if len(rules) == 0 {
		return ""
	}
	rules = slices.Clone(rules)
	slices.SortFunc(rules, func(a, b SkipRule) int {
		return cmp.Or(cmp.Compare(a.Question, b.Question), cmp.Compare(a.Answer, b.Answer))
	})
	b, _ := json.Marshal(slices.Compact(rules))
	return string(b)
}

// SkipSets is every skip_if rule set in defs, by SkipSetID, with the
// answer keys a definition skips with it.
func SkipSets(defs []Survey) map[string]SkipSet {
	out := map[string]SkipSet{}
	for _, s := range defs {
		for _, q := range s.Questions {
			id := SkipSetID(skipRulesOf(q))
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

// RowsTally counts s's rows for a comparison under any of sets. Channels
// count row.Client as given, so merge first (MergeClients).
func RowsTally(s Survey, rows []StoredAnswer, sets map[string]SkipSet) WaveTally {
	t := WaveTally{Responses: len(rows), Channels: map[string]int{}, Keys: TallyRows(s, rows).Keys, Skipped: map[string]map[string]map[int]*VerTally{}}
	for _, row := range rows {
		t.Channels[row.Client]++
	}
	for id, set := range sets {
		skipped := slices.DeleteFunc(slices.Clone(rows), func(row StoredAnswer) bool { return !SkipRules(set.Rules).skipped(row.Answers) })
		t.Skipped[id] = TallyRows(s, skipped).Keys
	}
	return t
}

// CompareWavesResult is the comparison of waves (oldest first) from their
// rows. The endpoint and «copilot-survey analyze» both count through
// CompareTallies, so the numbers online and from an exported CSV are
// computed by the same code.
func CompareWavesResult(waves []Survey, rows [][]StoredAnswer, now time.Time) CompareResult {
	sets := SkipSets(waves)
	tallies := make([]WaveTally, len(rows))
	for i, rs := range rows {
		tallies[i] = RowsTally(waves[i], rs, sets)
	}
	return CompareTallies(waves, tallies, now)
}

// CompareTallies is the comparison of waves (oldest first) from their
// tallies, live or stored.
func CompareTallies(waves []Survey, tallies []WaveTally, now time.Time) CompareResult {
	res := CompareResult{Questions: []compareQuestion{}}
	tallies = slices.Clone(tallies)
	for i, s := range waves {
		t := tallies[i]
		m := waveMeta{Summary{s.ID, s.Title, s.Starts, s.Ends, s.Status(now), t.Responses, s.Series, s.Published}, t.Responses < MinSegment, t.Channels}
		if m.Suppressed {
			tallies[i], m.Channels = WaveTally{Responses: t.Responses}, nil
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
				cq.Items = append(cq.Items, CompareItem{ID: it.ID, Text: it.Text, Break: br, Waves: ws})
			}
		} else {
			cq.Waves, cq.Break = compareWaves(q, waves, tallies)
		}
		res.Questions = append(res.Questions, cq)
	}
	// After the appends, so the pointers stay valid. Holm runs over every
	// tested item: statements for KI users and for everyone alike.
	var tested []*Significance
	for qi := range res.Questions {
		cq := &res.Questions[qi]
		if cq.Type == "scale" && testFirstLast(&cq.Significance, cq.Waves, cq.Break) {
			tested = append(tested, &cq.Significance)
		}
		for ii := range cq.Items {
			if it := &cq.Items[ii]; testFirstLast(&it.Significance, it.Waves, it.Break) {
				tested = append(tested, &it.Significance)
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
func asked(s Survey, id string) (version, top int, typ string, ok bool) {
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
func compareWaves(q Question, waves []Survey, tallies []WaveTally) ([]WaveCounts, bool) {
	out := make([]WaveCounts, len(tallies))
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
		if id := SkipSetID(skipRulesOf(q)); id != "" {
			kept, excluded = Without(kept, t.Skipped[id][q.ID])
		}
		out[i].Excluded = &excluded
		out[i].Versions = []waveStat{}
		for _, vr := range VersionsFrom(q, kept) {
			ws := waveStat{VersionResult: vr}
			if q.Type == "scale" {
				ws.Mean, ws.Agree = scaleStats(vr, top)
			}
			out[i].Versions = append(out[i].Versions, ws)
			seen[vr.Version] = true
		}
	}
	return out, broken || len(seen) > 1
}

// Without is all minus skipped, as tallyKey would count the rows left, and
// how many answers skipped held.
func Without(all, skipped map[int]*VerTally) (map[int]*VerTally, int) {
	out, excluded := map[int]*VerTally{}, 0
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
		kept := &VerTally{N: vt.N - s.N, Counts: map[string]int{}}
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
func scaleStats(vr VersionResult, top int) (*float64, *float64) {
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
