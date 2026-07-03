package cli

import (
	"fmt"
	"strings"
)

// formatUsageTerminal renders the usage summary as a human-friendly
// dashboard, mirroring the mockup in the PRD (issue #337). Numbers use
// Norwegian space-grouping to match the my-copilot web UI convention.
func formatUsageTerminal(u *usageResponse) string {
	var b strings.Builder

	title := "GitHub Copilot"
	if u.Period != "" {
		title += " — " + u.Period
	}
	fmt.Fprintf(&b, "  %s\n", bold(title))
	fmt.Fprintln(&b, "  "+strings.Repeat("─", 40))

	fmt.Fprintf(&b, "  Kreditt:        %s / %s (%d%%)  %s\n",
		formatNorwegianNumber(u.Credits.Used),
		formatNorwegianNumber(u.Credits.Limit),
		u.Credits.Percentage,
		progressBar(u.Credits.Percentage, 10),
	)
	fmt.Fprintf(&b, "  Interaksjoner:  %s\n", formatNorwegianNumber(u.Interactions.Total))
	fmt.Fprintf(&b, "  Akseptert kode: %s (%.0f%%)\n", formatNorwegianNumber(u.Interactions.Accepted), u.Interactions.AcceptanceRate)
	fmt.Fprintf(&b, "  Aktive dager:   %d\n", u.ActiveDays)
	fmt.Fprintln(&b, "  "+strings.Repeat("─", 40))

	if u.Forecast.ProjectedCredits > 0 {
		fmt.Fprintf(&b, "  Prognose:       %s kreditt\n", formatNorwegianNumber(u.Forecast.ProjectedCredits))
	}
	if u.Subscription.Status != "" {
		statusIcon := yellow("○")
		if strings.EqualFold(u.Subscription.Status, "active") {
			statusIcon = green("✓")
		}
		fmt.Fprintf(&b, "  Abonnement:     %s %s\n", capitalize(u.Subscription.Status), statusIcon)
	}

	return b.String()
}

// formatUsageTmux renders a compact single-line summary suitable for a tmux
// status bar segment (Phase 3 polish per the PRD — kept minimal for now).
func formatUsageTmux(u *usageResponse) string {
	return fmt.Sprintf("Copilot %d%%", u.Credits.Percentage)
}

// capitalize upper-cases the first rune of s, leaving the rest unchanged.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
func formatNorwegianNumber(n int) string {
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
