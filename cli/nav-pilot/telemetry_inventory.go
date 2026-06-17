package main

import "strings"

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

func normalizeCollectionLabel(collection string) string {
	switch strings.TrimSpace(collection) {
	case CollectionAll:
		return "all"
	case "fullstack", "kotlin-backend", "frontend", "nextjs-frontend", "platform":
		return collection
	default:
		return "other"
	}
}
