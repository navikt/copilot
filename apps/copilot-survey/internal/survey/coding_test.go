package survey

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// testCodebook codes testSurveys' comment (onske per code) and editor.other
// (never a wish).
var testCodebook = Codebook{
	Series: "dev", Version: 1,
	Fields: []CodebookField{
		{ID: "editor.other", Onske: "aldri", MaxCodes: 2, Categories: []string{"a", "annet"}},
		{ID: "comment", Onske: "kodes", MaxCodes: 2, Categories: []string{"a", "b", "c", "d", "e", "f", "annet"}},
	},
	Categories: []CodebookCategory{
		{ID: "a", Name: "A"}, {ID: "b", Name: "B", MergeWith: "c"}, {ID: "c", Name: "C", MergeWith: "b"},
		{ID: "d", Name: "D"}, {ID: "e", Name: "E", MergeWith: "f"}, {ID: "f", Name: "F", MergeWith: "e"}, {ID: "annet", Name: "Annet"},
	},
}

func TestKappa(t *testing.T) {
	for _, tc := range []struct {
		n, a, b, ab int
		want        float64 // NaN: undefined
	}{
		{10, 5, 5, 5, 1},
		{4, 2, 2, 1, 0},
		{10, 4, 5, 3, 0.4}, // po 0.7, pe 0.5
		{0, 0, 0, 0, math.NaN()},
		{5, 0, 0, 0, math.NaN()}, // nobody used it
		{5, 5, 5, 5, math.NaN()},
	} {
		got := kappa(tc.n, tc.a, tc.b, tc.ab)
		if math.IsNaN(tc.want) != (got == nil) || got != nil && math.Abs(*got-tc.want) > 1e-9 {
			t.Errorf("kappa%v = %v, want %v", tc, got, tc.want)
		}
	}
}

func TestCheckCodes(t *testing.T) {
	comment, _ := testCodebook.Field("comment")
	editor, _ := testCodebook.Field("editor.other")
	for _, tc := range []struct {
		f     CodebookField
		codes []Code
		want  string // "" ok, else part of the error
	}{
		{comment, []Code{{"b", true}, {"a", false}}, ""},
		{comment, []Code{{"a", true}, {"a", false}}, ""}, // a wish and an experience
		{comment, nil, "no codes"},
		{comment, []Code{{"a", false}, {"b", false}, {"c", false}}, "at most 2"},
		{comment, []Code{{"annet", false}, {"a", false}}, "alone"},
		{comment, []Code{{"x", false}}, "not coded"},
		{editor, []Code{{"b", false}}, "not coded"},
	} {
		_, err := CheckCodes(tc.f, tc.codes)
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%s %v: %v, want %q", tc.f.ID, tc.codes, err, tc.want)
		}
	}
	got, _ := CheckCodes(editor, []Code{{"a", true}, {"a", false}})
	if len(got) != 1 || got[0].Onske {
		t.Errorf("aldri: %v", got)
	}
}

func k(v float64) *float64 { return &v }

// The publication rule: alone at 0.6, else as the merge_with pair if the
// pair reaches it, else hidden; «<5» for counts and shares.
func TestCodingResultsRule(t *testing.T) {
	defs, _ := loadSurveys([]byte(testSurveys))
	s := defs[0]
	tally := FieldTally{Written: 20, Coded: 20, Cats: map[string]CatTally{
		"a": {Count: 8, Onske: 2, Kappa: k(0.8)},
		"b": {Count: 6, Kappa: k(0.5)}, "c": {Count: 3, Kappa: k(0.7)}, "b+c": {Count: 8, Onske: 6, Kappa: k(0.65)},
		"d": {Count: 9, Kappa: k(0.59)},
		"e": {Count: 9, Kappa: k(0.61)}, "f": {Count: 9, Kappa: k(0.4)}, "e+f": {Count: 12, Kappa: k(0.5)},
		"annet": {Count: 2, Kappa: nil},
	}}
	res := CodingResults(s, testCodebook, map[string]FieldTally{"comment": tally, "editor.other": {Written: 4, Cats: map[string]CatTally{}}})
	if len(res) != 2 || res[0].ID != "editor.other" || res[1].ID != "comment" {
		t.Fatalf("fields %+v", res)
	}
	if res[0].Published != nil {
		t.Errorf("4 writers published: %+v", res[0].Published)
	}
	got := map[string]string{}
	for _, c := range res[1].Categories {
		got[c.ID] = c.Status + c.MergedTo
		if c.Count == nil {
			t.Errorf("admin count hidden for %s", c.ID)
		}
	}
	want := map[string]string{"a": "published", "b": "mergedb+c", "c": "mergedb+c", "d": "hidden", "e": "published", "f": "hidden", "annet": "hidden"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("status %v\nwant   %v", got, want)
	}
	var pub []string
	for _, c := range res[1].Published {
		pub = append(pub, fmt.Sprintf("%s:%v:%v:%v", c.ID, deref(c.Count), deref(c.Onske), c.Small))
	}
	if strings.Join(pub, " ") != "a:8:<nil>:false b+c:8:6:false e:9:0:false" {
		t.Errorf("published %v", pub)
	}
	tally.Cats["a"] = CatTally{Count: 4, Onske: 1, Kappa: k(0.9)}
	res = CodingResults(s, testCodebook, map[string]FieldTally{"comment": tally})
	if a := res[0].Published[0]; !a.Small || a.Count != nil || a.Share != nil || a.Onske != nil {
		t.Errorf("small group shown: %+v", a)
	}
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestShippedCodebooksLoad(t *testing.T) {
	cbs, err := LoadCodebooks(SurveyFiles)
	if err != nil || cbs["utviklerundersokelsen"].Version < 1 {
		t.Fatalf("%v %v", err, cbs)
	}
}
