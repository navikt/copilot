package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/rogpeppe/go-internal/testscript"
)

// fakeSurveyDefs is one open survey: a labelled scale, a multi, a choice that
// is skipped when the multi includes copilot, and an optional text.
const fakeSurveyDefs = `{"surveys":[{"id":"e2e-2026","title":"E2E survey","starts":"2020-01-01","ends":"2099-12-31","questions":[
{"id":"useful","version":1,"type":"scale","text":"How useful is nav-pilot?","min":1,"max":5,"labels":["Helt uenig","Uenig","Nøytral","Enig","Helt enig"],"required":true},
{"id":"clients","version":1,"type":"multi","text":"Which clients do you use?","options":["copilot","opencode","pi"],"max_choices":2},
{"id":"why","version":1,"type":"choice","text":"Why not copilot?","options":["habit","other"],"skip_if":{"question":"clients","answer":"copilot"}},
{"id":"comment","version":1,"type":"text","text":"Anything else?","max_length":50}]}]}`

// fake-survey [-nudge-start] [-empty] [-404] [-hang] serves copilot-cli's survey endpoints on 127.0.0.1 and points
// nav-pilot at it (NAV_PILOT_COPILOT_CLI_URL). Every POST is described on one
// line of $WORK/survey-posts.log: the status it got, whether it carried a
// bearer token, its top-level and context keys, and whether the body holds
// the device id in $HOME/device-id. The first POST per token gets 201, the
// next 409, like the real dedup. $WORK/survey-gets.log counts the GETs.
// -empty lists no survey, -404 answers the list with 404 (the ingress where
// the gateway is not deployed), and -hang never answers it.
func cmdFakeSurvey(ts *testscript.TestScript, neg bool, args []string) {
	defs := fakeSurveyDefs
	status, hang := http.StatusOK, false
	for _, a := range args {
		switch a {
		case "-nudge-start":
			defs = strings.Replace(defs, `"title":"E2E survey",`, `"title":"E2E survey","nudge":"start",`, 1)
		case "-empty":
			defs = `{"surveys":[]}`
		case "-404":
			status = http.StatusNotFound
		case "-hang":
			hang = true
		default:
			ts.Fatalf("usage: fake-survey [-nudge-start] [-empty] [-404] [-hang]")
		}
	}
	if neg {
		ts.Fatalf("usage: fake-survey [-nudge-start] [-empty] [-404] [-hang]")
	}
	// Read here, not in the handler: the script's env is not safe to read
	// from the server's goroutines.
	work, home := ts.Getenv("WORK"), ts.Getenv("HOME")
	var mu sync.Mutex
	answered := map[string]bool{}
	appendLine := func(name, line string) {
		f, err := os.OpenFile(filepath.Join(work, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintln(f, line)
			_ = f.Close()
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/surveys/active":
			appendLine("survey-gets.log", "GET")
			switch {
			case hang:
				mu.Unlock()
				<-r.Context().Done() // until the client gives up
				mu.Lock()
			case status != http.StatusOK:
				w.WriteHeader(status)
			default:
				fmt.Fprint(w, defs)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/surveys/e2e-2026/responses":
			body, _ := io.ReadAll(r.Body)
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			var top map[string]json.RawMessage
			var ctx map[string]any
			_ = json.Unmarshal(body, &top)
			_ = json.Unmarshal(top["context"], &ctx)
			leak := "no"
			if id, err := os.ReadFile(filepath.Join(home, "device-id")); err == nil && strings.Contains(string(body), strings.TrimSpace(string(id))) {
				leak = "yes"
			}
			status := http.StatusCreated
			if answered[token] {
				status = http.StatusConflict
			}
			answered[token] = true
			appendLine("survey-posts.log", fmt.Sprintf("POST %d bearer=%t keys=%s context=%s device_id_leak=%s body=%s",
				status, token != "", sortedKeys(top), sortedKeys(ctx), leak, body))
			w.WriteHeader(status)
			if status == http.StatusConflict {
				fmt.Fprint(w, `{"error":"already answered"}`)
			} else {
				fmt.Fprint(w, `{"status":"recorded"}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	ts.Defer(srv.Close)
	ts.Setenv("NAV_PILOT_COPILOT_CLI_URL", srv.URL)
}

func sortedKeys[V any](m map[string]V) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return strings.Join(keys, ",")
}
