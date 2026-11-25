package registry

import (
	"os"
	"strings"
	"testing"
)

func TestStaticOwnerRejectsInvalidTeam(t *testing.T) {
	raw, err := os.ReadFile("../../allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := DecodeStaticRegistry(raw)
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
