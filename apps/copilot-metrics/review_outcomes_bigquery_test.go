package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
)

// Every source is inline; this executes the actual view SELECT, never its DDL.
func TestReviewOutcomesInlineBigQuery(t *testing.T) {
	if os.Getenv("VERIFY_REVIEW_OUTCOMES_BIGQUERY") != "true" {
		t.Skip("set VERIFY_REVIEW_OUTCOMES_BIGQUERY=true for inline-only SQL verification")
	}
	data, err := os.ReadFile("testdata/review-outcomes.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	var sources []string
	add := func(record map[string]any, enterprise, scope, scopeID, loaded string) {
		copy := make(map[string]any)
		for k, v := range record {
			copy[k] = v
		}
		if enterprise == "" {
			delete(copy, "enterprise_id")
		} else {
			copy["enterprise_id"] = enterprise
		}
		if _, ok := copy["repo_owner_name"]; !ok {
			copy["repo_owner_name"] = "synthetic"
		}
		if _, ok := copy["repo_name"]; !ok {
			copy["repo_name"] = fmt.Sprint(copy["repo_id"])
		}
		if _, ok := copy["pull_requests"]; !ok {
			copy["pull_requests"] = map[string]any{"total_created": 5}
		}
		raw, err := json.Marshal(copy)
		if err != nil {
			t.Fatal(err)
		}
		sources = append(sources, fmt.Sprintf("SELECT DATE %q AS day, %q AS scope, %q AS scope_id, PARSE_JSON(%q) AS raw_record, TIMESTAMP %q AS loaded_at", copy["day"], scope, scopeID, string(raw), loaded))
	}
	for _, record := range records {
		add(record, "1001", "enterprise", "nav", "2026-11-06 02:00:00+00")
	}
	// Duplicate organization source and stale enterprise copy must not inflate counts.
	duplicate := map[string]any{"day": "2026-11-05", "repo_id": 1, "repo_visibility": "PUBLIC", "copilot_code_review": map[string]any{"total_comments_evaluated": 99, "total_comments_addressed": 99}}
	add(duplicate, "1001", "organization", "navikt", "2026-11-06 03:00:00+00")
	add(duplicate, "1001", "enterprise", "nav", "2026-11-06 01:00:00+00")
	// A different enterprise with the same synthetic repo id remains separate.
	add(records[1], "2002", "enterprise", "other", "2026-11-06 02:00:00+00")
	malformed := map[string]any{"day": "2026-11-06", "repo_id": 1, "repo_visibility": "PUBLIC", "copilot_code_review": map[string]any{
		"total_comments_evaluated":    0,
		"review_comments_by_severity": []any{map[string]any{"comment_type": "wrong-key", "total_comments_evaluated": 1, "total_comments_addressed": 0}},
	}}
	add(malformed, "1001", "enterprise", "nav", "2026-11-07 02:00:00+00")
	private := map[string]any{"day": "2026-11-05", "repo_id": 3, "repo_visibility": "PRIVATE"}
	add(private, "1001", "enterprise", "nav", "2026-11-06 02:00:00+00")
	add(map[string]any{"day": "2026-11-03", "repo_id": 4, "repo_visibility": "PUBLIC"}, "1001", "enterprise", "nav", "2026-11-06 02:00:00+00")
	add(map[string]any{"day": "2026-11-03", "repo_id": 5, "repo_visibility": "PUBLIC", "pull_requests": map[string]any{"total_created": 1}}, "1001", "enterprise", "nav", "2026-11-06 02:00:00+00")
	// Renames retain the existing per-name privacy threshold and must not share breakdowns.
	for i, name := range []string{"old", "new"} {
		count := int64(4 + 2*i)
		add(map[string]any{
			"day": fmt.Sprintf("2026-11-0%d", 3+i), "repo_id": 6, "repo_name": name, "repo_visibility": "PUBLIC",
			"copilot_code_review": map[string]any{
				"total_comments_evaluated": count, "total_comments_addressed": int64(2),
				"review_comments_by_severity": []any{map[string]any{"severity": "high", "total_comments_evaluated": count, "total_comments_addressed": int64(2)}},
			},
		}, "1001", "enterprise", "nav", "2026-11-06 02:00:00+00")
	}
	add(map[string]any{
		"day": "2026-11-03", "repo_id": 8, "repo_visibility": "PUBLIC",
		"copilot_code_review": map[string]any{
			"review_comments_by_severity": []any{
				map[string]any{"severity": "high", "total_comments_evaluated": 10, "total_comments_addressed": 1},
				map[string]any{"severity": "high", "total_comments_evaluated": 10},
				map[string]any{"severity": "low", "total_comments_addressed": 0},
				map[string]any{"severity": "low", "total_comments_evaluated": 0, "total_comments_addressed": 0},
			},
		},
	}, "3003", "enterprise", "partial", "2026-11-06 02:00:00+00")
	// Ambiguous overlap suppresses every day of the repo, not only the duplicate day.
	for _, source := range []struct{ day, scope, scopeID string }{
		{"2026-11-03", "enterprise", "nav"},
		{"2026-11-04", "enterprise", "nav"},
		{"2026-11-04", "organization", "navikt"},
	} {
		add(map[string]any{"day": source.day, "repo_id": 9, "repo_visibility": "PUBLIC"}, "", source.scope, source.scopeID, "2026-11-06 02:00:00+00")
	}
	// An isolated fallback with no enterprise ID remains usable.
	add(map[string]any{"day": "2026-11-03", "repo_id": 10, "repo_visibility": "PUBLIC"}, "", "organization", "fallback", "2026-11-06 02:00:00+00")
	// Stale missing identity must not suppress latest fully identified scope copies.
	reloaded := map[string]any{"day": "2026-11-03", "repo_id": 11, "repo_visibility": "PUBLIC"}
	add(reloaded, "", "enterprise", "nav", "2026-11-06 01:00:00+00")
	add(reloaded, "1001", "enterprise", "nav", "2026-11-06 02:00:00+00")
	add(reloaded, "1001", "organization", "navikt", "2026-11-06 03:00:00+00")
	view, err := os.ReadFile("views/v_repository_usage.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(view)
	start := strings.Index(sql, "WITH scoped_latest AS")
	if start < 0 || strings.Count(sql, "{{repository_metrics}}") != 1 {
		t.Fatal("unexpected view source contract")
	}
	sql = strings.TrimSuffix(strings.TrimSpace(sql[start:]), ";")
	sql = strings.ReplaceAll(sql, "{{repository_metrics}}", "("+strings.Join(sources, " UNION ALL ")+")")
	if strings.Contains(sql, "{{") || strings.Contains(sql, "`") || strings.Contains(strings.ToUpper(sql), "CREATE ") {
		t.Fatal("query is not a standalone inline SELECT")
	}
	query := `SELECT COUNT(*) AS repos,
 SUM(IF(scope_id='nav',review_comments_evaluated,0)) AS evaluated,
 SUM(IF(scope_id='nav',review_comments_addressed,0)) AS addressed,
 SUM(IF(scope_id='nav',review_outcome_days,0)) AS days,
 COUNTIF(scope_id='other') AS other_scopes,
 COUNTIF(repo_id=2 AND review_comments_evaluated=0 AND review_merged_no_human_change_request=1 AND SAFE_DIVIDE(review_comments_addressed,review_comments_evaluated) IS NULL) AS zero_bot,
 (SELECT COUNT(*) FROM results, UNNEST(review_breakdowns) WHERE dimension='severity' AND label='unknown') AS unknown_labels,
 SAFE_DIVIDE(SUM(IF(scope_id='nav',review_comments_addressed,0)),SUM(IF(scope_id='nav',review_comments_evaluated,0))) AS rate,
 COUNTIF(repo_id=4 AND review_comments_evaluated IS NULL AND review_outcome_days=0) AS missing_objects,
 COUNTIF(scope_id='nav' AND repo_id=1 AND review_comment_days=2 AND review_outcome_days=3) AS partial_days,
 COUNTIF(repo_id=6 AND repo_name='old' AND review_comments_evaluated=4 AND ARRAY_LENGTH(review_breakdowns)=1 AND review_breakdowns[SAFE_OFFSET(0)].evaluated=4) AS old_name,
 COUNTIF(repo_id=6 AND repo_name='new' AND review_comments_evaluated=6 AND ARRAY_LENGTH(review_breakdowns)=1 AND review_breakdowns[SAFE_OFFSET(0)].evaluated=6) AS new_name,
 (SELECT COUNT(*) FROM results, UNNEST(review_breakdowns) WHERE repo_id=8 AND ((label='high' AND evaluated=20 AND addressed IS NULL) OR (label='low' AND evaluated IS NULL AND addressed=0))) AS partial_breakdowns,
 COUNTIF(repo_id=9) AS ambiguous_repo,
 COUNTIF(repo_id=10 AND scope_id='fallback') AS isolated_fallback,
 COUNTIF(repo_id=11 AND scope_id='nav' AND pr_total_created=5) AS latest_identity
 FROM results`
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client, err := bigquery.NewClient(ctx, "copilot-dev-e17a")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	it, err := client.Query("WITH results AS (" + sql + ") " + query).Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Repos, Evaluated, Addressed, Days, OtherScopes, ZeroBot, UnknownLabels, MissingObjects int64
		Rate                                                                                   float64
	}
	var row []bigquery.Value
	if err := it.Next(&row); err != nil {
		t.Fatal(err)
	}
	if len(row) != 16 {
		t.Fatalf("unexpected result: %v", row)
	}
	got.Repos = row[0].(int64)
	got.Evaluated = row[1].(int64)
	got.Addressed = row[2].(int64)
	got.Days = row[3].(int64)
	got.OtherScopes = row[4].(int64)
	got.ZeroBot = row[5].(int64)
	got.UnknownLabels = row[6].(int64)
	got.Rate = row[7].(float64)
	got.MissingObjects = row[8].(int64)
	if got.Repos != 9 || got.Evaluated != 20 || got.Addressed != 6 || got.Days != 6 || got.OtherScopes != 1 || got.ZeroBot != 1 || got.UnknownLabels != 1 || got.MissingObjects != 1 || got.Rate != 0.3 || row[9] != int64(1) || row[10] != int64(1) || row[11] != int64(1) || row[12] != int64(2) || row[13] != int64(0) || row[14] != int64(1) || row[15] != int64(1) {
		t.Fatalf("actual view result: %+v", got)
	}
}
