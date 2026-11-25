package storage

import (
	"strings"
	"testing"
)

func TestSeatCountsMergeSQL_KeysOnDateAndScope(t *testing.T) {
	sql := seatCountsMergeSQL("`p.d.seat_counts`")
	for _, want := range []string{
		"MERGE `p.d.seat_counts`",
		"ON t.snapshot_date = s.snapshot_date AND t.scope_id = s.scope_id",
		"WHEN MATCHED THEN UPDATE",
		"WHEN NOT MATCHED THEN INSERT",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("merge SQL missing %q:\n%s", want, sql)
		}
	}
}
