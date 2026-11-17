package main

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"slices"
	"testing"
	"time"
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
func waveRows() map[string][]storedAnswer {
	out := map[string][]storedAnswer{}
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

func compareAPI(t *testing.T, read func(context.Context, string) ([]storedAnswer, error)) *surveyAPI {
	t.Helper()
	defs, err := loadSurveys([]byte(compareSurveys))
	if err != nil {
		t.Fatal(err)
	}
	return &surveyAPI{surveys: defs, answers: read, now: func() time.Time { return time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) }}
}

func getCompare(t *testing.T, api *surveyAPI, ids string) compareResult {
	t.Helper()
	rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/admin/surveys/compare?ids="+ids, adminToken, "")
	var got compareResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return got
}

func checkCompare(t *testing.T, got compareResult) {
	t.Helper()
	if len(got.Surveys) != 2 || got.Surveys[0].ID != "spring" || got.Surveys[0].Channels["forms"] != 6 ||
		len(got.Surveys[1].Channels) != 1 || got.Surveys[1].Channels["annet"] != 5 { // web 3 and copilot 2 merged as in the export
		t.Fatalf("meta %+v", got.Surveys)
	}
	if len(got.Questions) != 2 {
		t.Fatalf("text question not left out: %+v", got.Questions)
	}
	tools, grid := got.Questions[0], got.Questions[1]
	if tools.Break || tools.Waves[0].Excluded != 0 || *tools.Waves[0].Versions[0].N != 6 {
		t.Errorf("tools %+v", tools)
	}
	fast, slow := grid.Items[0], grid.Items[1]
	// Spring: the two non-users are dropped by autumn's skip_if.
	s := fast.Waves[0]
	if fast.Break || s.Excluded != 2 || *s.Versions[0].N != 4 || *s.Versions[0].Mean != 4 || *s.Versions[0].Agree != 0.75 {
		t.Errorf("fast spring %+v mean %v agree %v", s, *s.Versions[0].Mean, *s.Versions[0].Agree)
	}
	if a := fast.Waves[1]; a.Excluded != 0 || *a.Versions[0].N != 5 || *a.Versions[0].Mean != 3 || *a.Versions[0].Agree != 0 {
		t.Errorf("fast autumn %+v", a)
	}
	if !slow.Break || slow.Waves[0].Versions[0].Version != 1 || slow.Waves[1].Versions[0].Version != 2 {
		t.Errorf("slow: no break %+v", slow)
	}
	// fast is the only item tested (slow has a break), so Holm leaves its p.
	if fast.P == nil || math.Abs(*fast.P-0.17752985241215352) > 1e-12 || *fast.PHolm != *fast.P {
		t.Errorf("fast test %+v", fast.significance)
	}
	if slow.P != nil || slow.PHolm != nil {
		t.Errorf("slow tested across a break: %+v", slow.significance)
	}
}

func TestCompare(t *testing.T) {
	data := waveRows()
	api := compareAPI(t, func(_ context.Context, id string) ([]storedAnswer, error) { return data[id], nil })
	checkCompare(t, getCompare(t, api, "spring,autumn"))

	// Control: without autumn's skip_if, spring keeps its non-users.
	rule := api.surveys[1].Questions[1].SkipIf
	api.surveys[1].Questions[1].SkipIf = nil
	if s := getCompare(t, api, "spring,autumn").Questions[1].Items[0].Waves[0]; s.Excluded != 0 || *s.Versions[0].N != 6 {
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
	s, err := openSurveyStore(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.pool.Close()
	_, _ = s.pool.Exec(t.Context(), `DELETE FROM survey_participation; DELETE FROM survey_answers`)
	api := compareAPI(t, s.answers)
	data := waveRows()
	var spring []map[string]any
	for _, r := range data["spring"] {
		spring = append(spring, r.Answers)
	}
	if _, err := writeImport(t.Context(), url, api.surveys[0], spring, time.Now().AddDate(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	for i, r := range data["autumn"] {
		resp := response{SurveyID: "autumn", Answers: r.Answers, QuestionVersions: r.QuestionVersions,
			Context: techContext{Version: "web", OS: "linux", Client: r.Client}, DeleteAfter: time.Now().AddDate(1, 0, 0)}
		if _, err := s.submit(t.Context(), "autumn", string(rune('a'+i)), time.Now().AddDate(0, 0, 1), resp); err != nil {
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
	api := compareAPI(t, func(_ context.Context, id string) ([]storedAnswer, error) { return data[id], nil })
	api.now = func() time.Time { return time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC) }
	stored := map[string]waveTally{}
	for _, s := range api.surveys {
		stored[s.ID] = rowsTally(s, mergeClients(data[s.ID]), skipKeys(api.surveys))
	}
	api.finalCompare = func(_ context.Context, id string) (*waveTally, error) {
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
	var res compareResult
	_ = json.Unmarshal([]byte(wantJSON), &res)
	checkCompare(t, res)

	api.surveys[1].Questions[1].SkipIf = &skipRule{Question: "tools", Answer: "a"}
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
	s, err := openSurveyStore(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.pool.Close()
	_, _ = s.pool.Exec(ctx, `DELETE FROM survey_participation; DELETE FROM survey_answers; DELETE FROM survey_results; DELETE FROM survey_skip_results`)
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	api := compareAPI(t, s.answers)
	api.now, api.finalCompare = func() time.Time { return now }, s.finalCompare
	data := waveRows()
	var spring []map[string]any
	for _, r := range data["spring"] {
		spring = append(spring, r.Answers)
	}
	if _, err := writeImport(ctx, url, api.surveys[0], spring, now.AddDate(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	for i, r := range data["autumn"] {
		resp := response{SurveyID: "autumn", Answers: r.Answers, QuestionVersions: r.QuestionVersions,
			Context: techContext{Version: "web", OS: "linux", Client: r.Client}, DeleteAfter: now.AddDate(1, 0, 0)}
		if _, err := s.submit(ctx, "autumn", string(rune('a'+i)), now, resp); err != nil {
			t.Fatal(err)
		}
	}
	wantJSON, wantCSV := compareBodies(t, api, "spring,autumn")
	var res compareResult
	_ = json.Unmarshal([]byte(wantJSON), &res)
	checkCompare(t, res)

	// Close-out before autumn's skip_if is known, then again once it is.
	autumn := api.surveys[1]
	autumn.Questions = slices.Clone(autumn.Questions)
	autumn.Questions[1].SkipIf = nil
	s.defs = []survey{api.surveys[0], autumn}
	s.closeOut(ctx, now)
	var rules int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM survey_skip_results`).Scan(&rules)
	if rules != 0 {
		t.Fatalf("skip counts without a skip rule: %d rows", rules)
	}
	s.defs = api.surveys
	s.closeOut(ctx, now)

	for _, id := range []string{"spring", "autumn"} {
		if _, err := s.pool.Exec(ctx, `DELETE FROM survey_answers WHERE survey_id = $1`, id); err != nil {
			t.Fatal(err)
		}
		if j, c := compareBodies(t, api, "spring,autumn"); j != wantJSON || c != wantCSV {
			t.Fatalf("after %s's rows are deleted:\n%s\nwant\n%s\n%s\nwant\n%s", id, j, wantJSON, c, wantCSV)
		}
	}
	// Close-out with no rows left leaves the stored counts alone.
	s.closeOut(ctx, now)
	if j, _ := compareBodies(t, api, "spring,autumn"); j != wantJSON {
		t.Fatalf("close-out without rows changed the stored counts:\n%s", j)
	}
}
