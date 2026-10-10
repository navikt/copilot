package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
)

// TeamSpend is a display-ready team row. Amounts are rounded to cents, and only
// teams with at least minTeamContributors contributors are ever built. Rows
// overlap: a person in several teams counts in each, so rows never sum to a total.
type TeamSpend struct {
	TeamID     string   `json:"team_id"`
	TeamSlug   string   `json:"team_slug"`
	Users      int64    `json:"users"`
	AmountUSD  float64  `json:"amount_usd"`
	PerUserUSD float64  `json:"per_user_usd"`
	ChangeUSD  *float64 `json:"change_usd"`
	Highlight  bool     `json:"highlight"`

	raw float64 // unrounded amount, so change is not computed from rounded values
}

// Comparison states the page explains in words.
const (
	comparisonOK              = "ok"
	comparisonCurrentMonth    = "current_month"
	comparisonPreviousNetMiss = "previous_net_missing"
	comparisonIncomplete      = "incomplete"
)

// The overviews expose no totals or hidden-team sums: with overlapping rows and
// only a count of hidden teams, nothing can be subtracted to recover a hidden team.
type TeamGrossOverview struct {
	Usage        map[string]TeamUsageComposition `json:"usage"`
	Month        string                          `json:"month"`
	Teams        []TeamSpend                     `json:"teams"`
	SmallTeams   int64                           `json:"small_teams"`
	LastUsageDay string                          `json:"last_usage_day"`
	Comparison   string                          `json:"comparison"`

	DaysWithUsage    int64   `json:"-"`
	DistinctGrossUSD float64 `json:"-"`
}

type TeamNetOverview struct {
	Month      string      `json:"month"`
	Teams      []TeamSpend `json:"teams"`
	SmallTeams int64       `json:"small_teams"`
	LoadedAt   string      `json:"loaded_at"`
	Comparison string      `json:"comparison"`

	KnownNetUSD      float64 `json:"-"`
	EnterpriseNetUSD float64 `json:"-"`
	ResidualNetUSD   float64 `json:"-"`
}

func roundCents(v float64) float64 { return math.Round(v*100) / 100 }

func teamSpend(id, slug string, users int64, amount float64) TeamSpend {
	return TeamSpend{TeamID: id, TeamSlug: slug, Users: users, AmountUSD: roundCents(amount), PerUserUSD: roundCents(amount / float64(users)), raw: amount}
}

// withChanges returns a copy of teams with month-over-month change from prev.
// A change is shown only when the team has n >= 5 in both months. Highlight
// marks a change of at least 10 USD and 10 % of the previous amount.
func withChanges(teams, prev []TeamSpend) []TeamSpend {
	old := map[string]TeamSpend{}
	for _, t := range prev {
		old[t.TeamID] = t
	}
	out := make([]TeamSpend, len(teams))
	for i, t := range teams {
		t.ChangeUSD, t.Highlight = nil, false
		if p, ok := old[t.TeamID]; ok && t.Users >= minTeamContributors && p.Users >= minTeamContributors {
			diff := t.raw - p.raw
			change := roundCents(diff)
			t.ChangeUSD = &change
			t.Highlight = math.Abs(diff) >= 10 && (p.raw == 0 || math.Abs(diff)/math.Abs(p.raw) >= 0.1)
		}
		out[i] = t
	}
	return out
}

// teamComparison decides whether month can be compared with the month before.
// Both months must be finished and have gross usage for every day.
func teamComparison(month string, now time.Time, grossDays int64, prevGross *TeamGrossOverview, net, prevNet bool) string {
	first, _ := time.Parse("2006-01", month)
	prev := first.AddDate(0, -1, 0)
	full := month < now.UTC().Format("2006-01") &&
		grossDays == int64(first.AddDate(0, 1, -1).Day()) &&
		prevGross != nil && prevGross.DaysWithUsage == int64(prev.AddDate(0, 1, -1).Day())
	switch {
	case full && (!net || prevNet):
		return comparisonOK
	case month == now.UTC().Format("2006-01"):
		return comparisonCurrentMonth
	case net && prevGross != nil && prevGross.LastUsageDay != "" && !prevNet:
		return comparisonPreviousNetMiss
	default:
		return comparisonIncomplete
	}
}

type teamNetRow struct {
	TeamID        string  `bigquery:"team_id"`
	TeamSlug      string  `bigquery:"team_slug"`
	Users         int64   `bigquery:"users"`
	Net           float64 `bigquery:"net"`
	SmallTeams    int64   `bigquery:"small_teams"`
	SmallUsers    int64   `bigquery:"small_users"`
	SmallNet      float64 `bigquery:"small_net"`
	KnownNet      float64 `bigquery:"known_net"`
	UnassignedNet float64 `bigquery:"unassigned_net"`
	NoUsageNet    float64 `bigquery:"no_usage_net"`
	EnterpriseNet float64 `bigquery:"enterprise_net"`
	LoadedAt      string  `bigquery:"loaded_at"`
}

func teamNetOverview(month string, rows []teamNetRow) *TeamNetOverview {
	if len(rows) == 0 {
		return nil
	}
	result := &TeamNetOverview{Month: month, Teams: []TeamSpend{}}
	for _, row := range rows {
		if row.TeamID != "" && row.Users >= minTeamContributors {
			result.Teams = append(result.Teams, teamSpend(row.TeamID, row.TeamSlug, row.Users, row.Net))
		}
		result.SmallTeams, result.KnownNetUSD, result.EnterpriseNetUSD = row.SmallTeams, row.KnownNet, row.EnterpriseNet
		result.LoadedAt = row.LoadedAt
	}
	result.ResidualNetUSD = result.EnterpriseNetUSD - result.KnownNetUSD
	return result
}

// GetTeamNetOverview only serves a month after the ingestion completion marker
// exists. Team rows overlap; only the distinct-user organization totals reconcile.
func (bq *BigQueryClient) GetTeamNetOverview(ctx context.Context, month string) (*TeamNetOverview, error) {
	if !isValidYearMonth(month) {
		return nil, fmt.Errorf("invalid month %q", month)
	}
	first, _ := time.Parse("2006-01", month)
	now := time.Now().UTC()
	if !first.Before(time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)) {
		return nil, nil
	}
	query := bq.client.Query(fmt.Sprintf(`
WITH completed AS (
 SELECT CAST(MAX(loaded_at) AS STRING) loaded_at
 FROM %s WHERE month=DATE(@month) AND scope_id='nav' AND status='complete'
), billed AS (
 SELECT user_id, SUM(net_amount) net
 FROM %s WHERE month=DATE(@month) AND scope_id='nav' AND user_id!='' AND sku IN UNNEST(@skus)
 GROUP BY user_id
), usage_days AS (
 SELECT day,JSON_VALUE(raw_record,'$.user_id') user_id,
   SAFE_CAST(JSON_VALUE(raw_record,'$.ai_credits_used') AS FLOAT64) gross
 FROM %s WHERE day>=DATE(@month) AND day<DATE_ADD(DATE(@month),INTERVAL 1 MONTH)
   AND scope='enterprise' AND scope_id='nav'
), weighted AS (
 -- Before June 2026 user_metrics has no ai_credits_used; spread net evenly over active days.
 SELECT u.day,u.user_id,b.net * IF(COUNT(u.gross) OVER(PARTITION BY u.user_id)=0,
   1/COUNT(*) OVER(PARTITION BY u.user_id),
   SAFE_DIVIDE(IFNULL(u.gross,0),SUM(u.gross) OVER(PARTITION BY u.user_id))) net
 FROM usage_days u JOIN billed b USING(user_id)
), daily AS (
 SELECT day,user_id,net FROM weighted WHERE net IS NOT NULL
), memberships AS (
 SELECT DISTINCT day,JSON_VALUE(raw_record,'$.user_id') user_id,
 JSON_VALUE(raw_record,'$.team_id') team_id,JSON_VALUE(raw_record,'$.slug') team_slug
 FROM %s WHERE day>=DATE(@month) AND day<DATE_ADD(DATE(@month),INTERVAL 1 MONTH)
 AND scope='enterprise' AND scope_id='nav' AND JSON_VALUE(raw_record,'$.slug')!='nav-it-github-users'
), exposure AS (
 SELECT m.day,m.user_id,m.team_id,m.team_slug,w.net
 FROM memberships m JOIN daily w USING(day,user_id)
), eligible_teams AS (
 SELECT m.team_id, COUNT(DISTINCT IF(w.net>0,m.user_id,NULL)) users
 FROM memberships m JOIN daily w USING(day,user_id)
 GROUP BY m.team_id
), team_counts AS (
 SELECT e.team_id,ANY_VALUE(e.team_slug) team_slug,MAX(k.users) users,SUM(e.net) net
 FROM exposure e JOIN eligible_teams k USING(team_id) GROUP BY e.team_id
), small_days AS (
 SELECT DISTINCT e.day,e.user_id,e.net FROM exposure e JOIN team_counts t USING(team_id)
 WHERE t.users<@minUsers
), small_contributors AS (
 SELECT COUNT(DISTINCT w.user_id) users FROM daily w JOIN memberships m USING(day,user_id)
 JOIN eligible_teams k USING(team_id) WHERE k.users<@minUsers AND w.net>0
), small AS (
 SELECT COALESCE(SUM(net),0) net FROM small_days
), assigned AS (
 SELECT COALESCE(SUM(net),0) net FROM daily w
 WHERE EXISTS (
 SELECT 1 FROM memberships m WHERE m.day=w.day AND m.user_id=w.user_id)
), totals AS (
 SELECT COALESCE(SUM(net),0) net FROM billed
), no_usage AS (
 SELECT COALESCE(SUM(b.net),0) net FROM billed b
 WHERE NOT EXISTS (SELECT 1 FROM usage_days u WHERE u.user_id=b.user_id AND IFNULL(u.gross>0,TRUE))
), enterprise_snapshot AS (
 SELECT COUNTIF(sku='done') markers,COALESCE(SUM(IF(sku IN UNNEST(@skus),net_amount,0)),0) net
 FROM %s WHERE month=DATE(@month) AND scope_id='nav' AND user_id=''
), enterprise AS (
 SELECT COALESCE(SUM(net_amount),0) net FROM %s
 WHERE day>=DATE(@month) AND day<DATE_ADD(DATE(@month),INTERVAL 1 MONTH)
 AND scope_id='nav' AND product='Copilot' AND sku IN UNNEST(@skus)
)
SELECT IFNULL(t.team_id,'') team_id,IFNULL(t.team_slug,'') team_slug,
 IFNULL(t.users,0) users,IFNULL(t.net,0) net,
 (SELECT COUNTIF(users<@minUsers) FROM team_counts) small_teams,
  sc.users small_users,s.net small_net,o.net known_net,
  o.net-a.net unassigned_net,z.net no_usage_net,IF(es.markers>0,es.net,e.net) enterprise_net,c.loaded_at
FROM completed c CROSS JOIN small s CROSS JOIN small_contributors sc CROSS JOIN assigned a CROSS JOIN totals o CROSS JOIN no_usage z CROSS JOIN enterprise e CROSS JOIN enterprise_snapshot es
LEFT JOIN team_counts t ON t.users>=@minUsers
WHERE c.loaded_at IS NOT NULL ORDER BY t.team_slug`,
		bq.tableRef(bq.metricsDataset, "billing_user_monthly_runs"),
		bq.tableRef(bq.metricsDataset, "billing_user_monthly"),
		bq.tableRef(bq.metricsDataset, "user_metrics"),
		bq.tableRef(bq.metricsDataset, "user_teams"),
		bq.tableRef(bq.metricsDataset, "billing_user_monthly"),
		bq.tableRef(bq.metricsDataset, "billing_usage_daily_model")))
	query.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month + "-01"}, {Name: "minUsers", Value: minTeamContributors}, {Name: "skus", Value: teamNetSKUs}}
	// The billing table is only created when an operator starts a backfill.
	if _, err := bq.client.Dataset(bq.metricsDataset).Table("billing_user_monthly_runs").Metadata(ctx); err != nil {
		var apiErr *googleapi.Error
		if errors.As(err, &apiErr) && apiErr.Code == 404 {
			return nil, errTeamNetNotReady
		}
		return nil, fmt.Errorf("read billing run metadata: %w", err)
	}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read team net usage: %w", err)
	}
	rows, err := readAllRows[teamNetRow](it)
	if err != nil {
		return nil, err
	}
	return teamNetOverview(month, rows), nil
}

func (c *CachedBigQueryClient) GetTeamNetOverview(ctx context.Context, month string) (*TeamNetOverview, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "team_net_"+month, func() (*TeamNetOverview, error) {
		return c.client.GetTeamNetOverview(ctx, month)
	})
}

// teamNetSKUs covers AI-credit months and the premium-request months before June 2026.
var teamNetSKUs = []string{"Copilot AI Credits", "Copilot Cloud Agent", "Copilot Premium Request", "Coding Agent Premium Request"}

const minTeamContributors = 5

func (bq *BigQueryClient) GetUserTeams(ctx context.Context, userLogin string) ([]string, error) {
	query := bq.client.Query(fmt.Sprintf(`SELECT DISTINCT JSON_VALUE(raw_record,'$.slug') team_slug
FROM %s WHERE day=(SELECT MAX(day) FROM %s WHERE scope='enterprise' AND scope_id='nav')
AND scope='enterprise' AND scope_id='nav'
AND LOWER(JSON_VALUE(raw_record,'$.user_login'))=LOWER(@login)
AND JSON_VALUE(raw_record,'$.slug')!='nav-it-github-users'
ORDER BY team_slug`, bq.tableRef(bq.metricsDataset, "user_teams"), bq.tableRef(bq.metricsDataset, "user_teams")))
	query.Parameters = []bigquery.QueryParameter{{Name: "login", Value: userLogin}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read caller teams: %w", err)
	}
	teams := []string{}
	for {
		var row struct {
			TeamSlug string `bigquery:"team_slug"`
		}
		if err := it.Next(&row); err != nil {
			if err == iterator.Done {
				return teams, nil
			}
			return nil, err
		}
		teams = append(teams, row.TeamSlug)
	}
}

func (c *CachedBigQueryClient) GetUserTeams(ctx context.Context, login string) ([]string, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "user_teams_"+login, func() ([]string, error) {
		return c.client.GetUserTeams(ctx, login)
	})
}

var errTeamNetNotReady = errors.New("team net billing not ready")

type teamGrossRow struct {
	TeamID          string  `bigquery:"team_id"`
	TeamSlug        string  `bigquery:"team_slug"`
	Users           int64   `bigquery:"users"`
	Gross           float64 `bigquery:"gross"`
	SmallTeams      int64   `bigquery:"small_teams"`
	SmallUsers      int64   `bigquery:"small_users"`
	SmallGross      float64 `bigquery:"small_gross"`
	DistinctGross   float64 `bigquery:"distinct_gross"`
	LastDay         string  `bigquery:"last_day"`
	DaysWithUsage   int64   `bigquery:"days_with_usage"`
	UnassignedGross float64 `bigquery:"unassigned_gross"`
}

// The small-team bucket counts distinct user-days, even when the same person
// belongs to several hidden teams. It can also overlap visible teams.
func (bq *BigQueryClient) GetTeamGrossOverview(ctx context.Context, month string) (*TeamGrossOverview, error) {
	if !isValidYearMonth(month) {
		return nil, fmt.Errorf("invalid month %q", month)
	}
	query := bq.client.Query(fmt.Sprintf(`
WITH users AS (
  SELECT day, JSON_VALUE(raw_record, '$.user_id') user_id,
    COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64), 0) * 0.01 gross
  FROM %s
  WHERE day >= DATE(@month) AND day < DATE_ADD(DATE(@month), INTERVAL 1 MONTH)
    AND scope = 'enterprise' AND scope_id = 'nav'
), teams AS (
  SELECT DISTINCT day, JSON_VALUE(raw_record, '$.user_id') user_id,
    JSON_VALUE(raw_record, '$.team_id') team_id, JSON_VALUE(raw_record, '$.slug') team_slug
  FROM %s
  WHERE day >= DATE(@month) AND day < DATE_ADD(DATE(@month), INTERVAL 1 MONTH)
    AND scope = 'enterprise' AND scope_id = 'nav'
    AND JSON_VALUE(raw_record, '$.slug') != 'nav-it-github-users'
), exposure AS (
  SELECT t.day, t.user_id, t.team_id, t.team_slug, u.gross
  FROM teams t JOIN users u USING (day, user_id)
), team_counts AS (
  SELECT team_id, ANY_VALUE(team_slug) team_slug, COUNT(DISTINCT IF(gross > 0,user_id,NULL)) users, SUM(gross) gross
  FROM exposure GROUP BY team_id
), hidden_days AS (
  SELECT DISTINCT e.day, e.user_id, e.gross
  FROM exposure e JOIN team_counts c USING (team_id) WHERE c.users < @minUsers
), summary AS (
  SELECT COUNTIF(users < @minUsers) small_teams FROM team_counts
), hidden AS (
  SELECT COUNT(DISTINCT IF(gross>0,user_id,NULL)) users, COALESCE(SUM(gross), 0) gross FROM hidden_days
), totals AS (
  SELECT COALESCE(SUM(gross), 0) gross, IFNULL(CAST(MAX(day) AS STRING), '') last_day,
    COUNT(DISTINCT day) days_with_usage FROM users
), unassigned AS (
  SELECT COALESCE(SUM(u.gross), 0) gross
  FROM users u LEFT JOIN teams t USING (day, user_id) WHERE t.user_id IS NULL
)
SELECT IFNULL(c.team_id, '') team_id, IFNULL(c.team_slug, '') team_slug,
  IFNULL(c.users, 0) users, IFNULL(c.gross, 0) gross,
  s.small_teams, h.users small_users, h.gross small_gross,
  o.gross distinct_gross, o.last_day, o.days_with_usage, a.gross unassigned_gross
FROM summary s CROSS JOIN hidden h
CROSS JOIN totals o CROSS JOIN unassigned a
LEFT JOIN team_counts c ON c.users >= @minUsers ORDER BY c.team_slug`, bq.tableRef(bq.metricsDataset, "user_metrics"), bq.tableRef(bq.metricsDataset, "user_teams")))
	query.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month + "-01"}, {Name: "minUsers", Value: minTeamContributors}, {Name: "skus", Value: teamNetSKUs}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read team gross usage: %w", err)
	}
	rows, err := readAllRows[teamGrossRow](it)
	if err != nil {
		return nil, err
	}
	usage, usageErr := bq.getTeamUsageComposition(ctx, month)
	return teamGrossOverview(month, rows, usage, usageErr), nil
}

func teamGrossOverview(month string, rows []teamGrossRow, usage map[string]TeamUsageComposition, usageErr error) *TeamGrossOverview {
	result := &TeamGrossOverview{Month: month, Teams: []TeamSpend{}}
	for _, row := range rows {
		if row.TeamID != "" && row.Users >= minTeamContributors {
			result.Teams = append(result.Teams, teamSpend(row.TeamID, row.TeamSlug, row.Users, row.Gross))
		}
		result.SmallTeams, result.DistinctGrossUSD, result.LastUsageDay = row.SmallTeams, row.DistinctGross, row.LastDay
		result.DaysWithUsage = row.DaysWithUsage
	}
	if usageErr != nil {
		slog.Warn("Team usage composition unavailable")
		return result
	}
	result.Usage = map[string]TeamUsageComposition{}
	for _, team := range result.Teams {
		composition := usage[team.TeamID]
		if len(composition.Providers) == 0 {
			composition.Providers = []string{}
		}
		if len(composition.Categories) == 0 {
			composition.Categories = []string{}
		}
		result.Usage[team.TeamID] = composition
	}
	return result
}

func (c *CachedBigQueryClient) GetTeamGrossOverview(ctx context.Context, month string) (*TeamGrossOverview, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "team_gross_"+month, func() (*TeamGrossOverview, error) {
		return c.client.GetTeamGrossOverview(ctx, month)
	})
}
