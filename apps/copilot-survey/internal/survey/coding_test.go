package survey

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// testCodebook codes testSurveys' comment (onske per code) and editor.other
// (never a wish).
var testCodebook = codebook{
	Series: "dev", Version: 1,
	Fields: []codebookField{
		{ID: "editor.other", Onske: "aldri", MaxCodes: 2, Categories: []string{"a", "annet"}},
		{ID: "comment", Onske: "kodes", MaxCodes: 2, Categories: []string{"a", "b", "c", "d", "e", "f", "annet"}},
	},
	Categories: []codebookCategory{
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
	comment, _ := testCodebook.field("comment")
	editor, _ := testCodebook.field("editor.other")
	for _, tc := range []struct {
		f     codebookField
		codes []code
		want  string // "" ok, else part of the error
	}{
		{comment, []code{{"b", true}, {"a", false}}, ""},
		{comment, []code{{"a", true}, {"a", false}}, ""}, // a wish and an experience
		{comment, nil, "no codes"},
		{comment, []code{{"a", false}, {"b", false}, {"c", false}}, "at most 2"},
		{comment, []code{{"annet", false}, {"a", false}}, "alone"},
		{comment, []code{{"x", false}}, "not coded"},
		{editor, []code{{"b", false}}, "not coded"},
	} {
		_, err := checkCodes(tc.f, tc.codes)
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%s %v: %v, want %q", tc.f.ID, tc.codes, err, tc.want)
		}
	}
	got, _ := checkCodes(editor, []code{{"a", true}, {"a", false}})
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
	tally := fieldTally{Written: 20, Coded: 20, Cats: map[string]catTally{
		"a": {Count: 8, Onske: 2, Kappa: k(0.8)},
		"b": {Count: 6, Kappa: k(0.5)}, "c": {Count: 3, Kappa: k(0.7)}, "b+c": {Count: 8, Onske: 6, Kappa: k(0.65)},
		"d": {Count: 9, Kappa: k(0.59)},
		"e": {Count: 9, Kappa: k(0.61)}, "f": {Count: 9, Kappa: k(0.4)}, "e+f": {Count: 12, Kappa: k(0.5)},
		"annet": {Count: 2, Kappa: nil},
	}}
	res := codingResults(s, testCodebook, map[string]fieldTally{"comment": tally, "editor.other": {Written: 4, Cats: map[string]catTally{}}})
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
	tally.Cats["a"] = catTally{Count: 4, Onske: 1, Kappa: k(0.9)}
	res = codingResults(s, testCodebook, map[string]fieldTally{"comment": tally})
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

// fakeCodes is survey_freetext_codes and the decision log in memory.
type fakeCodes struct {
	codes     map[string]*textCodes
	decisions []string
}

func withFakeCodes(api *surveyAPI) *fakeCodes {
	withFakeHidden(api)
	api.surveys[0].Series = "dev"
	api.codebooks = map[string]codebook{"dev": testCodebook}
	f := &fakeCodes{codes: map[string]*textCodes{}}
	api.codes = func(context.Context, string, int) (map[string]*textCodes, error) { return f.codes, nil }
	api.saveCodes = func(_ context.Context, _, field, hash, coder string, _ int, codes []code, by string) error {
		key := hiddenKey(field, hash)
		if f.codes[key] == nil {
			f.codes[key] = &textCodes{}
		}
		switch coder {
		case "human":
			if f.codes[key].Human != nil {
				return errCoded
			}
			f.codes[key].Human = codes
		case "model":
			f.codes[key].Model = codes
		case "final":
			f.codes[key].Final = codes
			f.decisions = append(f.decisions, field+" "+by)
		}
		return nil
	}
	return f
}

type codingResp struct {
	Next          *codingText      `json:"next"`
	Disagreements []codingText     `json:"disagreements"`
	Progress      []codingProgress `json:"progress"`
	Results       []codedField     `json:"results"`
}

func getCoding(t *testing.T, h http.Handler) (codingResp, string) {
	t.Helper()
	rec := do(h, "GET", "/api/v1/admin/surveys/q4-2026/freetext/coding", adminToken, "")
	var got codingResp
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return got, rec.Body.String()
}

func TestCodingFlow(t *testing.T) {
	rows := append([]storedAnswer{}, freetextRowsFixture...)
	rows = append(rows, row(`{"comment":"sakte"}`, `{}`)) // shares the codes of the other «sakte»
	api := adminAPI(t, rows)
	f := withFakeCodes(api)
	h := makeRouter(testAuthenticator(t), api)

	// The model codes first, through the file `copilot-survey code` writes.
	file := modelCodes{CodebookVersion: 1, Codes: []maskedCode{
		{Field: "comment", Text: textHash("mail meg på [e-post]"), Codes: []code{{"a", false}}},
		{Field: "comment", Text: textHash("sakte"), Codes: []code{{"b", false}}},
		{Field: "editor.other", Text: textHash("emacs"), Codes: []code{{"a", true}}},
		{Field: "comment", Text: textHash("finnes ikke"), Codes: []code{{"a", false}}},
	}}
	body, _ := json.Marshal(file)
	rec := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/coding/model", adminToken, string(body))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"saved":3`) || !strings.Contains(rec.Body.String(), `"unmatched":1`) {
		t.Fatalf("import %d %s", rec.Code, rec.Body)
	}
	if c := f.codes[hiddenKey("editor.other", textHash("emacs"))]; c == nil || c.Model[0].Onske {
		t.Errorf("aldri not applied on import: %+v", c)
	}

	// Blind: the text to code carries no codes, the answer to saving does.
	seen := map[string]bool{}
	for range 3 {
		got, body := getCoding(t, h)
		if got.Next == nil || got.Next.Model != nil || strings.Contains(body, "a@b.no") || strings.Contains(body, "opencode") {
			t.Fatalf("next %+v\n%s", got.Next, body)
		}
		seen[got.Next.Text] = true
		codes := `[{"category":"a","onske":true}]`
		if got.Next.Text == "sakte" {
			codes = `[{"category":"b","onske":false}]`
		}
		rec := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/coding", adminToken, `{"token":"`+got.Next.Token+`","coder":"human","codes":`+codes+`}`)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"model":[`) {
			t.Fatalf("save %d %s", rec.Code, rec.Body)
		}
		if again := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/coding", adminToken, `{"token":"`+got.Next.Token+`","coder":"human","codes":`+codes+`}`); again.Code != 409 {
			t.Errorf("second human coding: %d", again.Code)
		}
	}
	if len(seen) != 3 {
		t.Errorf("texts offered %v", seen)
	}
	got, _ := getCoding(t, h)
	if got.Next != nil {
		t.Errorf("all coded, still offered %+v", got.Next)
	}
	// comment «mail meg»: model a/false, human a/true. editor: both a/false.
	if len(got.Disagreements) != 1 || got.Disagreements[0].Field != "comment" || got.Disagreements[0].Human == nil || got.Disagreements[0].Model == nil {
		t.Fatalf("disagreements %+v", got.Disagreements)
	}
	if final := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/coding", adminToken, `{"token":"`+got.Disagreements[0].Token+`","coder":"final","codes":[{"category":"a","onske":true}]}`); final.Code != 200 {
		t.Fatalf("decide %d %s", final.Code, final.Body)
	}
	if len(f.decisions) != 1 || f.decisions[0] != "comment admin@nav.no" {
		t.Errorf("decision log %v", f.decisions)
	}
	got, _ = getCoding(t, h)
	if len(got.Disagreements) != 0 {
		t.Errorf("decided, still listed: %+v", got.Disagreements)
	}
	var comment codedField
	for _, r := range got.Results {
		if r.ID == "comment" {
			comment = r
		}
	}
	// Three respondents wrote; both «sakte» are b, «mail meg» is a wish a.
	counts := map[string]any{}
	for _, c := range comment.Categories {
		counts[c.ID] = fmt.Sprint(deref(c.Count), "/", deref(c.Onske))
	}
	if comment.Written != 3 || comment.Coded != 3 || counts["a"] != "1/1" || counts["b"] != "2/0" {
		t.Errorf("results %+v %v", comment, counts)
	}
}

func TestCodingRefuses(t *testing.T) {
	api := adminAPI(t, freetextRowsFixture)
	withFakeCodes(api)
	h := makeRouter(testAuthenticator(t), api)
	got, _ := getCoding(t, h)
	post := func(body string) int {
		return do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/coding", adminToken, body).Code
	}
	tok := got.Next.Token
	for body, want := range map[string]int{
		`{"token":"` + tok + `","coder":"model","codes":[{"category":"a"}]}`:           400,
		`{"token":"` + tok + `","coder":"human","codes":[{"category":"zz"}]}`:          400,
		`{"token":"` + tok + `","coder":"final","codes":[{"category":"annet"}]}`:       409, // nobody coded yet
		`{"token":"nope","coder":"human","codes":[{"category":"annet"}]}`:              404,
		`{"token":"` + tok + `","coder":"human","codes":[],"extra":1}`:                 400,
		`{"token":"` + tok + `","coder":"human","codes":[{"category":"annet"}]} trail`: 400,
	} {
		if c := post(body); c != want {
			t.Errorf("%s: %d, want %d", body, c, want)
		}
	}
	if rec := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/coding/model", adminToken, `{"codebook_version":2,"codes":[]}`); rec.Code != 409 {
		t.Errorf("other codebook version: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/coding/model", adminToken, `{"codebook_version":1,"codes":[{"field":"comment","text":"x","codes":[{"category":"zz"}]}]}`); rec.Code != 400 {
		t.Errorf("unknown category: %d", rec.Code)
	}
	api.surveys[0].Series = "other"
	if rec := do(h, "GET", "/api/v1/admin/surveys/q4-2026/freetext/coding", adminToken, ""); rec.Code != 404 {
		t.Errorf("no codebook: %d", rec.Code)
	}
	for _, path := range []string{"/api/v1/admin/surveys/q4-2026/freetext/coding"} {
		if rec := do(h, "GET", path, webToken, ""); rec.Code != 404 {
			t.Errorf("non-admin: %d", rec.Code)
		}
	}
}

func TestShippedCodebooksLoad(t *testing.T) {
	cbs, err := loadCodebooks(surveyFiles)
	if err != nil || cbs["utviklerundersokelsen"].Version < 1 {
		t.Fatalf("%v %v", err, cbs)
	}
}

// TestCodingSurvivesPurge (COPILOT_SURVEY_TEST_DB_URL): purge writes the
// coding counts before the rows go, the codes and decisions go with the
// last row, the counts stay, and `aggregate` does not wipe them.
func TestCodingSurvivesPurge(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	ctx := t.Context()
	st, err := openSurveyStore(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.pool.Close()
	_, _ = st.pool.Exec(ctx, `DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results; DELETE FROM survey_freetext_codes; DELETE FROM survey_freetext_decisions; DELETE FROM survey_freetext_hidden`)
	defs, _ := loadSurveys([]byte(testSurveys))
	s := defs[0]
	s.Series, s.Active, s.Starts, s.Ends = "dev", false, "2026-01-01", "2026-01-31"
	st.defs, st.codebooks = []survey{s}, map[string]codebook{"dev": testCodebook}
	for i := range 6 {
		text := "rask"
		if i%3 == 0 {
			text = "treg"
		}
		a, _ := json.Marshal(map[string]any{"overall": 3, "comment": text})
		if _, err := st.pool.Exec(ctx, `INSERT INTO survey_answers (survey_id, answers, question_versions, context, delete_after) VALUES ($1, $2, '{}', '{}', '2026-03-01')`, s.ID, a); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		text, coder string
		codes       []code
	}{
		{"rask", "model", []code{{"a", false}}}, {"rask", "human", []code{{"a", false}}},
		{"treg", "model", []code{{"b", false}}}, {"treg", "human", []code{{"c", true}}}, {"treg", "final", []code{{"c", true}}},
	} {
		if err := st.saveCodes(ctx, s.ID, "comment", textHash(c.text), c.coder, 1, c.codes, "admin@nav.no"); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.saveCodes(ctx, s.ID, "comment", textHash("rask"), "human", 1, []code{{"b", false}}, ""); err != errCoded {
		t.Errorf("second human coding: %v", err)
	}
	api := &surveyAPI{surveys: []survey{s}, now: func() time.Time { return time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC) },
		answers: st.answers, final: st.final, hidden: st.hidden, hide: st.hide, textKey: []byte("k"),
		codebooks: st.codebooks, codes: st.codes, saveCodes: st.saveCodes, finalCoding: st.finalCoding}
	h := makeRouter(testAuthenticator(t), api)
	before, _ := getCoding(t, h)

	st.retain(ctx, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC))
	var rows, codes, decisions int
	_ = st.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM survey_answers), (SELECT count(*) FROM survey_freetext_codes), (SELECT count(*) FROM survey_freetext_decisions)`).Scan(&rows, &codes, &decisions)
	if rows != 0 || codes != 0 || decisions != 0 {
		t.Fatalf("left after purge: %d rows, %d codes, %d decisions", rows, codes, decisions)
	}
	after, _ := getCoding(t, h)
	a, _ := json.Marshal(before.Results)
	b, _ := json.Marshal(after.Results)
	if string(a) != string(b) || !strings.Contains(string(a), `"id":"a","name":"A","count":4`) {
		t.Errorf("counts changed with the rows\nbefore %s\nafter  %s", a, b)
	}
	// `aggregate` over rows left behind rewrites the answers, not the coding.
	ft := func() (n int) {
		_ = st.pool.QueryRow(ctx, `SELECT count(*) FROM survey_results WHERE kind LIKE 'freetext%'`).Scan(&n)
		return n
	}
	n := ft()
	if _, err := st.pool.Exec(ctx, `INSERT INTO survey_answers (survey_id, answers, question_versions, context, delete_after) VALUES ($1, '{"overall":1}', '{}', '{}', '2027-01-01')`, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := finalize(ctx, st.pool, st.defs, s, api.now(), true); err != nil {
		t.Fatal(err)
	}
	if n == 0 || ft() != n {
		t.Errorf("aggregate wiped the coding counts: %d, then %d", n, ft())
	}
}

// TestAnalyzeCodingMatchesOnline: the coding results from the admin CSV's
// codes columns, as `analyze` counts them, equal the coding page's.
func TestAnalyzeCodingMatchesOnline(t *testing.T) {
	var rows []storedAnswer
	for i := range 12 {
		text := []string{"rask", "treg", "=formel", "mail a@b.no", "skjult"}[i%5]
		rows = append(rows, row(`{"overall":3,"comment":"`+text+`","editor":["Annet"],"editor.other":"nano"}`, `{"overall":1,"comment":1,"editor":1}`))
	}
	rows = append(rows, row(`{"overall":2}`, `{"overall":1}`))
	api := adminAPI(t, rows)
	f := withFakeCodes(api)
	h := makeRouter(testAuthenticator(t), api)
	for _, c := range []struct {
		field, text  string
		model, human []code
		final        []code
	}{
		{"comment", "rask", []code{{"a", false}}, []code{{"a", false}}, nil},
		{"comment", "treg", []code{{"b", false}}, []code{{"c", true}}, []code{{"c", true}}},
		{"comment", "=formel", []code{{"annet", false}}, []code{{"d", false}, {"e", true}}, nil}, // undecided
		{"comment", "mail a@b.no", []code{{"a", true}}, nil, nil},                                // human not yet
		{"comment", "skjult", []code{{"a", false}}, []code{{"a", false}}, nil},
		{"editor.other", "nano", []code{{"a", false}}, []code{{"a", false}}, nil},
	} {
		f.codes[hiddenKey(c.field, textHash(c.text))] = &textCodes{Model: c.model, Human: c.human, Final: c.final}
	}
	_ = api.hide(t.Context(), "q4-2026", "comment", textHash("skjult"))
	online, _ := getCoding(t, h)

	rec := do(h, "GET", "/api/v1/admin/surveys/q4-2026/export.csv", adminToken, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "codes.comment.final,codes.comment.model,codes.comment.human") || strings.Contains(rec.Body.String(), "a@b.no") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got, err := readExportCSV(api.surveys[0], rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	local := codingResults(api.surveys[0], testCodebook, tallyAll(testCodebook, csvCodes(testCodebook, got)))
	a, _ := json.Marshal(online.Results)
	b, _ := json.Marshal(local)
	if string(a) != string(b) {
		t.Errorf("analyze differs\nonline %s\ncsv    %s", a, b)
	}
	// Control: 10 respondents wrote a visible comment, 6 of them coded.
	if !strings.Contains(string(a), `"id":"comment","question":"Anything else?","written":10,"coded":6`) {
		t.Errorf("results %s", a)
	}
}

// TestPublishedCoding: the published series carries a wave's coded fields
// once every text in them is final, published categories only, «<5» in the
// JSON and the CSV, and nothing an admin alone sees.
func TestPublishedCoding(t *testing.T) {
	defs, _ := loadSurveys([]byte(testSurveys))
	mk := func(id, starts, ends string) survey {
		s := defs[0]
		s.ID, s.Series, s.Published, s.Active, s.Starts, s.Ends = id, "dev", true, false, starts, ends
		return s
	}
	data := map[string][]storedAnswer{}
	for i := range 10 {
		text := []string{"rask", "rask", "treg", "rask", "annet svar"}[i%5]
		data["spring"] = append(data["spring"], row(`{"overall":3,"comment":"`+text+`"}`, `{"overall":1,"comment":1}`))
		data["autumn"] = append(data["autumn"], row(`{"overall":3,"comment":"`+text+`"}`, `{"overall":1,"comment":1}`))
	}
	codes := map[string]map[string]*textCodes{"spring": {}, "autumn": {}}
	for _, id := range []string{"spring", "autumn"} {
		codes[id][hiddenKey("comment", textHash("rask"))] = &textCodes{Model: []code{{"a", false}}, Human: []code{{"a", false}}}
		codes[id][hiddenKey("comment", textHash("treg"))] = &textCodes{Model: []code{{"b", true}}, Human: []code{{"b", true}}}
		codes[id][hiddenKey("comment", textHash("annet svar"))] = &textCodes{Model: []code{{"annet", false}}, Human: []code{{"annet", false}}}
	}
	codes["autumn"][hiddenKey("comment", textHash("treg"))].Human = []code{{"c", false}} // undecided
	api := &surveyAPI{
		surveys: []survey{mk("spring", "2026-03-01", "2026-03-31"), mk("autumn", "2026-09-01", "2026-09-30")},
		now:     func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) },
		answers: func(_ context.Context, id string) ([]storedAnswer, error) { return data[id], nil },
		hidden:  func(context.Context, string) (map[string]bool, error) { return map[string]bool{}, nil },
		codes: func(_ context.Context, id string, _ int) (map[string]*textCodes, error) {
			return codes[id], nil
		},
		codebooks: map[string]codebook{"dev": testCodebook},
	}
	h := makeRouter(testAuthenticator(t), api)
	rec := do(h, "GET", "/api/v1/surveys/published/compare?series=dev", webToken, "")
	var got compareResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(got.Freetext) != 1 || got.Freetext[0].Survey != "spring" || got.Freetext[0].Coders == "" {
		t.Fatalf("freetext %+v", got.Freetext)
	}
	f := got.Freetext[0].Fields
	if len(f) != 1 || f[0].ID != "comment" || f[0].Categories != nil {
		t.Fatalf("fields %+v", f)
	}
	var pub []string
	for _, c := range f[0].Published {
		pub = append(pub, fmt.Sprint(c.ID, ":", deref(c.Count), ":", c.Small))
	}
	// a 6 of 10 (kappa 1); b 2, small; annet 2 and kappa 1, small.
	if strings.Join(pub, " ") != "a:6:false b:<nil>:true annet:<nil>:true" {
		t.Errorf("published %v", pub)
	}
	rec = do(h, "GET", "/api/v1/surveys/published/compare?series=dev&format=csv", webToken, "")
	body := rec.Body.String()
	for _, want := range []string{"freetext,spring,comment,,1,false,a,6,10,", "freetext,spring,comment,,1,false,b,<5,10,", "freetext_onske,spring,comment,,1,false,a,0,10,"} {
		if !strings.Contains(body, want) {
			t.Errorf("CSV lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "autumn,comment") || strings.Contains(body, "rask") {
		t.Errorf("CSV has an unfinished wave or a text:\n%s", body)
	}
	// After the rows are deleted the stored counts give the same.
	stored := tallyAll(testCodebook, respondentCodes(testCodebook, data["spring"], nil, codes["spring"]))
	data["spring"] = nil
	api.finalCoding = func(context.Context, string, int) (map[string]fieldTally, error) { return stored, nil }
	wf, err := api.publishedCoding(t.Context(), api.surveys[0])
	x, _ := json.Marshal(got.Freetext[0])
	y, _ := json.Marshal(wf)
	if err != nil || string(x) != string(y) {
		t.Errorf("after deletion %v\nbefore %s\nafter  %s", err, x, y)
	}
}
