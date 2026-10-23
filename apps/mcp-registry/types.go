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
	Type    string   `json:"type"`
	URL     string   `json:"url,omitempty"`
	Headers []Header `json:"headers,omitempty"`
}

// Header is a fixed HTTP header a client sends to a remote (KeyValueInput in
// the registry schema).
type Header struct {
	Name        string `json:"name"`
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
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
	Meta        *ServerMeta `json:"_meta,omitempty"`
}

// ServerMeta is server.json's own _meta (the schema allows any namespaced
// key). It travels with the server.json to whoever reads it.
type ServerMeta struct {
	Broker *BrokerExtension `json:"io.github.navikt/broker,omitempty"`
}

// BrokerExtension v1 tells cplt how to run a server on the host for the
// sandbox (navikt/cplt docs/design/mcp-host-broker.md, section 5.2).
// nav-pilot copies it into the spec it writes, adding the tools the user
// allowed. Tools.Classes is every tool cplt may broker; any other is refused.
type BrokerExtension struct {
	V           int               `json:"v"`
	Credential  string            `json:"credential"`
	Command     []string          `json:"command"`
	EgressHosts []string          `json:"egressHosts,omitempty"`
	Tools       BrokerTools       `json:"tools"`
	ArgChecks   map[string]string `json:"argChecks,omitempty"`
	Login       BrokerLogin       `json:"login"`
}

type BrokerTools struct {
	Classes map[string]string `json:"classes"`
}

type BrokerLogin struct {
	Run    string `json:"run"`
	Status string `json:"status"`
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

type SetupInstruction struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Commands    []string `json:"commands"`
}

type NavRegistryMeta struct {
	Tools             []string           `json:"tools,omitempty"`
	Tags              []string           `json:"tags,omitempty"`
	Examples          []UsageExample     `json:"examples,omitempty"`
	SetupInstructions []SetupInstruction `json:"setupInstructions,omitempty"`
	// SandboxHosts are hosts the server needs besides its remotes, such as
	// an OAuth endpoint. Bare host names; nav-pilot asks the user before
	// allowing them in cplt's sandbox.
	SandboxHosts []string `json:"sandboxHosts,omitempty"`
	// ToolRisk classes the tools that are more than a read: "write",
	// "external" or "host-exec". A tool with no entry is a read. nav-pilot
	// turns external and host-exec tools off unless the user picks them.
	ToolRisk map[string]string `json:"toolRisk,omitempty"`
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
	Schema            string             `json:"$schema,omitempty"`
	Name              string             `json:"name"`
	Description       string             `json:"description"`
	Version           string             `json:"version"`
	Status            string             `json:"status,omitempty"`
	PublishedAt       string             `json:"publishedAt,omitempty"`
	WebsiteURL        string             `json:"websiteUrl,omitempty"`
	Repository        *Repository        `json:"repository,omitempty"`
	Tools             []string           `json:"tools,omitempty"`
	Tags              []string           `json:"tags,omitempty"`
	Examples          []UsageExample     `json:"examples,omitempty"`
	SetupInstructions []SetupInstruction `json:"setupInstructions,omitempty"`
	SandboxHosts      []string           `json:"sandboxHosts,omitempty"`
	ToolRisk          map[string]string  `json:"toolRisk,omitempty"`
	Remotes           []Transport        `json:"remotes,omitempty"`
	Packages          []Package          `json:"packages,omitempty"`
	Meta              *ServerMeta        `json:"_meta,omitempty"`
}

type StaticRegistryData struct {
	Servers []StaticServerData `json:"servers"`
}
