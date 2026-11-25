package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// ingestTeamMembers stores today's membership. It never fails the job: a
// missing Members: read grant gives 403, which is logged and nothing is written.
func ingestTeamMembers(
	ctx context.Context,
	fetch func(context.Context) ([]types.TeamMember, error),
	store func(context.Context, time.Time, []types.TeamMember) error,
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
