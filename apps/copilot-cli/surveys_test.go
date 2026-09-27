package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testSurveys = `[{"id":"q4-2026","title":"Q4","active":true,"starts":"2026-10-01","ends":"2026-11-30","questions":[
 {"id":"overall","version":1,"type":"scale","text":"How useful?","min":1,"max":5,"required":true},
 {"id":"client","version":1,"type":"choice","text":"Which client?","options":["copilot","opencode"]},
 {"id":"tools","version":1,"type":"multi","text":"Which tools?","options":["a","b","c"],"max_choices":2},
 {"id":"why","version":1,"type":"choice","text":"Why not copilot?","options":["habit","other"],"skip_if":{"question":"client","answer":"copilot"}},
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

// fakeEmails stands in for the SAML lookup: hans on GitHub is the same person
// as the Entra test user.
func fakeEmails(_ context.Context, u *AuthenticatedUser) (string, error) {
	switch {
	case u.Issuer == issuerEntra:
		return u.email, nil
	case u.Login == "hans":
		return "hans.test@NAV.no ", nil
	}
	return "", errNoNavIdentity
}

func testRouter(t *testing.T) (http.Handler, *fakeStore) {
	defs, err := loadSurveys([]byte(testSurveys))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := testAuthenticator(t)
	store := &fakeStore{participants: map[string]bool{}}
	api := &surveyAPI{
		surveys:  defs,
		keys:     map[string][]byte{"q4-2026": []byte("0123456789abcdef0123456789abcdef")},
		emailFor: fakeEmails,
		store:    store.submit,
		now:      func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
	}
	return makeRouter(a, newCopilotAPIProxy("http://unused.invalid", newTexasClient("", "")), api), store
}

func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
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
	good := `{"answers":{"overall":4,"client":"opencode","tools":["c","a"],"comment":"  fine  "},` + goodCtx + `}`
	path := "/api/v1/surveys/q4-2026/responses"

	for _, tc := range []struct {
		name, token, path, body string
		want                    int
	}{
		{"no token", "", path, good, 401},
		{"github user", "good-token", path, good, 201},
		{"same github user again", "good-token", path, good, 409},
		{"same person on the web", entraToken, path, good, 409},
		{"github member without a Nav identity", "nosso-token", path, good, 403},
		{"closed survey", "good-token", "/api/v1/surveys/old/responses", `{"answers":{"a":1},` + goodCtx + `}`, 404},
		{"unknown survey", "good-token", "/api/v1/surveys/nope/responses", good, 404},
		{"not an org member", "outsider-token", path, good, 403},
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
	if r.Answers["comment"] != "fine" || r.QuestionVersions["overall"] != 1 || r.QuestionVersions["why"] != 0 {
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
		"skipped answered":   `{"answers":{"overall":3,"client":"copilot","why":"habit"},` + goodCtx + `}`,
		"bad os":             `{"answers":{"overall":3},"context":{"version":"1.0.0","os":"plan9","client":"copilot"}}`,
		"version is text":    `{"answers":{"overall":3},"context":{"version":"my repo","os":"darwin","client":"copilot"}}`,
		"too big":            `{"answers":{"comment":"` + strings.Repeat("x", 40<<10) + `"},` + goodCtx + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			if rec := do(h, "POST", path, "good-token", body); rec.Code != 400 {
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
	a, _ := testAuthenticator(t)
	h := makeRouter(a, nil, api)
	if rec := do(h, "GET", "/api/v1/surveys/active", "", ""); strings.Contains(rec.Body.String(), "q4-2026") {
		t.Fatalf("inactive survey served: %s", rec.Body)
	}
	if rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", "good-token", `{}`); rec.Code != 404 {
		t.Fatalf("inactive survey took a submission: %d", rec.Code)
	}
}

func TestSubmitWithoutStorage(t *testing.T) {
	defs, _ := loadSurveys([]byte(testSurveys))
	a, _ := testAuthenticator(t)
	api := &surveyAPI{surveys: defs, now: func() time.Time { return time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC) }}
	h := makeRouter(a, nil, api)
	if rec := do(h, "POST", "/api/v1/surveys/q4-2026/responses", "good-token", `{}`); rec.Code != 503 {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestUsageRoute(t *testing.T) {
	h, _ := testRouter(t)
	if rec := do(h, "POST", "/api/v1/usage", "", ""); rec.Code != 405 {
		t.Fatalf("POST usage = %d, want 405 before auth", rec.Code)
	}
	if rec := do(h, "GET", "/api/v1/usage", entraToken, ""); rec.Code != 403 {
		t.Fatalf("entra usage = %d, want 403", rec.Code)
	}
	if rec := do(h, "GET", "/api/v1/surveys/q4-2026/responses", "good-token", ""); rec.Code != 405 {
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
		"unknown field": `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}],"x":1}]`,
		"ends first":    `[{"id":"a","title":"t","starts":"2026-01-02","ends":"2026-01-01","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
		"bad type":      `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"essay","text":"?"}]}]`,
		"text no limit": `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"text","text":"?"}]}]`,
		"one option":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"choice","text":"?","options":["x"]}]}]`,
		"duplicate id":  `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]},{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
		"no version":    `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","type":"scale","text":"?","min":1,"max":5}]}]`,
		"labels count":  `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5,"labels":["a","b"]}]}]`,
		"skip_if later": `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"text","text":"?","max_length":5,"skip_if":{"question":"r","answer":"x"}},{"id":"r","version":1,"type":"choice","text":"?","options":["x","y"]}]}]`,
		"two texts":     `[{"id":"a","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"text","text":"?","max_length":5},{"id":"r","version":1,"type":"text","text":"?","max_length":5}]}]`,
		"id with slash": `[{"id":"a/b","title":"t","starts":"2026-01-01","ends":"2026-01-02","questions":[{"id":"q","version":1,"type":"scale","text":"?","min":1,"max":5}]}]`,
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

func TestContextKeepsOnlyCoarseVersion(t *testing.T) {
	for in, want := range map[string]string{"2026.09.24-120000-abc1234": "2026.09", "v1.2.3": "1.2", "dev": "dev"} {
		c := techContext{Version: in, OS: "linux", Client: "pi"}
		if err := c.validate(); err != nil || c.Version != want {
			t.Errorf("%s: got %q, %v; want %q", in, c.Version, err, want)
		}
	}
}
