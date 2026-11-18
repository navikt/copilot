package main

import (
	"encoding/csv"
	"io"
	"strconv"
)

// writeCompareCSV writes a comparison in long form, one row per number, so a
// spreadsheet, pandas or an agent reads it without knowing the JSON:
//
//	kind      survey_id question_id item_id version break option count n excluded mean agree p p_holm
//	responses q4-2026                                          211
//	channel   q4-2026                              web    183
//	answer    q4-2026   pastander   tilfredshet 1 false 5     71   203 0  4.15… 0.83… 0.109… 0.562…
//
// mean and agree are on every option row of their version, unrounded; the
// item's first-to-last test (significance) on every answer row of the item. A
// wave with nothing counted has one answer row with only excluded. In the
// published comparison (hideSmallWaveVersions) a version under 5 has one
// answer row with n «hidden» and nothing else counted.
// The published series adds coded free text (publishedCoding): a
// freetext row per published category or merged pair with its count and n
// (respondents who wrote in the field) and a freetext_onske row with the
// wishes among them; version is the codebook's. Counts under 5 are «<5».
// Never a text.
func writeCompareCSV(w io.Writer, res compareResult) error {
	if _, err := w.Write([]byte("\xef\xbb\xbf")); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"kind", "survey_id", "question_id", "item_id", "version", "break", "option", "count", "n", "excluded", "mean", "agree", "p", "p_holm"})
	for _, m := range res.Surveys {
		_ = cw.Write([]string{"responses", m.ID, "", "", "", "", "", strconv.Itoa(m.Responses), "", "", "", "", "", ""})
		for _, c := range sortedKeys(m.Channels) {
			_ = cw.Write([]string{"channel", m.ID, "", "", "", "", c, strconv.Itoa(m.Channels[c]), "", "", "", "", "", ""})
		}
	}
	waves := func(qid, iid string, br bool, ws []waveCounts, sig significance) {
		test := []string{floatCell(sig.P), floatCell(sig.PHolm)}
		for i, wc := range ws {
			excluded := intCell(wc.Excluded)
			if len(wc.Versions) == 0 { // nothing counted, but excluded must survive
				_ = cw.Write(append([]string{"answer", res.Surveys[i].ID, qid, iid, "", strconv.FormatBool(br), "", "", "", excluded, "", ""}, test...))
			}
			for _, v := range wc.Versions {
				if len(v.Options) == 0 { // e.g. only empty multi answers: keep n
					n := intCell(v.N)
					if v.Hidden {
						n = "hidden"
					}
					_ = cw.Write(append([]string{"answer", res.Surveys[i].ID, qid, iid, strconv.Itoa(v.Version), strconv.FormatBool(br), "", "", n, excluded, "", ""}, test...))
				}
				for _, o := range v.Options {
					_ = cw.Write(append([]string{"answer", res.Surveys[i].ID, qid, iid, strconv.Itoa(v.Version), strconv.FormatBool(br),
						o.Value, intCell(o.Count), intCell(v.N), excluded, floatCell(v.Mean), floatCell(v.Agree)}, test...))
				}
			}
		}
	}
	for _, q := range res.Questions {
		waves(q.ID, "", q.Break, q.Waves, q.significance)
		for _, it := range q.Items {
			waves(q.ID, it.ID, it.Break, it.Waves, it.significance)
		}
	}
	for _, wf := range res.Freetext {
		for _, f := range wf.Fields {
			for _, c := range f.Published {
				for _, row := range []struct {
					kind string
					n    *int
				}{{"freetext", c.Count}, {"freetext_onske", c.Onske}} {
					count := intCell(row.n)
					if row.n == nil {
						count = "<5"
					}
					_ = cw.Write([]string{row.kind, wf.Survey, f.ID, "", strconv.Itoa(wf.CodebookVersion), "false", c.ID, count, strconv.Itoa(f.Written), "", "", "", "", ""})
				}
			}
		}
	}
	cw.Flush()
	return cw.Error()
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
