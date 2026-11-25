package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeConsoleApps answers team(slug).applications with body, after checking the token and slug.
func fakeConsoleApps(t *testing.T, body string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if !strings.Contains(req.Query, "applications") || req.Variables["slug"] != "team-a" {
			t.Errorf("unexpected query: %+v", req)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func appsBody(nodes string) string {
	return `{"data":{"team":{"applications":{"nodes":[` + nodes + `],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}}`
}

func node(name, env string) string {
	return `{"name":"` + name + `","teamEnvironment":{"environment":{"name":"` + env + `"}}}`
}

func TestConsoleApps(t *testing.T) {
	ts := fakeConsoleApps(t, appsBody(node("b-mcp", "dev-gcp")+","+node("a-mcp", "prod-gcp")+","+node("a-mcp", "dev-gcp")))
	got, err := newConsoleTeams(ts.URL, tokenFile(t)).Apps(context.Background(), "team-a")
	if err != nil {
		t.Fatal(err)
	}
	want := []TeamApp{{"a-mcp", "dev-gcp"}, {"a-mcp", "prod-gcp"}, {"b-mcp", "dev-gcp"}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestConsoleAppsFailClosed(t *testing.T) {
	for name, body := range map[string]string{
		"graphql error": `{"errors":[{"message":"boom"}],"data":null}`,
		"no team":       `{"data":{"team":null}}`,
		"bad app name":  appsBody(node("Bad/App", "dev-gcp")),
		"bad env":       appsBody(node("ok", "../x")),
		"no cursor":     `{"data":{"team":{"applications":{"nodes":[],"pageInfo":{"hasNextPage":true,"endCursor":""}}}}}`,
		"not json":      `<html>`,
	} {
		t.Run(name, func(t *testing.T) {
			ts := fakeConsoleApps(t, body)
			if _, err := newConsoleTeams(ts.URL, tokenFile(t)).Apps(context.Background(), "team-a"); err == nil {
				t.Fatal("want error")
			}
		})
	}
	if _, err := newConsoleTeams("http://127.0.0.1:1", tokenFile(t)).Apps(context.Background(), "../x"); err == nil {
		t.Fatal("invalid team must fail before any call")
	}
}

func TestListTeamApps(t *testing.T) {
	ts := fakeConsoleApps(t, appsBody(node("a-mcp", "dev-gcp")))
	ct := newConsoleTeams(ts.URL, tokenFile(t))
	call := func(teams teamDirectory, c caller) (int, string, error) {
		r := httptest.NewRequest("GET", nsBase+"team-a/apps", nil)
		r.SetPathValue("namespace", "team-a")
		w := httptest.NewRecorder()
		err := listTeamApps(teams)(w, r, c, nil)
		return w.Code, w.Body.String(), err
	}
	code, body, err := call(ct, caller{Teams: []string{"team-a"}})
	if err != nil || code != 200 || !strings.Contains(body, `"a-mcp"`) || !strings.Contains(body, `"TeamAppList"`) {
		t.Fatalf("member: %d %s %v", code, body, err)
	}
	if _, _, err = call(ct, caller{Teams: []string{"team-b"}}); err != errV1NotFound {
		t.Fatalf("non-member must get 404, got %v", err)
	}
	_, _, err = call(staticTeams{"team-a": {"x@nav.no"}}, caller{Teams: []string{"team-a"}})
	if se, ok := err.(*statusError); !ok || se.code != http.StatusServiceUnavailable {
		t.Fatalf("no Console must be 503, got %v", err)
	}
}

// A Console failure is logged, never answered: the 503 body holds only the generic message.
func TestListTeamAppsHidesConsoleError(t *testing.T) {
	const secret = "kari.nordmann@nav.no is not allowed"
	for name, ts := range map[string]*httptest.Server{
		"graphql error": fakeConsoleApps(t, `{"errors":[{"message":"`+secret+`"}],"data":null}`),
		"http error": httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, secret, http.StatusBadGateway)
		})),
	} {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(ts.Close)
			r := httptest.NewRequest("GET", nsBase+"team-a/apps", nil)
			r.SetPathValue("namespace", "team-a")
			w := httptest.NewRecorder()
			err := listTeamApps(newConsoleTeams(ts.URL, tokenFile(t)))(w, r, caller{Teams: []string{"team-a"}}, nil)
			if err == nil {
				t.Fatal("want error")
			}
			writeStatus(w, r, "", err)
			if w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "kari") || strings.Contains(w.Body.String(), "allowed") {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}
