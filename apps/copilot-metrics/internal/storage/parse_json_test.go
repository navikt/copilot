package storage

import (
	"strings"
	"testing"
)

// GitHub's 2026-10-12 per-user report held 54.50040499999999, which the
// default PARSE_JSON (wide_number_mode 'exact') rejects. No BigQuery emulator
// here, so pin the SQL.
func TestAllocationReplacementSQLRoundsWideNumbers(t *testing.T) {
	sql := AllocationReplacementSQL("t", "r")
	if !strings.Contains(sql, "PARSE_JSON(raw, wide_number_mode=>'round')") {
		t.Fatal("PARSE_JSON must use wide_number_mode=>'round'")
	}
	if !strings.Contains(sql, "WHERE day=@day AND (scope_id=@scopeID OR @complete)") {
		t.Fatal("DELETE must clear fallback scope rows when @complete is true")
	}
}
