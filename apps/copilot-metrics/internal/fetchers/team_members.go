package fetchers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// FetchTeamMembers lists every team in the org and its direct and child-team
// members (the API default). Needs Organization "Members: read".
func (c *GitHubClient) FetchTeamMembers(ctx context.Context) ([]types.TeamMember, error) {
	client := c.HTTPClient
	if c.OrgHTTPClient != nil {
		client = c.OrgHTTPClient
	}
	base := "https://api.github.com/orgs/" + c.Org + "/teams"

	var teams []struct {
		Slug string `json:"slug"`
	}
	if err := getAllPages(ctx, client, base, &teams); err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}

	var members []types.TeamMember
	for _, team := range teams {
		var users []struct {
			Login string `json:"login"`
		}
		if err := getAllPages(ctx, client, base+"/"+team.Slug+"/members", &users); err != nil {
			if strings.HasPrefix(err.Error(), "status 404:") {
				slog.Warn("Team disappeared during snapshot, skipping", "team", team.Slug)
				continue
			}
			return nil, fmt.Errorf("list members of %s: %w", team.Slug, err)
		}
		for _, u := range users {
			members = append(members, types.TeamMember{TeamSlug: team.Slug, Login: u.Login})
		}
	}
	slog.Info("Fetched team members", "teams", len(teams), "rows", len(members))
	return members, nil
}

// getAllPages appends JSON array pages of url (per_page=100) into out until a
// short page is returned.
func getAllPages[T any](ctx context.Context, client *http.Client, url string, out *[]T) error {
	for page := 1; ; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?per_page=100&page=%d", url, page), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		body, err := getPage(ctx, client, req)
		if err != nil {
			return fmt.Errorf("%w on page %d", err, page)
		}
		var items []T
		if err := json.Unmarshal(body, &items); err != nil {
			return fmt.Errorf("decode page %d: %w", page, err)
		}
		*out = append(*out, items...)
		if len(items) < 100 {
			return nil
		}
	}
}

// getPage does one GET, retrying 429 and 5xx twice with a 2s/4s backoff.
func getPage(ctx context.Context, client *http.Client, req *http.Request) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * retryUnit):
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return body, nil
		}
		lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(body), 300))
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

// retryUnit is the backoff base; tests shrink it.
var retryUnit = time.Second
