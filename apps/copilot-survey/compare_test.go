package main

import (
	"context"
	"encoding/json"
	"os"
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
		r.Client = "forms"
		out["spring"] = append(out["spring"], r)
	}
	for range 2 {
		r := row(`{"tools":["none"],"fast":1,"slow":1}`, `{"tools":1,"fast":1,"slow":1}`)
		r.Client = "forms"
		out["spring"] = append(out["spring"], r)
	}
	for i := range 5 {
		r := row(`{"tools":["a"],"fast":3,"slow":4}`, `{"tools":1,"fast":1,"slow":2}`)
		r.Client = map[bool]string{true: "web", false: "copilot"}[i < 3]
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
		got.Surveys[1].Channels["web"] != 3 || got.Surveys[1].Channels["copilot"] != 2 {
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
}

func TestCompare(t *testing.T) {
	data := waveRows()
	api := compareAPI(t, func(_ context.Context, id string) ([]storedAnswer, error) { return data[id], nil })
	checkCompare(t, getCompare(t, api, "spring,autumn"))

	// Control: without autumn's skip_if, spring keeps its non-users.
	api.surveys[1].Questions[1].SkipIf = nil
	if s := getCompare(t, api, "spring,autumn").Questions[1].Items[0].Waves[0]; s.Excluded != 0 || *s.Versions[0].N != 6 {
		t.Errorf("control: %+v", s)
	}

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
	s.flush(t.Context())
	checkCompare(t, getCompare(t, api, "spring,autumn"))
}
