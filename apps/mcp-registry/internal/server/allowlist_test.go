package server

import (
	"os"
	"testing"

	"github.com/navikt/copilot/mcp-registry/internal/registry"
)

func TestDecodeStaticRegistry_RejectsUnknownFields(t *testing.T) {
	allowlistBytes, err := os.ReadFile("../../allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	substituted := substituteVariables(allowlistBytes, &Config{
		DomainInternal: "intern.dev.nav.no",
		DomainExternal: "ekstern.dev.nav.no",
	})

	tests := []struct {
		name    string
		raw     []byte
		wantErr bool
	}{
		{name: "current allowlist.json", raw: allowlistBytes, wantErr: false},
		{name: "substituted allowlist.json", raw: substituted, wantErr: false},
		{
			name:    "misspelled protocolVersion",
			raw:     []byte(`{"servers":[{"name":"io.github.test/s","description":"d","version":"1","protocolVersion":["2026-07-28"]}]}`),
			wantErr: true,
		},
		{
			name:    "misspelled sandboxHost",
			raw:     []byte(`{"servers":[{"name":"io.github.test/s","description":"d","version":"1","sandboxHost":["api.figma.com"]}]}`),
			wantErr: true,
		},
		{
			name:    "unknown top-level key",
			raw:     []byte(`{"servers":[{"name":"io.github.test/s","description":"d","version":"1"}],"extra":1}`),
			wantErr: true,
		},
		{
			name:    "trailing json object",
			raw:     []byte(`{"servers":[{"name":"io.github.test/s","description":"d","version":"1"}]}{"extra":1}`),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := registry.DecodeStaticRegistry(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Errorf("decodeStaticRegistry(%s) err = %v, wantErr=%v", tc.name, err, tc.wantErr)
			}
		})
	}
}
