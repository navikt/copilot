package server

import (
	"os"
	"testing"

	"github.com/navikt/copilot/mcp-registry/internal/registry"
)

func TestAllowlistEveryEntryHasOwner(t *testing.T) {
	raw, err := os.ReadFile("../../allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := registry.DecodeStaticRegistry(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range data.Servers {
		if s.Owner == "" {
			t.Errorf("%s: owner is required in allowlist.json", s.Name)
		}
		if m := navRegistryMeta(s); m == nil || m.Owner != s.Owner {
			t.Errorf("%s: owner must be served in _meta", s.Name)
		}
	}
}
