package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"cloud.google.com/go/bigquery"
)

// Anonymous behavioural segments for /innsikt/trender. The queries return
// counts per month only: no user IDs, logins or team names. The counts stay
// on the server: buildChart merges any band under minUsersForDistribution
// into a neighbour, hides the month if that is not enough, and returns
// whole-percent shares that sum to 100.
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

// Raw monthly counts per band, read from BigQuery. They never leave the
// server: buildChart merges, suppresses and rounds them first.
type segmentRow struct {
	Month string `bigquery:"month"`
	A     int64  `bigquery:"a"`
	B     int64  `bigquery:"b"`
	C     int64  `bigquery:"c"`
	D     int64  `bigquery:"d"`
}

// SegmentBand is one band of a chart: whole-percent shares, one per month in
// SegmentChart.Months. null: the band does not exist that month (merged into
// a neighbour) or the whole month is hidden.
type SegmentBand struct {
	Label  string   `json:"label"`
	Shares []*int64 `json:"shares"`
}

// SegmentChart is display-ready: in every month the shares sum to 100, or
// all are null.
type SegmentChart struct {
	Months []string      `json:"months"`
	Bands  []SegmentBand `json:"bands"`
	// Net is up minus down in percentage points (movement only).
	Net []*int64 `json:"net,omitempty"`
}

// UserSegments is the response of /api/v1/copilot/usage/segments.
type UserSegments struct {
	Intensity    SegmentChart `json:"intensity"`
	Mode         SegmentChart `json:"mode"`
	Movement     SegmentChart `json:"movement"`
	TeamAdoption SegmentChart `json:"team_adoption"`
}

// bandDef names the bands of one chart. label(i, j) names bands i..j merged.
type bandDef struct {
	names []string
	label func(names []string, i, j int) string
}

// ordinal labels a merged range by its threshold: «Under 200», «Minst 20».
// With three bands, a merged pair always touches an end.
func ordinal(thresholds ...string) func([]string, int, int) string {
	return func(names []string, i, j int) string {
		if i == j {
			return names[i]
		}
		if i == 0 {
			return "Under " + thresholds[j]
		}
		return "Minst " + thresholds[i-1]
	}
}

func nominal(names []string, i, j int) string {
	return strings.Join(names[i:j+1], " + ")
}

var (
	intensityBands = bandDef{[]string{"Lett", "Middels", "Tung"}, ordinal("20", "200")}
	modeBands      = bandDef{[]string{"Bare kodeforslag", "Chat", "Agentmodus", "CLI"}, nominal}
	movementBands  = bandDef{[]string{"Opp", "Samme", "Ned"}, nominal}
	adoptionBands  = bandDef{[]string{"Lav", "Middels", "Høy"}, ordinal("25 %", "60 %")}
)

type band struct {
	i, j int
	n    int64
}

// mergeBands merges every band under minUsersForDistribution into its smaller
// neighbour until none is left. ok is false when that is impossible (the
// month must be hidden): a band still under the limit, or a single band left.
func mergeBands(counts []int64) (bands []band, ok bool) {
	for k, n := range counts {
		bands = append(bands, band{k, k, n})
	}
	for len(bands) > 1 {
		k := -1
		for x, b := range bands {
			if b.n < minUsersForDistribution && (k < 0 || b.n < bands[k].n) {
				k = x
			}
		}
		if k < 0 {
			return bands, true
		}
		o := k - 1
		if k == 0 || (k < len(bands)-1 && bands[k+1].n < bands[k-1].n) {
			o = k + 1
		}
		lo, hi := min(k, o), max(k, o)
		bands[lo] = band{bands[lo].i, bands[hi].j, bands[lo].n + bands[hi].n}
		bands = append(bands[:hi], bands[hi+1:]...)
	}
	return bands, false
}

// wholePercents rounds shares of counts to whole percent summing to 100
// (largest remainder; ties go to the earlier band).
func wholePercents(counts []int64) []int64 {
	var total int64
	for _, n := range counts {
		total += n
	}
	out := make([]int64, len(counts))
	order := make([]int, len(counts))
	left := int64(100)
	for k, n := range counts {
		out[k] = 100 * n / total
		left -= out[k]
		order[k] = k
	}
	sort.SliceStable(order, func(a, b int) bool {
		return 100*counts[order[a]]%total > 100*counts[order[b]]%total
	})
	for _, k := range order[:left] {
		out[k]++
	}
	return out
}

// buildChart turns raw counts into a display-ready chart. A band label shows
// up as a series as soon as any month uses it.
func buildChart(rows []segmentRow, def bandDef) SegmentChart {
	type key struct{ i, j int }
	shares := map[key][]*int64{}
	var keys []key
	chart := SegmentChart{Months: []string{}, Bands: []SegmentBand{}}
	for m, r := range rows {
		chart.Months = append(chart.Months, r.Month)
		counts := []int64{r.A, r.B, r.C, r.D}[:len(def.names)]
		bands, ok := mergeBands(counts)
		if !ok {
			continue
		}
		ns := make([]int64, len(bands))
		for x, b := range bands {
			ns[x] = b.n
		}
		for x, p := range wholePercents(ns) {
			k := key{bands[x].i, bands[x].j}
			if shares[k] == nil {
				shares[k] = make([]*int64, len(rows))
				keys = append(keys, k)
			}
			shares[k][m] = &p
		}
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a].i != keys[b].i {
			return keys[a].i < keys[b].i
		}
		return keys[a].j < keys[b].j
	})
	for _, k := range keys {
		chart.Bands = append(chart.Bands, SegmentBand{def.label(def.names, k.i, k.j), shares[k]})
	}
	return chart
}

// withNet adds up minus down for months where neither is merged.
func withNet(c SegmentChart) SegmentChart {
	var up, down []*int64
	for _, b := range c.Bands {
		switch b.Label {
		case "Opp":
			up = b.Shares
		case "Ned":
			down = b.Shares
		}
	}
	c.Net = make([]*int64, len(c.Months))
	for m := range c.Net {
		if up != nil && down != nil && up[m] != nil && down[m] != nil {
			n := *up[m] - *down[m]
			c.Net[m] = &n
		}
	}
	return c
}

// userDaysFrom is user_metrics with one row per user and day. A day is
// normally stored under the enterprise scope, but under the organization
// scope when copilot-metrics fell back to the org endpoint; a user active in
// both counts once, and the enterprise row wins.
func userDaysFrom(table string) string {
	return `(SELECT * FROM ` + table + `
    WHERE scope IN ('enterprise', 'organization') AND JSON_VALUE(raw_record, '$.user_id') IS NOT NULL
    QUALIFY ROW_NUMBER() OVER (
      PARTITION BY JSON_VALUE(raw_record, '$.user_id'), day ORDER BY IF(scope = 'enterprise', 0, 1)) = 1)`
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
	with := "WITH " + fmt.Sprintf(userMonthCTE, userDaysFrom(metrics))

	var out UserSegments
	var rows []segmentRow
	var err error
	rows, err = readSegment[segmentRow](ctx, bq.segmentQuery(with+`
      SELECT FORMAT_DATE('%Y-%m', month) AS month,
        COUNTIF(tier = 1) AS a, COUNTIF(tier = 2) AS b, COUNTIF(tier = 3) AS c
      FROM tiered GROUP BY month ORDER BY month`))
	if err != nil {
		return nil, fmt.Errorf("intensity: %w", err)
	}
	out.Intensity = buildChart(rows, intensityBands)
	rows, err = readSegment[segmentRow](ctx, bq.segmentQuery(with+`
      SELECT FORMAT_DATE('%Y-%m', month) AS month,
        COUNTIF(NOT cli AND NOT agent AND NOT chat) AS a,
        COUNTIF(NOT cli AND NOT agent AND chat) AS b,
        COUNTIF(NOT cli AND agent) AS c,
        COUNTIF(cli) AS d
      FROM tiered GROUP BY month ORDER BY month`))
	if err != nil {
		return nil, fmt.Errorf("mode: %w", err)
	}
	out.Mode = buildChart(rows, modeBands)
	rows, err = readSegment[segmentRow](ctx, bq.segmentQuery(with+`
      SELECT FORMAT_DATE('%Y-%m', cur.month) AS month,
        COUNTIF(cur.tier > prev.tier) AS a,
        COUNTIF(cur.tier = prev.tier) AS b,
        COUNTIF(cur.tier < prev.tier) AS c
      FROM tiered cur JOIN tiered prev
        ON cur.user_id = prev.user_id AND prev.month = DATE_SUB(cur.month, INTERVAL 1 MONTH)
      GROUP BY cur.month ORDER BY cur.month`))
	if err != nil {
		return nil, fmt.Errorf("movement: %w", err)
	}
	out.Movement = buildChart(rows, movementBands)
	rows, err = readSegment[segmentRow](ctx, bq.segmentQuery(with+fmt.Sprintf(`,
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
      SELECT FORMAT_DATE('%%Y-%%m', month) AS month,
        COUNTIF(share < @adopt_medium) AS a,
        COUNTIF(share >= @adopt_medium AND share < @adopt_high) AS b,
        COUNTIF(share >= @adopt_high) AS c
      FROM adoption GROUP BY month ORDER BY month`, teams, teams),
		bigquery.QueryParameter{Name: "min_team", Value: minTeamContributors},
		bigquery.QueryParameter{Name: "adopt_medium", Value: adoptionMedium},
		bigquery.QueryParameter{Name: "adopt_high", Value: adoptionHigh},
	))
	if err != nil {
		return nil, fmt.Errorf("team adoption: %w", err)
	}
	out.TeamAdoption = buildChart(rows, adoptionBands)
	out.Movement = withNet(out.Movement)
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
