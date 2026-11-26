package api

import (
	"encoding/json"
	"time"
)

// MCPServerSpec is the v1alpha1 spec. target and path are derived; enabled
// is gone (revocation is an action with a trail).
type MCPServerSpec struct {
	DataCategory string          `json:"dataCategory,omitempty"`
	Upstream     *Upstream       `json:"upstream,omitempty"`
	AllowedTools []string        `json:"allowedTools,omitempty"`
	Registry     json.RawMessage `json:"registry,omitempty"`
	// Readme is markdown from the owner team, stored and returned as is.
	// Untrusted: clients must render it without raw HTML.
	Readme string `json:"readme,omitempty"`
	// Tools describes allowed tools for people. Optional per tool.
	Tools []ToolDoc `json:"tools,omitempty"`
	// ToolRisk classes allowed tools as Read or Write (#2903). A tool
	// without an entry is Write. Empty fields are omitted, so specs from
	// before #2903 keep their contentHash.
	ToolRisk map[string]string `json:"toolRisk,omitempty"`
	// DefaultRisk applies to allowed tools without a toolRisk entry (#2967).
	// Unset keeps the old rule: such a tool is write.
	DefaultRisk string `json:"defaultRisk,omitempty"`
	// RosRef points to the risk assessment (ROS). Required in prod when any
	// allowed tool is Write.
	RosRef string `json:"rosRef,omitempty"`
	// Authorization is reserved (owner decision 2) and refused with 422.
	Authorization json.RawMessage `json:"authorization,omitempty"`
}

// ToolDoc documents one tool of an MCP server.
type ToolDoc struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Caps on v0.8 text fields, in bytes. Both are part of the contentHash; empty
// fields are omitted, so specs from before v0.8 keep their hash.
const (
	maxReadmeBytes          = 64 << 10
	maxToolDescriptionBytes = 1024
	// maxServerTools bounds allowedTools and so tools: 256 x 1 KiB keeps a
	// spec well under 1 MiB, and real MCP servers list tens of tools.
	maxServerTools = 256
	maxRosRefBytes = 512
)

// Upstream is a route's upstream target.
type Upstream struct {
	App string `json:"app"`
	URL string `json:"url"`
}

// RouteStatus is the published route of an MCPServer.
type RouteStatus struct {
	Path string `json:"path"`
}

// Pending is the generation awaiting approval. Shown to the owner team only.
type Pending[S any] struct {
	Generation  int    `json:"generation"`
	ContentHash string `json:"contentHash,omitempty"`
	SubmittedBy *Actor `json:"submittedBy,omitempty"`
	// ExpiresAt is when the generation can no longer be approved (#2731).
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	Spec      *S         `json:"spec,omitempty"`
}

// MCPServerStatus is the status of an MCPServer resource.
type MCPServerStatus struct {
	ObservedGeneration   int64  `json:"observedGeneration"`
	PublishedGeneration  int    `json:"publishedGeneration,omitempty"`
	PublishedContentHash string `json:"publishedContentHash,omitempty"`
	// ContentHash is set only by GET .../generations/{n}.
	ContentHash string                  `json:"contentHash,omitempty"`
	Route       *RouteStatus            `json:"route,omitempty"`
	SubmittedBy *Actor                  `json:"submittedBy,omitempty"`
	ApprovedBy  *Actor                  `json:"approvedBy,omitempty"`
	ApprovedAt  *time.Time              `json:"approvedAt,omitempty"`
	Pending     *Pending[MCPServerSpec] `json:"pending,omitempty"`
	Conditions  []Condition             `json:"conditions,omitempty"`
}

// MCPServer is the MCPServer resource.
type MCPServer struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Metadata   ObjectMeta      `json:"metadata"`
	Spec       MCPServerSpec   `json:"spec"`
	Status     MCPServerStatus `json:"status"`
}
