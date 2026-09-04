package agentpakke

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDecl(t *testing.T, root, body string) {
	t.Helper()
	path := DeclarationFilePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDeclarationAbsentIsNotAnError(t *testing.T) {
	if _, err := LoadDeclaration(t.TempDir()); !errors.Is(err, ErrNoDeclaration) {
		t.Fatalf("LoadDeclaration on a repo without one = %v, want ErrNoDeclaration", err)
	}
}

func TestLoadDeclaration(t *testing.T) {
	root := t.TempDir()
	writeDecl(t, root, `{
  "contractVersion": "1",
  "source": "navikt/grillmester",
  "sha": "deadbee",
  "minNavPilotVersion": "2026.01.01-000000",
  "items": {"grillmester": "agent"}
}`)
	d, err := LoadDeclaration(root)
	if err != nil {
		t.Fatalf("LoadDeclaration: %v", err)
	}
	if d.Source != "navikt/grillmester" || d.SHA != "deadbee" || d.Items["grillmester"] != "agent" {
		t.Fatalf("LoadDeclaration = %+v", d)
	}
}

// A declaration nav-pilot cannot read must fail the command, not degrade to the
// config key: silently installing something else is exactly what the committed
// file exists to prevent.
func TestLoadDeclarationRejectsMalformed(t *testing.T) {
	tests := []struct {
		name, body, wantMsg string
	}{
		{"not json", `{`, "not valid JSON"},
		{"unsupported contract", `{"contractVersion":"2","source":"navikt/x"}`, "contractVersion"},
		{"no source", `{"contractVersion":"1","source":"  "}`, "must name a source"},
		{"bad item type", `{"contractVersion":"1","source":"navikt/x","items":{"a":"widget"}}`, `type "widget"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeDecl(t, root, tt.body)
			_, err := LoadDeclaration(root)
			if err == nil {
				t.Fatalf("LoadDeclaration accepted %s", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error %q does not mention %q", err, tt.wantMsg)
			}
		})
	}
}

// The merge property is the whole reason for the file's shape: two branches
// that each add an item, or that each bump the SHA, must not fight over a
// timestamp neither of them cares about.
func TestWriteDeclarationIsDeterministicAndTimestampFree(t *testing.T) {
	root := t.TempDir()
	d := &Declaration{
		ContractVersion: "1",
		Source:          "navikt/grillmester",
		SHA:             "deadbee",
		Items:           map[string]string{"zulu": "skill", "alpha": "agent", "mike": "agent"},
	}
	if err := WriteDeclaration(root, d); err != nil {
		t.Fatalf("WriteDeclaration: %v", err)
	}
	first, err := os.ReadFile(DeclarationFilePath(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteDeclaration(root, d); err != nil {
		t.Fatalf("WriteDeclaration (again): %v", err)
	}
	second, _ := os.ReadFile(DeclarationFilePath(root))
	if string(first) != string(second) {
		t.Errorf("two writes of the same declaration differ:\n%s\n---\n%s", first, second)
	}

	if got := strings.Index(string(first), `"alpha"`); got > strings.Index(string(first), `"mike"`) {
		t.Errorf("items are not written in sorted order:\n%s", first)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(first, &raw); err != nil {
		t.Fatal(err)
	}
	for key := range raw {
		if strings.Contains(strings.ToLower(key), "time") || strings.Contains(strings.ToLower(key), "at") {
			t.Errorf("declaration carries a time-like key %q, which conflicts on every parallel change", key)
		}
	}

	back, err := LoadDeclaration(root)
	if err != nil {
		t.Fatalf("round-trip load: %v", err)
	}
	if back.Source != d.Source || back.SHA != d.SHA || len(back.Items) != 3 {
		t.Errorf("round-trip = %+v, want %+v", back, d)
	}
}
