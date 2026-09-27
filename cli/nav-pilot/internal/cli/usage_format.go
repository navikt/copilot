package cli

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// formatUsageTerminal renders the usage summary as a human-friendly
// dashboard. Numbers use Norwegian space-grouping to match the my-copilot
// web UI convention. Reflects copilot-api's UserMetricsSummary fields
// (interactions/acceptances/lines/active days) — there is no per-user
// "credits" or "subscription" concept in that endpoint (see
// apps/copilot-api/bigquery_stats.go); a daily-credits endpoint exists
// separately (GetUserDailyCredits) but is not yet wired into this command.
func formatUsageTerminal(u *usageResponse) string {
	var b strings.Builder

	title := "GitHub Copilot"
	if u.UserLogin != "" {
		title += " — " + u.UserLogin
	}
	fmt.Fprintf(&b, "  %s\n", bold(title))
	fmt.Fprintln(&b, "  "+strings.Repeat("─", 40))

	fmt.Fprintf(&b, "  Periode:        %d dager (%d aktive)\n", u.DaysInPeriod, u.ActiveDays)
	fmt.Fprintf(&b, "  Interaksjoner:  %s\n", formatNorwegianNumber(u.TotalInteractions))
	fmt.Fprintf(&b, "  Akseptert kode: %s / %s (%.0f%%)\n",
		formatNorwegianNumber(u.TotalAcceptances),
		formatNorwegianNumber(u.TotalGenerations),
		u.acceptanceRate(),
	)
	fmt.Fprintf(&b, "  Linjer:         %s foreslått, %s akseptert\n",
		formatNorwegianNumber(u.TotalLinesSuggested),
		formatNorwegianNumber(u.TotalLinesAccepted),
	)
	if u.CLITotalRequests > 0 {
		fmt.Fprintf(&b, "  CLI:            %s forespørsler, %s økter\n",
			formatNorwegianNumber(u.CLITotalRequests),
			formatNorwegianNumber(u.CLISessions),
		)
	}
	fmt.Fprintln(&b, "  "+strings.Repeat("─", 40))

	if len(u.TopModels) > 0 {
		fmt.Fprintln(&b, "  Mest brukte modeller:")
		for _, m := range u.TopModels {
			fmt.Fprintf(&b, "    %-30s %s\n", m.Model, formatNorwegianNumber(m.Interactions))
		}
	}
	if len(u.Teams) > 0 {
		fmt.Fprintf(&b, "  Team:           %s\n", strings.Join(u.Teams, ", "))
	}

	return b.String()
}

// formatUsageTmux renders a compact single-line summary suitable for a tmux
// status bar segment (Phase 3 polish per the PRD — kept minimal for now).
func formatUsageTmux(u *usageResponse) string {
	return fmt.Sprintf("Copilot %.0f%%", u.acceptanceRate())
}

// capitalize upper-cases the first rune of s (UTF-8 safe), leaving the rest
// unchanged.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// progressBar renders a simple filled/empty block bar for percentage (0-100)
// over the given number of segments.
func progressBar(percentage, segments int) string {
	if segments <= 0 {
		return ""
	}
	filled := percentage * segments / 100
	if filled > segments {
		filled = segments
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", segments-filled)
}

// formatNorwegianNumber groups an integer with spaces every three digits,
// matching the my-copilot web UI's Norwegian locale formatting
// (see apps/my-copilot/src/lib/format.ts formatNumber).
func formatNorwegianNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	result := strings.Join(parts, " ")
	if neg {
		result = "-" + result
	}
	return result
}
