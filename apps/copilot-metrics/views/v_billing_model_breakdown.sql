CREATE OR REPLACE VIEW `%s.%s.v_billing_model_breakdown` AS
SELECT
  EXTRACT(YEAR FROM usage_month) AS year,
  EXTRACT(MONTH FROM usage_month) AS month,
  FORMAT_DATE('%Y-%m', usage_month) AS year_month,
  scope_id,
  model,
  SUM(gross_amount) AS gross_amount,
  SUM(net_amount) AS net_amount,
  ROUND(
    100.0 * SUM(net_amount) / NULLIF(
      SUM(SUM(net_amount)) OVER (PARTITION BY usage_month, scope_id),
      0
    ),
    2
  ) AS pct_of_monthly_net
FROM {{billing_usage}}
GROUP BY 1, 2, 3, 4, 5
ORDER BY year, month, scope_id, net_amount DESC;
