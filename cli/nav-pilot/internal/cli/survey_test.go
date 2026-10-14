package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

// A gateway that answers 404 is a different fault from one that cannot be
// reached, and must not be reported as naisdevice being off.
func TestFetchActiveSurveysSeparatesHTTPErrorFromUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := fetchActiveSurveys(srv.URL)
	var httpErr surveyHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("fetchActiveSurveys on 404 = %v, want surveyHTTPError", err)
	}
	if !strings.Contains(httpErr.Error(), "404") || !strings.Contains(httpErr.Error(), srv.URL) {
		t.Errorf("surveyHTTPError message %q names neither the status nor the URL", httpErr)
	}

	// A closed port is a transport failure, not an HTTP one.
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	addr := closed.URL
	closed.Close()
	if _, err := fetchActiveSurveys(addr); errors.As(err, &httpErr) {
		t.Errorf("unreachable gateway reported as surveyHTTPError: %v", err)
	} else if err == nil {
		t.Error("unreachable gateway returned no error")
	}
}

// A 502 or 503 (copilot-cli up but copilot-survey not, e.g. a missing
// secret) must be treated the same as a 404: nothing to report, no open
// surveys, not an error shown to the user.
func TestNoSurveyServiceCoversAny5xx(t *testing.T) {
	for _, status := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		_, err := fetchActiveSurveys(srv.URL)
		srv.Close()
		if !noSurveyService(err) {
			t.Errorf("noSurveyService(%v) on status %d = false, want true", err, status)
		}
	}
}

// An answer given on the web (copilot-survey's status route) stops the
// prompt and the start hint; any error leaves the survey to be asked.
func TestSurveyAnsweredElsewhereIsNotAsked(t *testing.T) {
	t.Setenv("NAV_PILOT_E2E_GITHUB_TOKEN", "gho_test")
	orig := e2eSeams
	e2eSeams = "1"
	t.Cleanup(func() { e2eSeams = orig })
	now := time.Now()
	day := now.UTC().Format(time.DateOnly)

	for _, tc := range []struct {
		name   string
		status int
		body   string
		asked  bool
	}{
		{"answered", 200, `{"answered":true}`, false},
		{"not answered", 200, `{"answered":false}`, true},
		{"server error", 503, `{"answered":true}`, true},
		{"refused", 403, ``, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NAV_PILOT_CONFIG", t.TempDir()+"/config.toml")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/surveys/s1/status" || r.Header.Get("Authorization") != "Bearer gho_test" {
					w.WriteHeader(http.StatusTeapot)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			st := surveyState{Surveys: map[string]*surveyRecord{}}
			for _, nudge := range []string{"calm", "start"} {
				st.Active = append(st.Active, surveyDef{ID: "s1", Nudge: nudge, Starts: day, Ends: day})
			}
			markAnsweredElsewhere(srv.URL, st, now)
			for _, nudge := range []string{"calm", "start"} {
				if got := nextSurvey(st, now, nudge) != nil; got != tc.asked {
					t.Errorf("%s: asked = %v, want %v", nudge, got, tc.asked)
				}
			}
			// Kept on disk, so a later nav-pilot survey hides it too.
			if hidden := answeredAndSent(readSurveyState(), "s1"); hidden == tc.asked {
				t.Errorf("hidden from nav-pilot survey on the next run = %v, want %v", hidden, !tc.asked)
			}
		})
	}
	// Unreachable is the same as an error.
	t.Setenv("NAV_PILOT_CONFIG", t.TempDir()+"/config.toml")
	st := surveyState{Surveys: map[string]*surveyRecord{}, Active: []surveyDef{{ID: "s1", Starts: day, Ends: day}}}
	markAnsweredElsewhere("http://127.0.0.1:1", st, now)
	if nextSurvey(st, now, "calm") == nil {
		t.Error("unreachable copilot-cli stopped the prompt")
	}
}

// A 409 on submit (answered elsewhere) drops the pending answer and keeps
// the survey answered, so it is neither resent nor asked again.
func TestSurveyConflictMarksAnswered(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", t.TempDir()+"/config.toml")
	t.Setenv("NAV_PILOT_E2E_GITHUB_TOKEN", "gho_test")
	orig := e2eSeams
	e2eSeams = "1"
	t.Cleanup(func() { e2eSeams = orig })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()
	st := surveyState{Surveys: map[string]*surveyRecord{"s1": {Done: "answered", Pending: []byte(`{"answers":{"a":1}}`)}}}
	sendAnswered(srv.URL, st, "s1")
	rec := readSurveyState().Surveys["s1"]
	if rec.Done != "answered" || len(rec.Pending) != 0 {
		t.Fatalf("after 409: %+v, want answered with nothing pending", rec)
	}
}
