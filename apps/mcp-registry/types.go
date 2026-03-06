package main

import "time"

const (
	CurrentSchemaURL = "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json"

	TransportTypeStreamableHTTP = "streamable-http"
	TransportTypeSSE            = "sse"
	TransportTypeStdio          = "stdio"

	StatusActive     = "active"
	StatusDeprecated = "deprecated"
	StatusDeleted    = "deleted"

	RegistryTypeNPM   = "npm"
	RegistryTypePyPI  = "pypi"
	RegistryTypeOCI   = "oci"
	RegistryTypeNuGet = "nuget"
	RegistryTypeMCPB  = "mcpb"

	NameMinLength        = 3
	NameMaxLength        = 200
	DescriptionMinLength = 1
	DescriptionMaxLength = 100
)

type Transport struct {
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

type EnvironmentVariable struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsRequired  bool   `json:"isRequired,omitempty"`
	IsSecret    bool   `json:"isSecret,omitempty"`
}

type Package struct {
	RegistryType         string                `json:"registryType"`
	Identifier           string                `json:"identifier"`
	Version              string                `json:"version,omitempty"`
	RuntimeHint          string                `json:"runtimeHint,omitempty"`
	Transport            Transport             `json:"transport"`
	EnvironmentVariables []EnvironmentVariable `json:"environmentVariables,omitempty"`
}

type ServerJSON struct {
	Schema      string      `json:"$schema"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Remotes     []Transport `json:"remotes,omitempty"`
	Packages    []Package   `json:"packages,omitempty"`
}

type RegistryExtensions struct {
	Status      string    `json:"status"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
	IsLatest    bool      `json:"isLatest"`
}

type ResponseMeta struct {
	Official *RegistryExtensions `json:"io.modelcontextprotocol.registry/official,omitempty"`
}

type ServerResponse struct {
	Server ServerJSON   `json:"server"`
	Meta   ResponseMeta `json:"_meta"`
}

type Metadata struct {
	NextCursor string `json:"nextCursor,omitempty"`
	Count      int    `json:"count"`
}

type ServerListResponse struct {
	Servers  []ServerResponse `json:"servers"`
	Metadata Metadata         `json:"metadata"`
}

type StaticServerData struct {
	Schema      string      `json:"$schema,omitempty"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Status      string      `json:"status,omitempty"`
	PublishedAt string      `json:"publishedAt,omitempty"`
	Remotes     []Transport `json:"remotes,omitempty"`
	Packages    []Package   `json:"packages,omitempty"`
}

type StaticRegistryData struct {
	Servers []StaticServerData `json:"servers"`
}
