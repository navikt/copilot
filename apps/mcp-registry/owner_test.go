package main

import (
	"os"
	"strings"
	"testing"
)

func TestAllowlistEveryEntryHasOwner(t *testing.T) {
	raw, err := os.ReadFile("allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := decodeStaticRegistry(raw)
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

func TestStaticOwnerRejectsInvalidTeam(t *testing.T) {
	raw, err := os.ReadFile("allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := decodeStaticRegistry(raw)
	if err != nil {
		t.Fatal(err)
	}
	s := data.Servers[0]
	if err := validateServerEntry(&s, &s, 0, map[string]bool{}); err != nil {
		t.Fatalf("baseline entry must be valid: %v", err)
	}
	s.Owner = "Not A Team"
	if err := validateServerEntry(&s, &s, 0, map[string]bool{}); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Error("expected invalid owner to be rejected")
	}
}
