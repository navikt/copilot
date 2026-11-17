package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// prmHandler serves RFC 9728 OAuth 2.0 Protected Resource Metadata.
func (g *Gateway) prmHandler(w http.ResponseWriter, r *http.Request) {
	resource := g.cfg.PrimaryResource
	const prefix = "/.well-known/oauth-protected-resource"
	if r.URL.Path != prefix {
		suffix := strings.TrimPrefix(r.URL.Path, prefix)
		candidate := g.cfg.BaseURL + suffix
		if !g.cfg.AllowedResources[candidate] {
			http.NotFound(w, r)
			return
		}
		resource = candidate
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 resource,
		"authorization_servers":    []string{g.cfg.BaseURL},
		"bearer_methods_supported": []string{"header"},
	})
}

// asMetadataHandler serves RFC 8414 OAuth 2.0 Authorization Server Metadata.
func (g *Gateway) asMetadataHandler(w http.ResponseWriter, _ *http.Request) {
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
	if g.cfg.RTStorage == rtStorageClientEncrypted {
		meta["grant_types_supported"] = []string{"authorization_code", "refresh_token"}
		meta["revocation_endpoint"] = g.cfg.BaseURL + "/revoke"
	}
	writeJSON(w, http.StatusOK, meta)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}
