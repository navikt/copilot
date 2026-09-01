package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Settings page TUI ───────────────────────────────────────────────────────
//
// A full-page settings view modeled on the Copilot CLI settings screen: a
// fixed header, a scrollable two-column row list (name left, value right),
// and a fixed bottom zone with the position indicator, the selected row's
// description and the keybinding help. Fixed chrome means no layout shift:
// only the row window scrolls.

// pageRowKind separates section headers from selectable rows.
type pageRowKind int

const (
	rowSection pageRowKind = iota
	rowEntry
	rowAction
)

// pageRow is one rendered line in the list window.
type pageRow struct {
	kind        pageRowKind
	key         string // entry key or action sentinel; "" for sections
	section     string // section title, when kind == rowSection
	entry       configPageEntry
	value       string // pre-rendered right-column value
	description string
}

// configPageStyles holds the lipgloss styles so tests can render plainly.
type configPageStyles struct {
	title    lipgloss.Style
	section  lipgloss.Style
	selected lipgloss.Style
	dim      lipgloss.Style
}

func defaultPageStyles() configPageStyles {
	return configPageStyles{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		section:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		selected: lipgloss.NewStyle().Bold(true),
		dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
}

// Fixed chrome heights: the header and the bottom zone never change size, so
// the list window is the only thing that scrolls.
const (
	pageHeaderLines = 3 // title, subtitle, blank
	pageFooterLines = 7 // blank, position, blank, 3 description lines, help
	pageDescLines   = 3
)

// buildPageRows flattens entries into sections (in first-seen group order)
// followed by the action rows.
func buildPageRows(entries []configPageEntry, preset string) []pageRow {
	var rows []pageRow
	seen := map[string]bool{}
	for _, e := range entries {
		group := "General"
		if kd := findKeyDef(e.Key); kd != nil && kd.group != "" {
			group = kd.group
		}
		if !seen[group] {
			seen[group] = true
			rows = append(rows, pageRow{kind: rowSection, section: group})
		}
		rows = append(rows, pageRow{
			kind:        rowEntry,
			key:         e.Key,
			entry:       e,
			value:       formatPageValue(e),
			description: e.Description,
		})
	}
	rows = append(rows, pageRow{kind: rowSection, section: "Actions"})
	rows = append(rows,
		pageRow{
			kind:        rowAction,
			key:         configPageSandbox,
			entry:       configPageEntry{Key: "Configure cplt sandbox settings…"},
			description: "Runs the cplt sandbox wizard (requires cplt on your PATH).",
		},
		pageRow{
			kind:        rowAction,
			key:         configPagePosture,
			entry:       configPageEntry{Key: "cplt security posture"},
			value:       cpltPostureValue(preset),
			description: "Sets cplt sandbox.preset = strict, which turns on gh_guard, git_guard and forced proxy in one key (requires cplt on your PATH).",
		},
		pageRow{
			kind:        rowAction,
			key:         configPageDone,
			entry:       configPageEntry{Key: "Done"},
			description: "Leave the settings page.",
		},
	)
	return rows
}

// formatPageValue renders the right column the way the Copilot CLI settings
// page does: the file value plainly, the built-in default spelled out, and
// "(unset)" when neither exists.
func formatPageValue(e configPageEntry) string {
	switch e.Source {
	case "file":
		return e.Value
	case "default":
		return "(default: " + e.Value + ")"
	default:
		return "(unset)"
	}
}

// cpltPostureValue renders the posture row's right column: the current preset,
// with the recommendation appended when it differs.
func cpltPostureValue(preset string) string {
	switch {
	case preset == "":
		return "(unknown — could not read it from cplt)"
	case cpltRecommendStrict(preset):
		return preset + " (recommended: " + cpltRecommendedPreset + ")"
	default:
		return preset
	}
}

// configPageModel is the bubbletea model for the settings page.
type configPageModel struct {
	rows     []pageRow
	filter   string
	filterOn bool
	cursor   int // index into the visible selectable rows
	offset   int // first visible row line
	width    int
	height   int
	choice   string // set on enter; read after the program quits
	styles   configPageStyles
	// reload rebuilds rows from disk (used by ctrl+r reset).
	reload func() []pageRow
	// resetKey clears an entry's key; returns a status line or an error.
	resetKey func(key string) (string, error)
	status   string
}

// selectable returns the indexes into rows that the cursor can land on.
func selectable(rows []pageRow, filter string) []int {
	var idx []int
	for i, r := range rows {
		if r.kind == rowSection {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(r.entry.Key), strings.ToLower(filter)) {
			continue
		}
		idx = append(idx, i)
	}
	return idx
}

func (m *configPageModel) move(delta int) {
	sel := selectable(m.rows, m.filter)
	if len(sel) == 0 {
		return
	}
	m.cursor = (m.cursor + delta + len(sel)) % len(sel)
}

// selectedRow returns the row under the cursor, or nil when the filter
// matches nothing.
func (m *configPageModel) selectedRow() *pageRow {
	sel := selectable(m.rows, m.filter)
	if len(sel) == 0 {
		return nil
	}
	if m.cursor >= len(sel) {
		m.cursor = len(sel) - 1
	}
	return &m.rows[sel[m.cursor]]
}

func (m configPageModel) Init() tea.Cmd { return nil }

func (m configPageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		// Filter input mode: runes edit the filter, esc/enter leave it.
		if m.filterOn {
			switch msg.Type {
			// Ctrl-C leaves, from here as much as from anywhere. Without this
			// case it fell through to the no-op return below, so opening the
			// filter with "/" disabled the one key a terminal user reaches for
			// when they want out. Esc worked, but nobody tries Esc first.
			case tea.KeyCtrlC:
				m.choice = configPageDone
				return m, tea.Quit
			case tea.KeyEsc, tea.KeyEnter:
				m.filterOn = false
			case tea.KeyBackspace:
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
				}
			case tea.KeyRunes:
				m.filter += string(msg.Runes)
				m.cursor = 0
			}
			return m, nil
		}

		switch msg.String() {
		case "esc", "q", "ctrl+c":
			if m.filter != "" {
				m.filter = ""
				m.cursor = 0
				return m, nil
			}
			m.choice = configPageDone
			return m, tea.Quit
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "/":
			m.filterOn = true
		case "enter":
			if r := m.selectedRow(); r != nil {
				m.choice = r.key
				return m, tea.Quit
			}
		case "ctrl+r":
			r := m.selectedRow()
			if r == nil || r.kind != rowEntry || r.entry.Source != "file" {
				m.status = "Only values set in the file can be reset."
				return m, nil
			}
			if m.resetKey != nil {
				if msg, err := m.resetKey(r.key); err != nil {
					m.status = err.Error()
				} else {
					m.status = msg
				}
			}
			if m.reload != nil {
				m.rows = m.reload()
			}
		}
	}
	return m, nil
}

// valueColumn returns the left edge of the right-hand value column.
func valueColumn(width int) int {
	col := width * 11 / 20
	if col < 36 {
		col = 36
	}
	if col > 64 {
		col = 64
	}
	return col
}

func (m configPageModel) View() string {
	if m.width == 0 {
		m.width = 80
		m.height = 24
	}
	s := m.styles

	var b strings.Builder

	// Header.
	b.WriteString(s.title.Render("Settings") + "\n")
	sub := "Configure your nav-pilot preferences."
	if m.filterOn || m.filter != "" {
		sub = "Filter: /" + m.filter
	}
	b.WriteString(s.dim.Render(sub) + "\n\n")

	// Row window.
	visible := m.height - pageHeaderLines - pageFooterLines
	if visible < 1 {
		visible = 1
	}
	lines := m.renderRows()
	cursorLine := m.cursorLine()
	offset := m.offset
	if cursorLine < offset {
		offset = cursorLine
	}
	if cursorLine >= offset+visible {
		offset = cursorLine - visible + 1
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + visible
	if end > len(lines) {
		end = len(lines)
	}
	for _, l := range lines[offset:end] {
		b.WriteString(l + "\n")
	}
	for i := end - offset; i < visible; i++ {
		b.WriteString("\n")
	}

	// Bottom zone: position, description, help. All fixed height.
	sel := selectable(m.rows, m.filter)
	b.WriteString("\n")
	pos := "0 of 0"
	if len(sel) > 0 {
		pos = fmt.Sprintf("%d of %d", m.cursor+1, len(sel))
	}
	if m.status != "" {
		pos += "  ·  " + m.status
	}
	b.WriteString(pos + "\n\n")

	desc := ""
	if r := m.selectedRow(); r != nil {
		desc = r.description
	}
	for _, l := range fixedLines(desc, pageDescLines, m.width-4) {
		b.WriteString(s.dim.Render(l) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(s.dim.Render("/ search · ↑/↓ navigate · enter edit · ctrl+r reset · esc close"))

	return b.String()
}

// renderRows renders every row line; the cursor row is highlighted.
func (m configPageModel) renderRows() []string {
	sel := selectable(m.rows, m.filter)
	selected := map[int]bool{}
	if len(sel) > 0 && m.cursor < len(sel) {
		selected[sel[m.cursor]] = true
	}
	valCol := valueColumn(m.width)

	var lines []string
	prevSection := false
	for i, r := range m.rows {
		if m.filter != "" && r.kind == rowSection {
			continue
		}
		if m.filter != "" && r.kind != rowSection && !containsFold(r.entry.Key, m.filter) {
			continue
		}
		switch r.kind {
		case rowSection:
			if len(lines) > 0 && !prevSection {
				lines = append(lines, "")
			}
			lines = append(lines, m.styles.section.Render(r.section))
			prevSection = true
		default:
			prevSection = false
			name := r.entry.Key
			prefix := "  "
			if selected[i] {
				prefix = "> "
				name = m.styles.selected.Render(name)
			}
			pad := valCol - 2 - len(r.entry.Key)
			if pad < 1 {
				pad = 1
			}
			lines = append(lines, prefix+name+strings.Repeat(" ", pad)+m.styles.dim.Render(r.value))
		}
	}
	return lines
}

// cursorLine returns the rendered-line index of the cursor row, for scrolling.
func (m configPageModel) cursorLine() int {
	sel := selectable(m.rows, m.filter)
	if len(sel) == 0 {
		return 0
	}
	if m.cursor >= len(sel) {
		m.cursor = len(sel) - 1
	}
	target := sel[m.cursor]
	line := 0
	prevSection := false
	for i, r := range m.rows {
		if m.filter != "" && r.kind == rowSection {
			continue
		}
		if m.filter != "" && r.kind != rowSection && !containsFold(r.entry.Key, m.filter) {
			continue
		}
		if r.kind == rowSection {
			if line > 0 && !prevSection {
				line++
			}
			prevSection = true
		} else {
			prevSection = false
		}
		if i == target {
			return line
		}
		line++
	}
	return 0
}

// fixedLines word-wraps s and pads/truncates to exactly n lines.
func fixedLines(s string, n, width int) []string {
	if width < 10 {
		width = 10
	}
	wrapped := strings.Split(wordWrap(s, width), "\n")
	if s == "" {
		wrapped = []string{""}
	}
	if len(wrapped) > n {
		wrapped = append(wrapped[:n-1], strings.TrimRight(wrapped[n-1], " ")+"…")
	}
	for len(wrapped) < n {
		wrapped = append(wrapped, "")
	}
	return wrapped
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// runConfigPageTUI shows the settings page once and returns the chosen row
// key (an entry key or one of the action sentinels).
func runConfigPageTUI(entries []configPageEntry, preset string) (string, error) {
	m := configPageModel{
		rows:   buildPageRows(entries, preset),
		styles: defaultPageStyles(),
		resetKey: func(key string) (string, error) {
			if err := clearConfigKey(key); err != nil {
				return "", err
			}
			return key + " reset to default", nil
		},
		reload: func() []pageRow {
			cfg, err := readConfig()
			if err != nil {
				return buildPageRows(entries, preset)
			}
			return buildPageRows(buildConfigPageEntries(cfg, resolve(cfg, CLIOverrides{})), preset)
		},
	}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", err
	}
	return final.(configPageModel).choice, nil
}
