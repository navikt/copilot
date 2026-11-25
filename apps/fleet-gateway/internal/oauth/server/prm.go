package server

import (
	"net/http"
	"strings"

	"github.com/navikt/copilot/fleet-gateway/internal/httpx"
)

// PRM serves RFC 9728 OAuth 2.0 Protected Resource Metadata.
func (g *Server) PRM(w http.ResponseWriter, r *http.Request) {
	table := g.table()
	resource := table.PrimaryResource
	const prefix = "/.well-known/oauth-protected-resource"
	if r.URL.Path != prefix {
		suffix := strings.TrimPrefix(r.URL.Path, prefix)
		candidate := g.cfg.BaseURL + suffix
		if !table.AllowedResources[candidate] {
			http.NotFound(w, r)
			return
		}
		resource = candidate
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"resource":                 resource,
		"authorization_servers":    []string{g.cfg.BaseURL},
		"bearer_methods_supported": []string{"header"},
	})
}

// ASMetadata serves RFC 8414 OAuth 2.0 Authorization Server Metadata.
func (g *Server) ASMetadata(w http.ResponseWriter, _ *http.Request) {
	meta := map[string]any{
		"issuer":                                         g.cfg.BaseURL,
		"authorization_endpoint":                         g.cfg.BaseURL + "/authorize",
		"token_endpoint":                                 g.cfg.BaseURL + "/token",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"client_id_metadata_document_supported":          true,
		"authorization_response_iss_parameter_supported": true,
	}
	if g.cfg.DCREnabled {
		meta["registration_endpoint"] = g.cfg.BaseURL + "/register"
	}
	if g.cfg.RTStorage == RTStorageClientEncrypted {
		meta["grant_types_supported"] = []string{"authorization_code", "refresh_token"}
		meta["revocation_endpoint"] = g.cfg.BaseURL + "/revoke"
	}
	httpx.WriteJSON(w, http.StatusOK, meta)
}
