package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// All MCP server manifests that set MCP_LEGACY_PROTOCOL_UNTIL must agree on the
// same YYYY-MM-DD cutoff date (#2565).
var expectedLegacyProtocolManifests = []string{
	"echo/.nais/app.yaml",
	"../mcp-dolly/.nais/app.yaml",
	"../mcp-kodeverk/.nais/app.yaml",
	"../mcp-onboarding/.nais/app.yaml",
	"../mcp-team-catalog/.nais/app.yaml",
}

func extractLegacyProtocolUntil(raw []byte) (string, bool, error) {
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- name:") && !strings.HasPrefix(trimmed, "name:") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "name:"))
		name = strings.Trim(name, `"'`)
		if name != "MCP_LEGACY_PROTOCOL_UNTIL" {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" || strings.HasPrefix(next, "#") {
				continue
			}
			if !strings.HasPrefix(next, "value:") {
				return "", true, fmt.Errorf("expected value: after MCP_LEGACY_PROTOCOL_UNTIL, got %q", next)
			}
			val := strings.TrimSpace(strings.TrimPrefix(next, "value:"))
			val = strings.Trim(val, `"'`)
			return val, true, nil
		}
		return "", true, fmt.Errorf("missing value: after MCP_LEGACY_PROTOCOL_UNTIL")
	}
	return "", false, nil
}

func checkLegacyProtocolDates(dates map[string]string) error {
	if len(dates) == 0 {
		return fmt.Errorf("no manifests with MCP_LEGACY_PROTOCOL_UNTIL found")
	}
	paths := make([]string, 0, len(dates))
	for p := range dates {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var expected string
	var mismatches []string
	for _, p := range paths {
		v := dates[p]
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return fmt.Errorf("%s: invalid MCP_LEGACY_PROTOCOL_UNTIL %q (want YYYY-MM-DD): %w", p, v, err)
		}
		if expected == "" {
			expected = v
			continue
		}
		if v != expected {
			mismatches = append(mismatches, fmt.Sprintf("%s=%s", p, v))
		}
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("MCP_LEGACY_PROTOCOL_UNTIL differs across manifests (%s=%s, mismatches: %s)", paths[0], expected, strings.Join(mismatches, ", "))
	}
	return nil
}

func TestLegacyProtocolUntilConsistent(t *testing.T) {
	manifests := map[string]bool{}
	for _, p := range expectedLegacyProtocolManifests {
		manifests[filepath.Clean(p)] = true
	}
	globs, err := filepath.Glob("../*/.nais/app.yaml")
	if err != nil {
		t.Fatalf("glob manifests: %v", err)
	}
	for _, p := range globs {
		raw, err := os.ReadFile(p) //nolint:gosec // repo manifest paths
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if strings.Contains(string(raw), "MCP_LEGACY_PROTOCOL_UNTIL") {
			manifests[filepath.Clean(p)] = true
		}
	}

	dates := make(map[string]string, len(manifests))
	for p := range manifests {
		raw, err := os.ReadFile(p) //nolint:gosec // repo manifest paths
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		val, found, err := extractLegacyProtocolUntil(raw)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if !found {
			t.Fatalf("%s: missing MCP_LEGACY_PROTOCOL_UNTIL", p)
		}
		dates[p] = val
	}

	if err := checkLegacyProtocolDates(dates); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyProtocolUntilDetectsDrift(t *testing.T) {
	if err := checkLegacyProtocolDates(map[string]string{
		"a.yaml": "2027-03-31",
		"b.yaml": "2027-03-31",
	}); err != nil {
		t.Fatalf("matching dates should pass: %v", err)
	}
	if err := checkLegacyProtocolDates(map[string]string{
		"a.yaml": "2027-03-31",
		"b.yaml": "2028-12-31",
	}); err == nil {
		t.Fatal("disagreeing dates must fail")
	}
	if err := checkLegacyProtocolDates(map[string]string{
		"a.yaml": "not-a-date",
	}); err == nil {
		t.Fatal("malformed date must fail")
	}
}
