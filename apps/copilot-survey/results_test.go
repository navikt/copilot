package main

import (
	"context"
	"encoding/json"
	"slices"
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

// TestCellsHideDeterminableGroups: a hidden cell must not follow from n and
// the shown cells. The result names the options that end up hidden.
func TestCellsHideDeterminableGroups(t *testing.T) {
	q := question{Type: "choice", Version: 1, Options: []string{"a", "b", "c", "d"}}
	hidden := func(q question, counts map[string]int) string {
		var s string
		for _, o := range cells(q, 1, counts) {
			if o.Suppressed {
				s += o.Value
			}
		}
		return s
	}
	for name, tc := range map[string]struct {
		counts map[string]int
		want   string
	}{
		"one small":   {map[string]int{"a": 1, "b": 7, "c": 9}, "ab"},
		"all ones":    {map[string]int{"a": 1, "b": 1, "c": 9}, "abc"},
		"all fours":   {map[string]int{"a": 4, "b": 4, "c": 9}, "abc"},
		"mixed small": {map[string]int{"a": 1, "b": 3, "c": 9}, "ab"},
		"none small":  {map[string]int{"a": 5, "b": 9}, ""},
	} {
		if got := hidden(q, tc.counts); got != tc.want {
			t.Errorf("%s: hid %q, want %q", name, got, tc.want)
		}
	}
	// Every answer in a small cell: no cell left to hide, so nothing is shown.
	for name, counts := range map[string]map[string]int{"two fours": {"a": 4, "b": 4}, "five ones": {"a": 1, "b": 1, "c": 1, "d": 1, "e": 1}} {
		if cells(q, 1, counts) != nil {
			t.Errorf("%s: shown, want nil", name)
		}
	}
	q.Type, q.MaxChoices = "multi", 1
	if got := hidden(q, map[string]int{"a": 1, "b": 7}); got != "ab" {
		t.Errorf("multi with one choice: hid %q, want ab", got)
	}
	q.MaxChoices = 0
	if got := hidden(q, map[string]int{"a": 1, "b": 7}); got != "a" {
		t.Errorf("multi: hid %q, want a", got)
	}
}

// TestSkippedQuestionHidesSourceCount: grid is skipped for client=opencode, so
// its n is the number who chose copilot. With opencode at 2 (hidden, and
// copilot hidden with it) grid must show nothing; with both shown it may.
func TestSkippedQuestionHidesSourceCount(t *testing.T) {
	rows := func(opencode int) []storedAnswer {
		var out []storedAnswer
		for i := range 12 {
			client := `"copilot","fast":3,"safe":3`
			if i < opencode {
				client = `"opencode"`
			}
			out = append(out, row(`{"overall":3,"client":`+client+`}`, `{"overall":1,"client":1,"fast":2,"safe":1}`))
		}
		return out
	}
	for opencode, want := range map[int]bool{2: false, 5: true} {
		got, _ := getResults(t, rows(opencode))
		i := slices.IndexFunc(got.Questions, func(q questionResult) bool { return q.ID == "grid" })
		for _, it := range got.Questions[i].Items {
			if shown := it.Versions[0].N != nil && it.Versions[0].Options != nil; shown != want {
				t.Errorf("opencode=%d: %s shown=%v, want %v", opencode, it.ID, shown, want)
			}
		}
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

func TestResultsSuppressAndSplit(t *testing.T) {
	// overall: 6×5, 1×4: 4 is hidden, and 5 too (it is n minus the rest).
	// tools (multi): 7 people, a×7, b×2: only b hidden.
	// fast: 4 on version 1 (n<5: neither n nor a distribution), 6 on version 2.
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
		if (o.Value == "4" || o.Value == "5") != o.Suppressed || (o.Suppressed && o.Count != nil) {
			t.Errorf("overall %+v", o)
		}
	}
	tools := q["tools"].Versions[0]
	if *tools.N != 7 || *tools.Options[0].Count != 7 || !tools.Options[1].Suppressed || *tools.Options[2].Count != 0 {
		t.Errorf("tools %+v", tools)
	}
	fast := q["grid"].Items[0]
	if fast.ID != "fast" || len(fast.Versions) != 2 || fast.Versions[0].Version != 1 || fast.Versions[0].N != nil || fast.Versions[0].Options != nil ||
		*fast.Versions[1].N != 6 || len(fast.Versions[1].Options) != 5 || *fast.Versions[1].Options[2].Count != 6 {
		t.Errorf("fast %+v", fast)
	}
}
