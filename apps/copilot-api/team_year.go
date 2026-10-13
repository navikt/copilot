package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/googleapi"
)

// Usage bases for a team month.
const (
	basisNet   = "net"   // billing loaded and complete: same amount as team-net
	basisGross = "gross" // list price before discounts: same amount as team-gross
	// Months before aiCreditStart were billed as premium requests: net only,
	// spread evenly over usage days, since user_metrics has no per-user gross.
	basisPremium = "premium"
	basisNone    = "none" // no per-user usage or no team membership history
)

// List prices per seat and month by plan_type. The licence amounts are an
// estimate at these prices, not billed amounts. An unknown plan_type is
// priced as business and logged.
const (
	licenceBusinessUSD   = 19.0
	licenceEnterpriseUSD = 39.0
)

// TeamYearMonth is one month for one team. Amounts are nil when the month has
// no data for them. Billing per team has no minimum member count.
type TeamYearMonth struct {
	Month         string   `json:"month"`
	Basis         string   `json:"basis"`
	Users         *int64   `json:"users"`
	NetUSD        *float64 `json:"net_usd"`
	GrossUSD      *float64 `json:"gross_usd"`
	NoUsageNetUSD *float64 `json:"no_usage_net_usd"`
	// ResidualPct is the share of the enterprise net bill that the backfill
	// could not assign to a user, for net and premium months. Nil for gross
	// months and for runs published before the residual columns existed
	// (republish the month to fill it).
	ResidualPct *float64 `json:"residual_pct"`
	// SeatMonths counts team members (team_members, usage or not) whose seat
	// covered the month, from the latest seat snapshot's created_at and
	// pending_cancellation_date. A seat held before the current assignment is
	// unknown and not counted. Months before the first team_members day use
	// that day's members. Nil without a seat snapshot or team_members. Kept
	// out of net and gross: those are usage amounts, this is a list-price
	// estimate.
	SeatMonths         *int64   `json:"seat_months"`
	LicenceUSDEstimate *float64 `json:"licence_usd_estimate"`
	// TotalUSD is the basis amount (net plus no-usage net, or gross) plus the
	// licence estimate. Nil when any part is missing.
	TotalUSD   *float64 `json:"total_usd"`
	TotalBasis string   `json:"total_basis"` // basis of TotalUSD, "none" when nil
}

// TeamYearCoverage says which months the data can answer for. It is derived
// from the tables, so it moves as backfills land.
type TeamYearCoverage struct {
	MembershipFrom string   `json:"membership_from"`
	GrossFrom      string   `json:"gross_from"`
	LastUsageDay   string   `json:"last_usage_day"`
	NetMonths      []string `json:"net_months"`
	// SeatsSnapshot is the latest copilot_seats day, "" before the first one.
	SeatsSnapshot string `json:"seats_snapshot"`
	// TeamMembersFrom is the first team_members day, "" before the first one.
	TeamMembersFrom string `json:"team_members_from"`
}

type TeamYearOverview struct {
	TeamID   string           `json:"team_id"`
	TeamSlug string           `json:"team_slug"` // latest slug in the year; teams get renamed
	Year     int              `json:"year"`
	Months   []TeamYearMonth  `json:"months"`
	Coverage TeamYearCoverage `json:"coverage"`
}

type teamYearRow struct {
	Month           string   `bigquery:"month"`
	NetComplete     bool     `bigquery:"net_complete"`
	ResidualPct     *float64 `bigquery:"residual_pct"` // decodeBQRow handles pointers, not bigquery.Null*
	GrossUsers      int64    `bigquery:"gross_users"`
	Gross           float64  `bigquery:"gross"`
	NetUsers        int64    `bigquery:"net_users"`
	Net             float64  `bigquery:"net"`
	NoUsageNet      float64  `bigquery:"no_usage_net"`
	TeamSlug        string   `bigquery:"team_slug"`
	MembershipFrom  string   `bigquery:"membership_from"`
	GrossFrom       string   `bigquery:"gross_from"`
	LastUsageDay    string   `bigquery:"last_usage_day"`
	SeatMonths      int64    `bigquery:"seat_months"`
	Licence         float64  `bigquery:"licence"`
	UnknownPlan     int64    `bigquery:"unknown_plan"`
	SeatsSnapshot   string   `bigquery:"seats_snapshot"`
	TeamMembersFrom string   `bigquery:"team_members_from"`
}

// GetTeamYearOverview returns one team's months in year, with the same
// attribution as GetTeamGrossOverview and GetTeamNetOverview for each month.
func (bq *BigQueryClient) GetTeamYearOverview(ctx context.Context, team string, year int) (*TeamYearOverview, error) {
	// Like GetTeamNetOverview: the billing tables only exist after an operator
	// starts a backfill. Without them every month falls back to gross.
	net, residual := true, false
	if md, err := bq.client.Dataset(bq.metricsDataset).Table("billing_user_monthly_runs").Metadata(ctx); err == nil {
		residual = hasColumns(md.Schema, "enterprise_net", "residual_net")
	} else {
		var apiErr *googleapi.Error
		if !errors.As(err, &apiErr) || apiErr.Code != 404 {
			return nil, fmt.Errorf("read billing run metadata: %w", err)
		}
		net = false
	}
	seats, err := bq.tableExists(ctx, "copilot_seats")
	if err != nil {
		return nil, err
	}
	if seats {
		if seats, err = bq.tableExists(ctx, "team_members"); err != nil {
			return nil, err
		}
	}
	it, err := bq.teamYearQuery(team, year, net, residual, seats).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read team year usage: %w", err)
	}
	rows, err := readAllRows[teamYearRow](it)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UnknownPlan > 0 {
			slog.Warn("Seats with unknown plan_type priced as business", "month", row.Month, "seats", row.UnknownPlan)
		}
	}
	return teamYearOverview(team, year, rows, time.Now()), nil
}

// teamYearNetStubs replace teamNetCTEs and the run marker when the billing
// tables do not exist, so the query still yields gross for every month.
const teamYearNetStubs = `billed AS (
 SELECT DATE(NULL) month,'' user_id,0.0 net FROM UNNEST([1]) WHERE FALSE
), daily AS (
 SELECT DATE(NULL) day,'' user_id,0.0 net FROM UNNEST([1]) WHERE FALSE
), unspread AS (
 SELECT DATE(NULL) month,'' user_id,0.0 net FROM UNNEST([1]) WHERE FALSE
)`

// teamYearSeatsStub replaces teamYearSeatsCTEs before copilot-metrics has
// written its first seat snapshot or team_members day.
const teamYearSeatsStub = `seat_snap AS (SELECT DATE(NULL) d
), tm_first AS (SELECT DATE(NULL) d
), seat_m AS (
 SELECT DATE(NULL) month,0 seat_months,0.0 licence,0 unknown_plan FROM UNNEST([1]) WHERE FALSE
)`

// teamYearSeatsCTEs counts, per month, the team's members with a seat that
// covered the month. Members come from team_members by the slug the team
// had on the same day (a slug can be reused by another team ID); months before its first day use that day's members.
// Seats are month ranges from each login's latest snapshot. GitHub's
// pending_cancellation_date is the first day of the next billing cycle, so the
// seat's last month is the month of the day before. A seat that left the list
// without a cancellation date ends in the month it was last seen. Both scans
// start at @from: a seat or member last seen before the year cannot cover a
// month in it.
func (bq *BigQueryClient) teamYearSeatsCTEs() string {
	members := bq.tableRef(bq.metricsDataset, "team_members")
	return bq.seatCTEs() + fmt.Sprintf(`, tm_first AS (SELECT MIN(date) d FROM %[1]s
), tm AS (
 SELECT t.date,DATE_TRUNC(t.date,MONTH) month,LOWER(t.login) login FROM %[1]s t
 WHERE t.date>=DATE(@from) AND t.date<DATE(@to)
 AND EXISTS(SELECT 1 FROM memberships x WHERE x.team_id=@team AND x.day=t.date AND x.team_slug=t.team_slug)
), team_lic AS (
 SELECT DISTINCT m month,tm.login
 FROM UNNEST(GENERATE_DATE_ARRAY(DATE(@from),DATE_SUB(DATE(@to),INTERVAL 1 DAY),INTERVAL 1 MONTH)) m
 CROSS JOIN tm_first f JOIN tm ON IF(m<DATE_TRUNC(f.d,MONTH),tm.date=f.d,tm.month=m)
), seat_m AS (
 SELECT t.month,COUNT(*) seat_months,`+seatPriceCols+`
 FROM team_lic t JOIN seats s USING(login)
 WHERE s.from_m<=t.month AND (s.to_m IS NULL OR s.to_m>=t.month) GROUP BY t.month
)`, members)
}

// seatPriceCols prices the seats s in a seat_m aggregate at list price.
var seatPriceCols = fmt.Sprintf(`
  SUM(IF(s.plan_type='enterprise',%.2f,%.2f)) licence,
  COUNTIF(IFNULL(s.plan_type,'') NOT IN ('business','enterprise')) unknown_plan`, licenceEnterpriseUSD, licenceBusinessUSD)

// seatCTEs gives seat_snap (latest copilot_seats day) and seats (one month
// range per login from its latest snapshot), shared by the team and Nav views.
func (bq *BigQueryClient) seatCTEs() string {
	return fmt.Sprintf(`seat_snap AS (SELECT MAX(snapshot_date) d FROM %[1]s
), seats AS (
 SELECT LOWER(login) login,plan_type,DATE_TRUNC(DATE(created_at),MONTH) from_m,
  COALESCE(DATE_TRUNC(DATE_SUB(pending_cancellation_date,INTERVAL 1 DAY),MONTH),
   IF(snapshot_date<(SELECT d FROM seat_snap),DATE_TRUNC(snapshot_date,MONTH),NULL)) to_m
 FROM %[1]s WHERE snapshot_date>=DATE(@from) AND created_at IS NOT NULL
 QUALIFY ROW_NUMBER() OVER(PARTITION BY user_id ORDER BY snapshot_date DESC)=1
)`, bq.tableRef(bq.metricsDataset, "copilot_seats"))
}

func (bq *BigQueryClient) tableExists(ctx context.Context, table string) (bool, error) {
	if _, err := bq.client.Dataset(bq.metricsDataset).Table(table).Metadata(ctx); err != nil {
		var apiErr *googleapi.Error
		if !errors.As(err, &apiErr) || apiErr.Code != 404 {
			return false, fmt.Errorf("read %s metadata: %w", table, err)
		}
		return false, nil
	}
	return true, nil
}

// hasColumns reports whether schema has every named column. A runs table
// published by a copilot-metrics build older than #1596 lacks the residual
// columns, and a query naming them fails as a whole.
func hasColumns(schema bigquery.Schema, names ...string) bool {
	have := map[string]bool{}
	for _, f := range schema {
		have[f.Name] = true
	}
	for _, n := range names {
		if !have[n] {
			return false
		}
	}
	return true
}

func (bq *BigQueryClient) teamYearQuery(team string, year int, net, residual, seats bool) *bigquery.Query {
	first := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	netCTEs, completed := teamYearNetStubs, "SELECT DATE(NULL) month,CAST(NULL AS FLOAT64) residual_pct FROM UNNEST([1]) WHERE FALSE"
	if net {
		netCTEs = bq.teamNetCTEs()
		pct := "CAST(NULL AS FLOAT64)"
		if residual {
			pct = "SAFE_DIVIDE(residual_net,enterprise_net)*100"
		}
		completed = fmt.Sprintf("SELECT month,ANY_VALUE(%s) residual_pct FROM %s WHERE month>=DATE(@from) AND month<DATE(@to) AND scope_id='nav' AND status='complete' GROUP BY month",
			pct, bq.tableRef(bq.metricsDataset, "billing_user_monthly_runs"))
	}
	seatCTEs := teamYearSeatsStub
	if seats {
		seatCTEs = bq.teamYearSeatsCTEs()
	}
	query := bq.client.Query(`
WITH ` + bq.teamUsageCTEs() + `, ` + netCTEs + `, ` + seatCTEs + fmt.Sprintf(`, team AS (
 SELECT DISTINCT day,month,user_id FROM memberships WHERE team_id=@team
), gross_m AS (
 SELECT t.month,COUNT(DISTINCT IF(u.gross>0,t.user_id,NULL)) users,SUM(u.gross * 0.01) gross
 FROM team t JOIN usage_days u USING(day,user_id) GROUP BY t.month
), net_m AS (
 SELECT t.month,COUNT(DISTINCT IF(d.net>0,t.user_id,NULL)) users,SUM(d.net) net
 FROM team t JOIN daily d USING(day,user_id) GROUP BY t.month
), no_usage_m AS (
 SELECT b.month,SUM(b.net) net FROM unspread b JOIN (SELECT DISTINCT month,user_id FROM team) USING(month,user_id)
 GROUP BY b.month
), completed AS (
 %s
), coverage AS (
 SELECT
  (SELECT IFNULL(CAST(MIN(day) AS STRING),'') FROM %s WHERE day>=DATE(@from) AND day<DATE(@to)
    AND scope='enterprise' AND scope_id='nav') membership_from,
  (SELECT IFNULL(CAST(MIN(day) AS STRING),'') FROM %s WHERE day>=DATE(@from) AND day<DATE(@to)
    AND scope='enterprise' AND scope_id='nav' AND JSON_VALUE(raw_record,'$.ai_credits_used') IS NOT NULL) gross_from,
  (SELECT IFNULL(CAST(MAX(day) AS STRING),'') FROM usage_days) last_usage_day,
  (SELECT IFNULL(MAX_BY(team_slug,day),'') FROM memberships WHERE team_id=@team) team_slug,
  (SELECT IFNULL(CAST(d AS STRING),'') FROM seat_snap) seats_snapshot,
  (SELECT IFNULL(CAST(d AS STRING),'') FROM tm_first) team_members_from
)
SELECT CAST(m AS STRING) month,c.month IS NOT NULL net_complete,c.residual_pct,
 IFNULL(g.users,0) gross_users,IFNULL(g.gross,0) gross,IFNULL(n.users,0) net_users,IFNULL(n.net,0) net,
 IFNULL(z.net,0) no_usage_net,v.team_slug,v.membership_from,v.gross_from,v.last_usage_day,
 IFNULL(s.seat_months,0) seat_months,IFNULL(s.licence,0) licence,IFNULL(s.unknown_plan,0) unknown_plan,
 v.seats_snapshot,v.team_members_from
FROM UNNEST(GENERATE_DATE_ARRAY(DATE(@from),DATE_SUB(DATE(@to),INTERVAL 1 DAY),INTERVAL 1 MONTH)) m
CROSS JOIN coverage v
LEFT JOIN gross_m g ON g.month=m LEFT JOIN net_m n ON n.month=m
LEFT JOIN no_usage_m z ON z.month=m LEFT JOIN completed c ON c.month=m LEFT JOIN seat_m s ON s.month=m
ORDER BY month`,
		completed,
		bq.tableRef(bq.metricsDataset, "user_teams"),
		bq.tableRef(bq.metricsDataset, "user_metrics")))
	query.Parameters = append(monthRange(first, 12), bigquery.QueryParameter{Name: "team", Value: team})
	return query
}

// aiCreditStart is the first month billed as AI credits; earlier months use basisPremium.
const aiCreditStart = "2026-06"

// teamYearOverview picks each month's basis the way the team page does: net
// once billing is complete for a finished month, else gross where per-user
// gross and membership history exist. Months after now are dropped.
func teamYearOverview(teamID string, year int, rows []teamYearRow, now time.Time) *TeamYearOverview {
	out := &TeamYearOverview{TeamID: teamID, Year: year, Months: []TeamYearMonth{}, Coverage: TeamYearCoverage{NetMonths: []string{}}}
	current := now.UTC().Format("2006-01")
	for _, row := range rows {
		month := row.Month[:7]
		out.TeamSlug = row.TeamSlug
		out.Coverage = TeamYearCoverage{MembershipFrom: row.MembershipFrom, GrossFrom: row.GrossFrom, LastUsageDay: row.LastUsageDay, NetMonths: out.Coverage.NetMonths, SeatsSnapshot: row.SeatsSnapshot, TeamMembersFrom: row.TeamMembersFrom}
		if month > current {
			continue
		}
		m := TeamYearMonth{Month: month, Basis: basisNone}
		hasTeams := row.MembershipFrom != "" && month >= row.MembershipFrom[:7]
		hasGross := row.GrossFrom != "" && month >= row.GrossFrom[:7]
		switch {
		case month < aiCreditStart && row.NetComplete && month < current:
			m.Basis = basisPremium
		case hasTeams && hasGross && row.NetComplete && month < current:
			m.Basis = basisNet
			out.Coverage.NetMonths = append(out.Coverage.NetMonths, month)
		case hasTeams && hasGross:
			m.Basis = basisGross
		}
		// Billing per team has no minimum member count (owner decision
		// 2026-10-12): every month shows its real amounts.
		if m.Basis == basisNet || m.Basis == basisGross {
			m.GrossUSD = cents(row.Gross)
		}
		net := m.Basis == basisNet || m.Basis == basisPremium
		if net && row.ResidualPct != nil {
			m.ResidualPct = cents(*row.ResidualPct)
		}
		if net {
			m.NetUSD, m.NoUsageNetUSD = cents(row.Net), cents(row.NoUsageNet)
		}
		users := row.GrossUsers
		if net {
			users = row.NetUsers
		}
		if m.Basis != basisNone {
			m.Users = &users
		}
		if row.SeatsSnapshot != "" && row.TeamMembersFrom != "" {
			seatMonths := row.SeatMonths
			m.SeatMonths, m.LicenceUSDEstimate = &seatMonths, cents(row.Licence)
		}
		m.TotalUSD, m.TotalBasis = teamYearTotal(m), basisNone
		if m.TotalUSD != nil {
			m.TotalBasis = m.Basis
		}
		out.Months = append(out.Months, m)
	}
	return out
}

// teamYearTotal is the month's basis amount plus the licence estimate, nil
// when any part is missing.
func teamYearTotal(m TeamYearMonth) *float64 {
	if m.LicenceUSDEstimate == nil {
		return nil
	}
	switch {
	case (m.Basis == basisNet || m.Basis == basisPremium) && m.NetUSD != nil && m.NoUsageNetUSD != nil:
		return cents(*m.NetUSD + *m.NoUsageNetUSD + *m.LicenceUSDEstimate)
	case m.Basis == basisGross && m.GrossUSD != nil:
		return cents(*m.GrossUSD + *m.LicenceUSDEstimate)
	}
	return nil
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
