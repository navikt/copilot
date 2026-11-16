package main

import (
	"strings"
	"testing"
)

func TestEditorViewScopeAndDeduplication(t *testing.T) {
	sql, err := viewsFS.ReadFile("views/v_editor_stats.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"scope = 'enterprise'", "LOWER(scope_id) = 'nav'", "ROW_NUMBER()", "ORDER BY loaded_at DESC", "FROM daily,", "$.totals_by_ide"} {
		if !strings.Contains(string(sql), want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Count(string(sql), "%s") != 5 {
		t.Fatal("view template parameters changed")
	}
}
