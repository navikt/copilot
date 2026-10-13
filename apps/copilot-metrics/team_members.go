package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// teamMembersTable holds one row per day, team and member login. The Copilot
// user-teams report only lists active Copilot users, so this is the unbiased
// denominator for team adoption. No backfill: the API only has current state.
const teamMembersTable = "team_members"

// TeamMember is one (team, login) pair.
type TeamMember struct {
	TeamSlug string `bigquery:"team_slug"`
	Login    string `bigquery:"login"`
}

// EnsureTeamMembersTableExists creates the team_members table if it doesn't exist.
func (c *BigQueryClient) EnsureTeamMembersTableExists(ctx context.Context) error {
	table := c.client.Dataset(c.dataset).Table(teamMembersTable)
	if _, err := table.Metadata(ctx); err == nil {
		return nil
	}

	slog.Info("Creating team_members table", "dataset", c.dataset)
	metadata := &bigquery.TableMetadata{
		Schema: bigquery.Schema{
			{Name: "date", Type: bigquery.DateFieldType, Required: true, Description: "Date the membership was read"},
			{Name: "team_slug", Type: bigquery.StringFieldType, Required: true, Description: "GitHub team slug"},
			{Name: "login", Type: bigquery.StringFieldType, Required: true, Description: "GitHub login of a team member"},
		},
		TimePartitioning: &bigquery.TimePartitioning{
			Type:       bigquery.DayPartitioningType,
			Field:      "date",
			Expiration: 400 * 24 * time.Hour,
		},
		Clustering:  &bigquery.Clustering{Fields: []string{"team_slug"}},
		Description: "Daily snapshot of GitHub team membership in the org. Partitions expire after 400 days.",
	}
	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create team_members table: %w", err)
	}
	return nil
}

// teamMembersMergeSQL replaces the day's partition in one statement: rows no
// longer present that day are deleted, new ones inserted. A single DML also
// avoids the streaming-buffer window that blocks delete-then-insert.
func teamMembersMergeSQL(tableRef string) string {
	return `MERGE ` + tableRef + ` t
USING (SELECT DISTINCT @date date, m.team_slug, m.login FROM UNNEST(@members) m) s
ON t.date = s.date AND t.team_slug = s.team_slug AND t.login = s.login
WHEN NOT MATCHED BY TARGET THEN INSERT (date, team_slug, login) VALUES (s.date, s.team_slug, s.login)
WHEN NOT MATCHED BY SOURCE AND t.date = @date THEN DELETE`
}

// ReplaceTeamMembers writes the membership snapshot for a day.
func (c *BigQueryClient) ReplaceTeamMembers(ctx context.Context, day time.Time, members []TeamMember) error {
	q := c.client.Query(teamMembersMergeSQL("`" + c.projectID + "." + c.dataset + "." + teamMembersTable + "`"))
	q.Parameters = []bigquery.QueryParameter{
		{Name: "date", Value: civil.DateOf(day)},
		{Name: "members", Value: members},
	}
	_, err := q.Read(ctx)
	return err
}

// FetchTeamMembers lists every team in the org and its direct and child-team
// members (the API default). Needs Organization "Members: read".
func (c *GitHubClient) FetchTeamMembers(ctx context.Context) ([]TeamMember, error) {
	client := c.httpClient
	if c.orgHttpClient != nil {
		client = c.orgHttpClient
	}
	base := "https://api.github.com/orgs/" + c.org + "/teams"

	var teams []struct {
		Slug string `json:"slug"`
	}
	if err := getAllPages(ctx, client, base, &teams); err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}

	var members []TeamMember
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
			members = append(members, TeamMember{TeamSlug: team.Slug, Login: u.Login})
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

// ingestTeamMembers stores today's membership. It never fails the job: a
// missing Members: read grant gives 403, which is logged and nothing is written.
func ingestTeamMembers(
	ctx context.Context,
	fetch func(context.Context) ([]TeamMember, error),
	store func(context.Context, time.Time, []TeamMember) error,
	now time.Time,
) {
	members, err := fetch(ctx)
	if err != nil {
		slog.Error("Team member fetch failed, nothing written", "error", err)
		return
	}
	if err := store(ctx, now.UTC(), members); err != nil {
		slog.Error("Team member store failed", "error", err)
	}
}
