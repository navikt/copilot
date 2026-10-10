package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"cloud.google.com/go/bigquery"
)

// Anonymous behavioural segments for /innsikt/trender. Every query returns
// counts per month only: no user IDs, logins or team names. A count below
// minUsersForDistribution comes back as null (hidden), never as a small number.
//
// Intensity uses fixed thresholds on a user's user_initiated_interaction_count
// summed over the month: lett 1-19 (or active through completions alone),
// middels 20-199, tung 200 or more. Interactions cover the whole history;
// ai_credits_used only exists from 2026-06-01, so it cannot carry the series.
//
// Way of working takes the most autonomous mode a user touched in the month:
// used_cli, then used_agent, then used_chat, otherwise completions only.
// user_metrics has no per-user coding agent flag, so «CLI» is CLI only.
//
// Team adoption uses the latest user_teams snapshot for every month (current
// membership applied backwards), teams with at least minTeamContributors
// members, and the share of members active that month: lav under 25 %,
// middels 25-59 %, høy 60 % or more.

const (
	intensityMedium = 20
	intensityHeavy  = 200
	adoptionMedium  = 0.25
	adoptionHigh    = 0.60
)

// SegmentIntensityMonth is the number of active users per intensity group.
type SegmentIntensityMonth struct {
	Month       string `bigquery:"month" json:"month"`
	ActiveUsers *int64 `bigquery:"active_users" json:"active_users"`
	Light       *int64 `bigquery:"light" json:"light"`
	Medium      *int64 `bigquery:"medium" json:"medium"`
	Heavy       *int64 `bigquery:"heavy" json:"heavy"`
}

// SegmentModeMonth is the number of active users per main way of working.
type SegmentModeMonth struct {
	Month       string `bigquery:"month" json:"month"`
	ActiveUsers *int64 `bigquery:"active_users" json:"active_users"`
	Completions *int64 `bigquery:"completions" json:"completions"`
	Chat        *int64 `bigquery:"chat" json:"chat"`
	Agent       *int64 `bigquery:"agent" json:"agent"`
	CLI         *int64 `bigquery:"cli" json:"cli"`
}

// SegmentMovementMonth counts users active in both this and the previous
// month by whether their intensity group went up, stayed or went down.
type SegmentMovementMonth struct {
	Month string `bigquery:"month" json:"month"`
	Up    *int64 `bigquery:"up" json:"up"`
	Stay  *int64 `bigquery:"stay" json:"stay"`
	Down  *int64 `bigquery:"down" json:"down"`
}

// SegmentTeamAdoptionMonth counts teams per adoption level.
type SegmentTeamAdoptionMonth struct {
	Month  string `bigquery:"month" json:"month"`
	Teams  *int64 `bigquery:"teams" json:"teams"`
	Low    *int64 `bigquery:"low" json:"low"`
	Medium *int64 `bigquery:"medium" json:"medium"`
	High   *int64 `bigquery:"high" json:"high"`
}

// UserSegments is the response of /api/v1/copilot/usage/segments.
type UserSegments struct {
	Intensity    []SegmentIntensityMonth    `json:"intensity"`
	Mode         []SegmentModeMonth         `json:"mode"`
	Movement     []SegmentMovementMonth     `json:"movement"`
	TeamAdoption []SegmentTeamAdoptionMonth `json:"team_adoption"`
}

// userMonthCTE has one row per active user and month, with the intensity tier
// (1-3) and main mode. Active as in GetCreditsPerUserMonthly.
const userMonthCTE = `
  user_month AS (
    SELECT
      JSON_VALUE(raw_record, '$.user_id') AS user_id,
      DATE_TRUNC(day, MONTH) AS month,
      SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64), 0)) AS interactions,
      LOGICAL_OR(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.used_cli') AS BOOL), FALSE)) AS cli,
      LOGICAL_OR(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.used_agent') AS BOOL), FALSE)) AS agent,
      LOGICAL_OR(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.used_chat') AS BOOL), FALSE)) AS chat
    FROM %s
    WHERE scope = 'enterprise' AND JSON_VALUE(raw_record, '$.user_id') IS NOT NULL
    GROUP BY user_id, month
    HAVING SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64), 0)) > 0
      OR SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64), 0)
        + COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64), 0)) > 0
  ),
  tiered AS (
    SELECT *,
      CASE WHEN interactions >= @heavy THEN 3 WHEN interactions >= @medium THEN 2 ELSE 1 END AS tier
    FROM user_month
  )`

func (bq *BigQueryClient) segmentQuery(sql string, extra ...bigquery.QueryParameter) *bigquery.Query {
	q := bq.client.Query(sql)
	q.Parameters = append([]bigquery.QueryParameter{
		{Name: "min", Value: minUsersForDistribution},
		{Name: "medium", Value: intensityMedium},
		{Name: "heavy", Value: intensityHeavy},
	}, extra...)
	return q
}

func readSegment[T any](ctx context.Context, q *bigquery.Query) ([]T, error) {
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[T](it)
}

// GetUserSegments runs the four segment queries. Months are YYYY-MM.
func (bq *BigQueryClient) GetUserSegments(ctx context.Context) (*UserSegments, error) {
	metrics := bq.tableRef(bq.metricsDataset, "user_metrics")
	teams := bq.tableRef(bq.metricsDataset, "user_teams")
	with := "WITH " + fmt.Sprintf(userMonthCTE, metrics)
	hide := func(expr string) string { return fmt.Sprintf("IF(%s < @min, NULL, %s)", expr, expr) }

	var out UserSegments
	var err error
	out.Intensity, err = readSegment[SegmentIntensityMonth](ctx, bq.segmentQuery(with+`
      SELECT FORMAT_DATE('%Y-%m', month) AS month, `+hide("COUNT(*)")+` AS active_users,
        `+hide("COUNTIF(tier = 1)")+` AS light, `+hide("COUNTIF(tier = 2)")+` AS medium, `+hide("COUNTIF(tier = 3)")+` AS heavy
      FROM tiered GROUP BY month ORDER BY month`))
	if err != nil {
		return nil, fmt.Errorf("intensity: %w", err)
	}
	out.Mode, err = readSegment[SegmentModeMonth](ctx, bq.segmentQuery(with+`
      SELECT FORMAT_DATE('%Y-%m', month) AS month, `+hide("COUNT(*)")+` AS active_users,
        `+hide("COUNTIF(NOT cli AND NOT agent AND NOT chat)")+` AS completions,
        `+hide("COUNTIF(NOT cli AND NOT agent AND chat)")+` AS chat,
        `+hide("COUNTIF(NOT cli AND agent)")+` AS agent,
        `+hide("COUNTIF(cli)")+` AS cli
      FROM tiered GROUP BY month ORDER BY month`))
	if err != nil {
		return nil, fmt.Errorf("mode: %w", err)
	}
	out.Movement, err = readSegment[SegmentMovementMonth](ctx, bq.segmentQuery(with+`
      SELECT FORMAT_DATE('%Y-%m', cur.month) AS month,
        `+hide("COUNTIF(cur.tier > prev.tier)")+` AS up,
        `+hide("COUNTIF(cur.tier = prev.tier)")+` AS stay,
        `+hide("COUNTIF(cur.tier < prev.tier)")+` AS down
      FROM tiered cur JOIN tiered prev
        ON cur.user_id = prev.user_id AND prev.month = DATE_SUB(cur.month, INTERVAL 1 MONTH)
      GROUP BY cur.month ORDER BY cur.month`))
	if err != nil {
		return nil, fmt.Errorf("movement: %w", err)
	}
	out.TeamAdoption, err = readSegment[SegmentTeamAdoptionMonth](ctx, bq.segmentQuery(with+fmt.Sprintf(`,
      members AS (
        SELECT DISTINCT JSON_VALUE(raw_record, '$.slug') AS team, JSON_VALUE(raw_record, '$.user_id') AS user_id
        FROM %s
        WHERE day = (SELECT MAX(day) FROM %s WHERE scope = 'enterprise' AND scope_id = 'nav')
          AND scope = 'enterprise' AND scope_id = 'nav'
          AND JSON_VALUE(raw_record, '$.slug') != 'nav-it-github-users'
      ),
      big_teams AS (
        SELECT team, COUNT(*) AS size FROM members GROUP BY team HAVING size >= @min_team
      ),
      months AS (SELECT DISTINCT month FROM tiered),
      adoption AS (
        SELECT mo.month, t.team,
          COUNT(DISTINCT u.user_id) / ANY_VALUE(t.size) AS share
        FROM months mo
        CROSS JOIN big_teams t
        JOIN members m ON m.team = t.team
        LEFT JOIN tiered u ON u.user_id = m.user_id AND u.month = mo.month
        GROUP BY mo.month, t.team
      )
      SELECT FORMAT_DATE('%%Y-%%m', month) AS month, `+hide("COUNT(*)")+` AS teams,
        `+hide("COUNTIF(share < @adopt_medium)")+` AS low,
        `+hide("COUNTIF(share >= @adopt_medium AND share < @adopt_high)")+` AS medium,
        `+hide("COUNTIF(share >= @adopt_high)")+` AS high
      FROM adoption GROUP BY month ORDER BY month`, teams, teams),
		bigquery.QueryParameter{Name: "min_team", Value: minTeamContributors},
		bigquery.QueryParameter{Name: "adopt_medium", Value: adoptionMedium},
		bigquery.QueryParameter{Name: "adopt_high", Value: adoptionHigh},
	))
	if err != nil {
		return nil, fmt.Errorf("team adoption: %w", err)
	}
	return &out, nil
}

func (c *CachedBigQueryClient) GetUserSegments(ctx context.Context) (*UserSegments, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "user_segments", func() (*UserSegments, error) {
		return c.client.GetUserSegments(ctx)
	})
}

// handleUserSegments handles GET /api/v1/copilot/usage/segments
func (h *BigQueryHandlers) handleUserSegments(w http.ResponseWriter, r *http.Request) {
	segments, err := h.bqClient.GetUserSegments(r.Context())
	if err != nil {
		slog.Error("Failed to fetch user segments", "error", err)
		respondError(w, "internal_error", "Failed to fetch user segments", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, segments, http.StatusOK)
}
