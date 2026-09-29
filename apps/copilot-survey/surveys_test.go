package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

const testSurveys = `[{"id":"q4-2026","title":"Q4","active":true,"starts":"2026-10-01","ends":"2026-11-30","questions":[
 {"id":"overall","version":1,"type":"scale","text":"How useful?","min":1,"max":5,"required":true},
 {"id":"client","version":1,"type":"choice","text":"Which client?","options":["copilot","opencode"]},
 {"id":"tools","version":1,"type":"multi","text":"Which tools?","options":["a","b","c"],"max_choices":2},
 {"id":"why","version":1,"type":"choice","text":"Why not copilot?","options":["habit","other"],"skip_if":{"question":"client","answer":"copilot"}},
 {"id":"editor","version":1,"type":"multi","text":"Which editors?","options":["vim","vscode"],"other":"Annet","max_length":10},
 {"id":"grid","version":1,"type":"matrix","text":"How much do you agree?","min":1,"max":5,"skip_if":{"question":"client","answer":"opencode"},
  "items":[{"id":"fast","version":2,"text":"It is fast."},{"id":"safe","version":1,"text":"It is safe.","reverse":true}]},
 {"id":"comment","version":1,"type":"text","text":"Anything else?","max_length":20}]},
 {"id":"old","title":"Old","active":true,"starts":"2025-01-01","ends":"2025-01-31","questions":[{"id":"a","version":1,"type":"scale","text":"?","min":1,"max":3}]}]`

// fakeStore is the two tables: participation by hash, answers with nothing.
type fakeStore struct {
	participants map[string]bool
	answers      []response
}

func (f *fakeStore) submit(_ context.Context, surveyID, hash string, _ time.Time, r response) (bool, error) {
	k := surveyID + "/" + hash
	if f.participants[k] {
		return false, nil
	}
	f.participants[k] = true
	f.answers = append(f.answers, r)
	return true, nil
}

// fakeEmails stands in for the SAML lookup in copilot-api: hans on GitHub is
// the same person as the web test user.
func fakeEmails(_ context.Context, c *caller) (string, error) {
	switch {
	case c.email != "":
		return c.email, nil
	case c.login == "hans":
		return "hans.test@NAV.no ", nil
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
		store:    store.submit,
		now:      func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
	}
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
	var got struct{ Surveys []survey }
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
	if !r.DeleteAfter.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC).Add(retention)) {
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

func TestSubmitWithoutStorage(t *testing.T) {
	defs, _ := loadSurveys([]byte(testSurveys))
	api := &surveyAPI{surveys: defs, now: func() time.Time { return time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) }}
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

func TestParticipantHash(t *testing.T) {
	k1 := []byte("0123456789abcdef0123456789abcdef")
	k2 := []byte("another key another key another k")
	if participantHash(k1, "Ola.Nordmann@nav.no ") != participantHash(k1, "ola.nordmann@NAV.no") {
		t.Fatal("e-mail is not normalised")
	}
	if participantHash(k1, "a@nav.no") == participantHash(k2, "a@nav.no") {
		t.Fatal("hash does not depend on the survey key")
	}
}

func TestLoadSurveysRejectsBadDefinitions(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown field":     `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}],"x":1}]`,
		"ends first":        `[{"id":"a","title":"t","starts":"2026-01-02","ends":"2026-01-01","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
		"bad type":          `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"essay","text":"?"}]}]`,
		"text no limit":     `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"text","text":"?"}]}]`,
		"one option":        `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x"]}]}]`,
		"duplicate id":      `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]},{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
		"no version":        `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","type":"scale","text":"?","min":1,"max":5}]}]`,
		"labels count":      `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5,"labels":["a","b"]}]}]`,
		"skip_if later":     `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"text","text":"?","max_length":5,"skip_if":{"question":"r","answer":"x"}},{"id":"r","version":1,"type":"choice","text":"?","options":["x","y"]}]}]`,
		"skip_if scale":     `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"r","version":1,"type":"scale","text":"?","min":1,"max":5,"options":["x"]},{"id":"q","version":1,"type":"text","text":"?","max_length":5,"skip_if":{"question":"r","answer":"x"}}]}]`,
		"stray ]":           `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]]`,
		"bad nudge":         `[{"id":"a","title":"t","nudge":"loud","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
		"two texts":         `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"text","text":"?","max_length":5},{"id":"r","version":1,"type":"text","text":"?","max_length":5}]}]`,
		"other on scale":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5,"other":"Annet","max_length":5}]}]`,
		"other is option":   `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x","Annet"],"other":"Annet","max_length":5}]}]`,
		"other no limit":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet"}]}]`,
		"other too long":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":201}]}]`,
		"four others":       `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"a","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":5},{"id":"b","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":5},{"id":"c","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":5},{"id":"d","version":1,"type":"choice","text":"?","options":["x","y"],"other":"Annet","max_length":5}]}]`,
		"limit no other":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x","y"],"max_length":5}]}]`,
		"matrix one item":   `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"matrix","text":"?","min":1,"max":5,"items":[{"id":"i","version":1,"text":"?"}]}]}]`,
		"matrix item id":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"matrix","text":"?","min":1,"max":5,"items":[{"id":"q","version":1,"text":"?"},{"id":"i","version":1,"text":"?"}]}]}]`,
		"matrix item twice": `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"i","version":1,"type":"scale","text":"?","min":1,"max":5},{"id":"q","version":1,"type":"matrix","text":"?","min":1,"max":5,"items":[{"id":"i","version":1,"text":"?"},{"id":"j","version":1,"text":"?"}]}]}]`,
		"matrix no version": `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"matrix","text":"?","min":1,"max":5,"items":[{"id":"i","text":"?"},{"id":"j","version":1,"text":"?"}]}]}]`,
		"items on scale":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5,"items":[{"id":"i","version":1,"text":"?"},{"id":"j","version":1,"text":"?"}]}]}]`,
		"id with slash":     `[{"id":"a/b","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
	} {
		if _, err := loadSurveys([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSurveyKeysOnlyForOpenSurveys(t *testing.T) {
	defs, _ := loadSurveys([]byte(testSurveys))
	testKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	t.Setenv("SURVEY_KEY_Q4_2026", testKey)
	t.Setenv("SURVEY_KEY_OLD", testKey)
	keys := surveyKeys(defs, time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if len(keys["q4-2026"]) != 32 || keys["old"] != nil {
		t.Fatalf("keys: %v", keys)
	}
}

// TestShippedSurveys is the CI check of the definitions in surveys/.
func TestShippedSurveys(t *testing.T) {
	if _, err := loadSurveyDir(surveyFiles); err != nil {
		t.Fatal(err)
	}
}

// TestActivateInDevOnly: SURVEY_ACTIVE_IDS opens a survey in dev-gcp and
// nowhere else, and the shipped dummy is inactive on its own.
func TestActivateInDevOnly(t *testing.T) {
	defs, err := loadSurveyDir(surveyFiles)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(defs, func(s survey) bool { return s.ID == "dev-e2e-test" })
	if i < 0 || defs[i].Active {
		t.Fatal("dev-e2e-test must ship, inactive")
	}
	for _, cluster := range []string{"prod-gcp", "local", ""} {
		activateInDev(defs, cluster, "dev-e2e-test")
		if defs[i].Active {
			t.Fatalf("activated in %q", cluster)
		}
	}
	activateInDev(defs, "dev-gcp", " dev-e2e-test ,nope")
	if !defs[i].Active {
		t.Fatal("not activated in dev-gcp")
	}
}

func TestContextKeepsOnlyCoarseVersion(t *testing.T) {
	for in, want := range map[string]string{"2026.09.24-120000-abc1234": "2026.09", "v1.2.3": "1.2", "dev": "dev", "web": "web"} {
		c := techContext{Version: in, OS: "linux", Client: "pi"}
		if err := c.validate(); err != nil || c.Version != want {
			t.Errorf("%s: got %q, %v; want %q", in, c.Version, err, want)
		}
	}
}

func TestOtherCountsTowardMaxChoices(t *testing.T) {
	if _, err := loadSurveys([]byte(`[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"multi","text":"?","options":["x","y"],"max_choices":3,"other":"Annet","max_length":5}]}]`)); err != nil {
		t.Fatal(err)
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
	got, err := validateAnswers(s, map[string]json.RawMessage{"overall": []byte("3"), "fast": []byte("4")})
	if err != nil || got["fast"] != 4 || got["safe"] != nil {
		t.Fatalf("%v %v", got, err)
	}
	h, store := testRouter(t)
	rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", "cli:hans", `{"answers":{"overall":3,"fast":4,"safe":2},`+goodCtx+`}`)
	if rec.Code != 201 || store.answers[0].QuestionVersions["fast"] != 2 || store.answers[0].QuestionVersions["safe"] != 1 || store.answers[0].Answers["safe"] != 2 {
		t.Fatalf("%d %+v", rec.Code, store.answers)
	}
	i := slices.IndexFunc(s.Questions, func(q question) bool { return q.ID == "grid" })
	s.Questions[i].Required, s.Questions[i].SkipIf = true, nil
	if _, err := validateAnswers(s, map[string]json.RawMessage{"overall": []byte("3"), "fast": []byte("4")}); err == nil {
		t.Fatal("required matrix took one item of two")
	}
}
