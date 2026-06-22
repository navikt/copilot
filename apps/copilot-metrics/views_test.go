package main

import (
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
