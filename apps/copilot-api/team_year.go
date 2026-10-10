package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/bigquery"
)

// Usage bases for a team month.
const (
	basisNet   = "net"   // billing loaded and complete: same amount as team-net
	basisGross = "gross" // list price before discounts: same amount as team-gross
	basisNone  = "none"  // no per-user usage or no team membership history
)

// TeamYearMonth is one month for one team. Amounts are nil when the month has
// no data or fewer than minTeamContributors contributors (Hidden).
type TeamYearMonth struct {
	Month         string   `json:"month"`
	Basis         string   `json:"basis"`
	Hidden        bool     `json:"hidden"`
	Users         *int64   `json:"users"`
	NetUSD        *float64 `json:"net_usd"`
	GrossUSD      *float64 `json:"gross_usd"`
	NoUsageNetUSD *float64 `json:"no_usage_net_usd"`
}

// TeamYearCoverage says which months the data can answer for. It is derived
// from the tables, so it moves as backfills land.
type TeamYearCoverage struct {
	MembershipFrom string   `json:"membership_from"`
	GrossFrom      string   `json:"gross_from"`
	LastUsageDay   string   `json:"last_usage_day"`
	NetMonths      []string `json:"net_months"`
}

type TeamYearOverview struct {
	TeamID   string           `json:"team_id"`
	TeamSlug string           `json:"team_slug"` // latest slug in the year; teams get renamed
	Year     int              `json:"year"`
	Months   []TeamYearMonth  `json:"months"`
	Coverage TeamYearCoverage `json:"coverage"`
}

type teamYearRow struct {
	Month          string  `bigquery:"month"`
	NetComplete    bool    `bigquery:"net_complete"`
	GrossUsers     int64   `bigquery:"gross_users"`
	Gross          float64 `bigquery:"gross"`
	NetUsers       int64   `bigquery:"net_users"`
	Net            float64 `bigquery:"net"`
	NoUsageNet     float64 `bigquery:"no_usage_net"`
	TeamSlug       string  `bigquery:"team_slug"`
	MembershipFrom string  `bigquery:"membership_from"`
	GrossFrom      string  `bigquery:"gross_from"`
	LastUsageDay   string  `bigquery:"last_usage_day"`
}

// GetTeamYearOverview returns one team's months in year, with the same
// attribution as GetTeamGrossOverview and GetTeamNetOverview for each month.
func (bq *BigQueryClient) GetTeamYearOverview(ctx context.Context, team string, year int) (*TeamYearOverview, error) {
	first := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	query := bq.client.Query(`
WITH ` + bq.teamUsageCTEs() + `, ` + bq.teamNetCTEs() + fmt.Sprintf(`, team AS (
 SELECT DISTINCT day,month,user_id FROM memberships WHERE team_id=@team
), gross_m AS (
 SELECT t.month,COUNT(DISTINCT IF(u.gross>0,t.user_id,NULL)) users,SUM(u.gross * 0.01) gross
 FROM team t JOIN usage_days u USING(day,user_id) GROUP BY t.month
), net_m AS (
 SELECT t.month,COUNT(DISTINCT IF(d.net>0,t.user_id,NULL)) users,SUM(d.net) net
 FROM team t JOIN daily d USING(day,user_id) GROUP BY t.month
), no_usage_m AS (
 SELECT b.month,SUM(b.net) net FROM billed b JOIN (SELECT DISTINCT month,user_id FROM team) USING(month,user_id)
 WHERE NOT EXISTS (SELECT 1 FROM usage_days u WHERE u.user_id=b.user_id AND u.month=b.month AND u.gross>0)
 GROUP BY b.month
), completed AS (
 SELECT DISTINCT month FROM %s WHERE month>=DATE(@from) AND month<DATE(@to) AND scope_id='nav' AND status='complete'
), coverage AS (
 SELECT
  (SELECT IFNULL(CAST(MIN(day) AS STRING),'') FROM %s WHERE scope='enterprise' AND scope_id='nav') membership_from,
  (SELECT IFNULL(CAST(MIN(day) AS STRING),'') FROM %s WHERE scope='enterprise' AND scope_id='nav'
    AND JSON_VALUE(raw_record,'$.ai_credits_used') IS NOT NULL) gross_from,
  (SELECT IFNULL(CAST(MAX(day) AS STRING),'') FROM usage_days) last_usage_day,
  (SELECT IFNULL(MAX_BY(team_slug,day),'') FROM memberships WHERE team_id=@team) team_slug
)
SELECT CAST(m AS STRING) month,c.month IS NOT NULL net_complete,
 IFNULL(g.users,0) gross_users,IFNULL(g.gross,0) gross,IFNULL(n.users,0) net_users,IFNULL(n.net,0) net,
 IFNULL(z.net,0) no_usage_net,v.team_slug,v.membership_from,v.gross_from,v.last_usage_day
FROM UNNEST(GENERATE_DATE_ARRAY(DATE(@from),DATE_SUB(DATE(@to),INTERVAL 1 DAY),INTERVAL 1 MONTH)) m
CROSS JOIN coverage v
LEFT JOIN gross_m g ON g.month=m LEFT JOIN net_m n ON n.month=m
LEFT JOIN no_usage_m z ON z.month=m LEFT JOIN completed c ON c.month=m
ORDER BY month`,
		bq.tableRef(bq.metricsDataset, "billing_user_monthly_runs"),
		bq.tableRef(bq.metricsDataset, "user_teams"),
		bq.tableRef(bq.metricsDataset, "user_metrics")))
	query.Parameters = append(monthRange(first, 12), bigquery.QueryParameter{Name: "team", Value: team})
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read team year usage: %w", err)
	}
	rows, err := readAllRows[teamYearRow](it)
	if err != nil {
		return nil, err
	}
	return teamYearOverview(team, year, rows, time.Now()), nil
}

// teamYearOverview picks each month's basis the way the team page does: net
// once billing is complete for a finished month, else gross where per-user
// gross and membership history exist. Months after now are dropped.
func teamYearOverview(teamID string, year int, rows []teamYearRow, now time.Time) *TeamYearOverview {
	out := &TeamYearOverview{TeamID: teamID, Year: year, Months: []TeamYearMonth{}, Coverage: TeamYearCoverage{NetMonths: []string{}}}
	current := now.UTC().Format("2006-01")
	for _, row := range rows {
		month := row.Month[:7]
		out.TeamSlug = row.TeamSlug
		out.Coverage = TeamYearCoverage{MembershipFrom: row.MembershipFrom, GrossFrom: row.GrossFrom, LastUsageDay: row.LastUsageDay, NetMonths: out.Coverage.NetMonths}
		if month > current {
			continue
		}
		m := TeamYearMonth{Month: month, Basis: basisNone}
		hasTeams := row.MembershipFrom != "" && month >= row.MembershipFrom[:7]
		hasGross := row.GrossFrom != "" && month >= row.GrossFrom[:7]
		switch {
		case hasTeams && row.NetComplete && month < current:
			m.Basis = basisNet
			out.Coverage.NetMonths = append(out.Coverage.NetMonths, month)
		case hasTeams && hasGross:
			m.Basis = basisGross
		}
		// Each amount follows the rule of its month endpoint: shown only with
		// at least minTeamContributors contributors to that amount.
		if m.Basis != basisNone && row.GrossUsers >= minTeamContributors {
			m.GrossUSD = cents(row.Gross)
		}
		if m.Basis == basisNet && row.NetUsers >= minTeamContributors {
			m.NetUSD, m.NoUsageNetUSD = cents(row.Net), cents(row.NoUsageNet)
		}
		users, shown := row.GrossUsers, m.GrossUSD != nil
		if m.Basis == basisNet {
			users, shown = row.NetUsers, m.NetUSD != nil
		}
		m.Hidden = m.Basis != basisNone && !shown
		if shown {
			m.Users = &users
		}
		out.Months = append(out.Months, m)
	}
	return out
}

func cents(v float64) *float64 {
	r := roundCents(v)
	return &r
}

func (c *CachedBigQueryClient) GetTeamYearOverview(ctx context.Context, team string, year int) (*TeamYearOverview, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "team_year_"+team+"_"+strconv.Itoa(year), func() (*TeamYearOverview, error) {
		return c.client.GetTeamYearOverview(ctx, team, year)
	})
}
