package main

import (
	"context"
	"fmt"

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
	PromptTokens int64  `bigquery:"prompt_tokens" json:"prompt_tokens"`
	OutputTokens int64  `bigquery:"output_tokens" json:"output_tokens"`
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
      WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH)
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
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH)
          AND scope = 'enterprise'
          AND JSON_VALUE(mf, '$.model') IS NOT NULL
          AND JSON_VALUE(mf, '$.model') != 'others'
        GROUP BY month, model
        HAVING (interactions + generations) > 0
      ),
      monthly_tokens AS (
        SELECT
          FORMAT_DATE('%%Y-%%m', day) AS month,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.prompt_tokens_sum') AS INT64)), 0) AS prompt_tokens,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.output_tokens_sum') AS INT64)), 0) AS output_tokens
        FROM %s
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH)
          AND scope = 'enterprise'
        GROUP BY month
      )
      SELECT
        ma.month,
        ma.model,
        (ma.interactions + ma.generations + ma.acceptances) AS interactions,
        COALESCE(mt.prompt_tokens, 0) AS prompt_tokens,
        COALESCE(mt.output_tokens, 0) AS output_tokens
      FROM model_activity ma
      LEFT JOIN monthly_tokens mt ON ma.month = mt.month
      ORDER BY ma.month, interactions DESC
    `, metricsRef, metricsRef)
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
      WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH)
        AND scope_id = 'nav'
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
        AVG(CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64)) AS avg_generations,
        AVG(CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64)) AS avg_acceptances,
        AVG(CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64)) AS avg_interactions,
        AVG(CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64)) AS avg_lines_added
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
