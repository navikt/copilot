package main

import (
	"strings"
	"testing"
	"time"
)

func TestUsageMetricsRow_Fields(t *testing.T) {
	now := time.Now().UTC()
	row := UsageMetricsRow{
		Day:       "2025-10-15",
		Scope:     "enterprise",
		ScopeID:   "nav",
		RawRecord: `{"daily_active_users":30}`,
		LoadedAt:  now,
	}

	if row.Day != "2025-10-15" {
		t.Errorf("Day = %q, want %q", row.Day, "2025-10-15")
	}
	if row.Scope != "enterprise" {
		t.Errorf("Scope = %q, want %q", row.Scope, "enterprise")
	}
	if row.ScopeID != "nav" {
		t.Errorf("ScopeID = %q, want %q", row.ScopeID, "nav")
	}
	if row.RawRecord != `{"daily_active_users":30}` {
		t.Errorf("RawRecord = %q, want JSON string", row.RawRecord)
	}
	if !row.LoadedAt.Equal(now) {
		t.Errorf("LoadedAt = %v, want %v", row.LoadedAt, now)
	}
}

func TestViewDefinitions(t *testing.T) {
	expectedViews := []string{
		"v_daily_summary",
		"v_language_stats",
		"v_editor_stats",
		"v_model_stats",
		"v_code_generation",
		"v_team_daily_summary",
		"v_adoption_cohorts",
		"v_user_credits_summary",
		"v_billing_monthly_trend",
		"v_billing_model_breakdown",
		"v_user_budget_trend",
		"v_repository_usage",
	}

	if len(views) != len(expectedViews) {
		t.Fatalf("expected %d views, got %d", len(expectedViews), len(views))
	}

	for i, want := range expectedViews {
		if views[i].name != want {
			t.Errorf("views[%d].name = %q, want %q", i, views[i].name, want)
		}

		// Verify SQL template is readable from embedded FS
		data, err := viewsFS.ReadFile(views[i].filename)
		if err != nil {
			t.Errorf("could not read embedded SQL for %s: %v", want, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("embedded SQL for %s is empty", want)
		}
	}
}

// TestRepositoryUsageViewPlaceholders asserts the repository usage view is wired
// to the {{repository_metrics}} placeholder that createOrReplaceView substitutes,
// and that its privacy safeguards (private-repo exclusion + k=5 suppression) are
// present in the SQL.
func TestRepositoryUsageViewPlaceholders(t *testing.T) {
	data, err := viewsFS.ReadFile("views/v_repository_usage.sql")
	if err != nil {
		t.Fatalf("could not read v_repository_usage.sql: %v", err)
	}
	sql := string(data)

	if !strings.Contains(sql, "{{repository_metrics}}") {
		t.Errorf("v_repository_usage.sql must reference the {{repository_metrics}} placeholder")
	}
	if !strings.Contains(sql, "IN ('PUBLIC', 'INTERNAL')") {
		t.Errorf("v_repository_usage.sql must exclude private repos via visibility filter")
	}
	if !strings.Contains(sql, "HAVING") || !strings.Contains(sql, "min_repo_activity") {
		t.Errorf("v_repository_usage.sql must suppress low-activity repos via a HAVING threshold")
	}
}
