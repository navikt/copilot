package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"cloud.google.com/go/bigquery"
)

// AdminMonth is one month of the whole Nav bill for the admin cost page.
// Net months reconcile: AssignedNetUSD + ResidualNetUSD = EnterpriseNetUSD.
// Amounts are nil when the month's basis has none.
type AdminMonth struct {
	Month string `json:"month"`
	Basis string `json:"basis"` // basisNet, basisPremium, basisGross or basisNone
	// EnterpriseNetUSD is the net bill for the enterprise, from the
	// billing run that published the month.
	EnterpriseNetUSD *float64 `json:"enterprise_net_usd"`
	// AssignedNetUSD is net billed to a person (billing_user_monthly).
	AssignedNetUSD *float64 `json:"assigned_net_usd"`
	// ResidualNetUSD is net the run could not assign to a person.
	ResidualNetUSD *float64 `json:"residual_net_usd"`
	ResidualPct    *float64 `json:"residual_pct"`
	// NoTeamNetUSD is assigned net that lands on no team, split the same way
	// as team-net. Nil before the first user_teams month.
	NoTeamNetUSD *float64 `json:"no_team_net_usd"`
	// GrossUSD is the preliminary list-price amount for a month without net.
	GrossUSD           *float64 `json:"gross_usd"`
	Seats              *int64   `json:"seats"`
	LicenceUSDEstimate *float64 `json:"licence_usd_estimate"`
	// TotalUSD is net (or gross) plus the licence estimate, nil when a part is missing.
	TotalUSD *float64 `json:"total_usd"`
	LoadedAt string   `json:"loaded_at"` // when net was read in, "" without net
}

type AdminMonthlyCoverage struct {
	MembershipFrom string `json:"membership_from"`
	LastGrossDay   string `json:"last_gross_day"`
	// GrossStale is true when the last gross day is more than two days old.
	GrossStale    bool   `json:"gross_stale"`
	LastNetMonth  string `json:"last_net_month"`
	SeatsSnapshot string `json:"seats_snapshot"`
}

type AdminMonthlyOverview struct {
	Year     int                  `json:"year"`
	Months   []AdminMonth         `json:"months"`
	Coverage AdminMonthlyCoverage `json:"coverage"`
}

type adminMonthRow struct {
	Month          string   `bigquery:"month"`
	NetComplete    bool     `bigquery:"net_complete"`
	EnterpriseNet  *float64 `bigquery:"enterprise_net"`
	ResidualNet    *float64 `bigquery:"residual_net"`
	LoadedAt       string   `bigquery:"loaded_at"`
	AssignedNet    float64  `bigquery:"assigned_net"`
	TeamNet        float64  `bigquery:"team_net"`
	Gross          *float64 `bigquery:"gross"`
	Seats          int64    `bigquery:"seats"`
	Licence        float64  `bigquery:"licence"`
	UnknownPlan    int64    `bigquery:"unknown_plan"`
	MembershipFrom string   `bigquery:"membership_from"`
	LastGrossDay   string   `bigquery:"last_gross_day"`
	SeatsSnapshot  string   `bigquery:"seats_snapshot"`
}

// GetAdminMonthlyOverview returns every month of year up to now for all of
// Nav. Net is split per person and team by the shared teamNetCTEs, so the
// no-team amount is the same as on the team pages.
func (bq *BigQueryClient) GetAdminMonthlyOverview(ctx context.Context, year int) (*AdminMonthlyOverview, error) {
	it, err := bq.adminMonthlyQuery(year).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read admin monthly usage: %w", err)
	}
	rows, err := readAllRows[adminMonthRow](it)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UnknownPlan > 0 {
			slog.Warn("Seats with unknown plan_type priced as business", "month", row.Month, "seats", row.UnknownPlan)
		}
	}
	return adminMonthlyOverview(year, rows, time.Now()), nil
}

func (bq *BigQueryClient) adminMonthlyQuery(year int) *bigquery.Query {
	query := bq.client.Query(`
WITH ` + bq.teamUsageCTEs() + `, ` + bq.teamNetCTEs() + `, ` + bq.seatCTEs() + fmt.Sprintf(`, months AS (
 SELECT m month FROM UNNEST(GENERATE_DATE_ARRAY(DATE(@from),DATE_SUB(DATE(@to),INTERVAL 1 DAY),INTERVAL 1 MONTH)) m
), runs AS (
 -- A rerun adds rows; the latest complete run is the one billed reflects.
 SELECT month,ARRAY_AGG(enterprise_net ORDER BY loaded_at DESC LIMIT 1)[OFFSET(0)] enterprise_net,
  ARRAY_AGG(residual_net ORDER BY loaded_at DESC LIMIT 1)[OFFSET(0)] residual_net,
  CAST(MAX(loaded_at) AS STRING) loaded_at
 FROM %[1]s WHERE month>=DATE(@from) AND month<DATE(@to) AND scope_id='nav' AND status='complete' GROUP BY month
), assigned_m AS (
 SELECT month,SUM(net) net FROM billed GROUP BY month
), team_m AS (
 SELECT DATE_TRUNC(day,MONTH) month,SUM(net) net FROM daily w
 WHERE EXISTS (SELECT 1 FROM memberships m WHERE m.day=w.day AND m.user_id=w.user_id) GROUP BY month
), gross_m AS (
 SELECT DATE_TRUNC(day,MONTH) month,SUM(gross_amount) gross FROM %[2]s
 WHERE day>=DATE(@from) AND day<DATE(@to) AND scope_id='nav' AND product='Copilot' AND sku IN (`+billedSKUs+`)
 GROUP BY month
), seat_m AS (
 SELECT t.month,COUNT(*) seats,`+seatPriceCols+`
 FROM months t CROSS JOIN seats s
 WHERE s.from_m<=t.month AND (s.to_m IS NULL OR s.to_m>=t.month) GROUP BY t.month
), coverage AS (
 SELECT
  (SELECT IFNULL(CAST(MIN(day) AS STRING),'') FROM %[3]s WHERE scope='enterprise' AND scope_id='nav') membership_from,
  (SELECT IFNULL(CAST(MAX(day) AS STRING),'') FROM %[2]s WHERE scope_id='nav' AND product='Copilot') last_gross_day,
  (SELECT IFNULL(CAST(d AS STRING),'') FROM seat_snap) seats_snapshot
)
SELECT CAST(t.month AS STRING) month,r.month IS NOT NULL net_complete,r.enterprise_net,r.residual_net,
 IFNULL(r.loaded_at,'') loaded_at,IFNULL(a.net,0) assigned_net,IFNULL(tm.net,0) team_net,g.gross,
 IFNULL(s.seats,0) seats,IFNULL(s.licence,0) licence,IFNULL(s.unknown_plan,0) unknown_plan,
 v.membership_from,v.last_gross_day,v.seats_snapshot
FROM months t CROSS JOIN coverage v
LEFT JOIN runs r ON r.month=t.month LEFT JOIN assigned_m a ON a.month=t.month
LEFT JOIN team_m tm ON tm.month=t.month LEFT JOIN gross_m g ON g.month=t.month
LEFT JOIN seat_m s ON s.month=t.month
ORDER BY month`,
		bq.tableRef(bq.metricsDataset, "billing_user_monthly_runs"),
		bq.tableRef(bq.metricsDataset, "billing_usage_daily_model"),
		bq.tableRef(bq.metricsDataset, "user_teams")))
	query.Parameters = monthRange(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), 12)
	return query
}

// adminMonthlyOverview picks each month's basis like teamYearOverview: net
// (premium before aiCreditStart) once the run is complete for a finished
// month, else preliminary gross. Months after now are dropped.
func adminMonthlyOverview(year int, rows []adminMonthRow, now time.Time) *AdminMonthlyOverview {
	out := &AdminMonthlyOverview{Year: year, Months: []AdminMonth{}}
	current := now.UTC().Format("2006-01")
	for _, row := range rows {
		month := row.Month[:7]
		out.Coverage = AdminMonthlyCoverage{MembershipFrom: row.MembershipFrom, LastGrossDay: row.LastGrossDay, SeatsSnapshot: row.SeatsSnapshot, LastNetMonth: out.Coverage.LastNetMonth}
		if month > current {
			continue
		}
		m := AdminMonth{Month: month, Basis: basisNone}
		switch {
		case row.NetComplete && month < current && row.EnterpriseNet != nil && row.ResidualNet != nil:
			m.Basis = basisNet
			if month < aiCreditStart {
				m.Basis = basisPremium
			}
			out.Coverage.LastNetMonth = month
			m.EnterpriseNetUSD, m.AssignedNetUSD, m.ResidualNetUSD = cents(*row.EnterpriseNet), cents(row.AssignedNet), cents(*row.ResidualNet)
			if *row.EnterpriseNet != 0 {
				m.ResidualPct = cents(*row.ResidualNet / *row.EnterpriseNet * 100)
			}
			if row.MembershipFrom != "" && month >= row.MembershipFrom[:7] {
				m.NoTeamNetUSD = cents(row.AssignedNet - row.TeamNet)
			}
			m.LoadedAt = row.LoadedAt
		case row.Gross != nil:
			m.Basis = basisGross
			m.GrossUSD = cents(*row.Gross)
		}
		if row.SeatsSnapshot != "" {
			seats := row.Seats
			m.Seats, m.LicenceUSDEstimate = &seats, cents(row.Licence)
		}
		amount := m.EnterpriseNetUSD
		if amount == nil {
			amount = m.GrossUSD
		}
		if amount != nil && m.LicenceUSDEstimate != nil {
			m.TotalUSD = cents(*amount + *m.LicenceUSDEstimate)
		}
		out.Months = append(out.Months, m)
	}
	if day, err := time.Parse("2006-01-02", out.Coverage.LastGrossDay); err == nil {
		out.Coverage.GrossStale = now.Sub(day) > 72*time.Hour // the day itself plus two days
	}
	return out
}

func (c *CachedBigQueryClient) GetAdminMonthlyOverview(ctx context.Context, year int) (*AdminMonthlyOverview, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "admin_monthly_"+strconv.Itoa(year), func() (*AdminMonthlyOverview, error) {
		return c.client.GetAdminMonthlyOverview(ctx, year)
	})
}
