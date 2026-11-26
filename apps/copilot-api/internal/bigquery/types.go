package bigquery

import (
	"time"
	_ "time/tzdata" // Oslo must resolve without system tzdata
)

type EditorUsage struct {
	Editor       string `bigquery:"editor" json:"editor"`
	FirstDay     string `bigquery:"first_day" json:"first_day"`
	LastDay      string `bigquery:"last_day" json:"last_day"`
	Days         int64  `bigquery:"days" json:"days"`
	Generations  *int64 `bigquery:"generations" json:"generations"`
	Acceptances  *int64 `bigquery:"acceptances" json:"acceptances"`
	Interactions *int64 `bigquery:"interactions" json:"interactions"`
}

// CostDay is one day of enterprise gross and net from billing_usage_daily_model.
type CostDay struct {
	Day   string  `bigquery:"day"`
	Net   float64 `bigquery:"net"`
	Gross float64 `bigquery:"gross"`
}

// UserDay is one user's gross (AI credits in USD) on one day, from user_metrics.
type UserDay struct {
	UserID string  `bigquery:"user_id"`
	Day    string  `bigquery:"day"`
	Gross  float64 `bigquery:"gross"`
}

// FactAggregates is one row of Nav-wide totals. The week is the seven days up
// to DataDay (the newest day in user_metrics), the month the 28 days up to it.
type FactAggregates struct {
	DataDay          string        `bigquery:"data_day"`
	WeekUsers        int64         `bigquery:"week_users"`
	MCPUsers         int64         `bigquery:"mcp_users"`
	SkillUsers       int64         `bigquery:"skill_users"`
	CustomAgentUsers int64         `bigquery:"custom_agent_users"`
	Users3PlusModels int64         `bigquery:"users_3plus_models"`
	MonthUsers       int64         `bigquery:"month_users"`
	MonthUserDays    int64         `bigquery:"month_user_days"`
	WeekendUserDays  int64         `bigquery:"weekend_user_days"`
	WeekendUsers     int64         `bigquery:"weekend_users"`
	WeekdayAvg       float64       `bigquery:"weekday_avg"`
	JuneUsers        int64         `bigquery:"june_users"`
	JuneWeekdayAvg   float64       `bigquery:"june_weekday_avg"`
	JulyWeekdayAvg   float64       `bigquery:"july_weekday_avg"`
	Families         []FamilyUsers `bigquery:"families"`
	// From the weekly repo scan, not user_metrics; filled by the handler.
	ActiveRepos            int64 `bigquery:"-"`
	ActiveReposWithCustoms int64 `bigquery:"-"`
}

var Oslo = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		panic(err) // tzdata is embedded below; a wrong day boundary is worse than no start
	}
	return loc
}()

// UsageQualityDay contains independent daily counts, not unique users over a period.
type UsageQualityDay struct {
	Day                      string `bigquery:"day" json:"day"`
	Scope                    string `bigquery:"scope" json:"scope"`
	ScopeID                  string `bigquery:"scope_id" json:"scope_id"`
	MissingClientTelemetry   *int64 `bigquery:"missing_client_telemetry_user_count" json:"missing_client_telemetry_user_count"`
	BelowUsageMetricsMinimum *int64 `bigquery:"below_usage_metrics_minimum_user_count" json:"below_usage_metrics_minimum_user_count"`
}

// FamilyUsers counts distinct users per model family the past week, largest first.
type FamilyUsers struct {
	Family string `bigquery:"family"`
	Users  int64  `bigquery:"users"`
}
