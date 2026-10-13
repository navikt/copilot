package main

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func adminAPI(t *testing.T, rows []storedAnswer) *surveyAPI {
	t.Helper()
	defs, err := loadSurveys([]byte(testSurveys))
	if err != nil {
		t.Fatal(err)
	}
	return &surveyAPI{
		surveys: defs,
		answers: func(context.Context, string) ([]storedAnswer, error) { return rows, nil },
		now:     func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
	}
}

func row(answers, versions string) storedAnswer {
	var a storedAnswer
	if json.Unmarshal([]byte(`{"answers":`+answers+`,"question_versions":`+versions+`}`), &a) != nil {
		panic(answers)
	}
	return a
}

func TestAdminRoutesNeedAdmin(t *testing.T) {
	h := makeRouter(testAuthenticator(t), adminAPI(t, nil))
	for _, path := range []string{"/api/v1/admin/surveys", "/api/v1/admin/surveys/q4-2026/results", "/api/v1/admin/anything"} {
		for _, tc := range []struct {
			token string
			want  int
		}{
			{"", 401},
			{webToken, 404},   // my-copilot user, not in the group
			{"cli:hans", 404}, // copilot-cli app token
			{cliAdmin, 401},   // admin user, but the token is not from my-copilot
		} {
			if rec := do(h, "GET", path, tc.token, ""); rec.Code != tc.want {
				t.Errorf("%s %q: %d, want %d", path, tc.token, rec.Code, tc.want)
			}
		}
	}
	if rec := do(h, "GET", "/api/v1/admin/surveys", adminToken, ""); rec.Code != 200 {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body)
	}
}

func TestAdminGroupUnsetFailsClosed(t *testing.T) {
	a := testAuthenticator(t)
	a.adminGroup = ""
	if rec := do(makeRouter(a, adminAPI(t, nil)), "GET", "/api/v1/admin/surveys", adminToken, ""); rec.Code != 404 {
		t.Fatalf("%d", rec.Code)
	}
}

func TestListStatus(t *testing.T) {
	api := adminAPI(t, []storedAnswer{{}, {}})
	api.surveys = append(api.surveys,
		survey{ID: "next", Title: "Next", Active: true, Starts: "2027-01-01", Ends: "2027-01-31"},
		survey{ID: "off", Title: "Off", Starts: "2026-10-01", Ends: "2026-10-31"})
	rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/admin/surveys", adminToken, "")
	var got struct{ Surveys []surveySummary }
	if json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatal(rec.Body)
	}
	status := map[string]string{}
	for _, s := range got.Surveys {
		status[s.ID] = s.Status
		if s.Responses != 2 {
			t.Errorf("%s: %d responses", s.ID, s.Responses)
		}
	}
	for id, st := range map[string]string{"q4-2026": "åpen", "old": "stengt", "next": "kommende", "off": "stengt"} {
		if status[id] != st {
			t.Errorf("%s: %q, want %q", id, status[id], st)
		}
	}
}

func getResults(t *testing.T, rows []storedAnswer) (surveyResults, string) {
	t.Helper()
	rec := do(makeRouter(testAuthenticator(t), adminAPI(t, rows)), "GET", "/api/v1/admin/surveys/q4-2026/results", adminToken, "")
	var got surveyResults
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return got, rec.Body.String()
}

func TestResultsBelowFiveShowNothing(t *testing.T) {
	var rows []storedAnswer
	for range 4 {
		rows = append(rows, row(`{"overall":3}`, `{"overall":1}`))
	}
	got, _ := getResults(t, rows)
	if !got.Suppressed || got.Questions != nil || got.Responses != 4 {
		t.Fatalf("%+v", got)
	}
}

func TestResultsRealCountsAndSplit(t *testing.T) {
	// overall: 6×5, 1×4. tools (multi): 7 people, a×7, b×2.
	// fast: 4 on version 1, 6 on version 2. Every count is real.
	var rows []storedAnswer
	for i := range 7 {
		overall, tools, ver := 5, `["a"]`, 2
		if i == 0 {
			overall = 4
		}
		if i < 2 {
			tools = `["a","b"]`
		}
		if i < 4 {
			ver = 1
		}
		rows = append(rows, row(
			`{"overall":`+strconv.Itoa(overall)+`,"client":"copilot","tools":`+tools+`,"fast":3,"comment":"secret name"}`,
			`{"overall":1,"client":1,"tools":1,"fast":`+strconv.Itoa(ver)+`,"comment":1}`))
	}
	for range 3 {
		rows = append(rows, row(`{"fast":3}`, `{"fast":2}`))
	}
	got, body := getResults(t, rows)
	if strings.Contains(body, "secret") || strings.Contains(body, `"comment"`) {
		t.Fatalf("free text leaked: %s", body)
	}
	q := map[string]questionResult{}
	for _, r := range got.Questions {
		q[r.ID] = r
	}
	for _, o := range q["overall"].Versions[0].Options {
		want := map[string]int{"4": 1, "5": 6}[o.Value]
		if o.Suppressed || o.Count == nil || *o.Count != want {
			t.Errorf("overall %+v", o)
		}
	}
	tools := q["tools"].Versions[0]
	if *tools.N != 7 || *tools.Options[0].Count != 7 || tools.Options[1].Suppressed || *tools.Options[1].Count != 2 || *tools.Options[2].Count != 0 {
		t.Errorf("tools %+v", tools)
	}
	fast := q["grid"].Items[0]
	if fast.ID != "fast" || len(fast.Versions) != 2 || fast.Versions[0].Version != 1 || fast.Versions[0].N == nil || *fast.Versions[0].N != 4 || *fast.Versions[0].Options[0].Count != 4 ||
		*fast.Versions[1].N != 6 || len(fast.Versions[1].Options) != 5 || *fast.Versions[1].Options[2].Count != 6 {
		t.Errorf("fast %+v", fast)
	}
}
