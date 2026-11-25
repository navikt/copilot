// Package model holds fleet-control's domain types: servers, routes and
// snapshots. Their JSON is fleet-gateway's wire contract; change no tag.
package model

import (
	"encoding/json"
	"time"
)

// Route is a fleet-gateway route. Path is /mcp/<namespace>/<name>.
type Route struct {
	Path        string   `json:"path"`
	Target      string   `json:"target"`
	UpstreamURL string   `json:"upstream_url"`
	Enabled     bool     `json:"enabled"`
	Tools       []string `json:"tools"`
	// ToolRisk is spec.toolRisk resolved with spec.defaultRisk (#2967); a
	// tool without an entry is write (#2903).
	ToolRisk map[string]string `json:"tool_risk,omitempty"`
}

// Server is one published server version: a registry entry, a route, or both.
type Server struct {
	Namespace    string          `json:"namespace"`
	Name         string          `json:"name"`
	RegistryName string          `json:"registryName,omitempty"`
	Registry     json.RawMessage `json:"registry,omitempty"`
	Route        *Route          `json:"-"` // served only by /internal/v1/routes and the routes snapshot
	Status       string          `json:"-"`
	// DataClass is spec.dataCategory of the published generation; empty for
	// generations written before 005 or by /admin/v1.
	DataClass string `json:"-"`
	// GatewayPath is the route a registry entry's gateway remote points to.
	GatewayPath string `json:"-"`
}

// Key is the server's identity, "<namespace>/<name>".
func (s Server) Key() string { return s.Namespace + "/" + s.Name }

// Snapshot is one published generation of a kind ("routes" or "registry").
// Items is the JSON array served to consumers.
type Snapshot struct {
	Kind       string
	Generation int64
	ETag       string
	Items      json.RawMessage
	// CheckedAt is when this replica last confirmed the snapshot is the
	// newest in the database. Served as checked_at; zero means never served.
	CheckedAt time.Time
}

// ResolveToolRisk gives every allowed tool its level when defaultRisk is
// set, so the gateway never needs defaultRisk. Without it the map is
// toolRisk as is, so snapshots of old specs do not change.
func ResolveToolRisk(tools []string, toolRisk map[string]string, defaultRisk string) map[string]string {
	if defaultRisk == "" || len(tools) == 0 {
		return toolRisk
	}
	out := make(map[string]string, len(tools))
	for _, t := range tools {
		out[t] = defaultRisk
		if r, ok := toolRisk[t]; ok {
			out[t] = r
		}
	}
	return out
}
