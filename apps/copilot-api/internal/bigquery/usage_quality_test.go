package bigquery

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
)

// Set USAGE_QUALITY_INLINE_SQL to generate a read-only query from the production SQL.
// Execution is deliberately left to the coordinator with approved BigQuery access.
func TestUsageQualityInlineQuery(t *testing.T) {
	row := func(offset int, scope, id, loaded, raw string) string {
		return fmt.Sprintf("SELECT DATE_SUB(CURRENT_DATE(), INTERVAL %d DAY) day, '%s' scope, '%s' scope_id, TIMESTAMP '%s' loaded_at, JSON '%s' raw_record", offset, scope, id, loaded, raw)
	}
	old, newer := "2026-11-01 00:00:00+00", "2026-11-02 00:00:00+00"
	rows := []string{
		row(0, "enterprise", "nav", old, `{}`),
		row(0, "organization", "navikt", newer, `{"missing_client_telemetry_user_count":72,"below_usage_metrics_minimum_user_count":63}`),
		row(0, "enterprise", "other", newer, `{"missing_client_telemetry_user_count":999}`),
		row(0, "organization", "other", newer, `{"missing_client_telemetry_user_count":999}`),
		row(1, "organization", "navikt", old, `{"missing_client_telemetry_user_count":72}`),
		row(1, "organization", "navikt", newer, `{"missing_client_telemetry_user_count":0,"below_usage_metrics_minimum_user_count":0}`),
		row(2, "enterprise", "nav", old, `{"missing_client_telemetry_user_count":5}`),
		row(2, "enterprise", "nav", newer, `{"missing_client_telemetry_user_count":6,"below_usage_metrics_minimum_user_count":6}`),
		row(3, "enterprise", "nav", old, `{"missing_client_telemetry_user_count":null,"below_usage_metrics_minimum_user_count":null}`),
		row(4, "enterprise", "nav", old, `{}`),
		row(5, "enterprise", "nav", old, `{"missing_client_telemetry_user_count":"72","below_usage_metrics_minimum_user_count":"bad"}`),
		row(6, "enterprise", "nav", old, `{"missing_client_telemetry_user_count":-3,"below_usage_metrics_minimum_user_count":-3}`),
		row(7, "enterprise", "nav", old, `{"missing_client_telemetry_user_count":3.5,"below_usage_metrics_minimum_user_count":3.5}`),
		row(8, "enterprise", "nav", old, `{"missing_client_telemetry_user_count":1e3,"below_usage_metrics_minimum_user_count":1e3}`),
		row(9, "enterprise", "nav", old, `{"missing_client_telemetry_user_count":72,"below_usage_metrics_minimum_user_count":63}`),
		row(28, "enterprise", "nav", newer, `{"missing_client_telemetry_user_count":999}`),
		row(10, "organization", "other", newer, `{"missing_client_telemetry_user_count":999}`),
	}
	// An exact duplicate must not multiply daily rows.
	rows = append(rows, rows[7])
	query := usageQualitySQL("(" + strings.Join(rows, "\nUNION ALL\n") + ")")
	var expected []string
	for offset, count := range []string{"NULL", "0", "6", "NULL", "NULL", "NULL", "NULL", "NULL", "1000", "72"} {
		scope, id, below := "enterprise", "nav", count
		if offset == 1 {
			scope, id = "organization", "navikt"
		}
		if offset == 9 {
			below = "63"
		}
		expected = append(expected, fmt.Sprintf("SELECT CAST(DATE_SUB(CURRENT_DATE(), INTERVAL %d DAY) AS STRING) day, '%s' scope, '%s' scope_id, CAST(%s AS INT64) missing_client_telemetry_user_count, CAST(%s AS INT64) below_usage_metrics_minimum_user_count", offset, scope, id, count, below))
	}
	sql := "-- Expected: actual_rows=10, expected_rows=10, mismatches=0. No tables or writes.\nWITH actual AS (" + query + "), expected AS (" + strings.Join(expected, "\nUNION ALL\n") + `), differences AS (
SELECT * FROM (SELECT * FROM actual EXCEPT DISTINCT SELECT * FROM expected)
UNION ALL
SELECT * FROM (SELECT * FROM expected EXCEPT DISTINCT SELECT * FROM actual)
)
SELECT (SELECT COUNT(*) FROM actual) actual_rows,
       (SELECT COUNT(*) FROM expected) expected_rows,
       (SELECT COUNT(*) FROM differences) mismatches;
`
	if strings.Contains(sql, "`") || strings.Contains(sql, "@") {
		t.Fatal("fixture must have no external table or parameter")
	}
	if output := os.Getenv("USAGE_QUALITY_INLINE_SQL"); output != "" {
		if err := os.WriteFile(output, []byte(sql), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUsageQualityDecodeNullableSQLRow(t *testing.T) {
	schema := bigquery.Schema{
		{Name: "day", Type: bigquery.StringFieldType},
		{Name: "missing_client_telemetry_user_count", Type: bigquery.IntegerFieldType},
		{Name: "below_usage_metrics_minimum_user_count", Type: bigquery.IntegerFieldType},
	}
	for _, value := range []bigquery.Value{nil, int64(0), int64(72)} {
		var day UsageQualityDay
		if err := DecodeRow(schema, []bigquery.Value{"2026-11-03", value, nil}, &day); err != nil {
			t.Fatal(err)
		}
		if day.BelowUsageMetricsMinimum != nil {
			t.Fatal("SQL NULL became zero")
		}
		if value == nil {
			if day.MissingClientTelemetry != nil {
				t.Fatal("SQL NULL became zero")
			}
		} else if day.MissingClientTelemetry == nil || *day.MissingClientTelemetry != value.(int64) {
			t.Fatalf("count changed: %+v", day)
		}
	}
}

func TestUsageQualitySQL(t *testing.T) {
	query := usageQualitySQL("`project.dataset.metrics`")
	for _, guard := range []string{"PARTITION BY day ORDER BY IF(scope = 'enterprise', 0, 1), loaded_at DESC, scope_id", "JSON_TYPE(JSON_QUERY", "= 'number'", "SAFE_CAST", "NULL", "INTERVAL 27 DAY", "FROM `project.dataset.metrics`"} {
		if !strings.Contains(query, guard) {
			t.Errorf("missing guard %q", guard)
		}
	}
	for _, forbidden := range []string{"SUM(", "COALESCE(", "2026-11-05", "user_login", "usage_metrics_requirements"} {
		if strings.Contains(query, forbidden) {
			t.Errorf("unexpected %q", forbidden)
		}
	}
}
