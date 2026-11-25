package survey

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Free-text coding (#2239). A codebook per series (surveys/codebooks/) gives
// each free-text field its categories. Two coders code every text: a model
// on the masked text, and a person, blind to the model until they have
// saved. Where the two differ, the person decides and the decision is logged
// (who, when; never the text). Codes are keyed by sha256 of the raw text per
// survey and field, as hiding is (freetext.go): identical texts share codes,
// and no code points at a row.
//
// What is counted: per field, the respondents who wrote in it (hidden texts
// left out) and, per category, how many of them got it as a final code.
// Agreement is Cohen's kappa per category between the two coders over the
// texts both coded. A category is published at kappa ≥ minKappa; below
// that together with its merge_with partner when the pair reaches it;
// otherwise not at all. A category nobody coded is not published and does
// not pull its partner into a pair. Counts and shares under minSegment show
// as «<5».

const minKappa = 0.6

type codebook struct {
	Series     string             `json:"series"`
	Version    int                `json:"version"`
	Rules      []string           `json:"rules"`
	Fields     []codebookField    `json:"fields"`
	Categories []codebookCategory `json:"categories"`
}

type codebookField struct {
	ID         string   `json:"id"`
	Onske      string   `json:"onske"` // kodes, alltid or aldri
	MaxCodes   int      `json:"max_codes"`
	Categories []string `json:"categories"`
}

type codebookCategory struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	MergeWith  string   `json:"merge_with,omitempty"`
	Definition string   `json:"definition"`
	Include    []string `json:"include"`
	Exclude    []string `json:"exclude"`
	Examples   []string `json:"examples"`
}

// code is one code a coder gave a text.
type code struct {
	Category string `json:"category"`
	Onske    bool   `json:"onske"`
}

// textCodes are one text's codes per coder; nil: that coder has not coded it.
type textCodes struct {
	Model, Human, Final []code
}

// loadCodebooks reads surveys/codebooks/<series>.json, by series.
func loadCodebooks(fsys fs.FS) (map[string]codebook, error) {
	names, err := fs.Glob(fsys, "surveys/codebooks/*.json")
	if err != nil {
		return nil, err
	}
	out := map[string]codebook{}
	for _, name := range names {
		if path.Base(name) == "schema.json" {
			continue
		}
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		var cb codebook
		if err := json.Unmarshal(raw, &cb); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out[cb.Series] = cb
	}
	return out, nil
}

func (cb codebook) field(id string) (codebookField, bool) {
	i := slices.IndexFunc(cb.Fields, func(f codebookField) bool { return f.ID == id })
	if i < 0 {
		return codebookField{}, false
	}
	return cb.Fields[i], true
}

func (cb codebook) category(id string) codebookCategory {
	i := slices.IndexFunc(cb.Categories, func(c codebookCategory) bool { return c.ID == id })
	if i < 0 {
		return codebookCategory{ID: id}
	}
	return cb.Categories[i]
}

// partner is c's merge_with partner when f codes it too, else "".
func (cb codebook) partner(f codebookField, c string) string {
	p := cb.category(c).MergeWith
	if p == "" || !slices.Contains(f.Categories, p) {
		return ""
	}
	return p
}

// pairID names a merged pair, in f's category order.
func pairID(f codebookField, a, b string) string {
	if slices.Index(f.Categories, a) > slices.Index(f.Categories, b) {
		a, b = b, a
	}
	return a + "+" + b
}

// checkCodes validates codes for f and returns them sorted: 1 to max_codes,
// each category in f's set and given once per onske, «annet» alone. onske is
// set from the field where the question fixes it.
func checkCodes(f codebookField, codes []code) ([]code, error) {
	out := slices.Clone(codes)
	for i := range out {
		switch f.Onske {
		case "alltid":
			out[i].Onske = true
		case "aldri":
			out[i].Onske = false
		}
	}
	slices.SortFunc(out, func(a, b code) int {
		if c := strings.Compare(a.Category, b.Category); c != 0 {
			return c
		}
		if a.Onske == b.Onske {
			return 0
		}
		if !a.Onske {
			return -1
		}
		return 1
	})
	out = slices.Compact(out)
	switch {
	case len(out) == 0:
		return nil, errors.New("no codes")
	case len(out) > f.MaxCodes:
		return nil, fmt.Errorf("at most %d codes", f.MaxCodes)
	}
	for _, c := range out {
		if !slices.Contains(f.Categories, c.Category) {
			return nil, fmt.Errorf("category %q is not coded in %s", c.Category, f.ID)
		}
		if c.Category == "annet" && len(out) > 1 {
			return nil, errors.New("annet stands alone")
		}
	}
	return out, nil
}

// final is a text's final codes: the decision, or the two coders' codes
// where they agree. nil: not final yet.
func (t textCodes) final() []code {
	switch {
	case t.Final != nil:
		return t.Final
	case t.Human != nil && t.Model != nil && slices.Equal(t.Human, t.Model):
		return t.Human
	}
	return nil
}

// disagree: both coders coded the text, differently, and nobody has decided.
func (t textCodes) disagree() bool {
	return t.Final == nil && t.Human != nil && t.Model != nil && !slices.Equal(t.Human, t.Model)
}

func hasAny(codes []code, cats ...string) bool {
	return slices.ContainsFunc(codes, func(c code) bool { return slices.Contains(cats, c.Category) })
}

func hasWish(codes []code, cats ...string) bool {
	return slices.ContainsFunc(codes, func(c code) bool { return c.Onske && slices.Contains(cats, c.Category) })
}

// catTally is one category's, or one merged pair's, numbers in a field.
type catTally struct {
	Count int      // respondents with it as a final code
	Onske int      // of those, coded as a wish
	Kappa *float64 // between the coders; nil when undefined
}

// fieldTally is one field's coding: Written respondents wrote in it, Coded
// of them have final codes. Cats is keyed by category, and by pairID for
// every merge_with pair the field codes.
type fieldTally struct {
	Written int
	Coded   int
	Cats    map[string]catTally
}

// tallyCodes counts one field from its respondents' texts' codes, one entry
// per respondent who wrote in it.
func tallyCodes(cb codebook, f codebookField, texts []textCodes) fieldTally {
	t := fieldTally{Written: len(texts), Cats: map[string]catTally{}}
	groups := map[string][]string{}
	for _, c := range f.Categories {
		groups[c] = []string{c}
		if p := cb.partner(f, c); p != "" {
			groups[pairID(f, c, p)] = []string{c, p}
		}
	}
	for _, tc := range texts {
		if tc.final() != nil {
			t.Coded++
		}
	}
	for id, cats := range groups {
		var ct catTally
		var both, a, b, ab int // texts both coded; model yes, human yes, both yes
		for _, tc := range texts {
			if fin := tc.final(); hasAny(fin, cats...) {
				ct.Count++
				if hasWish(fin, cats...) {
					ct.Onske++
				}
			}
			if tc.Model == nil || tc.Human == nil {
				continue
			}
			both++
			m, h := hasAny(tc.Model, cats...), hasAny(tc.Human, cats...)
			if m {
				a++
			}
			if h {
				b++
			}
			if m && h {
				ab++
			}
		}
		ct.Kappa = kappa(both, a, b, ab)
		t.Cats[id] = ct
	}
	return t
}

// kappa is Cohen's kappa for a yes/no code over n texts: a model yes, b
// human yes, ab both yes. nil when n is 0 or chance agreement is 1 (both
// coders gave the same answer to every text).
func kappa(n, a, b, ab int) *float64 {
	if n == 0 {
		return nil
	}
	nf := float64(n)
	po := float64(n-a-b+2*ab) / nf
	pe := (float64(a)*float64(b) + float64(n-a)*float64(n-b)) / (nf * nf)
	if pe >= 1 {
		return nil
	}
	k := (po - pe) / (1 - pe)
	return &k
}

// codedCategory is one row of a field's coding results.
type codedCategory struct {
	ID    string   `json:"id"` // a category, or «a+b» for a merged pair
	Name  string   `json:"name"`
	Count *int     `json:"count"` // nil with small
	Onske *int     `json:"onske"`
	Share *float64 `json:"share"` // of written; nil with small
	Small bool     `json:"small,omitempty"`
	Kappa *float64 `json:"kappa"`
	// Status: published (alone or as the pair), merged (in the pair
	// merged_into), or hidden.
	Status   string `json:"status"`
	MergedTo string `json:"merged_into,omitempty"`
}

type codedField struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Other    string `json:"other,omitempty"`
	Written  int    `json:"written"`
	Coded    int    `json:"coded"`
	// Categories: every category with its kappa and status (admin only).
	Categories []codedCategory `json:"categories,omitempty"`
	// Published: what may be shown, «<5» applied; null under minSegment writers.
	Published []codedCategory `json:"published"`
}

// codingResults applies the publication rule to a survey's field tallies,
// in codebook order. Admin sees every category's real count; published
// rows get «<5».
func codingResults(s survey, cb codebook, tallies map[string]fieldTally) []codedField {
	out := []codedField{}
	for _, ff := range freetextFields(s) {
		f, ok := cb.field(ff.ID)
		t, counted := tallies[ff.ID]
		if !ok || !counted {
			continue
		}
		res := codedField{ID: f.ID, Question: ff.Question, Other: ff.Other, Written: t.Written, Coded: t.Coded}
		status := map[string]string{}
		merged := map[string]string{}
		var published []string
		passes := func(id string) bool { k := t.Cats[id].Kappa; return k != nil && *k >= minKappa }
		// Nobody coded it: nothing to publish, and no reason to merge its partner.
		unused := func(id string) bool { return t.Cats[id].Kappa == nil && t.Cats[id].Count == 0 }
		for _, c := range f.Categories {
			if status[c] != "" {
				continue
			}
			p := cb.partner(f, c)
			switch {
			case p == "" || unused(c) || unused(p) || passes(c) && passes(p):
				status[c] = "hidden"
				if passes(c) {
					status[c], published = "published", append(published, c)
				}
			case passes(pairID(f, c, p)):
				id := pairID(f, c, p)
				status[c], status[p], merged[c], merged[p] = "merged", "merged", id, id
				published = append(published, id)
			default:
				for _, x := range []string{c, p} {
					status[x] = "hidden"
					if passes(x) {
						status[x], published = "published", append(published, x)
					}
				}
			}
		}
		row := func(id string, mask bool) codedCategory {
			ct := t.Cats[id]
			name := cb.category(id).Name
			if a, b, ok := strings.Cut(id, "+"); ok {
				name = cb.category(a).Name + " / " + cb.category(b).Name
			}
			r := codedCategory{ID: id, Name: name, Kappa: ct.Kappa, Status: status[id], MergedTo: merged[id]}
			count, onske := ct.Count, ct.Onske
			if mask && small(count) {
				r.Small = true
			} else {
				r.Count = &count
				if t.Written > 0 {
					share := float64(count) / float64(t.Written)
					r.Share = &share
				}
			}
			if !mask || !small(onske) && !r.Small {
				r.Onske = &onske
			}
			return r
		}
		for _, c := range f.Categories {
			res.Categories = append(res.Categories, row(c, false))
		}
		if t.Written >= minSegment {
			res.Published = []codedCategory{}
			for _, id := range published {
				r := row(id, true)
				r.Status = "published"
				res.Published = append(res.Published, r)
			}
		}
		out = append(out, res)
	}
	return out
}
