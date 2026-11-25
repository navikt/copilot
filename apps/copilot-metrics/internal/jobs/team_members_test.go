package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/fetchers"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

func TestIngestTeamMembers_ForbiddenWritesNothing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Resource not accessible by integration", http.StatusForbidden)
	}))
	defer server.Close()
	client := newTestGitHubClient(server)
	stored := false
	ingestTeamMembers(context.Background(), client.FetchTeamMembers,
		func(context.Context, time.Time, []types.TeamMember) error { stored = true; return nil }, time.Now())
	if stored {
		t.Fatal("store called after 403")
	}
}

func newTestGitHubClient(server *httptest.Server) *fetchers.GitHubClient {
	return &fetchers.GitHubClient{
		HTTPClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		Org:        "navikt",
	}
}
