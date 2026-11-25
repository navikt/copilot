package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// teamMembersTable holds one row per day, team and member login. The Copilot
// user-teams report only lists active Copilot users, so this is the unbiased
// denominator for team adoption. No backfill: the API only has current state.
const teamMembersTable = "team_members"

// EnsureTeamMembersTableExists creates the team_members table if it doesn't exist.
func (c *BigQueryClient) EnsureTeamMembersTableExists(ctx context.Context) error {
	table := c.Client.Dataset(c.dataset).Table(teamMembersTable)
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
func (c *BigQueryClient) ReplaceTeamMembers(ctx context.Context, day time.Time, members []types.TeamMember) error {
	q := c.Client.Query(teamMembersMergeSQL("`" + c.projectID + "." + c.dataset + "." + teamMembersTable + "`"))
	q.Parameters = []bigquery.QueryParameter{
		{Name: "date", Value: civil.DateOf(day)},
		{Name: "members", Value: members},
	}
	_, err := q.Read(ctx)
	return err
}
