package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		httpClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		org:        "navikt",
	}
	got, err := client.FetchTeamMembers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := 101 + 149; len(got) != want {
		t.Fatalf("got %d rows, want %d", len(got), want)
	}
	if got[100] != (TeamMember{TeamSlug: "team-0", Login: "u2-0"}) {
		t.Errorf("second member page missing: %+v", got[100])
	}
	if got[len(got)-1] != (TeamMember{TeamSlug: "team-149", Login: "solo"}) {
		t.Errorf("second team page missing: %+v", got[len(got)-1])
	}
}

func TestFetchTeamMembers_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Resource not accessible by integration", http.StatusForbidden)
	}))
	defer server.Close()
	client := &GitHubClient{
		httpClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		org:        "navikt",
	}
	if _, err := client.FetchTeamMembers(context.Background()); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("want 403 error, got %v", err)
	}
}

func TestTeamMembersMergeSQL_ReplacesDay(t *testing.T) {
	sql := teamMembersMergeSQL("`p.d.team_members`")
	for _, want := range []string{
		"MERGE `p.d.team_members`",
		"UNNEST(@members)",
		"WHEN NOT MATCHED BY SOURCE AND t.date = @date THEN DELETE",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("merge SQL missing %q:\n%s", want, sql)
		}
	}
}
