package main

import (
	"encoding/csv"
	"io"
	"strconv"
)

// writeCompareCSV writes a comparison in long form, one row per number, so a
// spreadsheet, pandas or an agent reads it without knowing the JSON:
//
//	kind      survey_id question_id item_id version break option count n excluded mean agree
//	responses q4-2026                                          211
//	channel   q4-2026                              web    183
//	answer    q4-2026   pastander   tilfredshet 1 false 5     71   203 0  4.15… 0.83…
//
// mean and agree are on every option row of their version, unrounded.
// Free text is never in a comparison.
func writeCompareCSV(w io.Writer, res compareResult) {
	_, _ = w.Write([]byte("\xef\xbb\xbf"))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"kind", "survey_id", "question_id", "item_id", "version", "break", "option", "count", "n", "excluded", "mean", "agree"})
	for _, m := range res.Surveys {
		_ = cw.Write([]string{"responses", m.ID, "", "", "", "", "", strconv.Itoa(m.Responses), "", "", "", ""})
		for _, c := range sortedKeys(m.Channels) {
			_ = cw.Write([]string{"channel", m.ID, "", "", "", "", c, strconv.Itoa(m.Channels[c]), "", "", "", ""})
		}
	}
	waves := func(qid, iid string, br bool, ws []waveCounts) {
		for i, wc := range ws {
			for _, v := range wc.Versions {
				for _, o := range v.Options {
					_ = cw.Write([]string{"answer", res.Surveys[i].ID, qid, iid, strconv.Itoa(v.Version), strconv.FormatBool(br),
						o.Value, intCell(o.Count), intCell(v.N), strconv.Itoa(wc.Excluded), floatCell(v.Mean), floatCell(v.Agree)})
				}
			}
		}
	}
	for _, q := range res.Questions {
		waves(q.ID, "", q.Break, q.Waves)
		for _, it := range q.Items {
			waves(q.ID, it.ID, it.Break, it.Waves)
		}
	}
	cw.Flush()
}

func intCell(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}

func floatCell(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}
