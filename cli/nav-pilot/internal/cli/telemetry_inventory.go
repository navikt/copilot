package cli

import (
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

func recordInstallState(scopeName string, state *StateFile, stateErr error) {
	if stateErr != nil {
		telemetry.RecordStalenessCheck("collection", scopeName, "corrupted")
		return
	}
	if state == nil {
		telemetry.RecordInstallPresent(scopeName, "other", false)
		telemetry.RecordStalenessCheck("collection", scopeName, "no_install")
		return
	}

	telemetry.RecordInstallPresent(scopeName, normalizeCollectionLabel(state.Collection), true)
	counts := countInstalledItemsByTypeAndStatus(state.Files)
	for _, c := range counts {
		telemetry.RecordInstalledItems(scopeName, c.itemType, c.status, c.count)
	}
}

type installedItemCount struct {
	itemType string
	status   string
	count    int64
}

func countInstalledItemsByTypeAndStatus(files []InstalledFile) []installedItemCount {
	acc := map[string]int64{}
	for _, f := range files {
		itemType := installedItemType(f.Path)
		status := installedItemStatus(f.Status)
		acc[itemType+"|"+status]++
	}

	out := make([]installedItemCount, 0, len(acc))
	for key, count := range acc {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, installedItemCount{
			itemType: parts[0],
			status:   parts[1],
			count:    count,
		})
	}
	return out
}

func installedItemType(path string) string {
	normalized := strings.TrimPrefix(path, ".github/")
	switch {
	case strings.HasPrefix(normalized, "agents/"):
		return "agent"
	case strings.HasPrefix(normalized, "skills/"):
		return "skill"
	case strings.HasPrefix(normalized, "instructions/"):
		return "instruction"
	case strings.HasPrefix(normalized, "prompts/"):
		return "prompt"
	default:
		return "unknown"
	}
}

func installedItemStatus(status string) string {
	switch status {
	case fileStatusIgnored:
		return "ignored"
	case fileStatusConflict:
		return "conflict"
	default:
		return "active"
	}
}

// normalizeCollectionLabel buckets a state file's collection value for
// telemetry. The question it answers is not which collection someone has — the
// five were folded into one agentpakke (#468) — but how many installs are still
// waiting to be migrated. So it buckets on the shape of the value, not on the
// names, which leave the code with the collections themselves.
//
// The four buckets are constants, so nothing a user typed into a state file
// can reach a metric label.
func normalizeCollectionLabel(collection string) string {
	switch value := strings.TrimSpace(collection); {
	case value == CollectionAll:
		return "all"
	case value == CollectionAlaCarte:
		return "alacarte"
	// Before the identifier test: a retired collection name has the shape of a
	// pakke identifier, and counting it as migrated would hide exactly the
	// installs this metric exists to count down.
	case agentpakke.IsLegacyCollection(value):
		return "legacy"
	case agentpakke.IsIdentifier(value):
		return "pakke"
	default:
		return "legacy"
	}
}
