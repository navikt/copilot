package fetchers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

func TestFetchTeamMembers_Paginates(t *testing.T) {
	// 150 teams over two pages; team-0 has 101 members over two pages.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if r.URL.Query().Get("per_page") != "100" {
			t.Errorf("missing per_page=100: %s", r.URL)
		}
		var items []string
		switch {
		case r.URL.Path == "/orgs/navikt/teams":
			n, off := 100, 0
			if page == "2" {
				n, off = 50, 100
			}
			for i := range n {
				items = append(items, fmt.Sprintf(`{"slug":"team-%d"}`, off+i))
			}
		case r.URL.Path == "/orgs/navikt/teams/team-0/members":
			n := 100
			if page == "2" {
				n = 1
			}
			for i := range n {
				items = append(items, fmt.Sprintf(`{"login":"u%s-%d"}`, page, i))
			}
		case strings.HasSuffix(r.URL.Path, "/members"):
			items = []string{`{"login":"solo"}`}
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, "["+strings.Join(items, ",")+"]")
	}))
	defer server.Close()

	client := &GitHubClient{
		HTTPClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		Org:        "navikt",
	}
	got, err := client.FetchTeamMembers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := 101 + 149; len(got) != want {
		t.Fatalf("got %d rows, want %d", len(got), want)
	}
	if got[100] != (types.TeamMember{TeamSlug: "team-0", Login: "u2-0"}) {
		t.Errorf("second member page missing: %+v", got[100])
	}
	if got[len(got)-1] != (types.TeamMember{TeamSlug: "team-149", Login: "solo"}) {
		t.Errorf("second team page missing: %+v", got[len(got)-1])
	}
}

func TestFetchTeamMembers_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Resource not accessible by integration", http.StatusForbidden)
	}))
	defer server.Close()
	client := &GitHubClient{
		HTTPClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		Org:        "navikt",
	}
	if _, err := client.FetchTeamMembers(context.Background()); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("want 403 error, got %v", err)
	}
}

func TestFetchTeamMembers_RetriesAndSkipsGoneTeam(t *testing.T) {
	retryUnit = time.Millisecond
	defer func() { retryUnit = time.Second }()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/navikt/teams":
			if calls++; calls == 1 {
				http.Error(w, "boom", http.StatusBadGateway)
				return
			}
			_, _ = fmt.Fprint(w, `[{"slug":"a"},{"slug":"gone"}]`)
		case "/orgs/navikt/teams/a/members":
			_, _ = fmt.Fprint(w, `[{"login":"x"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	got, err := newTestGitHubClient(server).FetchTeamMembers(context.Background())
	if err != nil || len(got) != 1 || got[0].Login != "x" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func newTestGitHubClient(server *httptest.Server) *GitHubClient {
	return &GitHubClient{
		HTTPClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		Org:        "navikt",
	}
}
