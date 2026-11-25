package survey

import (
	"slices"
	"strconv"
	"time"
)

// StoredAnswer is one survey_answers row as the admin aggregates read it.
type StoredAnswer struct {
	Answers          map[string]any       `json:"answers"`
	QuestionVersions map[string]int       `json:"question_versions"`
	Client           string               `json:"-"` // context.client: web, copilot, forms (import), …
	Context          map[string]any       `json:"-"` // version, os, client, local_models; for the CSV export
	Codes            map[string]TextCodes `json:"-"` // per field, read back from the CSV's codes columns
}

// Summary is a survey as listed for admins.
type Summary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Starts    string `json:"starts"`
	Ends      string `json:"ends"`
	Status    string `json:"status"` // kommende, åpen or stengt
	Responses int    `json:"responses"`
	Series    string `json:"series,omitempty"`
	Published bool   `json:"published,omitempty"`
}

// Summary is s as listed, with its number of responses.
func (s Survey) Summary(now time.Time, responses int) Summary {
	return Summary{s.ID, s.Title, s.Starts, s.Ends, s.Status(now), responses, s.Series, s.Published}
}

// Status is s's state on now.
func (s Survey) Status(now time.Time) string {
	switch {
	case s.ActiveOn(now):
		return "åpen"
	case now.UTC().Format(time.DateOnly) < s.Starts:
		return "kommende"
	}
	return "stengt"
}

// OptionCount is one option's count. Suppressed is always false; it stays
// for clients that still read it.
type OptionCount struct {
	Value      string `json:"value"`
	Count      *int   `json:"count"`
	Suppressed bool   `json:"suppressed"`
}

// VersionResult is the counts for one version of a question.
type VersionResult struct {
	Version int `json:"version"`
	// N is respondents who answered this version (people, also for multi).
	N       *int          `json:"n"`
	Options []OptionCount `json:"options"`
	// Hidden: published results left out a small group; N and Options are null.
	Hidden bool `json:"hidden,omitempty"`
}

type itemResult struct {
	ID       string          `json:"id"`
	Text     string          `json:"text"`
	Versions []VersionResult `json:"versions"`
}

// QuestionResult is the results for one question.
type QuestionResult struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Versions []VersionResult `json:"versions,omitempty"` // scale, choice, multi
	Items    []itemResult    `json:"items,omitempty"`    // matrix: one per statement
}

// Results is a survey's aggregated results.
type Results struct {
	Summary
	// Suppressed: fewer than MinSegment responses, and Questions is null.
	Suppressed bool             `json:"suppressed"`
	Channels   map[string]int   `json:"channels,omitempty"`
	Questions  []QuestionResult `json:"questions"`
}

// MergeSmallChannels folds every channel under MinSegment into «annet», the
// export's rule for one context value. Admin /results keeps them apart.
func MergeSmallChannels(channels map[string]int) map[string]int {
	if channels == nil {
		return nil
	}
	out := map[string]int{}
	for c, n := range channels {
		if n < MinSegment {
			c = merged
		}
		out[c] += n
	}
	return out
}

// small is a group of 1 to MinSegment-1: shown, its answers could be told apart.
func small(n int) bool { return n > 0 && n < MinSegment }

func (v *VersionResult) hide() { v.N, v.Options, v.Hidden = nil, nil, true }

// HideSmallVersions hides every version under MinSegment respondents in
// published /results: one person answering an old version would otherwise
// show their answer.
func HideSmallVersions(res *Results) {
	hide := func(vs []VersionResult) {
		for i := range vs {
			if vs[i].N != nil && small(*vs[i].N) {
				vs[i].hide()
			}
		}
	}
	for qi := range res.Questions {
		hide(res.Questions[qi].Versions)
		for ii := range res.Questions[qi].Items {
			hide(res.Questions[qi].Items[ii].Versions)
		}
	}
}

// HideSmallWaveVersions hides every version under MinSegment respondents in
// a published comparison, as HideSmallVersions does in /results, and the
// item's test when the first or last wave has one. Holm keeps the hidden
// items (owner decision). It drops excluded, which is 0 without the rule.
func HideSmallWaveVersions(res *CompareResult) {
	hide := func(sig *Significance, ws []WaveCounts) {
		for i := range ws {
			ws[i].Excluded = nil
			for vi := range ws[i].Versions {
				v := &ws[i].Versions[vi]
				if v.N == nil || !small(*v.N) {
					continue
				}
				v.hide()
				v.Mean, v.Agree = nil, nil
				if i == 0 || i == len(ws)-1 {
					*sig = Significance{}
				}
			}
		}
	}
	for qi := range res.Questions {
		q := &res.Questions[qi]
		hide(&q.Significance, q.Waves)
		for ii := range q.Items {
			hide(&q.Items[ii].Significance, q.Items[ii].Waves)
		}
	}
}

// Aggregate counts answers per question and question version. Free text is
// never read. Below MinSegment respondents for the whole survey nothing is
// shown; otherwise every count is real (owner decision 2026-10-13).
func Aggregate(s Survey, rows []StoredAnswer, now time.Time) Results {
	return AggregateTally(s, TallyRows(s, rows), now)
}

// AggregateTally is aggregate from counts, live or stored (survey_results).
func AggregateTally(s Survey, t Tally, now time.Time) Results {
	res := Results{Summary: s.Summary(now, t.Responses)}
	if t.Responses < MinSegment {
		res.Suppressed = true
		return res
	}
	if len(t.Channels) > 0 {
		res.Channels = t.Channels
	}
	res.Questions = []QuestionResult{}
	for _, q := range s.Questions {
		qr := QuestionResult{ID: q.ID, Type: q.Type, Text: q.Text}
		switch q.Type {
		case "text":
			continue
		case "matrix":
			for _, it := range q.Items {
				item := q
				item.Type, item.ID, item.Version = "scale", it.ID, it.Version
				qr.Items = append(qr.Items, itemResult{it.ID, it.Text, VersionsFrom(item, t.Keys[it.ID])})
			}
		default:
			qr.Versions = VersionsFrom(q, t.Keys[q.ID])
		}
		res.Questions = append(res.Questions, qr)
	}
	return res
}

// VersionsFrom turns one answer key's tally into the API's versions, with
// options as cells lists them.
func VersionsFrom(q Question, t map[int]*VerTally) []VersionResult {
	out := []VersionResult{}
	for _, ver := range sortedKeys(t) {
		n := t[ver].N
		out = append(out, VersionResult{Version: ver, N: &n, Options: Cells(q, ver, t[ver].Counts)})
	}
	return out
}

// Cells lists the options as defined (current version) or as seen (older
// versions) with their real counts.
func Cells(q Question, ver int, counts map[string]int) []OptionCount {
	var values []string
	if ver == q.Version {
		if q.Type == "scale" {
			for n := q.Min; n <= q.Max; n++ {
				values = append(values, strconv.Itoa(n))
			}
			if q.Unsure != "" {
				values = append(values, q.Unsure)
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
	out := make([]OptionCount, len(values))
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
