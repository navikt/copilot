package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
)

// TeamGrossUsage is overlapping member usage, not additive team billing.
type TeamGrossUsage struct {
	TeamID   string  `bigquery:"team_id" json:"team_id"`
	TeamSlug string  `bigquery:"team_slug" json:"team_slug"`
	Users    int64   `bigquery:"users" json:"users"`
	GrossUSD float64 `bigquery:"gross_usd" json:"gross_usd"`
}

type TeamNetUsage struct {
	TeamID   string  `json:"team_id"`
	TeamSlug string  `json:"team_slug"`
	Users    int64   `json:"users"`
	NetUSD   float64 `json:"net_usd"`
}

type TeamGrossOverview struct {
	Usage              map[string]TeamUsageComposition `json:"usage"`
	Month              string                          `json:"month"`
	Teams              []TeamGrossUsage                `json:"teams"`
	SmallTeams         int64                           `json:"small_teams"`
	SmallTeamsUsers    int64                           `json:"small_teams_users"`
	SmallTeamsGrossUSD float64                         `json:"small_teams_gross_usd"`
	DistinctGrossUSD   float64                         `json:"distinct_gross_usd"`
	UnassignedGrossUSD float64                         `json:"unassigned_gross_usd"`
	LastUsageDay       string                          `json:"last_usage_day"`
	DaysWithUsage      int64                           `json:"days_with_usage"`
}

type TeamNetOverview struct {
	Month            string         `json:"month"`
	Teams            []TeamNetUsage `json:"teams"`
	SmallTeams       int64          `json:"small_teams"`
	SmallTeamsUsers  int64          `json:"small_teams_users"`
	SmallTeamsNetUSD float64        `json:"small_teams_net_usd"`
	KnownNetUSD      float64        `json:"known_net_usd"`
	UnassignedNetUSD float64        `json:"unassigned_net_usd"`
	NoUsageNetUSD    float64        `json:"no_usage_net_usd"`
	EnterpriseNetUSD float64        `json:"enterprise_net_usd"`
	ResidualNetUSD   float64        `json:"residual_net_usd"`
	LoadedAt         string         `json:"loaded_at"`
	EstimatedTiming  bool           `json:"estimated_timing"`
	SKU              string         `json:"sku"`
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
	result := &TeamNetOverview{Month: month, Teams: []TeamNetUsage{}}
	for _, row := range rows {
		if row.TeamID != "" {
			result.Teams = append(result.Teams, TeamNetUsage{row.TeamID, row.TeamSlug, row.Users, row.Net})
		}
		result.SmallTeams, result.SmallTeamsUsers, result.SmallTeamsNetUSD = row.SmallTeams, row.SmallUsers, row.SmallNet
		result.KnownNetUSD, result.UnassignedNetUSD, result.EnterpriseNetUSD = row.KnownNet, row.UnassignedNet, row.EnterpriseNet
		result.NoUsageNetUSD = row.NoUsageNet
		result.LoadedAt = row.LoadedAt
	}
	result.ResidualNetUSD = result.EnterpriseNetUSD - result.KnownNetUSD
	result.EstimatedTiming = true
	result.SKU = "Copilot AI Credits + Copilot Cloud Agent"
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
 FROM %s WHERE month=DATE(@month) AND scope_id='nav' AND user_id!='' AND sku IN ('Copilot AI Credits','Copilot Cloud Agent')
 GROUP BY user_id
), usage_days AS (
 SELECT day,JSON_VALUE(raw_record,'$.user_id') user_id,
   COALESCE(SAFE_CAST(JSON_VALUE(raw_record,'$.ai_credits_used') AS FLOAT64),0) gross
 FROM %s WHERE day>=DATE(@month) AND day<DATE_ADD(DATE(@month),INTERVAL 1 MONTH)
   AND scope='enterprise' AND scope_id='nav'
), weighted AS (
 SELECT u.day,u.user_id,b.net * SAFE_DIVIDE(u.gross,SUM(u.gross) OVER(PARTITION BY u.user_id)) net
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
 WHERE NOT EXISTS (SELECT 1 FROM usage_days u WHERE u.user_id=b.user_id AND u.gross>0)
), enterprise_snapshot AS (
 SELECT COUNTIF(sku='done') markers,COALESCE(SUM(IF(sku IN ('Copilot AI Credits','Copilot Cloud Agent'),net_amount,0)),0) net
 FROM %s WHERE month=DATE(@month) AND scope_id='nav' AND user_id=''
), enterprise AS (
 SELECT COALESCE(SUM(net_amount),0) net FROM %s
 WHERE day>=DATE(@month) AND day<DATE_ADD(DATE(@month),INTERVAL 1 MONTH)
 AND scope_id='nav' AND product='Copilot' AND sku IN ('Copilot AI Credits','Copilot Cloud Agent')
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
	query.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month + "-01"}, {Name: "minUsers", Value: minTeamContributors}}
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
	query.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month + "-01"}, {Name: "minUsers", Value: minTeamContributors}}
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
	result := &TeamGrossOverview{Month: month, Teams: []TeamGrossUsage{}}
	for _, row := range rows {
		if row.TeamID != "" && row.Users >= minTeamContributors {
			result.Teams = append(result.Teams, TeamGrossUsage{row.TeamID, row.TeamSlug, row.Users, row.Gross})
		}
		result.SmallTeams, result.SmallTeamsUsers, result.SmallTeamsGrossUSD = row.SmallTeams, row.SmallUsers, row.SmallGross
		result.DistinctGrossUSD, result.UnassignedGrossUSD, result.LastUsageDay = row.DistinctGross, row.UnassignedGross, row.LastDay
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
