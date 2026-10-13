package main

import (
	"fmt"
	"strings"
)

// budgetShareFrom is the first day with ai_credits_used in user_metrics.
const budgetShareFrom = "2026-06-19"

// budgetShareCTE returns a read-only CTE, budget_share, with one row per user
// and day from budgetShareFrom: the personal limit (enterprise default or
// override), from the user's first day in user_metrics that month, gross usage from the 1st of the month, their ratio, and source.
// source is 'snapshot' where user_budget_snapshots holds GitHub's own figure
// for that user and day, else 'estimate' (cumulative gross from user_metrics).
// Pooled included credits are ignored on purpose: users do not see them.
// Snapshots are taken during the day, estimates use whole days.
// June 2026 starts on the 19th, so its month-to-date is low.
// Callers aggregate; never return rows per person.
func (bq *BigQueryClient) budgetShareCTE() string {
	return fmt.Sprintf(`budget_share AS (
  WITH u AS (
    SELECT day, JSON_VALUE(raw_record,'$.user_id') user_id,
      LOWER(JSON_VALUE(raw_record,'$.user_login')) login,
      COALESCE(SAFE_CAST(JSON_VALUE(raw_record,'$.ai_credits_used') AS FLOAT64),0) * 0.01 gross
    FROM %s
    WHERE day >= DATE '%s'
  ),
  um AS (
    SELECT DATE_TRUNC(day, MONTH) m, user_id, MIN(day) first_day, ARRAY_AGG(login ORDER BY day DESC LIMIT 1)[OFFSET(0)] login FROM u GROUP BY 1, 2
  ),
  spine AS (
    SELECT um.user_id, um.login, d day FROM um,
      UNNEST(GENERATE_DATE_ARRAY(um.first_day,
        LEAST(LAST_DAY(um.m), (SELECT MAX(day) FROM u)))) d
  ),
  mtd AS (
    SELECT s.user_id, s.login, s.day,
      SUM(COALESCE(u.gross, 0)) OVER (PARTITION BY s.user_id, DATE_TRUNC(s.day, MONTH) ORDER BY s.day) gross
    FROM spine s LEFT JOIN u USING (user_id, day)
  ),
  dflt AS (
    SELECT snapshot_date day, MAX(budget_amount) amt FROM %s
    WHERE scope_id='nav' AND budget_scope='multi_user_customer' GROUP BY 1
  ),
  ovr AS (
    SELECT snapshot_date day, LOWER(entity_name) login, MAX(budget_amount) amt FROM %[3]s
    WHERE scope_id='nav' AND budget_scope='user' GROUP BY 1, 2
  ),
  snap AS (
    SELECT snapshot_date day, LOWER(github_login) login, MAX(budget_amount) amt, MAX(consumed_amount) used FROM %s
    WHERE scope_id='nav' GROUP BY 1, 2
  )
  SELECT m.day, m.user_id,
    IF(s.login IS NULL, COALESCE(o.amt, d.amt, %s), s.amt) limit_usd,
    IF(s.login IS NULL, m.gross, s.used) used_usd,
    SAFE_DIVIDE(IF(s.login IS NULL, m.gross, s.used), IF(s.login IS NULL, COALESCE(o.amt, d.amt, %[5]s), s.amt)) share,
    IF(s.login IS NULL, 'estimate', 'snapshot') source
  FROM mtd m
  LEFT JOIN snap s ON s.day = m.day AND s.login = m.login
  LEFT JOIN ovr o ON o.day = m.day AND o.login = m.login
  LEFT JOIN dflt d ON d.day = m.day
)`,
		userDaysFrom(bq.tableRef(bq.metricsDataset, "user_metrics")), budgetShareFrom,
		bq.tableRef(bq.metricsDataset, "budget_snapshots"),
		bq.tableRef(bq.metricsDataset, "user_budget_snapshots"),
		spendLimitSQL("m.day"))
}

// spendLimitSQL is the documented default limit on day, for days without a
// budget_snapshots row (before 2026-07-03 and any missed day). Days before
// the first entry give NULL; the CTE starts after it.
func spendLimitSQL(day string) string {
	var b strings.Builder
	b.WriteString("CASE")
	for i := len(spendLimits) - 1; i >= 0; i-- {
		fmt.Fprintf(&b, " WHEN %s >= DATE '%s' THEN %g", day, spendLimits[i].from, spendLimits[i].usd)
	}
	b.WriteString(" END")
	return b.String()
}
