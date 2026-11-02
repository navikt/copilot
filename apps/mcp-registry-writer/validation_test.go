package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateServerName(t *testing.T) {
	valid := []string{
		"io.github.navikt/github-mcp",
		"nais/nais-mcp",
		"org-1.0/server_2",
		"a.b.c/d-e_f",
		"a.b/c",
	}
	for _, name := range valid {
		if err := validateServerName(name); err != nil {
			t.Errorf("expected valid name %q, got error: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"ab", // length < 3
		"noslash",
		"/leadingslash",
		"trailingslash/",
		"too/many/slashes",
		"invalid character/server",
		"server/invalid char",
		"inv@lid/server",
		".io/server",       // leading dot
		"-io/server",       // leading dash
		"io/.server",       // leading dot after slash
		"io/-server",       // leading dash after slash
		"io/_server",       // leading underscore after slash
		strings.Repeat("a", 201) + "/b", // > 200 chars
	}
	for _, name := range invalid {
		if err := validateServerName(name); err == nil {
			t.Errorf("expected name %q to be rejected, but it passed", name)
		}
	}
}

func TestValidateHTTPSURL(t *testing.T) {
	valid := []string{
		"https://mcp.example.com/api",
		"https://mcp.example.com:443/api",
		"https://api.github.com/repos",
		"https://sub.domain.example.org/path?query=1",
	}
	for _, u := range valid {
		if err := validateHTTPSURL(u); err != nil {
			t.Errorf("expected valid URL %q, got error: %v", u, err)
		}
	}

	invalid := []struct {
		url    string
		reason string
	}{
		{"", "empty URL"},
		{"   ", "whitespace only"},
		{"http://mcp.example.com/api", "non-https scheme"},
		{"ftp://mcp.example.com/api", "ftp scheme"},
		{"https://user:pass@mcp.example.com/api", "userinfo with password"},
		{"https://user@mcp.example.com/api", "userinfo without password"},
		{"https://mcp.example.com:8080/api", "explicit non-443 port 8080"},
		{"https://mcp.example.com:80/api", "explicit non-443 port 80"},
		{"https://mcp.example.com:8443/api", "explicit non-443 port 8443"},
		{"https://mcp.exämple.com/api", "non-ASCII hostname"},
		{"https://mcp.example.com/spëcial", "non-ASCII path"},
		{"https://xn--mller-kva.com/api", "IDN punycode host"},
		{"https://localhost/api", "localhost hostname"},
		{"https://api.localhost/api", "subdomain of localhost"},
		{"https://internal/api", "single label without dot"},
		{"https://.example.com/api", "leading dot in host"},
		{"https://example.com./api", "trailing dot in host"},
		{"https://example..com/api", "consecutive dots in host"},
		{"https://127.0.0.1/api", "IPv4 literal"},
		{"https://192.168.1.1/api", "IPv4 literal private"},
		{"https://10.0.0.1:443/api", "IPv4 literal with port 443"},
		{"https://[::1]/api", "IPv6 loopback literal"},
		{"https://[fe80::1]/api", "IPv6 literal"},
		{"https://[fe80::1%eth0]/api", "IPv6 literal with zone ID"},
		{"https://0x7f.0.0.1/api", "hex IP literal component"},
		{"https://0x7f000001.org/api", "hex label in host"},
		{"https://host.123/api", "numeric last label"},
		{"https://service.intern.nav.no/api", "forbidden suffix .intern.nav.no"},
		{"https://service.intern.dev.nav.no/api", "forbidden suffix .intern.dev.nav.no"},
		{"https://service.svc.cluster.local/api", "forbidden suffix .svc.cluster.local"},
		{"https://host.adeo.no/api", "forbidden suffix .adeo.no"},
		{"https://service.internal/api", "forbidden suffix .internal"},
		{"https://service.local/api", "forbidden suffix .local"},
		{"https://ADEO.NO/api", "uppercase forbidden suffix"},
	}

	for _, tc := range invalid {
		t.Run(tc.reason, func(t *testing.T) {
			if err := validateHTTPSURL(tc.url); err == nil {
				t.Errorf("expected URL %q to be rejected (%s), but it passed", tc.url, tc.reason)
			}
		})
	}
}

func TestValidateOwner(t *testing.T) {
	valid := []string{"team-rocket", "nais", "team1", "mcp-team-dev"}
	for _, o := range valid {
		if err := validateOwner(o); err != nil {
			t.Errorf("expected valid owner %q, got: %v", o, err)
		}
	}

	invalid := []string{
		"",
		"   ",
		"Team-Rocket", // uppercase not allowed
		"team_rocket", // underscore not allowed
		"team.rocket", // dot not allowed
		"team/rocket", // slash not allowed
		"team@rocket", // special char
	}
	for _, o := range invalid {
		if err := validateOwner(o); err == nil {
			t.Errorf("expected owner %q to be rejected, but it passed", o)
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
				"description": "Valid test server description",
				"version": "1.0.0",
				"owner": "team-test",
				"websiteUrl": "https://example.com/server",
				"repository": {
					"source": "github",
					"url": "https://github.com/navikt/test-server"
				},
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: false,
		},
		{
			name: "valid entry with sse remote and no optional fields",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Minimal server",
				"version": "0.1.0",
				"owner": "team-test",
				"remotes": [{"type": "sse", "url": "https://mcp.example.com/sse"}]
			}`,
			wantErr: false,
		},
		{
			name: "missing name",
			json: `{
				"description": "Missing name",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "missing description",
			json: `{
				"name": "io.github.navikt/test-server",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "missing version",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Missing version",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "missing owner",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Missing owner",
				"version": "1.0.0",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "invalid owner uppercase",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Invalid owner",
				"version": "1.0.0",
				"owner": "TEAM-UPPER",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "missing remotes",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Missing remotes",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": []
			}`,
			wantErr: true,
		},
		{
			name: "remote invalid transport type stdio",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Stdio remote",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "stdio", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "remote with headers (unknown field)",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Remote with headers",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api", "headers": [{"name": "X-Auth"}]}]
			}`,
			wantErr: true,
		},
		{
			name: "entry with packages (unknown field)",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "With packages",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}],
				"packages": []
			}`,
			wantErr: true,
		},
		{
			name: "entry with _meta (unknown field)",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "With meta",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}],
				"_meta": {}
			}`,
			wantErr: true,
		},
		{
			name: "entry with sandboxHosts (unknown field)",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "With sandboxHosts",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}],
				"sandboxHosts": ["api.example.com"]
			}`,
			wantErr: true,
		},
		{
			name: "entry with setupInstructions (unknown field)",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "With setupInstructions",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}],
				"setupInstructions": []
			}`,
			wantErr: true,
		},
		{
			name: "entry with status (unknown field)",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "With status",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}],
				"status": "pending"
			}`,
			wantErr: true,
		},
		{
			name: "remote with internal URL",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Internal URL",
				"version": "1.0.0",
				"owner": "team-test",
				"remotes": [{"type": "streamable-http", "url": "https://service.intern.nav.no/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "invalid websiteUrl",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Bad websiteUrl",
				"version": "1.0.0",
				"owner": "team-test",
				"websiteUrl": "http://insecure.example.com",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}`,
			wantErr: true,
		},
		{
			name: "invalid repository without source",
			json: `{
				"name": "io.github.navikt/test-server",
				"description": "Bad repo",
				"version": "1.0.0",
				"owner": "team-test",
				"repository": {"url": "https://github.com/navikt/test"},
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
			sub, canonical, err := validateServerEntry([]byte(tc.json))
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateServerEntry() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr {
				if sub == nil || len(canonical) == 0 {
					t.Fatalf("expected non-nil submission and non-empty canonical JSON")
				}
				// Verify re-marshaled canonical JSON parses cleanly back to Submission
				var redecoded Submission
				if err := json.Unmarshal(canonical, &redecoded); err != nil {
					t.Fatalf("failed to unmarshal canonical JSON: %v", err)
				}
				if redecoded.Name != sub.Name {
					t.Errorf("canonical name = %q, want %q", redecoded.Name, sub.Name)
				}
			}
		})
	}
}
