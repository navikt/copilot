package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestEditorUsageInlineFixture(t *testing.T) {
	const input = `WITH fixture_usage_metrics AS (
  SELECT * FROM UNNEST(ARRAY<STRUCT<day DATE, scope STRING, scope_id STRING, loaded_at TIMESTAMP, raw_record STRING>>[
    (DATE '2026-11-05', 'enterprise', 'nav', TIMESTAMP '2026-11-06 01:00:00+00', '{"totals_by_ide":[{"ide":"vscode","code_generation_activity_count":999,"code_acceptance_activity_count":999,"user_initiated_interaction_count":999}]}'),
    (DATE '2026-11-05', 'enterprise', 'NAV', TIMESTAMP '2026-11-06 02:00:00+00', '{"totals_by_ide":[{"ide":"vscode","code_generation_activity_count":0,"code_acceptance_activity_count":0,"user_initiated_interaction_count":4},{"ide":"jetbrains","code_generation_activity_count":7,"user_initiated_interaction_count":0}]}'),
    (DATE '2026-11-04', 'enterprise', 'nav', TIMESTAMP '2026-11-05 02:00:00+00', '{"totals_by_ide":[{"ide":"vscode","code_generation_activity_count":3,"code_acceptance_activity_count":2,"user_initiated_interaction_count":6},{"ide":"jetbrains","code_generation_activity_count":5,"code_acceptance_activity_count":1}]}'),
    (DATE '2026-11-03', 'enterprise', 'nav', TIMESTAMP '2026-11-04 02:00:00+00', '{"totals_by_ide":[{"ide":"fractional","code_generation_activity_count":"1.5","code_acceptance_activity_count":0,"user_initiated_interaction_count":0}]}'),
    (DATE '2026-10-09', 'enterprise', 'nav', TIMESTAMP '2026-10-10 02:00:00+00', '{"totals_by_ide":[{"ide":"vscode","code_generation_activity_count":2,"code_acceptance_activity_count":1,"user_initiated_interaction_count":2}]}'),
    (DATE '2026-10-08', 'enterprise', 'nav', TIMESTAMP '2026-10-09 02:00:00+00', '{"totals_by_ide":[{"ide":"vscode","code_generation_activity_count":900,"code_acceptance_activity_count":900,"user_initiated_interaction_count":900}]}'),
    (DATE '2026-11-05', 'organization', 'nav', TIMESTAMP '2026-11-06 03:00:00+00', '{"totals_by_ide":[{"ide":"vscode","code_generation_activity_count":9999}]}'),
    (DATE '2026-11-05', 'organization', 'nav', TIMESTAMP '2026-11-06 04:00:00+00', '{"totals_by_ide":[{"ide":"vscode","code_generation_activity_count":9999}]}'),
    (DATE '2026-11-06', 'enterprise', 'other', TIMESTAMP '2026-11-07 02:00:00+00', '{"totals_by_ide":[{"ide":"vscode","code_generation_activity_count":9999}]}')
  ])
), actual AS (
`
	const checks = `
)
SELECT
  IF((SELECT COUNT(*) FROM actual) = 3, 'PASS', ERROR('unexpected editors or scope leakage')) AS editor_count,
  IF((SELECT COUNT(*) FROM actual WHERE editor = 'vscode' AND generations = 5 AND acceptances = 3 AND interactions = 12 AND days = 3 AND first_day = '2026-10-09' AND last_day = '2026-11-05') = 1, 'PASS', ERROR('deduplication, scope or 28-day boundary failed')) AS vscode_totals,
  IF((SELECT COUNT(*) FROM actual WHERE editor = 'jetbrains' AND generations = 12 AND acceptances IS NULL AND interactions IS NULL AND days = 2) = 1, 'PASS', ERROR('partial missing counts became known')) AS missing_counts,
  IF((SELECT COUNT(*) FROM actual WHERE editor = 'fractional' AND generations IS NULL AND acceptances = 0 AND interactions = 0 AND days = 1) = 1, 'PASS', ERROR('invalid fractional count or explicit zero mishandled')) AS invalid_and_zero;
`
	sql := input + editorUsageQuery("fixture_usage_metrics") + checks
	if strings.Contains(sql, "`") || strings.Contains(sql, "%s") || regexp.MustCompile(`(?i)\b(CREATE|ALTER|DROP|INSERT|UPDATE|DELETE|MERGE|TRUNCATE)\b`).MatchString(sql) {
		t.Fatal("fixture must be inline-only, resolved SELECT SQL without DDL")
	}
	path := filepath.Join("..", "..", ".local", "editor-usage-inline.sql")
	if os.Getenv("UPDATE_EDITOR_INLINE") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(sql), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != sql {
		t.Fatal("inline fixture is stale; regenerate with UPDATE_EDITOR_INLINE=1 go test -run TestEditorUsageInlineFixture")
	}
}
