package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A 429 is "try later", not a rejection: the answer must stay on disk.
func TestSendAnsweredKeepsAnswerOn429(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("NAV_PILOT_E2E_GITHUB_TOKEN", "gho_test")
	orig := e2eSeams
	e2eSeams = "1"
	t.Cleanup(func() { e2eSeams = orig })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	st := surveyState{Surveys: map[string]*surveyRecord{"s1": {Done: "answered", Pending: []byte(`{"answers":{"a":1}}`)}}}
	sendAnswered(srv.URL, st, "s1")
	if len(readSurveyState().Surveys["s1"].Pending) == 0 {
		t.Fatal("answer was deleted after a 429")
	}
}
