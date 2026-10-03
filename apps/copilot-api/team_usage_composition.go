package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
)

type TeamUsageComposition struct {
	Models   []string `json:"models"`
	Feature  string   `json:"feature"`
	Language string   `json:"language"`
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
 WHERE d.activity>0 AND d.label IS NOT NULL AND d.label NOT IN ('others','unknown')
 GROUP BY team_id,dimension,label HAVING COUNT(DISTINCT user_id)>=@minUsers
), ranked AS (
 SELECT *,ROW_NUMBER() OVER(PARTITION BY team_id,dimension ORDER BY activity DESC,label) rank FROM categories
)
SELECT team_id,dimension,label FROM ranked WHERE rank<=IF(dimension='model',3,1) ORDER BY team_id,dimension,rank`, bq.tableRef(bq.metricsDataset, "user_metrics"), bq.tableRef(bq.metricsDataset, "user_teams")))
	query.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month + "-01"}, {Name: "minUsers", Value: minTeamContributors}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read team usage composition: %w", err)
	}
	rows, err := readAllRows[struct {
		TeamID    string `bigquery:"team_id"`
		Dimension string `bigquery:"dimension"`
		Label     string `bigquery:"label"`
	}](it)
	if err != nil {
		return nil, err
	}
	result := map[string]TeamUsageComposition{}
	for _, row := range rows {
		usage := result[row.TeamID]
		if usage.Models == nil {
			usage.Models = []string{}
		}
		switch row.Dimension {
		case "model":
			usage.Models = append(usage.Models, row.Label)
		case "feature":
			usage.Feature = row.Label
		case "language":
			usage.Language = row.Label
		}
		result[row.TeamID] = usage
	}
	return result, nil
}
