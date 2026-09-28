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

// The default gateway URL has to name the cluster copilot-cli is actually
// deployed to. It named prod while the app only ran in dev-gcp, so every
// `nav-pilot survey` and `nav-pilot usage` got Nav's 404 ingress error page
// and blamed naisdevice for it. Update this alongside
// .github/workflows/copilot-cli.yaml when copilot-cli reaches a new cluster.
func TestDefaultCopilotCLIURLNamesADeployedCluster(t *testing.T) {
	if defaultCopilotCLIURL != "https://copilot-cli.intern.dev.nav.no" {
		t.Errorf("defaultCopilotCLIURL = %q; copilot-cli is deployed to dev-gcp only (see .github/workflows/copilot-cli.yaml)", defaultCopilotCLIURL)
	}
	if !allowedCopilotCLIURL(defaultCopilotCLIURL) {
		t.Errorf("defaultCopilotCLIURL %q is rejected by its own allowlist", defaultCopilotCLIURL)
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
