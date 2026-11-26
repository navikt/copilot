package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"go.yaml.in/yaml/v3"
)

// Once prod reads routes from fleet-control (#2760), pilot-routes.prod.yaml is
// frozen: fleet-control seeded from it once and never reads it again, so an edit
// would reach nothing and mislead the reader. New routes go through
// fleet-control's write API with approval. The gateway still reads the file for
// the primary resource only.
const frozenProdRoutesSHA256 = "daac5d03b334b28411cac8fef3882cd385891711dcb787e343240c80e3b3f266"

func TestProdRoutesFileFrozenAfterSwitch(t *testing.T) {
	manifest, err := os.ReadFile(".nais/prod-gcp.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err = yaml.Unmarshal(manifest, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["routes_source"] != "fleet-control" {
		t.Skip("prod still reads routes from the Git file")
	}
	raw, err := os.ReadFile("pilot-routes.prod.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != frozenProdRoutesSHA256 {
		t.Fatal("pilot-routes.prod.yaml changed after prod switched to fleet-control; register route changes in fleet-control instead (runbook: Switch prod routes to fleet-control). To roll back to the file, revert routes_source in .nais/prod-gcp.yaml in the same PR.")
	}
}
