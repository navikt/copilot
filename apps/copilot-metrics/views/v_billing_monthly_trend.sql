CREATE OR REPLACE VIEW `%s.%s.v_billing_monthly_trend` AS
SELECT
  EXTRACT(YEAR FROM usage_month) AS year,
  EXTRACT(MONTH FROM usage_month) AS month,
  FORMAT_DATE('%Y-%m', usage_month) AS year_month,
  scope_id,
  SUM(gross_amount) AS total_gross_amount,
  SUM(net_amount) AS total_net_amount,
  CASE
    WHEN SUM(gross_amount) > 0
    THEN ROUND(100.0 * (1 - SUM(net_amount) / SUM(gross_amount)), 2)
    ELSE 0
  END AS discount_rate_pct,
  COUNT(DISTINCT model) AS distinct_models
FROM {{billing_usage}}
GROUP BY 1, 2, 3, 4
ORDER BY year, month, scope_id;
