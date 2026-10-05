package main

import (
	"context"
	"fmt"
	"sort"

	"cloud.google.com/go/bigquery"
)

type TeamUsageComposition struct {
	Providers  []string `json:"providers"`
	Categories []string `json:"categories"`
	Feature    string   `json:"feature"`
	Language   string   `json:"language"`
}

type teamCompositionRow struct {
	TeamID    string `bigquery:"team_id"`
	Dimension string `bigquery:"dimension"`
	Label     string `bigquery:"label"`
	Activity  int64  `bigquery:"activity"`
}

func (bq *BigQueryClient) getTeamUsageComposition(ctx context.Context, month string) (map[string]TeamUsageComposition, error) {
	query := bq.client.Query(fmt.Sprintf(`WITH users AS (
 SELECT day,JSON_VALUE(raw_record,'$.user_id') user_id,raw_record FROM %s
 WHERE day>=DATE(@month) AND day<DATE_ADD(DATE(@month),INTERVAL 1 MONTH) AND scope='enterprise' AND scope_id='nav'
), memberships AS (
 SELECT DISTINCT day,JSON_VALUE(raw_record,'$.user_id') user_id,JSON_VALUE(raw_record,'$.team_id') team_id FROM %s
 WHERE day>=DATE(@month) AND day<DATE_ADD(DATE(@month),INTERVAL 1 MONTH) AND scope='enterprise' AND scope_id='nav'
 AND JSON_VALUE(raw_record,'$.slug')!='nav-it-github-users'
), dimensions AS (
 SELECT u.day,u.user_id,d.dimension,
 CASE d.dimension WHEN 'model' THEN JSON_VALUE(item,'$.model') WHEN 'feature' THEN JSON_VALUE(item,'$.feature') ELSE JSON_VALUE(item,'$.language') END label,
 COALESCE(SAFE_CAST(IF(d.dimension='language',JSON_VALUE(item,'$.code_generation_activity_count'),JSON_VALUE(item,'$.user_initiated_interaction_count')) AS INT64),0) activity
 FROM users u CROSS JOIN UNNEST([
 STRUCT('model' AS dimension,JSON_QUERY_ARRAY(raw_record,'$.totals_by_model_feature') AS items),
 STRUCT('feature',JSON_QUERY_ARRAY(raw_record,'$.totals_by_feature')),
 STRUCT('language',JSON_QUERY_ARRAY(raw_record,'$.totals_by_language_feature'))
 ]) d CROSS JOIN UNNEST(d.items) item
), categories AS (
 SELECT m.team_id,d.dimension,d.label,SUM(d.activity) activity
 FROM dimensions d JOIN memberships m USING(day,user_id)
 WHERE d.activity>0 AND (d.dimension='model' OR (d.label IS NOT NULL AND d.label NOT IN ('others','unknown')))
 GROUP BY team_id,dimension,label HAVING dimension='model' OR COUNT(DISTINCT user_id)>=@minUsers
), ranked AS (
 SELECT *,ROW_NUMBER() OVER(PARTITION BY team_id,dimension ORDER BY activity DESC,label) rank FROM categories
)
SELECT team_id,dimension,IFNULL(label,'unknown') label,activity FROM ranked WHERE dimension='model' OR rank=1 ORDER BY team_id,dimension,rank`, bq.tableRef(bq.metricsDataset, "user_metrics"), bq.tableRef(bq.metricsDataset, "user_teams")))
	query.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month + "-01"}, {Name: "minUsers", Value: minTeamContributors}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read team usage composition: %w", err)
	}
	rows, err := readAllRows[teamCompositionRow](it)
	if err != nil {
		return nil, err
	}
	return aggregateTeamComposition(rows), nil
}

func aggregateTeamComposition(rows []teamCompositionRow) map[string]TeamUsageComposition {
	result := map[string]TeamUsageComposition{}
	providers := map[string]map[string]int64{}
	categories := map[string]map[string]int64{}
	for _, row := range rows {
		usage := result[row.TeamID]
		switch row.Dimension {
		case "model":
			if row.Activity <= 0 {
				continue
			}
			if providers[row.TeamID] == nil {
				providers[row.TeamID] = map[string]int64{}
				categories[row.TeamID] = map[string]int64{}
			}
			model := classifyModel(row.Label)
			providers[row.TeamID][model.Provider] += row.Activity
			categories[row.TeamID][model.Category] += row.Activity
		case "feature":
			usage.Feature = row.Label
		case "language":
			usage.Language = row.Label
		}
		result[row.TeamID] = usage
	}
	for id, usage := range result {
		usage.Providers = rankedComposition(providers[id])
		usage.Categories = rankedComposition(categories[id])
		result[id] = usage
	}
	return result
}

func rankedComposition(totals map[string]int64) []string {
	names := make([]string, 0, len(totals))
	for name := range totals {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if totals[names[i]] == totals[names[j]] {
			return names[i] < names[j]
		}
		return totals[names[i]] > totals[names[j]]
	})
	return names
}
