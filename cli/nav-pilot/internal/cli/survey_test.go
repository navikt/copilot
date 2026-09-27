package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"sync/atomic"
	"testing"
)

// Answers that could not be sent are kept and delivered at the next calm
// moment; a 5xx is retried while the user waits, a 409 ends it.
func TestSurveyOfflineAnswersAreDeliveredLater(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("NAV_PILOT_E2E_GITHUB_TOKEN", "gho_test")
	orig := e2eSeams
	e2eSeams = "1"
	t.Cleanup(func() { e2eSeams = orig })

	var up atomic.Bool
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		if r.Header.Get("Authorization") != "Bearer gho_test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !up.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	st := surveyState{Surveys: map[string]*surveyRecord{"s1": {Done: "answered", Pending: []byte(`{"answers":{"a":1}}`)}}}
	sendAnswered(srv.URL, st, "s1")
	if posts.Load() != 3 {
		t.Fatalf("posts = %d, want 3 tries while the user waits", posts.Load())
	}
	if len(readSurveyState().Surveys["s1"].Pending) == 0 {
		t.Fatal("undelivered answers were not kept")
	}

	up.Store(true)
	deliverPending(srv.URL, readSurveyState())
	if p := readSurveyState().Surveys["s1"].Pending; len(p) != 0 {
		t.Fatalf("pending after delivery: %s", p)
	}
}

func TestSurveyVersionShape(t *testing.T) {
	for v, want := range map[string]bool{
		"2026.09.24-120000-abc1234": true,
		"dev":                       false,
		"":                          false,
		"1.2.3 /Users/me":           false,
	} {
		if versionShape(v) != want {
			t.Errorf("versionShape(%q) = %v", v, !want)
		}
	}
}

func TestSessionEndedCalmly(t *testing.T) {
	t.Cleanup(func() { sessionClient = "" })
	exitWith := func(code int) error {
		err := exec.Command("sh", "-c", "exit "+strconv.Itoa(code)).Run()
		return err
	}
	sessionClient = ""
	if sessionEndedCalmly(nil) {
		t.Error("no session ran, yet it counted")
	}
	sessionClient = "copilot"
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{nil, true},
		{exitWith(1), true},
		{exitWith(130), false},
		{exitWith(143), false},
		{errors.New("nav-pilot failed"), false},
	} {
		if got := sessionEndedCalmly(tc.err); got != tc.want {
			t.Errorf("sessionEndedCalmly(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
