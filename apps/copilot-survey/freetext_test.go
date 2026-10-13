package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

type freetextResp struct {
	Fields []freetextField `json:"fields"`
}

func getFreetext(t *testing.T, h http.Handler) (freetextResp, string) {
	t.Helper()
	rec := do(h, "GET", "/api/v1/admin/surveys/q4-2026/freetext", adminToken, "")
	var got freetextResp
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return got, rec.Body.String()
}

func texts(r freetextResp, field string) []string {
	var out []string
	for _, f := range r.Fields {
		if f.ID == field {
			for _, x := range f.Texts {
				out = append(out, x.Text)
			}
		}
	}
	return out
}

// freetextRowsFixture: free text next to answers that must never come along.
var freetextRowsFixture = []storedAnswer{
	row(`{"overall":5,"client":"opencode","editor":["other"],"editor.other":"emacs","comment":"mail meg på a@b.no"}`, `{"overall":1}`),
	row(`{"overall":1,"client":"copilot","comment":"sakte"}`, `{"overall":1}`),
	row(`{"overall":2,"client":"copilot","comment":""}`, `{"overall":1}`),
}

func withFakeHidden(api *surveyAPI) {
	set := map[string]bool{}
	api.textKey = []byte("test-key")
	api.hidden = func(context.Context, string) (map[string]bool, error) { return set, nil }
	api.hide = func(_ context.Context, _, field, hash string) error { set[hiddenKey(field, hash)] = true; return nil }
}

func TestFreetextAlone(t *testing.T) {
	api := adminAPI(t, freetextRowsFixture)
	withFakeHidden(api)
	h := makeRouter(testAuthenticator(t), api)
	got, body := getFreetext(t, h)

	if c := texts(got, "comment"); len(c) != 2 || !strings.Contains(strings.Join(c, "|"), "mail meg på [e-post]") {
		t.Fatalf("comment texts %q", c)
	}
	if e := texts(got, "editor.other"); len(e) != 1 || e[0] != "emacs" {
		t.Fatalf("editor.other %q", e)
	}
	// Control: the fixture holds these, so their absence below means
	// something.
	for _, s := range []string{"opencode", `"overall"`, "a@b.no", "client", "ctid", "row"} {
		if strings.Contains(body, s) {
			t.Errorf("response carries %q: %s", s, body)
		}
	}
	// Only the documented keys, so no row id can slip in.
	var raw struct {
		Fields []map[string]any `json:"fields"`
	}
	_ = json.Unmarshal([]byte(body), &raw)
	for _, f := range raw.Fields {
		for _, x := range f["texts"].([]any) {
			if len(x.(map[string]any)) != 2 {
				t.Errorf("text has extra keys: %v", x)
			}
		}
	}

	// Hide «sakte» by its token.
	var token string
	for _, f := range got.Fields {
		for _, x := range f.Texts {
			if x.Text == "sakte" {
				token = x.Token
			}
		}
	}
	if rec := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/hide", adminToken, `{"token":"nope"}`); rec.Code != 404 {
		t.Fatalf("unknown token: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/hide", webToken, `{"token":"`+token+`"}`); rec.Code != 404 {
		t.Fatalf("non-admin hide: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/hide", adminToken, `{"token":"`+token+`"}`); rec.Code != 204 {
		t.Fatalf("hide: %d %s", rec.Code, rec.Body)
	}
	got, _ = getFreetext(t, h)
	if c := texts(got, "comment"); len(c) != 1 || c[0] == "sakte" {
		t.Fatalf("after hide %q", c)
	}
}

func TestFreetextNeedsStorage(t *testing.T) {
	api := adminAPI(t, freetextRowsFixture)
	if rec := do(makeRouter(testAuthenticator(t), api), "GET", "/api/v1/admin/surveys/q4-2026/freetext", adminToken, ""); rec.Code != 503 {
		t.Fatalf("%d", rec.Code)
	}
}

func TestExportSkipsHidden(t *testing.T) {
	in := `{"answers":{"c":"skjul meg","d":"behold"},"question_versions":{},"context":{},"hidden":["c ` + textHash("skjul meg") + `","d ` + textHash("annet") + `"]}`
	var out strings.Builder
	if err := runExport(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "skjul meg") || !strings.Contains(out.String(), "behold") {
		t.Fatal(out.String())
	}
}

// TestFreetextStore runs hide and hidden against Postgres, as TestSurveyStore.
func TestFreetextStore(t *testing.T) {
	url := os.Getenv("COPILOT_SURVEY_TEST_DB_URL")
	if url == "" {
		t.Skip("COPILOT_SURVEY_TEST_DB_URL not set")
	}
	s, err := openSurveyStore(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.pool.Close()
	_, _ = s.pool.Exec(t.Context(), `DELETE FROM survey_answers; DELETE FROM survey_freetext_hidden`)
	for _, r := range freetextRowsFixture {
		a, _ := json.Marshal(r.Answers)
		if _, err := s.pool.Exec(t.Context(), `INSERT INTO survey_answers VALUES ('q4-2026', $1, '{}', '{}', '2030-01-01')`, a); err != nil {
			t.Fatal(err)
		}
	}
	api := adminAPI(t, nil)
	api.answers, api.hidden, api.hide, api.textKey = s.answers, s.hidden, s.hide, []byte("k")
	h := makeRouter(testAuthenticator(t), api)
	got, _ := getFreetext(t, h)
	var token string
	for _, f := range got.Fields {
		for _, x := range f.Texts {
			if x.Text == "emacs" {
				token = x.Token
			}
		}
	}
	for range 2 { // twice: hiding again is not an error
		if rec := do(h, "POST", "/api/v1/admin/surveys/q4-2026/freetext/hide", adminToken, `{"token":"`+token+`"}`); rec.Code != 204 {
			t.Fatalf("hide: %d %s", rec.Code, rec.Body)
		}
	}
	got, _ = getFreetext(t, h)
	if e := texts(got, "editor.other"); len(e) != 0 || len(texts(got, "comment")) != 2 {
		t.Fatalf("after hide: %v", got)
	}
	var n int
	_ = s.pool.QueryRow(t.Context(), `SELECT count(*) FROM survey_answers`).Scan(&n)
	if n != 3 {
		t.Fatalf("hiding touched survey_answers: %d rows", n)
	}
}
