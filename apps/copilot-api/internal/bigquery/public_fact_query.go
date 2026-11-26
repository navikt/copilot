package bigquery

import (
	"github.com/navikt/copilot/apps/copilot-api/internal/cache"

	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

func (bq *BigQueryClient) GetFactAggregates(ctx context.Context) (*FactAggregates, error) {
	ref := bq.tableRef(bq.metricsDataset, "user_metrics")
	queryStr := fmt.Sprintf(`
WITH base AS (
  SELECT day, JSON_VALUE(raw_record, '$.user_login') AS u, raw_record AS r
  FROM %s
  WHERE scope = 'enterprise' AND day >= DATE '2026-06-01'
    AND (SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) > 0
      OR SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64) > 0)
),
last AS (SELECT MAX(day) AS d FROM base),
w AS (SELECT base.* FROM base, last WHERE day > DATE_SUB(d, INTERVAL 7 DAY)),
m28 AS (SELECT base.* FROM base, last WHERE day > DATE_SUB(d, INTERVAL 28 DAY)),
models AS (
  SELECT DISTINCT u, JSON_VALUE(mf, '$.model') AS model
  FROM w, UNNEST(JSON_QUERY_ARRAY(r, '$.totals_by_model_feature')) AS mf
  WHERE JSON_VALUE(mf, '$.model') NOT IN ('auto', 'unknown', 'others')
),
fam AS (
  SELECT CASE
      WHEN REGEXP_CONTAINS(model, r'opus') THEN 'Claude Opus'
      WHEN REGEXP_CONTAINS(model, r'sonnet') THEN 'Claude Sonnet'
      WHEN REGEXP_CONTAINS(model, r'haiku') THEN 'Claude Haiku'
      WHEN REGEXP_CONTAINS(model, r'^gpt.*(mini|small|nano)') THEN 'GPT mini'
      WHEN STARTS_WITH(model, 'gpt') THEN 'GPT'
      WHEN STARTS_WITH(model, 'gemini') THEN 'Gemini'
      ELSE 'andre' END AS family,
    COUNT(DISTINCT u) AS users
  FROM models GROUP BY family
),
daily AS (SELECT day, COUNT(DISTINCT u) AS n FROM base WHERE EXTRACT(DAYOFWEEK FROM day) BETWEEN 2 AND 6 GROUP BY day)
SELECT
  CAST((SELECT d FROM last) AS STRING) AS data_day,
  (SELECT COUNT(DISTINCT u) FROM w) AS week_users,
  (SELECT COUNT(DISTINCT u) FROM w WHERE SAFE_CAST(JSON_VALUE(r, '$.distinct_mcp_use_count') AS INT64) > 0) AS mcp_users,
  (SELECT COUNT(DISTINCT u) FROM w WHERE SAFE_CAST(JSON_VALUE(r, '$.distinct_skill_use_count') AS INT64) > 0) AS skill_users,
  (SELECT COUNT(DISTINCT u) FROM w WHERE SAFE_CAST(JSON_VALUE(r, '$.distinct_custom_agent_use_count') AS INT64) > 0) AS custom_agent_users,
  (SELECT COUNT(*) FROM (SELECT u FROM models GROUP BY u HAVING COUNT(*) >= 3)) AS users_3plus_models,
  (SELECT COUNT(DISTINCT u) FROM m28) AS month_users,
  (SELECT COUNT(*) FROM m28) AS month_user_days,
  (SELECT COUNTIF(EXTRACT(DAYOFWEEK FROM day) IN (1, 7)) FROM m28) AS weekend_user_days,
  (SELECT COUNT(DISTINCT u) FROM m28 WHERE EXTRACT(DAYOFWEEK FROM day) IN (1, 7)) AS weekend_users,
  (SELECT IFNULL(AVG(n), 0) FROM daily, last WHERE day > DATE_SUB(d, INTERVAL 28 DAY)) AS weekday_avg,
  (SELECT COUNT(DISTINCT u) FROM base WHERE day BETWEEN DATE '2026-06-03' AND DATE '2026-06-30') AS june_users,
  (SELECT IFNULL(AVG(n), 0) FROM daily WHERE day BETWEEN DATE '2026-06-01' AND DATE '2026-06-30') AS june_weekday_avg,
  (SELECT IFNULL(AVG(n), 0) FROM daily WHERE day BETWEEN DATE '2026-07-01' AND DATE '2026-07-31') AS july_weekday_avg,
  ARRAY(SELECT AS STRUCT family, users FROM fam WHERE family != 'andre' ORDER BY users DESC) AS families
`, ref)
	// ponytail: June/July 2026 are fixed baselines for the growth and holiday
	// facts; move them forward when a newer year's data is the better story.
	it, err := bq.client.Query(queryStr).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return ReadSingleRow[FactAggregates](it)
}

// GetFactAggregates is cached per Oslo date, so the day's fact is stable.
func (c *CachedBigQueryClient) GetFactAggregates(ctx context.Context) (*FactAggregates, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	date := time.Now().In(Oslo).Format(time.DateOnly)
	key := "fact_aggregates_" + date
	agg, err := getCachedValue(c, key, func() (*FactAggregates, error) {
		agg, err := c.client.GetFactAggregates(ctx)
		if err != nil {
			// Cache the miss as nil (no fact) for the TTL, so a failing query
			// cannot be re-run by every anonymous front-page view.
			slog.Error("Failed to fetch fact aggregates", "error", err)
			return nil, nil
		}
		return agg, nil
	})
	if err != nil || agg == nil {
		return agg, err
	}
	// The repo count is optional: wait at most adoptionBudget for it, so a slow
	// scan query never costs the front page its fact. The load keeps running
	// and fills the cache for the next request.
	ch := make(chan *AdoptionSummary, 1)
	go func() {
		ch <- cachedFactAdoption(c.cache, "fact_adoption_"+date, func() (*AdoptionSummary, error) {
			v, err, _ := c.group.Do("fact_adoption_"+date, func() (any, error) {
				qctx, cancel := withQueryTimeout(ctx)
				defer cancel()
				return c.client.GetAdoptionSummary(qctx)
			})
			s, _ := v.(*AdoptionSummary)
			return s, err
		})
	}()
	select {
	case s := <-ch:
		return withRepoCounts(agg, s), nil
	case <-time.After(adoptionBudget):
		slog.Warn("Adoption summary too slow for fact; serving without repo count")
		return agg, nil
	}
}

// cachedFactAdoption loads the repo scan once per key. A failure or an empty
// scan is cached as nil for negativeCacheTTL, so anonymous front-page views
// cannot re-run a failing query (#1572).
func cachedFactAdoption(c *cache.Cache, key string, load func() (*AdoptionSummary, error)) *AdoptionSummary {
	if v, ok := c.Get(key); ok {
		s, _ := v.(*AdoptionSummary)
		return s
	}
	s, err := load()
	if err != nil || s == nil {
		if err != nil {
			slog.Error("Failed to fetch adoption summary for fact", "error", err)
		}
		c.SetWithTTL(key, (*AdoptionSummary)(nil), negativeCacheTTL)
		return nil
	}
	c.SetWithTTL(key, s, 24*time.Hour)
	return s
}

const adoptionBudget = 2 * time.Second

// withRepoCounts adds the repo scan to a copy; the cached aggregates are shared.
func withRepoCounts(agg *FactAggregates, s *AdoptionSummary) *FactAggregates {
	if agg == nil || s == nil {
		return agg
	}
	a := *agg
	a.ActiveRepos = s.ActiveReposWithRecentCommits
	a.ActiveReposWithCustoms = int64(math.Round(s.AdoptionRateActiveOnly * float64(s.ActiveReposWithRecentCommits)))
	return &a
}
