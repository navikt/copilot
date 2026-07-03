package main

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

type ModelInteractions struct {
	Model        string `bigquery:"model" json:"model"`
	Interactions int64  `bigquery:"interactions" json:"interactions"`
}

type TeamUsageSummary struct {
	TeamSlug            string              `bigquery:"team_slug" json:"team_slug"`
	AvgActiveUsers      int64               `bigquery:"avg_active_users" json:"avg_active_users"`
	TotalUsers          int64               `bigquery:"total_users" json:"total_users"`
	TotalGenerations    int64               `bigquery:"total_generations" json:"total_generations"`
	TotalAcceptances    int64               `bigquery:"total_acceptances" json:"total_acceptances"`
	TotalInteractions   int64               `bigquery:"total_interactions" json:"total_interactions"`
	TotalLinesSuggested int64               `bigquery:"total_lines_suggested" json:"total_lines_suggested"`
	TotalLinesAccepted  int64               `bigquery:"total_lines_accepted" json:"total_lines_accepted"`
	AgentUsers          int64               `bigquery:"agent_users" json:"agent_users"`
	DaysWithData        int64               `bigquery:"days_with_data" json:"days_with_data"`
	TopModels           []ModelInteractions `bigquery:"top_models" json:"top_models,omitempty"`
}

type UserMetricsSummary struct {
	UserLogin           string              `bigquery:"user_login" json:"user_login"`
	TotalAcceptances    int64               `bigquery:"total_acceptances" json:"total_acceptances"`
	TotalInteractions   int64               `bigquery:"total_interactions" json:"total_interactions"`
	TotalGenerations    int64               `bigquery:"total_generations" json:"total_generations"`
	TotalLinesSuggested int64               `bigquery:"total_lines_suggested" json:"total_lines_suggested"`
	TotalLinesAccepted  int64               `bigquery:"total_lines_accepted" json:"total_lines_accepted"`
	TotalLinesDeleted   int64               `bigquery:"total_lines_deleted" json:"total_lines_deleted"`
	ActiveDays          int64               `bigquery:"active_days" json:"active_days"`
	DaysInPeriod        int64               `bigquery:"days_in_period" json:"days_in_period"`
	DaysUsedAgent       int64               `bigquery:"days_used_agent" json:"days_used_agent"`
	DaysUsedChat        int64               `bigquery:"days_used_chat" json:"days_used_chat"`
	DaysUsedCLI         int64               `bigquery:"days_used_cli" json:"days_used_cli"`
	DaysUsedCodeReview  int64               `bigquery:"days_used_code_review" json:"days_used_code_review"`
	ChatAgentRequests   int64               `bigquery:"chat_agent_requests" json:"chat_agent_requests"`
	ChatAskRequests     int64               `bigquery:"chat_ask_requests" json:"chat_ask_requests"`
	ChatEditRequests    int64               `bigquery:"chat_edit_requests" json:"chat_edit_requests"`
	ChatPlanRequests    int64               `bigquery:"chat_plan_requests" json:"chat_plan_requests"`
	ChatCustomRequests  int64               `bigquery:"chat_custom_requests" json:"chat_custom_requests"`
	CLITotalRequests    int64               `bigquery:"cli_total_requests" json:"cli_total_requests"`
	CLIPrompts          int64               `bigquery:"cli_prompts" json:"cli_prompts"`
	CLISessions         int64               `bigquery:"cli_sessions" json:"cli_sessions"`
	CLIPromptTokens     int64               `bigquery:"cli_prompt_tokens" json:"cli_prompt_tokens"`
	CLIOutputTokens     int64               `bigquery:"cli_output_tokens" json:"cli_output_tokens"`
	TopModels           []ModelInteractions `bigquery:"top_models" json:"top_models"`
	Teams               []string            `bigquery:"teams" json:"teams"`
}

type MonthlyTrend struct {
	Month           string `bigquery:"month" json:"month"`
	DaysInMonth     int64  `bigquery:"days_in_month" json:"days_in_month"`
	UniqueUsers     int64  `bigquery:"unique_users" json:"unique_users"`
	IDEInteractions int64  `bigquery:"ide_interactions" json:"ide_interactions"`
	CodeGenerations int64  `bigquery:"code_generations" json:"code_generations"`
	CLIRequests     int64  `bigquery:"cli_requests" json:"cli_requests"`
	PromptTokens    int64  `bigquery:"prompt_tokens" json:"prompt_tokens"`
	OutputTokens    int64  `bigquery:"output_tokens" json:"output_tokens"`
	LinesAdded      int64  `bigquery:"lines_added" json:"lines_added"`
	LinesDeleted    int64  `bigquery:"lines_deleted" json:"lines_deleted"`
	Acceptances     int64  `bigquery:"acceptances" json:"acceptances"`
	AgentUsers      int64  `bigquery:"agent_users" json:"agent_users"`
	ChatUsers       int64  `bigquery:"chat_users" json:"chat_users"`
	CLIUsers        int64  `bigquery:"cli_users" json:"cli_users"`
}

type MonthlyModelUsage struct {
	Month        string `bigquery:"month" json:"month"`
	Model        string `bigquery:"model" json:"model"`
	Interactions int64  `bigquery:"interactions" json:"interactions"`
}

type MonthlyBillingUsage struct {
	Month         string  `bigquery:"month" json:"month"`
	Model         string  `bigquery:"model" json:"model"`
	SKU           string  `bigquery:"sku" json:"sku"`
	GrossRequests int64   `bigquery:"gross_requests" json:"gross_requests"`
	NetRequests   int64   `bigquery:"net_requests" json:"net_requests"`
	GrossAmount   float64 `bigquery:"gross_amount" json:"gross_amount"`
	NetAmount     float64 `bigquery:"net_amount" json:"net_amount"`
}

type BillingMonthlyTrend struct {
	YearMonth        string  `bigquery:"year_month" json:"year_month"`
	TotalGrossAmount float64 `bigquery:"total_gross_amount" json:"total_gross_amount"`
	TotalNetAmount   float64 `bigquery:"total_net_amount" json:"total_net_amount"`
	DiscountRatePct  float64 `bigquery:"discount_rate_pct" json:"discount_rate_pct"`
	DistinctModels   int64   `bigquery:"distinct_models" json:"distinct_models"`
}

type BillingModelBreakdown struct {
	YearMonth       string  `bigquery:"year_month" json:"year_month"`
	Model           string  `bigquery:"model" json:"model"`
	GrossAmount     float64 `bigquery:"gross_amount" json:"gross_amount"`
	NetAmount       float64 `bigquery:"net_amount" json:"net_amount"`
	PctOfMonthlyNet float64 `bigquery:"pct_of_monthly_net" json:"pct_of_monthly_net"`
}

type BillingModelDailyCost struct {
	Day           string  `bigquery:"day" json:"day"`
	Model         string  `bigquery:"model" json:"model"`
	GrossRequests int64   `bigquery:"gross_requests" json:"gross_requests"`
	NetRequests   int64   `bigquery:"net_requests" json:"net_requests"`
	GrossAmount   float64 `bigquery:"gross_amount" json:"gross_amount"`
	NetAmount     float64 `bigquery:"net_amount" json:"net_amount"`
}

type BillingModelForecastPoint struct {
	Day                 string   `json:"day"`
	ActualCumulative    *float64 `json:"actual_cumulative,omitempty"`
	ProjectedCumulative float64  `json:"projected_cumulative"`
	IsActual            bool     `json:"is_actual"`
}

type BillingModelForecast struct {
	Month                 string                      `json:"month"`
	DaysInMonth           int                         `json:"days_in_month"`
	DaysElapsed           int                         `json:"days_elapsed"`
	LastActualDay         string                      `json:"last_actual_day,omitempty"`
	ActualMTDNetAmount    float64                     `json:"actual_mtd_net_amount"`
	ProjectedDailyRunRate float64                     `json:"projected_daily_run_rate"`
	ProjectedEOMNetAmount float64                     `json:"projected_eom_net_amount"`
	LowerEOMNetAmount     float64                     `json:"lower_eom_net_amount"`
	UpperEOMNetAmount     float64                     `json:"upper_eom_net_amount"`
	Points                []BillingModelForecastPoint `json:"points"`
}

type WeeklyTrend struct {
	Week         string              `bigquery:"week" json:"week"`
	Interactions int64               `bigquery:"interactions" json:"interactions"`
	CLIRequests  int64               `bigquery:"cli_requests" json:"cli_requests"`
	Acceptances  int64               `bigquery:"acceptances" json:"acceptances"`
	LinesAdded   int64               `bigquery:"lines_added" json:"lines_added"`
	LinesDeleted int64               `bigquery:"lines_deleted" json:"lines_deleted"`
	PromptTokens int64               `bigquery:"prompt_tokens" json:"prompt_tokens"`
	OutputTokens int64               `bigquery:"output_tokens" json:"output_tokens"`
	ActiveDays   int64               `bigquery:"active_days" json:"active_days"`
	Models       []ModelInteractions `bigquery:"models" json:"models,omitempty"`
}

type AdoptionCohortDay struct {
	Day             civil.Date `bigquery:"day" json:"day"`
	Phase           int64      `bigquery:"phase" json:"phase"`
	PhaseVersion    string     `bigquery:"phase_version" json:"phase_version"`
	UserCount       int64      `bigquery:"user_count" json:"user_count"`
	AvgGenerations  float64    `bigquery:"avg_generations" json:"avg_generations"`
	AvgAcceptances  float64    `bigquery:"avg_acceptances" json:"avg_acceptances"`
	AvgInteractions float64    `bigquery:"avg_interactions" json:"avg_interactions"`
	AvgLinesAdded   float64    `bigquery:"avg_lines_added" json:"avg_lines_added"`
}

func (bq *BigQueryClient) GetTeamUsageSummary(ctx context.Context, days int) ([]TeamUsageSummary, error) {
	teamsRef := bq.tableRef(bq.metricsDataset, "user_teams")
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	queryStr := fmt.Sprintf(`
      WITH latest_teams AS (
        SELECT
          JSON_VALUE(raw_record, '$.user_id') AS user_id,
          JSON_VALUE(raw_record, '$.slug') AS team_slug
        FROM %s
        WHERE day = (SELECT MAX(day) FROM %s WHERE scope = 'enterprise')
          AND scope = 'enterprise'
        GROUP BY user_id, team_slug
      ),
      metrics AS (
        SELECT
          JSON_VALUE(raw_record, '$.user_id') AS user_id,
          day,
          SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64) AS generations,
          SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
          SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) AS interactions,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_suggested_to_add_sum') AS INT64) AS lines_suggested,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64) AS lines_accepted,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_agent') AS BOOL) AS used_agent,
          raw_record
        FROM %s
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND scope = 'enterprise'
      ),
      team_metrics AS (
        SELECT
          t.team_slug,
          m.user_id,
          m.day,
          COALESCE(m.generations, 0) AS generations,
          COALESCE(m.acceptances, 0) AS acceptances,
          COALESCE(m.interactions, 0) AS interactions,
          COALESCE(m.lines_suggested, 0) AS lines_suggested,
          COALESCE(m.lines_accepted, 0) AS lines_accepted,
          COALESCE(m.used_agent, FALSE) AS used_agent,
          m.raw_record
        FROM latest_teams t
        INNER JOIN metrics m ON t.user_id = m.user_id
      ),
      team_model_usage AS (
        SELECT
          tm.team_slug,
          JSON_VALUE(mf, '$.model') AS model,
          SUM(SAFE_CAST(JSON_VALUE(mf, '$.user_initiated_interaction_count') AS INT64))
            + SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_generation_activity_count') AS INT64))
            + SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_acceptance_activity_count') AS INT64)) AS interactions
        FROM team_metrics tm,
          UNNEST(JSON_QUERY_ARRAY(tm.raw_record, '$.totals_by_model_feature')) AS mf
        WHERE JSON_VALUE(mf, '$.model') IS NOT NULL
          AND JSON_VALUE(mf, '$.model') != 'others'
        GROUP BY tm.team_slug, model
        HAVING interactions > 0
      ),
      team_model_ranked AS (
        SELECT
          team_slug,
          model,
          interactions,
          ROW_NUMBER() OVER (PARTITION BY team_slug ORDER BY interactions DESC) AS rn
        FROM team_model_usage
      ),
      team_models_agg AS (
        SELECT
          team_slug,
          ARRAY_AGG(STRUCT(model, interactions) ORDER BY interactions DESC) AS top_models
        FROM team_model_ranked
        WHERE rn <= 3
        GROUP BY team_slug
      ),
      team_summary AS (
        SELECT
          tm.team_slug,
          COUNT(DISTINCT CASE WHEN tm.acceptances + tm.interactions > 0 THEN tm.user_id END) AS avg_active_users,
          COUNT(DISTINCT tm.user_id) AS total_users,
          SUM(tm.generations) AS total_generations,
          SUM(tm.acceptances) AS total_acceptances,
          SUM(tm.interactions) AS total_interactions,
          SUM(tm.lines_suggested) AS total_lines_suggested,
          SUM(tm.lines_accepted) AS total_lines_accepted,
          COUNT(DISTINCT CASE WHEN tm.used_agent THEN tm.user_id END) AS agent_users,
          COUNT(DISTINCT tm.day) AS days_with_data
        FROM team_metrics tm
        GROUP BY tm.team_slug
      )
      SELECT
        ts.*,
        tma.top_models
      FROM team_summary ts
      LEFT JOIN team_models_agg tma ON tma.team_slug = ts.team_slug
      ORDER BY ts.avg_active_users DESC
    `, teamsRef, teamsRef, metricsRef)

	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "days", Value: days}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[TeamUsageSummary](it)
}

func (bq *BigQueryClient) GetUserMetrics(ctx context.Context, userLogin string, days int) (*UserMetricsSummary, error) {
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	teamsRef := bq.tableRef(bq.metricsDataset, "user_teams")
	queryStr := fmt.Sprintf(`
      WITH user_activity AS (
        SELECT
          JSON_VALUE(raw_record, '$.user_login') AS user_login,
          SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64) AS generations,
          SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
          SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) AS interactions,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_suggested_to_add_sum') AS INT64) AS lines_suggested,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64) AS lines_accepted,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_deleted_sum') AS INT64) AS lines_deleted,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_agent') AS BOOL) AS used_agent,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_chat') AS BOOL) AS used_chat,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_cli') AS BOOL) AS used_cli,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_copilot_code_review_active') AS BOOL) AS used_code_review,
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_agent_mode') AS INT64) AS chat_agent_mode,
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_ask_mode') AS INT64) AS chat_ask_mode,
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_edit_mode') AS INT64) AS chat_edit_mode,
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_plan_mode') AS INT64) AS chat_plan_mode,
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_custom_mode') AS INT64) AS chat_custom_mode,
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64) AS cli_requests,
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.prompt_count') AS INT64) AS cli_prompts,
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.session_count') AS INT64) AS cli_sessions,
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.prompt_tokens_sum') AS INT64) AS cli_prompt_tokens,
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.output_tokens_sum') AS INT64) AS cli_output_tokens,
          raw_record
        FROM %s
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
      ),
      user_team_list AS (
        SELECT DISTINCT JSON_VALUE(raw_record, '$.slug') AS team_slug
        FROM %s
        WHERE day = (SELECT MAX(day) FROM %s WHERE scope = 'enterprise')
          AND scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
      ),
      model_usage AS (
        SELECT
          JSON_VALUE(mf, '$.model') AS model,
          SUM(SAFE_CAST(JSON_VALUE(mf, '$.user_initiated_interaction_count') AS INT64))
            + SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_generation_activity_count') AS INT64))
            + SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_acceptance_activity_count') AS INT64)) AS interactions
        FROM user_activity,
          UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_model_feature')) AS mf
        WHERE JSON_VALUE(mf, '$.model') IS NOT NULL
          AND JSON_VALUE(mf, '$.model') != 'others'
        GROUP BY model
        HAVING interactions > 0
        ORDER BY interactions DESC
        LIMIT 5
      ),
      model_agg AS (
        SELECT ARRAY_AGG(STRUCT(model, interactions) ORDER BY interactions DESC) AS top_models
        FROM model_usage
      ),
      team_agg AS (
        SELECT ARRAY_AGG(team_slug) AS teams
        FROM user_team_list
      )
      SELECT
        @userLogin AS user_login,
        COALESCE(SUM(ua.generations), 0) AS total_generations,
        COALESCE(SUM(ua.acceptances), 0) AS total_acceptances,
        COALESCE(SUM(ua.interactions), 0) AS total_interactions,
        COALESCE(SUM(ua.lines_suggested), 0) AS total_lines_suggested,
        COALESCE(SUM(ua.lines_accepted), 0) AS total_lines_accepted,
        COALESCE(SUM(ua.lines_deleted), 0) AS total_lines_deleted,
        COUNTIF(COALESCE(ua.acceptances, 0) + COALESCE(ua.interactions, 0) > 0) AS active_days,
        COUNT(*) AS days_in_period,
        COUNTIF(ua.used_agent) AS days_used_agent,
        COUNTIF(ua.used_chat) AS days_used_chat,
        COUNTIF(ua.used_cli) AS days_used_cli,
        COUNTIF(ua.used_code_review) AS days_used_code_review,
        COALESCE(SUM(ua.chat_agent_mode), 0) AS chat_agent_requests,
        COALESCE(SUM(ua.chat_ask_mode), 0) AS chat_ask_requests,
        COALESCE(SUM(ua.chat_edit_mode), 0) AS chat_edit_requests,
        COALESCE(SUM(ua.chat_plan_mode), 0) AS chat_plan_requests,
        COALESCE(SUM(ua.chat_custom_mode), 0) AS chat_custom_requests,
        COALESCE(SUM(ua.cli_requests), 0) AS cli_total_requests,
        COALESCE(SUM(ua.cli_prompts), 0) AS cli_prompts,
        COALESCE(SUM(ua.cli_sessions), 0) AS cli_sessions,
        COALESCE(SUM(ua.cli_prompt_tokens), 0) AS cli_prompt_tokens,
        COALESCE(SUM(ua.cli_output_tokens), 0) AS cli_output_tokens,
        ANY_VALUE(ma.top_models) AS top_models,
        ANY_VALUE(ta.teams) AS teams
      FROM user_activity ua
      CROSS JOIN model_agg ma
      CROSS JOIN team_agg ta
    `, metricsRef, teamsRef, teamsRef)

	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "days", Value: days}, {Name: "userLogin", Value: userLogin}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	summaryPtr, err := readSingleRow[UserMetricsSummary](it)
	if err != nil {
		return nil, err
	}
	if summaryPtr == nil || summaryPtr.DaysInPeriod == 0 {
		return nil, nil
	}
	return summaryPtr, nil
}

type DailyCredits struct {
	Day          string  `bigquery:"day" json:"day"`
	Credits      float64 `bigquery:"credits" json:"credits"`
	Generations  int64   `bigquery:"generations" json:"generations"`
	Acceptances  int64   `bigquery:"acceptances" json:"acceptances"`
	Interactions int64   `bigquery:"interactions" json:"interactions"`
	CLIRequests  int64   `bigquery:"cli_requests" json:"cli_requests"`
}

func (bq *BigQueryClient) GetUserDailyCredits(ctx context.Context, userLogin string, days int) ([]DailyCredits, error) {
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	queryStr := fmt.Sprintf(`
      WITH date_spine AS (
        SELECT day
        FROM UNNEST(GENERATE_DATE_ARRAY(DATE_SUB(CURRENT_DATE(), INTERVAL (@days - 1) DAY), CURRENT_DATE())) AS day
      ),
      user_data AS (
        SELECT
          day,
          COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64), 0.0) AS credits,
          COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64), 0) AS generations,
          COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64), 0) AS acceptances,
          COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64), 0) AS interactions,
          COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64), 0) AS cli_requests
        FROM %s
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
      )
      SELECT
        CAST(d.day AS STRING) AS day,
        COALESCE(u.credits, 0.0) AS credits,
        COALESCE(u.generations, 0) AS generations,
        COALESCE(u.acceptances, 0) AS acceptances,
        COALESCE(u.interactions, 0) AS interactions,
        COALESCE(u.cli_requests, 0) AS cli_requests
      FROM date_spine d
      LEFT JOIN user_data u USING (day)
      ORDER BY d.day ASC
    `, metricsRef)

	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "days", Value: days},
		{Name: "userLogin", Value: userLogin},
	}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[DailyCredits](it)
}

func (bq *BigQueryClient) GetMonthlyTrends(ctx context.Context, months int) ([]MonthlyTrend, error) {
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	queryStr := fmt.Sprintf(`
      SELECT
        FORMAT_DATE('%%Y-%%m', day) AS month,
        COUNT(DISTINCT day) AS days_in_month,
        COUNT(DISTINCT IF(
          COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64), 0)
          + COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64), 0)
          + COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64), 0) > 0,
          JSON_VALUE(raw_record, '$.user_id'),
          NULL
        )) AS unique_users,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64)), 0) AS ide_interactions,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64)), 0) AS code_generations,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64)), 0) AS cli_requests,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.prompt_tokens_sum') AS INT64)), 0) AS prompt_tokens,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.output_tokens_sum') AS INT64)), 0) AS output_tokens,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64)), 0) AS lines_added,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.loc_deleted_sum') AS INT64)), 0) AS lines_deleted,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64)), 0) AS acceptances,
        COUNT(DISTINCT IF(SAFE_CAST(JSON_VALUE(raw_record, '$.used_agent') AS BOOL), JSON_VALUE(raw_record, '$.user_id'), NULL)) AS agent_users,
        COUNT(DISTINCT IF(SAFE_CAST(JSON_VALUE(raw_record, '$.used_chat') AS BOOL), JSON_VALUE(raw_record, '$.user_id'), NULL)) AS chat_users,
        COUNT(DISTINCT IF(SAFE_CAST(JSON_VALUE(raw_record, '$.used_cli') AS BOOL), JSON_VALUE(raw_record, '$.user_id'), NULL)) AS cli_users
      FROM %s
      WHERE day >= DATE_TRUNC(DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH), MONTH)
        AND scope = 'enterprise'
      GROUP BY month
      ORDER BY month
    `, metricsRef)
	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "months", Value: months}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[MonthlyTrend](it)
}

func (bq *BigQueryClient) GetMonthlyModelUsage(ctx context.Context, months int) ([]MonthlyModelUsage, error) {
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	queryStr := fmt.Sprintf(`
      WITH model_activity AS (
        SELECT
          FORMAT_DATE('%%Y-%%m', day) AS month,
          JSON_VALUE(mf, '$.model') AS model,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(mf, '$.user_initiated_interaction_count') AS INT64)), 0) AS interactions,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_generation_activity_count') AS INT64)), 0) AS generations,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_acceptance_activity_count') AS INT64)), 0) AS acceptances
        FROM %s,
          UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_model_feature')) AS mf
        WHERE day >= DATE_TRUNC(DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH), MONTH)
          AND scope = 'enterprise'
          AND JSON_VALUE(mf, '$.model') IS NOT NULL
          AND JSON_VALUE(mf, '$.model') != 'others'
        GROUP BY month, model
        HAVING (interactions + generations) > 0
      )
      SELECT
        ma.month,
        ma.model,
        (ma.interactions + ma.generations + ma.acceptances) AS interactions
      FROM model_activity ma
      ORDER BY ma.month, interactions DESC
    `, metricsRef)
	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "months", Value: months}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[MonthlyModelUsage](it)
}

func (bq *BigQueryClient) GetMonthlyBillingUsage(ctx context.Context, months int) ([]MonthlyBillingUsage, error) {
	billingRef := bq.tableRef(bq.metricsDataset, "billing_usage")
	queryStr := fmt.Sprintf(`
      SELECT
        FORMAT_DATE('%%Y-%%m', day) AS month,
        model,
        sku,
        SUM(gross_quantity) AS gross_requests,
        SUM(net_quantity) AS net_requests,
        SUM(gross_amount) AS gross_amount,
        SUM(net_amount) AS net_amount
      FROM %s
      WHERE day >= DATE_TRUNC(DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH), MONTH)
        AND LOWER(scope_id) = 'nav'
        AND gross_quantity > 0
      GROUP BY month, model, sku
      ORDER BY month, gross_requests DESC
    `, billingRef)
	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "months", Value: months}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[MonthlyBillingUsage](it)
}

func (bq *BigQueryClient) GetBillingModelDailyCosts(ctx context.Context, month string) ([]BillingModelDailyCost, error) {
	if !isValidYearMonth(month) {
		return nil, fmt.Errorf("invalid month format %q (expected YYYY-MM)", month)
	}

	billingRef := bq.tableRef(bq.metricsDataset, "billing_usage_daily_model")
	queryStr := fmt.Sprintf(`
      SELECT
        FORMAT_DATE('%%Y-%%m-%%d', day) AS day,
        model,
        CAST(SUM(gross_quantity) AS INT64) AS gross_requests,
        CAST(SUM(net_quantity) AS INT64) AS net_requests,
        SUM(gross_amount) AS gross_amount,
        SUM(net_amount) AS net_amount
      FROM %s
      WHERE day >= DATE(@month || '-01')
        AND day < DATE_ADD(DATE(@month || '-01'), INTERVAL 1 MONTH)
        AND LOWER(scope_id) = 'nav'
        AND gross_quantity > 0
      GROUP BY day, model
      ORDER BY day ASC, gross_amount DESC
    `, billingRef)

	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[BillingModelDailyCost](it)
}

func (bq *BigQueryClient) GetBillingModelForecast(ctx context.Context, month string) (*BillingModelForecast, error) {
	if !isValidYearMonth(month) {
		return nil, fmt.Errorf("invalid month format %q (expected YYYY-MM)", month)
	}

	monthStart, err := time.Parse("2006-01", month)
	if err != nil {
		return nil, fmt.Errorf("parse month: %w", err)
	}
	daysInMonth := time.Date(monthStart.Year(), monthStart.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()

	// Fetch current month + previous month daily data for weekday-aligned forecasting
	billingRef := bq.tableRef(bq.metricsDataset, "billing_usage_daily_model")
	prevMonthStart := monthStart.AddDate(0, -1, 0)
	queryStr := fmt.Sprintf(`
      SELECT
        FORMAT_DATE('%%Y-%%m-%%d', day) AS day,
        SUM(net_amount) AS net_amount
      FROM %s
      WHERE day >= DATE(@prev_month_start)
        AND day < DATE_ADD(DATE(@month || '-01'), INTERVAL 1 MONTH)
        AND LOWER(scope_id) = 'nav'
        AND gross_quantity > 0
      GROUP BY day
      ORDER BY day ASC
    `, billingRef)

	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "month", Value: month},
		{Name: "prev_month_start", Value: prevMonthStart.Format("2006-01-02")},
	}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	type dayAmountRow struct {
		Day       string  `bigquery:"day"`
		NetAmount float64 `bigquery:"net_amount"`
	}

	rows, err := readAllRows[dayAmountRow](it)
	if err != nil {
		return nil, err
	}

	// Separate current month and previous month data
	dailyAmounts := make(map[int]float64, 31)
	dayIndices := make([]int, 0, 31)
	prevMonthWeekdayAmounts := make([]float64, 0, 22)
	prevMonthWeekendAmounts := make([]float64, 0, 9)

	for _, row := range rows {
		dayTime, parseErr := time.Parse("2006-01-02", row.Day)
		if parseErr != nil {
			return nil, fmt.Errorf("parse day %q: %w", row.Day, parseErr)
		}
		if dayTime.Month() == monthStart.Month() && dayTime.Year() == monthStart.Year() {
			day := dayTime.Day()
			dailyAmounts[day] = row.NetAmount
			dayIndices = append(dayIndices, day)
		} else {
			// Previous month — collect by day type for baseline
			if isWeekend(dayTime) {
				prevMonthWeekendAmounts = append(prevMonthWeekendAmounts, row.NetAmount)
			} else {
				prevMonthWeekdayAmounts = append(prevMonthWeekdayAmounts, row.NetAmount)
			}
		}
	}
	slices.Sort(dayIndices)

	forecast := &BillingModelForecast{
		Month:       month,
		DaysInMonth: daysInMonth,
	}
	if len(dayIndices) == 0 {
		return forecast, nil
	}

	lastActualDay := dayIndices[len(dayIndices)-1]
	forecast.LastActualDay = fmt.Sprintf("%s-%02d", month, lastActualDay)
	forecast.DaysElapsed = lastActualDay

	actualMTD := 0.0
	weekdayAmounts := make([]float64, 0, lastActualDay)
	weekendAmounts := make([]float64, 0, lastActualDay)
	dailySeries := make([]float64, 0, lastActualDay)

	for day := 1; day <= lastActualDay; day++ {
		value := dailyAmounts[day]
		dailySeries = append(dailySeries, value)
		actualMTD += value
		dayDate := time.Date(monthStart.Year(), monthStart.Month(), day, 0, 0, 0, 0, time.UTC)
		if isWeekend(dayDate) {
			weekendAmounts = append(weekendAmounts, value)
		} else {
			weekdayAmounts = append(weekdayAmounts, value)
		}
	}
	forecast.ActualMTDNetAmount = actualMTD

	// If the last actual day is today, it's likely a partial day (ingestion
	// may not have completed). Exclude it from the run rate calculation to
	// avoid systematically depressing projections.
	today := time.Now().UTC().Day()
	if lastActualDay == today && len(dailySeries) > 1 {
		lastDate := time.Date(monthStart.Year(), monthStart.Month(), lastActualDay, 0, 0, 0, 0, time.UTC)
		if isWeekend(lastDate) {
			if len(weekendAmounts) > 1 {
				weekendAmounts = weekendAmounts[:len(weekendAmounts)-1]
			}
		} else {
			if len(weekdayAmounts) > 1 {
				weekdayAmounts = weekdayAmounts[:len(weekdayAmounts)-1]
			}
		}
		dailySeries = dailySeries[:len(dailySeries)-1]
	}

	// Compute weekday-aware run rates
	weekdayRate := weightedRunRate(weekdayAmounts, 7)
	weekendRate := weightedRunRate(weekendAmounts, 4)

	// Blend with previous month data if current month has few data points.
	// This stabilizes the forecast early in the month when we have <5 weekdays.
	if len(weekdayAmounts) < 5 && len(prevMonthWeekdayAmounts) > 5 {
		prevWeekdayRate := simpleAverage(prevMonthWeekdayAmounts)
		if weekdayRate <= 0 {
			weekdayRate = prevWeekdayRate
		} else {
			// Blend: weight current data more as we get more of it
			currentWeight := float64(len(weekdayAmounts)) / 5.0
			weekdayRate = weekdayRate*currentWeight + prevWeekdayRate*(1-currentWeight)
		}
	}
	if len(weekendAmounts) < 2 && len(prevMonthWeekendAmounts) > 2 {
		prevWeekendRate := simpleAverage(prevMonthWeekendAmounts)
		if weekendRate <= 0 {
			weekendRate = prevWeekendRate
		} else {
			currentWeight := float64(len(weekendAmounts)) / 2.0
			weekendRate = weekendRate*currentWeight + prevWeekendRate*(1-currentWeight)
		}
	}

	// If weekend rate is still 0 (no weekends yet this month and no prev data),
	// estimate it as ~5% of weekday rate (typical for developer tools).
	if weekendRate <= 0 && weekdayRate > 0 {
		weekendRate = weekdayRate * 0.05
	}

	// Compute effective blended run rate for reporting
	var totalRemaining float64
	remainingWeekdays := 0
	remainingWeekends := 0
	for day := lastActualDay + 1; day <= daysInMonth; day++ {
		dayDate := time.Date(monthStart.Year(), monthStart.Month(), day, 0, 0, 0, 0, time.UTC)
		if isWeekend(dayDate) {
			remainingWeekends++
			totalRemaining += weekendRate
		} else {
			remainingWeekdays++
			totalRemaining += weekdayRate
		}
	}

	remainingDays := daysInMonth - lastActualDay
	if remainingDays > 0 {
		forecast.ProjectedDailyRunRate = totalRemaining / float64(remainingDays)
	} else {
		forecast.ProjectedDailyRunRate = weekdayRate
	}

	projectedEOM := actualMTD + totalRemaining
	forecast.ProjectedEOMNetAmount = projectedEOM

	// Uncertainty compounds with the square root of the number of remaining
	// days (random-walk assumption: independent daily errors).
	volatility := sampleStdDev(tail(dailySeries, 14)) * math.Sqrt(float64(remainingDays))
	lower := projectedEOM - volatility
	if lower < actualMTD {
		lower = actualMTD
	}
	forecast.LowerEOMNetAmount = lower
	forecast.UpperEOMNetAmount = projectedEOM + volatility

	// Build day-by-day points with weekday-aware projection
	points := make([]BillingModelForecastPoint, 0, daysInMonth)
	actualCumulative := 0.0
	projectedCumulative := actualMTD
	for day := 1; day <= daysInMonth; day++ {
		date := fmt.Sprintf("%s-%02d", month, day)
		if day <= lastActualDay {
			actualCumulative += dailyAmounts[day]
			actualCopy := actualCumulative
			points = append(points, BillingModelForecastPoint{
				Day:                 date,
				ActualCumulative:    &actualCopy,
				ProjectedCumulative: actualCumulative,
				IsActual:            true,
			})
			continue
		}

		dayDate := time.Date(monthStart.Year(), monthStart.Month(), day, 0, 0, 0, 0, time.UTC)
		if isWeekend(dayDate) {
			projectedCumulative += weekendRate
		} else {
			projectedCumulative += weekdayRate
		}
		points = append(points, BillingModelForecastPoint{
			Day:                 date,
			ProjectedCumulative: projectedCumulative,
			IsActual:            false,
		})
	}

	forecast.Points = points
	return forecast, nil
}

// isWeekend returns true for Saturday and Sunday.
func isWeekend(t time.Time) bool {
	wd := t.Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

// simpleAverage returns the arithmetic mean of a slice.
func simpleAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func weightedRunRate(series []float64, window int) float64 {
	values := tail(series, window)
	if len(values) == 0 {
		return 0
	}
	weightedSum := 0.0
	weightTotal := 0.0
	for i, value := range values {
		weight := float64(i + 1)
		weightedSum += value * weight
		weightTotal += weight
	}
	if weightTotal == 0 {
		return 0
	}
	return weightedSum / weightTotal
}

func sampleStdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := 0.0
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))

	variance := 0.0
	for _, value := range values {
		diff := value - mean
		variance += diff * diff
	}
	variance /= float64(len(values) - 1)
	return math.Sqrt(variance)
}

func tail(values []float64, n int) []float64 {
	if n <= 0 || len(values) == 0 {
		return nil
	}
	if len(values) <= n {
		out := make([]float64, len(values))
		copy(out, values)
		return out
	}
	out := make([]float64, n)
	copy(out, values[len(values)-n:])
	return out
}

// minUsersForDistribution enforces k-anonymity: below this many users,
// aggregate stats could still let someone infer an individual's usage.
const minUsersForDistribution = 5

// UsageHistogramBucket is one bucket of the credits histogram: how many
// users fall in a given usage range. No individual users are identifiable.
type UsageHistogramBucket struct {
	Bucket   string `bigquery:"bucket" json:"bucket"`
	NumUsers int64  `bigquery:"num_users" json:"num_users"`
}

// UsageDistribution is a privacy-preserving, aggregate-only view of how
// Copilot usage is spread across all users in a given month: percentiles
// (deciles) per metric plus a credits histogram. It never contains
// per-user identifiers.
type UsageDistribution struct {
	Month               string                 `json:"month"`
	NumUsers            int64                  `json:"num_users"`
	TotalLicensedSeats  int64                  `json:"total_licensed_seats"`
	BudgetCredits       float64                `json:"budget_credits"`
	CreditsDeciles      []float64              `json:"credits_deciles"`
	InteractionsDeciles []int64                `json:"interactions_deciles"`
	AcceptancesDeciles  []int64                `json:"acceptances_deciles"`
	CreditsHistogram    []UsageHistogramBucket `json:"credits_histogram"`
}

// GetUsageDistribution returns an aggregate-only usage distribution for the given
// month. budgetCredits is the per-user AI credit budget (already converted from
// USD) and is used to bucket the credits histogram as % of budget consumed,
// so the chart stays meaningful regardless of the raw credit scale.
func (bq *BigQueryClient) GetUsageDistribution(ctx context.Context, month string, budgetCredits float64) (*UsageDistribution, error) {
	if !isValidYearMonth(month) {
		return nil, fmt.Errorf("invalid month format %q (expected YYYY-MM)", month)
	}

	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")

	distribution, numUsers, err := bq.getUsageDistributionData(ctx, metricsRef, month, budgetCredits)
	if err != nil {
		return nil, err
	}
	if numUsers < minUsersForDistribution {
		// Too few users to safely aggregate without risking re-identification.
		// Omit the exact (small) count too — even that alone can aid re-identification.
		return &UsageDistribution{Month: month, BudgetCredits: budgetCredits}, nil
	}

	distribution.Month = month
	distribution.NumUsers = numUsers
	distribution.BudgetCredits = budgetCredits
	return distribution, nil
}

// getUsageDistributionData computes per-user credits/interactions/acceptances once
// and derives both the decile summary and the credits histogram from it in a
// single query, avoiding a second full scan of user_metrics for the same month.
func (bq *BigQueryClient) getUsageDistributionData(ctx context.Context, metricsRef, month string, budgetCredits float64) (*UsageDistribution, int64, error) {
	queryStr := fmt.Sprintf(`
      WITH per_user AS (
        SELECT
          JSON_VALUE(raw_record, '$.user_login') AS user_login,
          SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64), 0.0)) AS credits,
          SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64), 0)) AS interactions,
          SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64), 0)) AS acceptances
        FROM %s
        WHERE day >= DATE(@month || '-01')
          AND day < DATE_ADD(DATE(@month || '-01'), INTERVAL 1 MONTH)
          AND scope = 'enterprise'
        GROUP BY user_login
      ),
      deciles AS (
        SELECT
          APPROX_QUANTILES(credits, 10) AS credits_deciles,
          APPROX_QUANTILES(interactions, 10) AS interactions_deciles,
          APPROX_QUANTILES(acceptances, 10) AS acceptances_deciles,
          COUNT(*) AS num_users
        FROM per_user
      ),
      bucketed AS (
        SELECT
          CASE
            WHEN credits = 0 THEN '0%%'
            WHEN credits < @budget * 0.10 THEN '1-9%%'
            WHEN credits < @budget * 0.25 THEN '10-24%%'
            WHEN credits < @budget * 0.50 THEN '25-49%%'
            WHEN credits < @budget * 0.75 THEN '50-74%%'
            WHEN credits < @budget THEN '75-99%%'
            ELSE '100%%+'
          END AS bucket
        FROM per_user
      ),
      -- All bucket labels in display order, so empty buckets still show as 0
      -- instead of disappearing from the chart.
      all_buckets AS (
        SELECT bucket, offset AS bucket_order
        FROM UNNEST(['0%%', '1-9%%', '10-24%%', '25-49%%', '50-74%%', '75-99%%', '100%%+']) AS bucket WITH OFFSET
      ),
      histogram AS (
        SELECT
          all_buckets.bucket AS bucket,
          COUNT(bucketed.bucket) AS num_users
        FROM all_buckets
        LEFT JOIN bucketed ON bucketed.bucket = all_buckets.bucket
        GROUP BY all_buckets.bucket, all_buckets.bucket_order
        ORDER BY all_buckets.bucket_order
      )
      SELECT
        (SELECT AS STRUCT deciles.* FROM deciles) AS deciles,
        ARRAY(SELECT AS STRUCT histogram.* FROM histogram) AS histogram
    `, metricsRef)

	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "month", Value: month},
		{Name: "budget", Value: budgetCredits},
	}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("execute query: %w", err)
	}

	type decilesStruct struct {
		CreditsDeciles      []float64 `bigquery:"credits_deciles"`
		InteractionsDeciles []int64   `bigquery:"interactions_deciles"`
		AcceptancesDeciles  []int64   `bigquery:"acceptances_deciles"`
		NumUsers            int64     `bigquery:"num_users"`
	}
	type combinedRow struct {
		Deciles   decilesStruct          `bigquery:"deciles"`
		Histogram []UsageHistogramBucket `bigquery:"histogram"`
	}

	rows, err := readAllRows[combinedRow](it)
	if err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return &UsageDistribution{}, 0, nil
	}

	row := rows[0]
	return &UsageDistribution{
		CreditsDeciles:      row.Deciles.CreditsDeciles,
		InteractionsDeciles: row.Deciles.InteractionsDeciles,
		AcceptancesDeciles:  row.Deciles.AcceptancesDeciles,
		CreditsHistogram:    row.Histogram,
	}, row.Deciles.NumUsers, nil
}

func isValidYearMonth(v string) bool {
	parsed, err := time.Parse("2006-01", v)
	if err != nil {
		return false
	}
	return parsed.Format("2006-01") == v
}

func (bq *BigQueryClient) GetUserWeeklyTrends(ctx context.Context, userLogin string, weeks int) ([]WeeklyTrend, error) {
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	days := weeks * 7
	queryStr := fmt.Sprintf(`
      WITH weekly_data AS (
        SELECT
          FORMAT_DATE('%%G-W%%V', day) AS week,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64)), 0) AS interactions,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64)), 0) AS cli_requests,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64)), 0) AS acceptances,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64)), 0) AS lines_added,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.loc_deleted_sum') AS INT64)), 0) AS lines_deleted,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.prompt_tokens_sum') AS INT64)), 0) AS prompt_tokens,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.output_tokens_sum') AS INT64)), 0) AS output_tokens,
          COUNT(*) AS active_days
        FROM %s
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
        GROUP BY week
      ),
      weekly_models AS (
        SELECT
          FORMAT_DATE('%%G-W%%V', day) AS week,
          JSON_VALUE(mf, '$.model') AS model,
          SUM(SAFE_CAST(JSON_VALUE(mf, '$.user_initiated_interaction_count') AS INT64))
            + SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_generation_activity_count') AS INT64))
            + SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_acceptance_activity_count') AS INT64)) AS interactions
        FROM %s,
          UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_model_feature')) AS mf
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
          AND JSON_VALUE(mf, '$.model') IS NOT NULL
          AND JSON_VALUE(mf, '$.model') != 'others'
        GROUP BY week, model
        HAVING interactions > 0
      ),
      weekly_models_ranked AS (
        SELECT
          week,
          model,
          interactions,
          ROW_NUMBER() OVER (PARTITION BY week ORDER BY interactions DESC) AS rn
        FROM weekly_models
      ),
      weekly_models_agg AS (
        SELECT
          week,
          ARRAY_AGG(STRUCT(model, interactions) ORDER BY interactions DESC) AS models
        FROM weekly_models_ranked
        WHERE rn <= 5
        GROUP BY week
      )
      SELECT
        wd.week,
        wd.interactions,
        wd.cli_requests,
        wd.acceptances,
        wd.lines_added,
        wd.lines_deleted,
        wd.prompt_tokens,
        wd.output_tokens,
        wd.active_days,
        wma.models
      FROM weekly_data wd
      LEFT JOIN weekly_models_agg wma ON wma.week = wd.week
      ORDER BY wd.week
    `, metricsRef, metricsRef)
	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "days", Value: days}, {Name: "userLogin", Value: userLogin}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[WeeklyTrend](it)
}

func (bq *BigQueryClient) GetAdoptionCohorts(ctx context.Context, days int) ([]AdoptionCohortDay, error) {
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	queryStr := fmt.Sprintf(`
      SELECT
        day,
        SAFE_CAST(REGEXP_EXTRACT(JSON_VALUE(raw_record, '$.ai_adoption_phase.phase'), r'\d+') AS INT64) AS phase,
        JSON_VALUE(raw_record, '$.ai_adoption_phase.version') AS phase_version,
        COUNT(DISTINCT JSON_VALUE(raw_record, '$.user_id')) AS user_count,
        AVG(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64)) AS avg_generations,
        AVG(SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64)) AS avg_acceptances,
        AVG(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64)) AS avg_interactions,
        AVG(SAFE_CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64)) AS avg_lines_added
      FROM %s
      WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
        AND scope = 'enterprise'
        AND JSON_VALUE(raw_record, '$.ai_adoption_phase.phase') IS NOT NULL
      GROUP BY day, phase, phase_version
      HAVING phase IS NOT NULL
      ORDER BY day, phase
    `, metricsRef)
	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "days", Value: days}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[AdoptionCohortDay](it)
}

func (bq *BigQueryClient) GetBillingMonthlyTrend(ctx context.Context, months int) ([]BillingMonthlyTrend, error) {
	viewRef := bq.viewRef(bq.metricsDataset, "v_billing_monthly_trend")
	queryStr := fmt.Sprintf(`
      SELECT year_month, total_gross_amount, total_net_amount, discount_rate_pct, distinct_models
      FROM %s
      WHERE LOWER(scope_id) = 'nav'
        AND PARSE_DATE('%%Y-%%m', year_month) >= DATE_TRUNC(DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH), MONTH)
      ORDER BY year_month ASC
    `, viewRef)
	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "months", Value: months}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[BillingMonthlyTrend](it)
}

type DailySummary struct {
	Date                    string  `bigquery:"day" json:"date"`
	DailyActiveUsers        int64   `bigquery:"daily_active_users" json:"daily_active_users"`
	WeeklyActiveUsers       int64   `bigquery:"weekly_active_users" json:"weekly_active_users"`
	MonthlyActiveUsers      int64   `bigquery:"monthly_active_users" json:"monthly_active_users"`
	MonthlyActiveChatUsers  int64   `bigquery:"monthly_active_chat_users" json:"monthly_active_chat_users"`
	MonthlyActiveAgentUsers int64   `bigquery:"monthly_active_agent_users" json:"monthly_active_agent_users"`
	DailyActiveCliUsers     int64   `bigquery:"daily_active_cli_users" json:"daily_active_cli_users"`
	PrReviewedByCopilot     int64   `bigquery:"pr_reviewed_by_copilot" json:"pr_reviewed_by_copilot"`
	PrCreatedByCopilot      int64   `bigquery:"pr_created_by_copilot" json:"pr_created_by_copilot"`
	PrMergedCopilotAuthored int64   `bigquery:"pr_merged_copilot_authored" json:"pr_merged_copilot_authored"`
	CliSessionCount         int64   `bigquery:"cli_session_count" json:"cli_session_count"`
	CliRequestCount         int64   `bigquery:"cli_request_count" json:"cli_request_count"`
	PrMedianMinutesToMerge  float64 `bigquery:"pr_median_minutes_to_merge" json:"pr_median_minutes_to_merge"`
}

func (bq *BigQueryClient) GetDailySummary(ctx context.Context) (*DailySummary, error) {
	viewRef := bq.viewRef(bq.metricsDataset, "v_daily_summary")
	queryStr := fmt.Sprintf(`
      SELECT
        CAST(day AS STRING) AS day,
        daily_active_users, weekly_active_users, monthly_active_users,
        monthly_active_chat_users, monthly_active_agent_users, daily_active_cli_users,
        pr_reviewed_by_copilot, pr_created_by_copilot, pr_merged_copilot_authored,
        cli_session_count, cli_request_count, pr_median_minutes_to_merge
      FROM %s
      WHERE scope = 'organization' AND scope_id = 'navikt'
      ORDER BY day DESC
      LIMIT 1
    `, viewRef)
	it, err := bq.client.Query(queryStr).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	rows, err := readAllRows[DailySummary](it)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (bq *BigQueryClient) GetBillingModelBreakdown(ctx context.Context, months int) ([]BillingModelBreakdown, error) {
	viewRef := bq.viewRef(bq.metricsDataset, "v_billing_model_breakdown")
	queryStr := fmt.Sprintf(`
      SELECT year_month, model, gross_amount, net_amount, pct_of_monthly_net
      FROM %s
      WHERE LOWER(scope_id) = 'nav'
        AND PARSE_DATE('%%Y-%%m', year_month) >= DATE_TRUNC(DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH), MONTH)
      ORDER BY year_month ASC, net_amount DESC
    `, viewRef)
	query := bq.client.Query(queryStr)
	query.Parameters = []bigquery.QueryParameter{{Name: "months", Value: months}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[BillingModelBreakdown](it)
}
