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

type Argument struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Value       string `json:"value,omitempty"`
	ValueHint   string `json:"valueHint,omitempty"`
	Description string `json:"description,omitempty"`
}

type Package struct {
	RegistryType         string                `json:"registryType"`
	Identifier           string                `json:"identifier"`
	Version              string                `json:"version,omitempty"`
	RuntimeHint          string                `json:"runtimeHint,omitempty"`
	Transport            Transport             `json:"transport"`
	EnvironmentVariables []EnvironmentVariable `json:"environmentVariables,omitempty"`
	PackageArguments     []Argument            `json:"packageArguments,omitempty"`
	RuntimeArguments     []Argument            `json:"runtimeArguments,omitempty"`
}

type Repository struct {
	URL       string `json:"url"`
	Source    string `json:"source"`
	ID        string `json:"id,omitempty"`
	Subfolder string `json:"subfolder,omitempty"`
}

type ServerJSON struct {
	Schema      string      `json:"$schema"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	WebsiteURL  string      `json:"websiteUrl,omitempty"`
	Repository  *Repository `json:"repository,omitempty"`
	Remotes     []Transport `json:"remotes,omitempty"`
	Packages    []Package   `json:"packages,omitempty"`
}

type RegistryExtensions struct {
	Status      string    `json:"status"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
	IsLatest    bool      `json:"isLatest"`
}

type UsageExample struct {
	Prompt   string `json:"prompt"`
	Scenario string `json:"scenario"`
}

type NavRegistryMeta struct {
	Tools    []string       `json:"tools,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Examples []UsageExample `json:"examples,omitempty"`
}

type ResponseMeta struct {
	Official    *RegistryExtensions `json:"io.modelcontextprotocol.registry/official,omitempty"`
	NavRegistry *NavRegistryMeta    `json:"io.github.navikt/registry,omitempty"`
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
	Schema      string         `json:"$schema,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Status      string         `json:"status,omitempty"`
	PublishedAt string         `json:"publishedAt,omitempty"`
	WebsiteURL  string         `json:"websiteUrl,omitempty"`
	Repository  *Repository    `json:"repository,omitempty"`
	Tools       []string       `json:"tools,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Examples    []UsageExample `json:"examples,omitempty"`
	Remotes     []Transport    `json:"remotes,omitempty"`
	Packages    []Package      `json:"packages,omitempty"`
}

type StaticRegistryData struct {
	Servers []StaticServerData `json:"servers"`
}
