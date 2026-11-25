package api

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-survey/internal/store"
	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// Two waves. Spring showed the statements to everyone; autumn skips them
// for "none". "slow" changed version between the waves.
const compareSurveys = `[
{"id":"spring","title":"Vår","starts":"2026-03-01","ends":"2026-03-31","questions":[
 {"id":"tools","version":1,"type":"multi","text":"Tools?","options":["a","none"]},
 {"id":"grid","version":1,"type":"matrix","text":"Agree?","min":1,"max":5,
  "items":[{"id":"fast","version":1,"text":"Fast."},{"id":"slow","version":1,"text":"Slow."}]},
 {"id":"note","version":1,"type":"text","text":"?","max_length":9}]},
{"id":"autumn","title":"Høst","starts":"2026-10-01","ends":"2026-10-31","questions":[
 {"id":"tools","version":1,"type":"multi","text":"Tools?","options":["a","none"]},
 {"id":"grid","version":1,"type":"matrix","text":"Agree?","min":1,"max":5,"skip_if":{"question":"tools","answer":"none"},
  "items":[{"id":"fast","version":1,"text":"Fast."},{"id":"slow","version":2,"text":"Slow, reworded."}]},
 {"id":"note","version":1,"type":"text","text":"?","max_length":9}]}]`

// waveRows: spring 4 users (fast 5,5,4,2) + 2 non-users (fast 1);
// autumn 5 users (fast 3). Spring from Forms, autumn 3 web + 2 copilot.
func waveRows() map[string][]survey.StoredAnswer {
	out := map[string][]survey.StoredAnswer{}
	for _, f := range []string{"5", "5", "4", "2"} {
		r := row(`{"tools":["a"],"fast":`+f+`,"slow":3,"note":"x"}`, `{"tools":1,"fast":1,"slow":1,"note":1}`)
		r.Client, r.Context = "forms", map[string]any{"client": "forms"}
		out["spring"] = append(out["spring"], r)
	}
	for range 2 {
		r := row(`{"tools":["none"],"fast":1,"slow":1}`, `{"tools":1,"fast":1,"slow":1}`)
		r.Client, r.Context = "forms", map[string]any{"client": "forms"}
		out["spring"] = append(out["spring"], r)
	}
	for i := range 5 {
		r := row(`{"tools":["a"],"fast":3,"slow":4}`, `{"tools":1,"fast":1,"slow":2}`)
		r.Client = map[bool]string{true: "web", false: "copilot"}[i < 3]
		r.Context = map[string]any{"client": r.Client}
		out["autumn"] = append(out["autumn"], r)
	}
	return out
}

func compareAPI(t *testing.T, read func(context.Context, string) ([]survey.StoredAnswer, error)) *surveyAPI {
	t.Helper()
	defs, err := loadSurveys([]byte(compareSurveys))
	if err != nil {
		t.Fatal(err)
	}
	return &surveyAPI{surveys: defs, now: func() time.Time { return time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) }, store: &funcStore{answers: read}}
}

func getCompare(t *testing.T, api *surveyAPI, ids string) survey.CompareResult {
	t.Helper()
	rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/admin/surveys/compare?ids="+ids, adminToken, "")
	var got survey.CompareResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return got
}

func checkCompare(t *testing.T, got survey.CompareResult) {
	t.Helper()
	if len(got.Surveys) != 2 || got.Surveys[0].ID != "spring" || got.Surveys[0].Channels["forms"] != 6 ||
		len(got.Surveys[1].Channels) != 1 || got.Surveys[1].Channels["annet"] != 5 { // web 3 and copilot 2 merged as in the export
		t.Fatalf("meta %+v", got.Surveys)
	}
	if len(got.Questions) != 2 {
		t.Fatalf("text question not left out: %+v", got.Questions)
	}
	tools, grid := got.Questions[0], got.Questions[1]
	if tools.Break || *tools.Waves[0].Excluded != 0 || *tools.Waves[0].Versions[0].N != 6 {
		t.Errorf("tools %+v", tools)
	}
	fast, slow := grid.Items[0], grid.Items[1]
	// Spring: the two non-users are dropped by autumn's skip_if.
	s := fast.Waves[0]
	if fast.Break || *s.Excluded != 2 || *s.Versions[0].N != 4 || *s.Versions[0].Mean != 4 || *s.Versions[0].Agree != 0.75 {
		t.Errorf("fast spring %+v mean %v agree %v", s, *s.Versions[0].Mean, *s.Versions[0].Agree)
	}
	if a := fast.Waves[1]; *a.Excluded != 0 || *a.Versions[0].N != 5 || *a.Versions[0].Mean != 3 || *a.Versions[0].Agree != 0 {
		t.Errorf("fast autumn %+v", a)
	}
	if !slow.Break || slow.Waves[0].Versions[0].Version != 1 || slow.Waves[1].Versions[0].Version != 2 {
		t.Errorf("slow: no break %+v", slow)
	}
	// fast is the only item tested (slow has a break), so Holm leaves its p.
	if fast.P == nil || math.Abs(*fast.P-0.17752985241215352) > 1e-12 || *fast.PHolm != *fast.P {
		t.Errorf("fast test %+v", fast.Significance)
	}
	if slow.P != nil || slow.PHolm != nil {
		t.Errorf("slow tested across a break: %+v", slow.Significance)
	}
}

func TestCompare(t *testing.T) {
	data := waveRows()
	api := compareAPI(t, func(_ context.Context, id string) ([]survey.StoredAnswer, error) { return data[id], nil })
	checkCompare(t, getCompare(t, api, "spring,autumn"))

	// Admin JSON always carries excluded, also when it is 0.
	rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/admin/surveys/compare?ids=spring,autumn", adminToken, "")
	if !strings.Contains(rec.Body.String(), `"excluded":0`) {
		t.Errorf("admin excluded 0 omitted: %s", rec.Body)
	}

	// Control: without autumn's skip_if, spring keeps its non-users.
	rule := api.surveys[1].Questions[1].SkipIf
	api.surveys[1].Questions[1].SkipIf = nil
	if s := getCompare(t, api, "spring,autumn").Questions[1].Items[0].Waves[0]; *s.Excluded != 0 || *s.Versions[0].N != 6 {
		t.Errorf("control: %+v", s)
	}
	api.surveys[1].Questions[1].SkipIf = rule

	// Each wave's own scale: spring on 1–6 counts agree at 5 and 6 (5,5 of 4).
	api.surveys[0].Questions[1].Max = 6
	if s := getCompare(t, api, "spring,autumn").Questions[1].Items[0].Waves[0].Versions[0]; *s.Agree != 0.5 {
		t.Errorf("spring agree on its own scale: %v", *s.Agree)
	}
	api.surveys[0].Questions[1].Max = 5

	// A declared version change is a break even before anyone answered it.
	api.surveys[0].Questions[1].Items[0].Version = 3
	if !getCompare(t, api, "spring,autumn").Questions[1].Items[0].Break {
		t.Error("declared version change: no break")
	}
	api.surveys[0].Questions[1].Items[0].Version = 1

	// A wave under five answers shows nothing but its total.
	data["autumn"] = data["autumn"][:4]
	got := getCompare(t, api, "spring,autumn")
	if !got.Surveys[1].Suppressed || got.Surveys[1].Channels != nil || len(got.Questions[0].Waves[1].Versions) != 0 || got.Surveys[1].Responses != 4 {
		t.Errorf("small wave %+v %+v", got.Surveys[1], got.Questions[0].Waves[1])
	}

	h := makeRouter(testAuthenticator(t), api)
	for ids, want := range map[string]int{"": 400, "spring": 400, "spring,nope": 404, "spring,spring": 404} {
		if rec := do(h, "GET", "/api/v1/admin/surveys/compare?ids="+ids, adminToken, ""); rec.Code != want {
			t.Errorf("ids=%q: %d, want %d", ids, rec.Code, want)
		}
	}
}

// TestCompareStore runs the same comparison on rows read back from Postgres
// (COPILOT_SURVEY_TEST_DB_URL): jsonb arrays, context.client from the
// import and from submissions.
func TestCompareStore(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	s, err := store.Open(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Pool.Close()
	_, _ = s.Pool.Exec(t.Context(), `DELETE FROM survey_participation; DELETE FROM survey_answers`)
	api := compareAPI(t, s.Answers)
	data := waveRows()
	var spring []map[string]any
	for _, r := range data["spring"] {
		spring = append(spring, r.Answers)
	}
	if _, err := store.WriteImport(t.Context(), url, api.surveys[0], spring, time.Now().AddDate(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	for i, r := range data["autumn"] {
		resp := survey.Response{SurveyID: "autumn", Answers: r.Answers, QuestionVersions: r.QuestionVersions,
			Context: survey.TechContext{Version: "web", OS: "linux", Client: r.Client}, DeleteAfter: time.Now().AddDate(1, 0, 0)}
		if _, err := s.Submit(t.Context(), "autumn", string(rune('a'+i)), time.Now().AddDate(0, 0, 1), resp); err != nil {
			t.Fatal(err)
		}
	}
	checkCompare(t, getCompare(t, api, "spring,autumn"))
}

// compareBodies is the comparison of ids as JSON and as CSV, byte for byte.
func compareBodies(t *testing.T, api *surveyAPI, ids string) (string, string) {
	t.Helper()
	h := makeRouter(testAuthenticator(t), api)
	j := do(h, "GET", "/api/v1/admin/surveys/compare?ids="+ids, adminToken, "")
	c := do(h, "GET", "/api/v1/admin/surveys/compare?ids="+ids+"&format=csv", adminToken, "")
	if j.Code != 200 || c.Code != 200 {
		t.Fatalf("compare: %d %s / %d %s", j.Code, j.Body, c.Code, c.Body)
	}
	return j.Body.String(), c.Body.String()
}

// Once a wave's rows are deleted, compare reads its stored tally and gives
// the same JSON and CSV; a skip rule the stored tally lacks is refused.
func TestCompareAfterRowsDeleted(t *testing.T) {
	data := waveRows()
	api := compareAPI(t, func(_ context.Context, id string) ([]survey.StoredAnswer, error) { return data[id], nil })
	api.now = func() time.Time { return time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC) }
	stored := map[string]survey.WaveTally{}
	for _, s := range api.surveys {
		stored[s.ID] = survey.RowsTally(s, survey.MergeClients(data[s.ID]), survey.SkipSets(api.surveys))
	}
	fake(api).finalCompare = func(_ context.Context, id string) (*survey.WaveTally, error) {
		t := stored[id]
		return &t, nil
	}
	wantJSON, wantCSV := compareBodies(t, api, "spring,autumn")
	delete(data, "spring")
	if j, c := compareBodies(t, api, "spring,autumn"); j != wantJSON || c != wantCSV {
		t.Fatalf("after spring's rows are deleted:\n%s\nwant\n%s", j, wantJSON)
	}
	delete(data, "autumn")
	if j, c := compareBodies(t, api, "spring,autumn"); j != wantJSON || c != wantCSV {
		t.Fatalf("after both waves' rows are deleted:\n%s\nwant\n%s", c, wantCSV)
	}
	var res survey.CompareResult
	_ = json.Unmarshal([]byte(wantJSON), &res)
	checkCompare(t, res)

	// A later definition adds a statement under the same skip_if. The
	// stored tallies predate it, but no wave answered it: nothing to
	// exclude, same output before and after the rows are deleted.
	grid := &api.surveys[1].Questions[1]
	grid.Items = append(slices.Clone(grid.Items), survey.MatrixItem{ID: "new", Version: 1, Text: "New."})
	data = waveRows()
	wantJSON, wantCSV = compareBodies(t, api, "spring,autumn")
	data = map[string][]survey.StoredAnswer{}
	if j, c := compareBodies(t, api, "spring,autumn"); j != wantJSON || c != wantCSV {
		t.Fatalf("new statement after deletion:\n%s\nwant\n%s", j, wantJSON)
	}

	api.surveys[1].Questions[1].SkipIf = survey.SkipRules{{Question: "tools", Answer: "a"}}
	if rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/admin/surveys/compare?ids=spring,autumn", adminToken, ""); rec.Code != 409 {
		t.Fatalf("skip rule missing from the stored tally: %d %s", rec.Code, rec.Body)
	}
}

// TestCompareStoreAfterRetention is TestCompareAfterRowsDeleted on Postgres
// (COPILOT_SURVEY_TEST_DB_URL): close-out stores the tallies, a skip rule
// defined later is added while the rows exist, and compare gives the same
// JSON and CSV after the rows are deleted.
func TestCompareStoreAfterRetention(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	ctx := t.Context()
	s, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Pool.Close()
	_, _ = s.Pool.Exec(ctx, `DELETE FROM survey_participation; DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results`)
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	api := compareAPI(t, s.Answers)
	api.now, fake(api).finalCompare = func() time.Time { return now }, s.FinalCompare
	data := waveRows()
	var spring []map[string]any
	for _, r := range data["spring"] {
		spring = append(spring, r.Answers)
	}
	if _, err := store.WriteImport(ctx, url, api.surveys[0], spring, now.AddDate(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	for i, r := range data["autumn"] {
		resp := survey.Response{SurveyID: "autumn", Answers: r.Answers, QuestionVersions: r.QuestionVersions,
			Context: survey.TechContext{Version: "web", OS: "linux", Client: r.Client}, DeleteAfter: now.AddDate(1, 0, 0)}
		if _, err := s.Submit(ctx, "autumn", string(rune('a'+i)), now, resp); err != nil {
			t.Fatal(err)
		}
	}
	wantJSON, wantCSV := compareBodies(t, api, "spring,autumn")
	var res survey.CompareResult
	_ = json.Unmarshal([]byte(wantJSON), &res)
	checkCompare(t, res)

	// Close-out before autumn's skip_if is known, then again once it is.
	autumn := api.surveys[1]
	autumn.Questions = slices.Clone(autumn.Questions)
	autumn.Questions[1].SkipIf = nil
	s.Defs = []survey.Survey{api.surveys[0], autumn}
	s.CloseOut(ctx, now)
	var rules int
	_ = s.Pool.QueryRow(ctx, `SELECT count(*) FROM survey_skip_results`).Scan(&rules)
	if rules != 0 {
		t.Fatalf("skip counts without a skip rule: %d rows", rules)
	}
	s.Defs = api.surveys
	s.CloseOut(ctx, now)

	for _, id := range []string{"spring", "autumn"} {
		if _, err := s.Pool.Exec(ctx, `DELETE FROM survey_answers WHERE survey_id = $1`, id); err != nil {
			t.Fatal(err)
		}
		if j, c := compareBodies(t, api, "spring,autumn"); j != wantJSON || c != wantCSV {
			t.Fatalf("after %s's rows are deleted:\n%s\nwant\n%s\n%s\nwant\n%s", id, j, wantJSON, c, wantCSV)
		}
	}
	// Close-out with no rows left leaves the stored counts alone.
	s.CloseOut(ctx, now)
	if j, _ := compareBodies(t, api, "spring,autumn"); j != wantJSON {
		t.Fatalf("close-out without rows changed the stored counts:\n%s", j)
	}
}

// TestImportInvalidatesAggregates (COPILOT_SURVEY_TEST_DB_URL): a corrected
// import with as many rows drops the wave's aggregates and skip counts, so
// close-out counts the new rows, and compare after deletion matches them.
func TestImportInvalidatesAggregates(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	ctx := t.Context()
	st, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Pool.Close()
	_, _ = st.Pool.Exec(ctx, `DELETE FROM survey_participation; DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results`)
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	api := compareAPI(t, st.Answers)
	api.now, fake(api).finalCompare = func() time.Time { return now }, st.FinalCompare
	st.Defs = api.surveys
	importSpring := func(fast string) []survey.StoredAnswer {
		var answers []map[string]any
		var rows []survey.StoredAnswer
		for _, r := range waveRows()["spring"] {
			if r.Answers["tools"].([]any)[0] == "a" {
				r.Answers["fast"] = json.Number(fast)
			}
			b, _ := json.Marshal(r.Answers)
			var a map[string]any
			_ = json.Unmarshal(b, &a)
			answers = append(answers, a)
			r.Answers, r.Context = a, map[string]any{"client": "forms"}
			rows = append(rows, r)
		}
		if _, err := store.WriteImport(ctx, url, api.surveys[0], answers, now.AddDate(1, 0, 0)); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	autumn := waveRows()["autumn"]
	for i, r := range autumn {
		resp := survey.Response{SurveyID: "autumn", Answers: r.Answers, QuestionVersions: r.QuestionVersions,
			Context: survey.TechContext{Version: "web", OS: "linux", Client: r.Client}, DeleteAfter: now.AddDate(1, 0, 0)}
		if _, err := st.Submit(ctx, "autumn", string(rune('a'+i)), now, resp); err != nil {
			t.Fatal(err)
		}
	}
	importSpring("5")
	st.CloseOut(ctx, now)
	corrected := importSpring("2")
	var left int
	_ = st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM survey_results WHERE survey_id = 'spring') + (SELECT count(*) FROM survey_skip_results WHERE survey_id = 'spring')`).Scan(&left)
	if left != 0 {
		t.Fatalf("import left %d aggregate rows for spring", left)
	}
	st.CloseOut(ctx, now)
	want := survey.CompareWavesResult(api.surveys, [][]survey.StoredAnswer{survey.MergeClients(corrected), survey.MergeClients(autumn)}, now)
	_, _ = st.Pool.Exec(ctx, `DELETE FROM survey_answers WHERE survey_id = 'spring'`)
	got := getCompare(t, api, "spring,autumn")
	wj, _ := json.Marshal(want)
	gj, _ := json.Marshal(got)
	if string(wj) != string(gj) {
		t.Fatalf("after corrected import and deletion:\n%s\nwant\n%s", gj, wj)
	}
	if s := got.Questions[1].Items[0].Waves[0]; *s.Excluded != 2 || *s.Versions[0].Mean != 2 {
		t.Fatalf("spring fast: %+v", s)
	}
}

// An id asked as free text in an earlier wave is not counted as the later
// wave's question: those answers are someone's text, not options.
func TestCompareTypeChange(t *testing.T) {
	data := waveRows()
	for i := range data["spring"] {
		data["spring"][i].Answers["tools"] = "a"
	}
	api := compareAPI(t, func(_ context.Context, id string) ([]survey.StoredAnswer, error) { return data[id], nil })
	api.surveys[0].Questions[0].Type, api.surveys[0].Questions[0].Options = "text", nil
	tools := getCompare(t, api, "spring,autumn").Questions[0]
	if !tools.Break || len(tools.Waves[0].Versions) != 0 || *tools.Waves[1].Versions[0].N != 5 ||
		tools.Waves[0].Excluded == nil || *tools.Waves[0].Excluded != 0 { // admin always says excluded
		t.Errorf("free text counted: %+v", tools)
	}
}

// Autumn skips fast for a = x or b = p, as a skip_if list in the definition.
// Spring asked fast of everyone: one respondent matches both rules, one each
// matches only one, five match neither. The union drops 3, not 4.
const listSkipSurveys = `[
{"id":"lspring","title":"Vår","starts":"2026-03-01","ends":"2026-03-31","questions":[
 {"id":"a","version":1,"type":"choice","text":"A?","options":["x","y"]},
 {"id":"b","version":1,"type":"choice","text":"B?","options":["p","q"]},
 {"id":"fast","version":1,"type":"scale","text":"Fast?","min":1,"max":5}]},
{"id":"lautumn","title":"Høst","starts":"2026-10-01","ends":"2026-10-31","questions":[
 {"id":"a","version":1,"type":"choice","text":"A?","options":["x","y"]},
 {"id":"b","version":1,"type":"choice","text":"B?","options":["p","q"]},
 {"id":"fast","version":1,"type":"scale","text":"Fast?","min":1,"max":5,
  "skip_if":[{"question":"b","answer":"p"},{"question":"a","answer":"x"}]}]}]`

func listSkipRows() map[string][]survey.StoredAnswer {
	out := map[string][]survey.StoredAnswer{}
	add := func(id, answers string) {
		r := row(answers, `{"a":1,"b":1,"fast":1}`)
		r.Client, r.Context = "web", map[string]any{"client": "web"}
		out[id] = append(out[id], r)
	}
	for _, a := range []string{`{"a":"x","b":"p","fast":1}`, `{"a":"x","b":"q","fast":1}`, `{"a":"y","b":"p","fast":2}`,
		`{"a":"y","b":"q","fast":2}`, `{"a":"y","b":"q","fast":3}`, `{"a":"y","b":"q","fast":4}`, `{"a":"y","b":"q","fast":5}`, `{"a":"y","b":"q","fast":5}`} {
		add("lspring", a)
	}
	for range 5 {
		add("lautumn", `{"a":"y","b":"q","fast":3}`)
	}
	add("lautumn", `{"a":"x","b":"q"}`)
	return out
}

func listSkipAPI(t *testing.T, read func(context.Context, string) ([]survey.StoredAnswer, error)) *surveyAPI {
	t.Helper()
	defs, err := loadSurveys([]byte(listSkipSurveys))
	if err != nil {
		t.Fatal(err)
	}
	if len(defs[1].Questions[2].SkipIf) != 2 {
		t.Fatalf("skip_if list not read: %+v", defs[1].Questions[2].SkipIf)
	}
	return &surveyAPI{surveys: defs, now: func() time.Time { return time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC) }, store: &funcStore{answers: read}}
}

func checkListSkip(t *testing.T, body string) {
	t.Helper()
	var res survey.CompareResult
	if err := json.Unmarshal([]byte(body), &res); err != nil {
		t.Fatal(err)
	}
	fast := res.Questions[2]
	if s := fast.Waves[0]; fast.ID != "fast" || *s.Excluded != 3 || *s.Versions[0].N != 5 || *s.Versions[0].Mean != 3.8 {
		t.Fatalf("spring fast: union of the skip_if list not applied once: %+v", s)
	}
	if fast.P == nil {
		t.Fatalf("fast not tested: %+v", fast)
	}
}

// TestCompareSkipIfList: the union, and the same JSON and CSV before and
// after the rows are deleted (stored tallies as close-out writes them).
func TestCompareSkipIfList(t *testing.T) {
	data := listSkipRows()
	api := listSkipAPI(t, func(_ context.Context, id string) ([]survey.StoredAnswer, error) { return data[id], nil })
	stored := map[string]survey.WaveTally{}
	for _, s := range api.surveys {
		stored[s.ID] = survey.RowsTally(s, survey.MergeClients(data[s.ID]), survey.SkipSets(api.surveys))
	}
	fake(api).finalCompare = func(_ context.Context, id string) (*survey.WaveTally, error) {
		t := stored[id]
		return &t, nil
	}
	wantJSON, wantCSV := compareBodies(t, api, "lspring,lautumn")
	checkListSkip(t, wantJSON)
	data = map[string][]survey.StoredAnswer{}
	if j, c := compareBodies(t, api, "lspring,lautumn"); j != wantJSON || c != wantCSV {
		t.Fatalf("after deletion:\n%s\nwant\n%s", c, wantCSV)
	}
}

// TestCompareSkipIfListStore is TestCompareSkipIfList on Postgres
// (COPILOT_SURVEY_TEST_DB_URL), with close-out writing the tallies.
func TestCompareSkipIfListStore(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	ctx := t.Context()
	st, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Pool.Close()
	_, _ = st.Pool.Exec(ctx, `DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results`)
	api := listSkipAPI(t, st.Answers)
	fake(api).finalCompare, st.Defs = st.FinalCompare, api.surveys
	for id, rows := range listSkipRows() {
		for _, r := range rows {
			aj, _ := json.Marshal(r.Answers)
			vj, _ := json.Marshal(r.QuestionVersions)
			cj, _ := json.Marshal(r.Context)
			if _, err = st.Pool.Exec(ctx, `INSERT INTO survey_answers VALUES ($1, $2, $3, $4, '2027-01-01')`, id, aj, vj, cj); err != nil {
				t.Fatal(err)
			}
		}
	}
	st.CloseOut(ctx, api.now())
	wantJSON, wantCSV := compareBodies(t, api, "lspring,lautumn")
	checkListSkip(t, wantJSON)
	if _, err = st.Pool.Exec(ctx, `DELETE FROM survey_answers`); err != nil {
		t.Fatal(err)
	}
	if j, c := compareBodies(t, api, "lspring,lautumn"); j != wantJSON || c != wantCSV {
		t.Fatalf("after deletion:\n%s\nwant\n%s", c, wantCSV)
	}
}
