package main

import (
	"testing"

	"github.com/navikt/copilot/mcp-registry/internal/registry"
)

func TestValidateAllowListFile(t *testing.T) {
	err := registry.ValidateAllowListFile()
	if err != nil {
		t.Fatalf("allowlist.json validation failed: %v", err)
	}
}
