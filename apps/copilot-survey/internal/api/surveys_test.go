package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

const testSurveys = `[{"id":"q4-2026","title":"Q4","active":true,"starts":"2026-10-01","ends":"2026-11-30","questions":[
 {"id":"overall","version":1,"type":"scale","text":"How useful?","min":1,"max":5,"required":true},
 {"id":"client","version":1,"type":"choice","text":"Which client?","options":["copilot","opencode"]},
 {"id":"tools","version":1,"type":"multi","text":"Which tools?","options":["a","b","c"],"max_choices":2},
 {"id":"why","version":1,"type":"choice","text":"Why not copilot?","options":["habit","other"],"skip_if":{"question":"client","answer":"copilot"}},
 {"id":"editor","version":1,"type":"multi","text":"Which editors?","options":["vim","vscode"],"other":"Annet","max_length":10},
 {"id":"grid","version":1,"type":"matrix","text":"How much do you agree?","min":1,"max":5,"unsure":"Vet ikke","skip_if":{"question":"client","answer":"opencode"},
  "items":[{"id":"fast","version":2,"text":"It is fast."},{"id":"safe","version":1,"text":"It is safe.","reverse":true}]},
 {"id":"comment","version":1,"type":"text","text":"Anything else?","max_length":20}]},
 {"id":"old","title":"Old","active":true,"starts":"2025-01-01","ends":"2025-01-31","questions":[{"id":"a","version":1,"type":"scale","text":"?","min":1,"max":3}]}]`

func loadSurveys(raw []byte) ([]survey.Survey, error) {
	var surveys []survey.Survey
	if err := survey.DecodeStrict(raw, &surveys); err != nil {
		return nil, err
	}
	return surveys, survey.ValidateSurveys(surveys)
}

// fakeStore is the two tables: participation by hash, answers with nothing.
type fakeStore struct {
	participants map[string]bool
	answers      []survey.Response
}

// fakeEmails stands in for the SAML lookup in copilot-api: hans on GitHub is
// the same person as the web test user.
func fakeEmails(_ context.Context, c *caller) (string, error) {
	switch {
	case c.email != "":
		return c.email, nil
	case c.login == "hans":
		return "hans.test@NAV.no ", nil
	case c.login == "guest":
		return "guest@example.com", nil
	}
	return "", errNoNavIdentity
}

func testRouter(t *testing.T) (http.Handler, *fakeStore) {
	defs, err := loadSurveys([]byte(testSurveys))
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{participants: map[string]bool{}}
	api := &surveyAPI{
		surveys:  defs,
		keys:     map[string][]byte{"q4-2026": []byte("0123456789abcdef0123456789abcdef")},
		emailFor: fakeEmails,
		now:      func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		store: &funcStore{submit: store.submit, answered: func(_ context.Context, surveyID, hash string) (bool, error) {
			return store.participants[surveyID+"/"+hash], nil
		}}}
	return makeRouter(testAuthenticator(t), api), store
}

// do sends a request; a token of the form "cli:<login>" is copilot-cli's app
// token with X-On-Behalf-Of: <login>.
func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if login, ok := strings.CutPrefix(token, "cli:"); ok {
		token = cliToken
		req.Header.Set("X-On-Behalf-Of", login)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const goodCtx = `"context":{"version":"1.42.0","os":"darwin","client":"copilot","local_models":true}`

func TestActiveSurveysArePublic(t *testing.T) {
	h, _ := testRouter(t)
	rec := do(h, "GET", "/api/v1/surveys/active", "", "")
	var got struct{ Surveys []survey.Survey }
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil || len(got.Surveys) != 1 || got.Surveys[0].ID != "q4-2026" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestSubmit(t *testing.T) {
	h, store := testRouter(t)
	good := `{"answers":{"overall":4,"client":"opencode","tools":["c","a"],"editor":["vim","Annet"],"editor.other":" Zed ","comment":"  fine  "},` + goodCtx + `}`
	path := "/api/v1/surveys/q4-2026/responses"

	for _, tc := range []struct {
		name, token, path, body string
		want                    int
	}{
		{"no token", "", path, good, 401},
		{"nav-pilot user via copilot-cli", "cli:hans", path, good, 201},
		{"same nav-pilot user again", "cli:hans", path, good, 409},
		{"same person on the web", webToken, path, good, 409},
		{"github member without a Nav identity", "cli:nosso", path, good, 403},
		{"closed survey", "cli:hans", "/api/v1/surveys/old/responses", `{"answers":{"a":1},` + goodCtx + `}`, 404},
		{"unknown survey", "cli:hans", "/api/v1/surveys/nope/responses", good, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rec := do(h, "POST", tc.path, tc.token, tc.body); rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}
	if len(store.answers) != 1 || len(store.participants) != 1 {
		t.Fatalf("answers = %d, participants = %d, want 1 and 1", len(store.answers), len(store.participants))
	}
	r := store.answers[0]
	if r.Answers["comment"] != "fine" || r.Answers["grid"] != nil || r.QuestionVersions["grid"] != 0 || r.Answers["editor.other"] != "Zed" || r.QuestionVersions["editor.other"] != 0 || r.QuestionVersions["overall"] != 1 || r.QuestionVersions["why"] != 0 {
		t.Fatalf("stored answers: %+v", r)
	}
	if !r.DeleteAfter.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC).Add(survey.Retention)) {
		t.Fatalf("delete_after = %v", r.DeleteAfter)
	}
	if r.Context.Version != "1.42" {
		t.Fatalf("version kept as %q, want coarsened to 1.42", r.Context.Version)
	}
	for k := range store.participants {
		if strings.Contains(strings.ToLower(k), "hans") || len(k) != len("q4-2026/")+64 {
			t.Fatalf("participation key %q", k)
		}
	}
}

func TestStatus(t *testing.T) {
	h, _ := testRouter(t)
	status := func(token, id string) (int, string) {
		rec := do(h, "GET", "/api/v1/surveys/"+id+"/status", token, "")
		if rec.Code == 200 && rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("Cache-Control = %q, want no-store on a per-user answer", rec.Header().Get("Cache-Control"))
		}
		return rec.Code, strings.TrimSpace(rec.Body.String())
	}
	if code, body := status("cli:hans", "q4-2026"); code != 200 || body != `{"answered":false}` {
		t.Fatalf("before answering: %d %s", code, body)
	}
	good := `{"answers":{"overall":4,"client":"opencode"},` + goodCtx + `}`
	if rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", "cli:hans", good); rec.Code != 201 {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body)
	}
	for _, tc := range []struct {
		name, token, id string
		want            int
		body            string
	}{
		{"same user", "cli:hans", "q4-2026", 200, `{"answered":true}`},
		{"same person on the web", webToken, "q4-2026", 200, `{"answered":true}`},
		{"someone else", "cli:guest", "q4-2026", 200, `{"answered":false}`},
		{"no token", "", "q4-2026", 401, ""},
		{"no Nav identity", "cli:nosso", "q4-2026", 403, ""},
		{"closed survey", "cli:hans", "old", 404, ""},
		{"unknown survey", "cli:hans", "nope", 404, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body := status(tc.token, tc.id)
			if code != tc.want || (tc.body != "" && body != tc.body) {
				t.Fatalf("%d %s, want %d %s", code, body, tc.want, tc.body)
			}
		})
	}
}

func TestSubmitValidation(t *testing.T) {
	h, store := testRouter(t)
	path := "/api/v1/surveys/q4-2026/responses"
	for name, body := range map[string]string{
		"not json":           `{`,
		"trailing data":      `{"answers":{"overall":3},` + goodCtx + `}{}`,
		"stray delimiter":    `{"answers":{"overall":3},` + goodCtx + `}]`,
		"unknown top field":  `{"answers":{"overall":3},"device_id":"abc",` + goodCtx + `}`,
		"unknown ctx field":  `{"answers":{"overall":3},"context":{"version":"1.0.0","os":"darwin","client":"copilot","cwd":"/x"}}`,
		"missing required":   `{"answers":{"client":"copilot"},` + goodCtx + `}`,
		"scale out of range": `{"answers":{"overall":6},` + goodCtx + `}`,
		"scale not integer":  `{"answers":{"overall":3.5},` + goodCtx + `}`,
		"choice not option":  `{"answers":{"overall":3,"client":"vim"},` + goodCtx + `}`,
		"text too long":      `{"answers":{"overall":3,"comment":"` + strings.Repeat("x", 21) + `"},` + goodCtx + `}`,
		"unknown question":   `{"answers":{"overall":3,"extra":1},` + goodCtx + `}`,
		"multi not a list":   `{"answers":{"overall":3,"tools":"a"},` + goodCtx + `}`,
		"multi empty":        `{"answers":{"overall":3,"tools":[]},` + goodCtx + `}`,
		"multi twice":        `{"answers":{"overall":3,"tools":["a","a"]},` + goodCtx + `}`,
		"multi not option":   `{"answers":{"overall":3,"tools":["a","z"]},` + goodCtx + `}`,
		"multi too many":     `{"answers":{"overall":3,"tools":["a","b","c"]},` + goodCtx + `}`,
		"other not chosen":   `{"answers":{"overall":3,"editor":["vim"],"editor.other":"Zed"},` + goodCtx + `}`,
		"other alone":        `{"answers":{"overall":3,"editor.other":"Zed"},` + goodCtx + `}`,
		"other too long":     `{"answers":{"overall":3,"editor":["Annet"],"editor.other":"` + strings.Repeat("x", 11) + `"},` + goodCtx + `}`,
		"text not utf-8":     "{\"answers\":{\"overall\":3,\"comment\":\"a\xffb\"}," + goodCtx + "}",
		"other not text":     `{"answers":{"overall":3,"editor":["Annet"],"editor.other":7},` + goodCtx + `}`,
		"other not offered":  `{"answers":{"overall":3,"client":"copilot","client.other":"x"},` + goodCtx + `}`,
		"matrix as one":      `{"answers":{"overall":3,"grid":4},` + goodCtx + `}`,
		"matrix item range":  `{"answers":{"overall":3,"fast":6},` + goodCtx + `}`,
		"matrix skipped":     `{"answers":{"overall":3,"client":"opencode","fast":4},` + goodCtx + `}`,
		"skipped answered":   `{"answers":{"overall":3,"client":"copilot","why":"habit"},` + goodCtx + `}`,
		"bad os":             `{"answers":{"overall":3},"context":{"version":"1.0.0","os":"plan9","client":"copilot"}}`,
		"version is text":    `{"answers":{"overall":3},"context":{"version":"my repo","os":"darwin","client":"copilot"}}`,
		"too big":            `{"answers":{"comment":"` + strings.Repeat("x", 40<<10) + `"},` + goodCtx + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			if rec := do(h, "POST", path, "cli:hans", body); rec.Code != 400 {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body)
			}
		})
	}
	if len(store.answers) != 0 || len(store.participants) != 0 {
		t.Fatalf("an invalid submission was stored")
	}
}

func TestInactiveSurveyIsNotServed(t *testing.T) {
	defs, _ := loadSurveys([]byte(strings.Replace(testSurveys, `"active":true,`, "", 1)))
	api := &surveyAPI{surveys: defs, now: func() time.Time { return time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) }}
	h := makeRouter(testAuthenticator(t), api)
	if rec := do(h, "GET", "/api/v1/surveys/active", "", ""); strings.Contains(rec.Body.String(), "q4-2026") {
		t.Fatalf("inactive survey served: %s", rec.Body)
	}
	if rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", "cli:hans", `{}`); rec.Code != 404 {
		t.Fatalf("inactive survey took a submission: %d", rec.Code)
	}
}

func TestSurveyWithoutKeyIsNotServed(t *testing.T) {
	defs, _ := loadSurveys([]byte(testSurveys))
	api := &surveyAPI{surveys: defs, now: func() time.Time { return time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) }}
	h := makeRouter(testAuthenticator(t), api)
	if rec := do(h, "GET", "/api/v1/surveys/active", "", ""); strings.Contains(rec.Body.String(), "q4-2026") {
		t.Fatalf("survey without a key served: %s", rec.Body)
	}
	if rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", "cli:hans", `{}`); rec.Code != 404 {
		t.Fatalf("survey without a key: status = %d, want 404", rec.Code)
	}
}

func TestSubmitWithoutStorage(t *testing.T) {
	defs, _ := loadSurveys([]byte(testSurveys))
	api := &surveyAPI{surveys: defs, keys: map[string][]byte{"q4-2026": bytes.Repeat([]byte{7}, 32)}, now: func() time.Time { return time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) }}
	h := makeRouter(testAuthenticator(t), api)
	if rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", "cli:hans", `{}`); rec.Code != 503 {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestWrongMethod(t *testing.T) {
	h, _ := testRouter(t)
	if rec := do(h, "GET", "/api/v1/surveys/q4-2026/responses", "cli:hans", ""); rec.Code != 405 {
		t.Fatalf("GET responses = %d, want 405", rec.Code)
	}
}

// A matrix item is stored as a scale question of its own: its id, its
// version, a number. A required matrix needs every item.
func TestMatrixItemsStoredAsScales(t *testing.T) {
	defs, err := loadSurveys([]byte(testSurveys))
	if err != nil {
		t.Fatal(err)
	}
	s := defs[0]
	got, err := survey.ValidateAnswers(s, map[string]json.RawMessage{"overall": []byte("3"), "fast": []byte("4")})
	if err != nil || got["fast"] != 4 || got["safe"] != nil {
		t.Fatalf("%v %v", got, err)
	}
	h, store := testRouter(t)
	rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", "cli:hans", `{"answers":{"overall":3,"fast":4,"safe":2},`+goodCtx+`}`)
	if rec.Code != 201 || store.answers[0].QuestionVersions["fast"] != 2 || store.answers[0].QuestionVersions["safe"] != 1 || store.answers[0].Answers["safe"] != 2 {
		t.Fatalf("%d %+v", rec.Code, store.answers)
	}
	i := slices.IndexFunc(s.Questions, func(q survey.Question) bool { return q.ID == "grid" })
	s.Questions[i].Required, s.Questions[i].SkipIf = true, nil
	if _, err := survey.ValidateAnswers(s, map[string]json.RawMessage{"overall": []byte("3"), "fast": []byte("4")}); err == nil {
		t.Fatal("required matrix took one item of two")
	}
}

func TestMinCLIVersion(t *testing.T) {
	defs, err := loadSurveys([]byte(strings.Replace(testSurveys, `"title":"Q4",`, `"title":"Q4","min_cli_version":"2026.09.29-072249",`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	a := &surveyAPI{surveys: defs, keys: map[string][]byte{"q4-2026": bytes.Repeat([]byte{7}, 32)}, now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) }}
	for name, tc := range map[string]struct {
		query, ua string
		served    bool
	}{
		"web":                        {"", "node", true},
		"web, no user agent":         {"", "", true},
		"nav-pilot, same":            {"?client=nav-pilot", "nav-pilot/2026.09.29-072249-08895fb", true},
		"nav-pilot, newer":           {"?client=nav-pilot", "nav-pilot/2026.09.29-073815-8801b8d", true},
		"nav-pilot, next day":        {"?client=nav-pilot", "nav-pilot/2026.09.30", true},
		"nav-pilot, older":           {"?client=nav-pilot", "nav-pilot/2026.09.29-071203-c69719c", false},
		"nav-pilot, dev build":       {"?client=nav-pilot", "nav-pilot/dev", false},
		"nav-pilot, no version sent": {"?client=nav-pilot", "Go-http-client/1.1", false},
	} {
		req := httptest.NewRequest("GET", "/api/v1/surveys/active"+tc.query, nil)
		req.Header.Set("User-Agent", tc.ua)
		rec := httptest.NewRecorder()
		a.active(rec, req)
		var got struct{ Surveys []survey.Survey }
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || (len(got.Surveys) == 1) != tc.served {
			t.Errorf("%s: %s", name, rec.Body)
		}
	}
	for _, bad := range []string{"2026.9.29", "v2026.09.29", "2026.09.29-0722", "latest", "2026.13.40", "2026.09.29-256199"} {
		if _, err := loadSurveys([]byte(strings.Replace(testSurveys, `"title":"Q4",`, `"title":"Q4","min_cli_version":"`+bad+`",`, 1))); err == nil {
			t.Errorf("min_cli_version %q accepted", bad)
		}
	}
}

// A nav-pilot submission whose nameId is not @nav.no is counted, nothing more.
func TestUnexpectedNameIDCounted(t *testing.T) {
	h, _ := testRouter(t)
	before := testutil.ToFloat64(unexpectedNameIDs)
	body := `{"answers":{"overall":4},` + goodCtx + `}`
	for _, token := range []string{"cli:hans", "cli:guest"} {
		if rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", token, body); rec.Code != 201 {
			t.Fatalf("%s: %d %s", token, rec.Code, rec.Body)
		}
	}
	if got := testutil.ToFloat64(unexpectedNameIDs) - before; got != 1 {
		t.Fatalf("counted %v, want 1", got)
	}
}

// A store failure is counted under status 503, the series the
// CopilotSurveySubmissionFailures alert reads; a stored answer is not.
func TestSubmitFailureCounted(t *testing.T) {
	h, _ := testRouter(t)
	body := `{"answers":{"overall":4},` + goodCtx + `}`
	c503 := submissions.WithLabelValues("q4-2026", "503")
	c201 := submissions.WithLabelValues("q4-2026", "201")
	b503, b201 := testutil.ToFloat64(c503), testutil.ToFloat64(c201)
	if rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", "cli:hans", body); rec.Code != 201 {
		t.Fatalf("ok submit: %d", rec.Code)
	}
	if testutil.ToFloat64(c503) != b503 || testutil.ToFloat64(c201) != b201+1 {
		t.Fatal("success counted wrong")
	}

	defs, _ := loadSurveys([]byte(testSurveys))
	api := &surveyAPI{
		surveys:  defs,
		keys:     map[string][]byte{"q4-2026": []byte("0123456789abcdef0123456789abcdef")},
		emailFor: fakeEmails,
		now:      func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		store: &funcStore{submit: func(context.Context, string, string, time.Time, survey.Response) (bool, error) {
			return false, errors.New("db down")
		}}}
	rec := do(makeRouter(testAuthenticator(t), api), "POST", "/api/v1/surveys/q4-2026/responses", "cli:hans", body)
	if rec.Code != 503 {
		t.Fatalf("failing store: %d", rec.Code)
	}
	if got := testutil.ToFloat64(c503) - b503; got != 1 {
		t.Fatalf("503 counted %v, want 1", got)
	}
}
func (f *fakeStore) submit(_ context.Context, surveyID, hash string, _ time.Time, r survey.Response) (bool, error) {
	k := surveyID + "/" + hash
	if f.participants[k] {
		return false, nil
	}
	f.participants[k] = true
	f.answers = append(f.answers, r)
	return true, nil
}
