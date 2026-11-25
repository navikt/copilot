package survey

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"time"
)

// Final aggregates (#1774). When a survey closes, its counts go to
// survey_results, one row per (kind, key, version, option):
//
//   - total:   key "", option "", count = respondents.
//   - channel: key "", option = context.client, count = respondents for
//     that client (unmerged, so finalized results match live TallyRows).
//   - answer:  key = question id or matrix item id; count per option, n =
//     respondents for that version, mean for scale and matrix items. Free
//     text is never read.
//   - context: key = client, os, version, local_models; option = value,
//     with values held by fewer than MinSegment rows merged into «annet»
//     (the export's rule, SuppressSmallSegments).
//
// The admin results read these for a closed survey, so the individual
// answers can go. They are kept until the next wave of the same series has
// closed, or the survey's own retention if that is later (see
// extendRetention).

// VerTally is the counts for one version of an answer key.
type VerTally struct {
	N      int
	Counts map[string]int
}

// Tally is a survey's counts: per answer key and version, and per context.
type Tally struct {
	Responses int
	Keys      map[string]map[int]*VerTally // answer key → version
	Channels  map[string]int
}

func tallyKey(id string, rows []StoredAnswer) map[int]*VerTally {
	t := map[int]*VerTally{}
	for _, row := range rows {
		v, ok := row.Answers[id]
		if !ok || v == nil {
			continue
		}
		ver := row.QuestionVersions[id]
		if t[ver] == nil {
			t[ver] = &VerTally{Counts: map[string]int{}}
		}
		t[ver].N++
		switch v := v.(type) {
		case []any:
			for _, p := range v {
				t[ver].Counts[fmt.Sprint(p)]++
			}
		case float64:
			t[ver].Counts[strconv.FormatFloat(v, 'f', -1, 64)]++
		default:
			t[ver].Counts[fmt.Sprint(v)]++
		}
	}
	return t
}

// AnswerKeys are the keys counted for s, with whether each is on a scale.
func AnswerKeys(s Survey) map[string]bool {
	keys := map[string]bool{}
	for _, q := range s.Questions {
		switch q.Type {
		case "text":
		case "matrix":
			for _, it := range q.Items {
				keys[it.ID] = true
			}
		default:
			keys[q.ID] = q.Type == "scale"
		}
	}
	return keys
}

// TallyRows counts rows for s.
func TallyRows(s Survey, rows []StoredAnswer) Tally {
	t := Tally{Responses: len(rows), Keys: map[string]map[int]*VerTally{}, Channels: map[string]int{}}
	for k := range AnswerKeys(s) {
		t.Keys[k] = tallyKey(k, rows)
	}
	for _, row := range rows {
		if row.Client != "" {
			t.Channels[row.Client]++
		}
	}
	return t
}

// ContextStrings is c with its string values only.
func ContextStrings(c map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range c {
		if b, ok := v.(bool); ok {
			out[k] = strconv.FormatBool(b)
		} else {
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

// NextRoundEnd is the last day of the next wave of s's series (by start
// date) that has closed. false: none has closed yet, or s has no series.
func NextRoundEnd(defs []Survey, s Survey, now time.Time) (time.Time, bool) {
	var next *Survey
	for i, d := range defs {
		if s.Series != "" && d.Series == s.Series && d.Starts > s.Starts && (next == nil || d.Starts < next.Starts) {
			next = &defs[i]
		}
	}
	if next == nil || next.ClosesOn().After(now) {
		return time.Time{}, false
	}
	end, err := time.Parse(time.DateOnly, next.Ends)
	return end, err == nil
}

// Aggregator writes a closed survey's final aggregates (store.Aggregate) and
// returns the number of responses.
type Aggregator func(ctx context.Context, dbURL string, defs []Survey, s Survey, now time.Time) (int, error)

// RunAggregate is `copilot-survey aggregate --survey <id>`: (re)writes a
// closed survey's aggregates from its rows, for a survey that had no
// close-out in this service (an import such as q2-2026), or to redo one.
func RunAggregate(args []string, out io.Writer, dbURL string, now time.Time, aggregate Aggregator) error {
	fs := flag.NewFlagSet("aggregate", flag.ContinueOnError)
	id := fs.String("survey", "", "survey id, e.g. q2-2026")
	if err := fs.Parse(args); err != nil {
		return err
	}
	defs, err := LoadSurveyDir(SurveyFiles)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(defs, func(s Survey) bool { return s.ID == *id })
	if i < 0 {
		return fmt.Errorf("no survey %q", *id)
	}
	if defs[i].ClosesOn().After(now) {
		return fmt.Errorf("%s has not closed", *id)
	}
	if dbURL == "" {
		return errors.New("DB_URL is not set")
	}
	n, err := aggregate(context.Background(), dbURL, defs, defs[i], now)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s has no rows in survey_answers: nothing written", *id)
	}
	fmt.Fprintf(out, "wrote aggregates for %s: %d responses\n", *id, n)
	return nil
}
