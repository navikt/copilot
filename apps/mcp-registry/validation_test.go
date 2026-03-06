package main

import (
	"strings"
	"testing"
)

func TestValidateAllowListFile(t *testing.T) {
	err := validateAllowListFile()
	if err != nil {
		t.Fatalf("allowlist.json validation failed: %v", err)
	}
}

func TestValidateRegistry_RequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		data        *StaticRegistryData
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid registry",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty servers",
			data: &StaticRegistryData{
				Servers: []StaticServerData{},
			},
			expectError: true,
			errorMsg:    "registry must contain at least one server",
		},
		{
			name: "missing name",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Description: "Test Description",
						Version:     "1.0.0",
					},
				},
			},
			expectError: true,
			errorMsg:    "'name' is required",
		},
		{
			name: "name without slash",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "test-server",
						Description: "Test Description",
						Version:     "1.0.0",
					},
				},
			},
			expectError: true,
			errorMsg:    "reverse-DNS format",
		},
		{
			name: "name with multiple slashes",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server/extra",
						Description: "Test Description",
						Version:     "1.0.0",
					},
				},
			},
			expectError: true,
			errorMsg:    "cannot contain multiple slashes",
		},
		{
			name: "invalid name format - empty namespace",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "/server",
						Description: "Test Description",
						Version:     "1.0.0",
					},
				},
			},
			expectError: true,
			errorMsg:    "format is invalid",
		},
		{
			name: "invalid name format - empty name part",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/",
						Description: "Test Description",
						Version:     "1.0.0",
					},
				},
			},
			expectError: true,
			errorMsg:    "format is invalid",
		},
		{
			name: "missing description",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:    "io.github.test/server",
						Version: "1.0.0",
					},
				},
			},
			expectError: true,
			errorMsg:    "'description' is required",
		},
		{
			name: "missing version",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
					},
				},
			},
			expectError: true,
			errorMsg:    "'version' is required",
		},
		{
			name: "duplicate server names",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description 1",
						Version:     "1.0.0",
					},
					{
						Name:        "io.github.test/server",
						Description: "Test Description 2",
						Version:     "1.0.0",
					},
				},
			},
			expectError: true,
			errorMsg:    "duplicate server name",
		},
		{
			name: "invalid publishedAt format",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						PublishedAt: "2025-01-01",
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid publishedAt format",
		},
		{
			name: "valid publishedAt RFC3339",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						PublishedAt: "2025-01-01T00:00:00Z",
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid registry with remotes",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Remotes: []Transport{
							{Type: TransportTypeStreamableHTTP, URL: "https://example.com/mcp/"},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid remote url empty",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Remotes: []Transport{
							{Type: TransportTypeStreamableHTTP, URL: ""},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "remotes[0]: 'url' is required",
		},
		{
			name: "invalid transport type",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Remotes: []Transport{
							{Type: "invalid-type", URL: "https://example.com/mcp/"},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "'type' must be one of",
		},
		{
			name: "invalid status",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Status:      "invalid-status",
					},
				},
			},
			expectError: true,
			errorMsg:    "'status' must be one of",
		},
		{
			name: "valid status active",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Status:      StatusActive,
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid status deprecated",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Status:      StatusDeprecated,
					},
				},
			},
			expectError: false,
		},
		{
			name: "description too long",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "This description is way too long and exceeds the 100 character limit set by the MCP registry specification",
						Version:     "1.0.0",
					},
				},
			},
			expectError: true,
			errorMsg:    "'description' must be at most 100 characters",
		},
		{
			name: "name too short",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "a/b",
						Description: "Test Description",
						Version:     "1.0.0",
					},
				},
			},
			expectError: false,
		},
		{
			name: "stdio transport without url is valid",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Remotes: []Transport{
							{Type: TransportTypeStdio},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid package with npm stdio",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{RegistryType: RegistryTypeNPM, Identifier: "@test/mcp", Transport: Transport{Type: TransportTypeStdio}},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid package with pypi",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{RegistryType: RegistryTypePyPI, Identifier: "mcp-server-test", Transport: Transport{Type: TransportTypeStdio}},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid package with oci",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{RegistryType: RegistryTypeOCI, Identifier: "ghcr.io/test/mcp-server", Transport: Transport{Type: TransportTypeStdio}},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid package with nuget",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{RegistryType: RegistryTypeNuGet, Identifier: "TestMcpServer", Transport: Transport{Type: TransportTypeStdio}},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid package with mcpb",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{RegistryType: RegistryTypeMCPB, Identifier: "https://example.com/server.mcpb", Transport: Transport{Type: TransportTypeStdio}},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid server with both remotes and packages",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Remotes: []Transport{
							{Type: TransportTypeStreamableHTTP, URL: "https://example.com/mcp"},
						},
						Packages: []Package{
							{RegistryType: RegistryTypeNPM, Identifier: "@test/mcp", Transport: Transport{Type: TransportTypeStdio}},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "package missing registryType",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{Identifier: "@test/mcp", Transport: Transport{Type: TransportTypeStdio}},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "'registryType' is required",
		},
		{
			name: "package invalid registryType",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{RegistryType: "invalid", Identifier: "@test/mcp", Transport: Transport{Type: TransportTypeStdio}},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "'registryType' must be one of",
		},
		{
			name: "package missing identifier",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{RegistryType: RegistryTypeNPM, Transport: Transport{Type: TransportTypeStdio}},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "'identifier' is required",
		},
		{
			name: "package missing transport type",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{RegistryType: RegistryTypeNPM, Identifier: "@test/mcp"},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "'transport.type' is required",
		},
		{
			name: "package invalid transport type",
			data: &StaticRegistryData{
				Servers: []StaticServerData{
					{
						Name:        "io.github.test/server",
						Description: "Test Description",
						Version:     "1.0.0",
						Packages: []Package{
							{RegistryType: RegistryTypeNPM, Identifier: "@test/mcp", Transport: Transport{Type: "websocket"}},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "'transport.type' must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRegistry(tt.data)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}
