package main

import (
	"testing"
)

func TestValidateServerName(t *testing.T) {
	valid := []string{
		"io.github.navikt/github-mcp",
		"nais/nais-mcp",
		"org-1.0/server_2",
		"a.b.c/d-e_f",
	}
	for _, name := range valid {
		if err := validateServerName(name); err != nil {
			t.Errorf("expected valid name %q, got error: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"noslash",
		"/leadingslash",
		"trailingslash/",
		"too/many/slashes",
		"invalid character/server",
		"server/invalid char",
		"inv@lid/server",
	}
	for _, name := range invalid {
		if err := validateServerName(name); err == nil {
			t.Errorf("expected name %q to be rejected, but it passed", name)
		}
	}
}

func TestValidateServerEntry(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "valid entry",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: false,
		},
		{
			name: "missing remotes",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": []
			}`,
			wantErr: true,
		},
		{
			name: "http instead of https",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "http://mcp.example.com/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "localhost rejected",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "https://localhost:8080/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "ipv4 literal rejected",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "https://127.0.0.1:8080/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "ipv6 literal rejected",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "https://[::1]:8080/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "internal suffix .intern.nav.no rejected",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "https://service.intern.nav.no/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "internal suffix .intern.dev.nav.no rejected",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "https://service.intern.dev.nav.no/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "internal suffix .svc.cluster.local rejected",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "https://service.namespace.svc.cluster.local/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "internal suffix .adeo.no rejected",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "https://host.adeo.no/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "internal suffix with uppercase and trailing dot rejected",
			json: `{
				"name": "io.github.navikt/test-server",
				"remotes": [{"type": "streamable-http", "url": "https://host.ADEO.NO./api"}]
			}`,
			wantErr: true,
		},
		{
			name: "invalid name in entry",
			json: `{
				"name": "invalid-name-no-slash",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: true,
		},
		{
			name:    "empty json",
			json:    `{}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateServerEntry([]byte(tc.json))
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateServerEntry() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
