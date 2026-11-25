package fetchers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// FetchCopilotSeats returns today's org seats as rows. Like
// FetchAllCopilotLogins it uses the org installation token: the enterprise
// seats endpoint does not accept GitHub App installation tokens.
func (c *GitHubClient) FetchCopilotSeats(ctx context.Context) ([]*types.SeatRow, error) {
	if c.OrgHTTPClient == nil {
		return nil, errors.New("org GitHub client not configured: org installation is required to list Copilot seats")
	}
	seats, _, err := c.fetchSeats(ctx, "https://api.github.com/orgs/"+c.Org, c.OrgHTTPClient)
	if seatsPermissionDenied(err) {
		return nil, fmt.Errorf("%w: %w", ErrSeatsNoPermission, err)
	}
	if err != nil {
		return nil, err
	}
	return seatRows(c.Org, seats, time.Now()), nil
}

func seatRows(scopeID string, seats []copilotSeat, now time.Time) []*types.SeatRow {
	day, loaded := civil.DateOf(now.UTC()), now.UTC()
	rows := make([]*types.SeatRow, 0, len(seats))
	seen := make(map[int64]bool, len(seats))
	for _, s := range seats {
		if s.Assignee.ID == 0 || seen[s.Assignee.ID] {
			continue // seat without a user, or repeated across pages
		}
		seen[s.Assignee.ID] = true
		r := &types.SeatRow{SnapshotDate: day, ScopeID: scopeID, UserID: s.Assignee.ID, Login: s.Assignee.Login, PlanType: s.PlanType, LoadedAt: loaded}
		if !s.CreatedAt.IsZero() {
			r.CreatedAt = bigquery.NullTimestamp{Timestamp: s.CreatedAt, Valid: true}
		}
		if d, err := civil.ParseDate(s.PendingCancellationDate); err == nil {
			r.PendingCancellationDate = bigquery.NullDate{Date: d, Valid: true}
		}
		rows = append(rows, r)
	}
	return rows
}
