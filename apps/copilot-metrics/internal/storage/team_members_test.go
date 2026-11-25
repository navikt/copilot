package storage

import (
	"strings"
	"testing"
)

func TestTeamMembersMergeSQL_ReplacesDay(t *testing.T) {
	sql := teamMembersMergeSQL("`p.d.team_members`")
	for _, want := range []string{
		"MERGE `p.d.team_members`",
		"UNNEST(@members)",
		"WHEN NOT MATCHED BY SOURCE AND t.date = @date THEN DELETE",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("merge SQL missing %q:\n%s", want, sql)
		}
	}
}
