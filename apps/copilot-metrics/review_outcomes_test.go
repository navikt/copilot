package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestReviewOutcomeFixturePresenceAndBotSemantics(t *testing.T) {
	data, err := os.ReadFile("testdata/review-outcomes.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Review *struct {
			Evaluated int64 `json:"total_comments_evaluated"`
			Addressed int64 `json:"total_comments_addressed"`
			Merged    int64 `json:"total_merged_no_human_change_request_after_review"`
		} `json:"copilot_code_review"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[0].Review != nil {
		t.Fatal("historical absence must remain unknown")
	}
	if rows[3].Review == nil || rows[3].Review.Evaluated != 0 || rows[3].Review.Merged != 1 {
		t.Fatal("zero evaluations can coexist with bot-inclusive merges")
	}
	var evaluated, addressed int64
	for _, row := range rows {
		if row.Review != nil {
			evaluated += row.Review.Evaluated
			addressed += row.Review.Addressed
		}
	}
	if evaluated != 10 || addressed != 2 {
		t.Fatalf("weighted fixture counts: %d/%d", addressed, evaluated)
	}
}

func TestReviewOutcomeViewPreservesNullAndPrivacy(t *testing.T) {
	data, err := os.ReadFile("views/v_repository_usage.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(data)
	for _, want := range []string{"IN ('PUBLIC', 'INTERNAL')", "HAVING pr_total_created >= 5", "SUM(review_comments_evaluated)", "COUNT(DISTINCT IF(has_review_outcomes, day, NULL))", "MIN(IF(has_review_outcomes, day, NULL))", "GROUP BY repo_id, repo_owner, repo_name, scope_id, dimension, label"} {
		if !strings.Contains(sql, want) {
			t.Errorf("missing SQL contract: %s", want)
		}
	}
	if strings.Contains(sql, "COALESCE(review_comments") || strings.Contains(sql, "day >= '2026-11-05'") {
		t.Fatal("must not turn unknown into zero or discard pre-announcement data")
	}
}

func TestRepositoryViewIdentityContracts(t *testing.T) {
	data, err := os.ReadFile("views/v_repository_usage.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(data)
	for _, want := range []string{
		"PARTITION BY COALESCE(NULLIF(JSON_VALUE(raw_record, '$.enterprise_id'), ''), CONCAT(scope, ':', scope_id))",
		"JSON_VALUE(raw_record, '$.repo_id'), day",
		"ORDER BY IF(scope = 'enterprise', 0, 1), loaded_at DESC",
		"GROUP BY repo_id, repo_owner, repo_name, scope_id, dimension, label",
		"GROUP BY repo_id, repo_owner, repo_name, scope_id\n), totals",
		"GROUP BY repo_id, repo_owner, repo_name, scope_id\n-- Privacy",
		"LEFT JOIN breakdowns USING (repo_id, repo_owner, repo_name, scope_id)",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("missing identity contract: %s", want)
		}
	}
}

func TestRepositoryBreakdownsRequireCompleteCounts(t *testing.T) {
	data, err := os.ReadFile("views/v_repository_usage.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"evaluated", "addressed"} {
		want := "IF(COUNT(SAFE_CAST(JSON_VALUE(entry, '$.total_comments_" + field + "') AS INT64)) = COUNT(*),\n      SUM(SAFE_CAST(JSON_VALUE(entry, '$.total_comments_" + field + "') AS INT64)), NULL) AS " + field
		if !strings.Contains(string(data), want) {
			t.Errorf("partial %s breakdowns must remain unknown", field)
		}
	}
}
