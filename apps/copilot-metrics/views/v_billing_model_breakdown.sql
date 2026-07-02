CREATE OR REPLACE VIEW `%s.%s.v_billing_model_breakdown` AS
WITH monthly AS (
  SELECT
    EXTRACT(YEAR FROM day) AS year,
    EXTRACT(MONTH FROM day) AS month,
    FORMAT_DATE('%Y-%m', DATE_TRUNC(day, MONTH)) AS year_month,
    scope_id,
    model,
    SUM(gross_amount) AS gross_amount,
    SUM(net_amount) AS net_amount
  FROM {{billing_usage_daily_model}}
  GROUP BY 1, 2, 3, 4, 5
)
SELECT
  year,
  month,
  year_month,
  scope_id,
  model,
  gross_amount,
  net_amount,
  ROUND(
    100.0 * net_amount / NULLIF(
      SUM(net_amount) OVER (PARTITION BY year, month, scope_id),
      0
    ),
    2
  ) AS pct_of_monthly_net
FROM monthly
ORDER BY year, month, scope_id, net_amount DESC;
