package cli

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testEntries() []configPageEntry {
	return []configPageEntry{
		{Key: "client", Value: "copilot", Source: "default", Description: "Coding-agent CLI to launch."},
		{Key: "model", Value: "GPT-5.5 (gpt-5.5)", Source: "file", Description: "Model id."},
		{Key: "log_level", Value: "", Source: "unset", Description: "Log level."},
		{Key: "local_enabled", Value: "false", Source: "default", Description: "Dispatch to a local model server."},
	}
}

func TestBuildPageRows(t *testing.T) {
	rows := buildPageRows(testEntries(), "strict")

	var sections []string
	var keys []string
	for _, r := range rows {
		if r.kind == rowSection {
			sections = append(sections, r.section)
		} else {
			keys = append(keys, r.key)
		}
	}

	wantSections := []string{"General", "Logging", "Local models (alpha)", "Actions"}
	if strings.Join(sections, ",") != strings.Join(wantSections, ",") {
		t.Errorf("sections = %v, want %v", sections, wantSections)
	}

	// Every entry plus the three action rows is selectable.
	if len(keys) != len(testEntries())+3 {
		t.Errorf("selectable rows = %d, want %d", len(keys), len(testEntries())+3)
	}
	for _, sentinel := range []string{configPageSandbox, configPagePosture, configPageDone} {
		found := false
		for _, k := range keys {
			if k == sentinel {
				found = true
			}
		}
		if !found {
			t.Errorf("action %q missing from rows", sentinel)
		}
	}

	// The posture row carries its value; plain action rows do not.
	for _, r := range rows {
		if r.key == configPagePosture && r.value != "strict" {
			t.Errorf("posture value = %q, want %q", r.value, "strict")
		}
	}
}

func TestFormatPageValue(t *testing.T) {
	tests := []struct {
		entry configPageEntry
		want  string
	}{
		{configPageEntry{Value: "plan", Source: "file"}, "plan"},
		{configPageEntry{Value: "copilot", Source: "default"}, "(default: copilot)"},
		{configPageEntry{Value: "", Source: "unset"}, "(unset)"},
	}
	for _, tc := range tests {
		if got := formatPageValue(tc.entry); got != tc.want {
			t.Errorf("formatPageValue(%+v) = %q, want %q", tc.entry, got, tc.want)
		}
	}
}

func TestConfigPageModelNavigation(t *testing.T) {
	m := configPageModel{rows: buildPageRows(testEntries(), ""), styles: defaultPageStyles()}

	sel := selectable(m.rows, "")
	// First selectable row must be an entry, never a section header.
	if m.rows[sel[0]].kind != rowEntry {
		t.Fatalf("cursor starts on a section header")
	}

	m.move(1)
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	// Wrap-around both ways.
	m.move(-2)
	if m.cursor != len(sel)-1 {
		t.Errorf("wrap backwards: cursor = %d, want %d", m.cursor, len(sel)-1)
	}
	m.move(1)
	if m.cursor != 0 {
		t.Errorf("wrap forwards: cursor = %d, want 0", m.cursor)
	}

	// Enter selects the row under the cursor.
	m.cursor = 0
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should quit the program")
	}
	if got := next.(configPageModel).choice; got != "client" {
		t.Errorf("enter recorded choice %q, want client", got)
	}
}

func TestConfigPageModelFilter(t *testing.T) {
	m := configPageModel{rows: buildPageRows(testEntries(), ""), styles: defaultPageStyles()}
	total := len(selectable(m.rows, ""))

	m.filter = "log"
	sel := selectable(m.rows, m.filter)
	if len(sel) != 1 || m.rows[sel[0]].key != "log_level" {
		t.Fatalf("filter 'log' matched %v, want only log_level", sel)
	}

	// Filtering hides section headers and caps the position indicator.
	m.cursor = 0
	view := m.View()
	if !strings.Contains(view, "1 of 1") {
		t.Errorf("filtered view missing position indicator:\n%s", view)
	}
	if strings.Contains(view, "General") {
		t.Errorf("filtered view still shows section headers:\n%s", view)
	}

	// A filter that matches nothing must not crash selection or view.
	m.filter = "zzz"
	if r := m.selectedRow(); r != nil {
		t.Errorf("selectedRow with no matches = %+v, want nil", r)
	}
	_ = m.View()

	m.filter = ""
	if got := len(selectable(m.rows, "")); got != total {
		t.Errorf("cleared filter restored %d rows, want %d", got, total)
	}
}

func TestConfigPageModelReset(t *testing.T) {
	writeTestConfig(t, "version = 1\nmodel = \"gpt-5.5\"\n")

	var resetKey string
	m := configPageModel{
		rows:   buildPageRows(testEntries(), ""),
		styles: defaultPageStyles(),
		resetKey: func(key string) (string, error) {
			resetKey = key
			return key + " reset to default", nil
		},
		reload: func() []pageRow { return buildPageRows(testEntries(), "") },
	}

	// Cursor on "model" (a file-sourced entry).
	m.cursor = 1
	if got := m.selectedRow().key; got != "model" {
		t.Fatalf("cursor row = %q, want model", got)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(configPageModel)
	if resetKey != "model" {
		t.Errorf("reset key = %q, want model", resetKey)
	}
	if !strings.Contains(m.status, "reset") {
		t.Errorf("status = %q, want a reset confirmation", m.status)
	}

	// Default-sourced rows refuse a reset.
	m.cursor = 0
	m.status = ""
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(configPageModel)
	if !strings.Contains(m.status, "Only values set in the file") {
		t.Errorf("status = %q, want refusal", m.status)
	}
}

func TestConfigPageViewFixedHeight(t *testing.T) {
	m := configPageModel{
		rows:   buildPageRows(testEntries(), ""),
		styles: defaultPageStyles(),
		width:  80,
		height: 24,
	}

	base := strings.Count(m.View(), "\n")
	for m.cursor = 0; m.cursor < len(selectable(m.rows, "")); m.cursor++ {
		if got := strings.Count(m.View(), "\n"); got != base {
			t.Errorf("cursor %d: view height %d lines, want fixed %d", m.cursor, got, base)
		}
	}

	// The fixed bottom zone: description and keybinding help.
	m.cursor = 0
	view := m.View()
	if !strings.Contains(view, "esc close") {
		t.Errorf("view missing keybinding help:\n%s", view)
	}
	if !strings.Contains(view, "Coding-agent CLI to launch.") {
		t.Errorf("view missing selected-row description:\n%s", view)
	}
}

func TestFixedLines(t *testing.T) {
	// Pads short text, wraps long text, truncates with an ellipsis.
	if got := fixedLines("short", 3, 76); len(got) != 3 || got[0] != "short" || got[1] != "" {
		t.Errorf("fixedLines short = %v", got)
	}
	long := strings.Repeat("word ", 40)
	got := fixedLines(long, 2, 30)
	if len(got) != 2 || !strings.HasSuffix(got[1], "…") {
		t.Errorf("fixedLines long = %v", got)
	}
	if got := fixedLines("", 3, 76); len(got) != 3 {
		t.Errorf("fixedLines empty = %v", got)
	}
}

// TestConfigPageCtrlCAlwaysQuits pins the escape hatch.
//
// Filter mode handled esc, enter, backspace and runes, and returned a no-op for
// everything else — so "/" made Ctrl-C do nothing. Esc still worked, but Ctrl-C
// is the reflex in a terminal, and a settings page you cannot leave with it
// reads as a hung program.
func TestConfigPageCtrlCAlwaysQuits(t *testing.T) {
	for _, filtering := range []bool{false, true} {
		m := configPageModel{rows: buildPageRows(nil, ""), filterOn: filtering}
		got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Errorf("filterOn=%v: ctrl+c produced no command, want tea.Quit", filtering)
		}
		if c := got.(configPageModel).choice; c != configPageDone {
			t.Errorf("filterOn=%v: choice = %q, want %q", filtering, c, configPageDone)
		}
	}
}
