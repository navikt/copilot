package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/civil"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/fetchers"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

func TestIngestTodayCopilotSeats(t *testing.T) {
	now := time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)
	var stored int
	store := func(_ context.Context, day time.Time, rows []*types.SeatRow) error {
		if day.Format("20060102") != "20261011" {
			t.Errorf("day = %v", day)
		}
		for _, r := range rows {
			if r.SnapshotDate.String() != "2026-10-11" {
				t.Errorf("row stamped %s, partition is 2026-10-11", r.SnapshotDate)
			}
		}
		stored = len(rows)
		return nil
	}
	if err := ingestTodayCopilotSeats(context.Background(), func(context.Context) ([]*types.SeatRow, error) {
		// Fetched before midnight: the row's own day must not win.
		return []*types.SeatRow{{UserID: 1, SnapshotDate: civil.Date{Year: 2026, Month: 10, Day: 10}}}, nil
	}, store, now); err != nil || stored != 1 {
		t.Fatalf("err=%v stored=%d", err, stored)
	}
	if err := ingestTodayCopilotSeats(context.Background(), func(context.Context) ([]*types.SeatRow, error) {
		return nil, fetchers.ErrSeatsNoPermission
	}, store, now); err != nil {
		t.Errorf("403 should be skipped: %v", err)
	}
	if err := ingestTodayCopilotSeats(context.Background(), func(context.Context) ([]*types.SeatRow, error) {
		return nil, errors.New("boom")
	}, store, now); err == nil {
		t.Error("fetch error should fail the job")
	}
}
