package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

// dcrClientIDPrefix marks gateway-issued client_ids. The rest is random and
// carries no meaning; it only keeps them apart from CIMD URLs in logs.
const (
	dcrClientIDPrefix  = "dcr-"
	maxDCRClientName   = 64
	maxDCRRedirectURIs = 5
)

type dcrRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// registerHandler implements RFC 7591 dynamic client registration for public
// native clients only: loopback redirect URIs, no client secret. PKCE is
// enforced at /authorize for every client.
func (g *Gateway) registerHandler(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, code, desc string) {
		writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
	}
	ip := extractClientIP(r)
	if !g.dcrLimiter.Allow(ip) {
		w.Header().Set("Retry-After", "3600")
		fail(http.StatusTooManyRequests, "temporarily_unavailable", "registration rate limit exceeded")
		return
	}

	var req dcrRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxDCRBodyBytes))
	if err := dec.Decode(&req); err != nil {
		fail(http.StatusBadRequest, "invalid_client_metadata", "body must be a JSON object")
		return
	}

	redirects, err := filterDCRRedirectURIs(req.RedirectURIs)
	if err != nil {
		fail(http.StatusBadRequest, "invalid_redirect_uri", err.Error())
		return
	}
	if m := req.TokenEndpointAuthMethod; m != "" && m != "none" {
		fail(http.StatusBadRequest, "invalid_client_metadata", "token_endpoint_auth_method must be none")
		return
	}
	grants := req.GrantTypes
	if len(grants) == 0 {
		grants = []string{"authorization_code"}
	}
	for _, gt := range grants {
		if gt != "authorization_code" && gt != "refresh_token" {
			fail(http.StatusBadRequest, "invalid_client_metadata", "unsupported grant_type")
			return
		}
	}
	for _, rt := range req.ResponseTypes {
		if rt != "code" {
			fail(http.StatusBadRequest, "invalid_client_metadata", "response_types must be code")
			return
		}
	}

	random, err := randomToken()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	meta := clientMetadata{
		ClientID:                dcrClientIDPrefix + random,
		ClientName:              sanitizeClientName(req.ClientName),
		RedirectURIs:            redirects,
		GrantTypes:              grants,
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := g.store.Put(r.Context(), kindDCRClient, meta.ClientID, raw, g.cfg.DCRClientTTL); err != nil {
		if errors.Is(err, errStoreFull) {
			w.Header().Set("Retry-After", "3600")
			fail(http.StatusServiceUnavailable, "temporarily_unavailable", "client registration capacity reached")
			return
		}
		slog.Warn("store dcr client failed", "error", err)
		fail(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
		return
	}
	slog.Info("audit dcr", "outcome", "registered", "client_id", meta.ClientID, "client_name", sanitizeLogValue(meta.ClientName))

	now := g.now()
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  meta.ClientID,
		"client_id_issued_at":        now.Unix(),
		"client_name":                meta.ClientName,
		"redirect_uris":              meta.RedirectURIs,
		"grant_types":                meta.GrantTypes,
		"response_types":             meta.ResponseTypes,
		"token_endpoint_auth_method": "none",
	})
}

// filterDCRRedirectURIs keeps http loopback IP-literal redirect URIs. Entries
// on the "localhost" name are dropped (RFC 7591 3.2.1 lets the server replace
// requested values; some clients register both). Anything else is rejected.
func filterDCRRedirectURIs(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, errors.New("redirect_uris is required")
	}
	if len(in) > maxDCRRedirectURIs {
		return nil, errors.New("too many redirect_uris")
	}
	var out []string
	for _, raw := range in {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, errors.New("invalid redirect_uri")
		}
		if isLoopbackRedirect(u) {
			out = append(out, raw)
			continue
		}
		if u.Scheme == "http" && u.Hostname() == "localhost" {
			continue
		}
		return nil, errors.New("only http loopback redirect_uris (127.0.0.1 or [::1]) are allowed")
	}
	if len(out) == 0 {
		return nil, errors.New("at least one redirect_uri on 127.0.0.1 or [::1] is required")
	}
	return out, nil
}

// sanitizeClientName keeps the self-asserted name short and printable. It is
// shown on the consent page as unverified.
func sanitizeClientName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, strings.TrimSpace(name))
	if r := []rune(name); len(r) > maxDCRClientName {
		name = string(r[:maxDCRClientName])
	}
	if name == "" {
		name = "unnamed client"
	}
	return name
}

// registeredClient loads a client registered through DCR. ok is false for
// unknown or expired ids, and always when DCR is disabled.
func (g *Gateway) registeredClient(ctx context.Context, clientID string) (*clientMetadata, bool, error) {
	if !g.cfg.DCREnabled || !strings.HasPrefix(clientID, dcrClientIDPrefix) {
		return nil, false, nil
	}
	raw, ok, err := g.store.Get(ctx, kindDCRClient, clientID)
	if err != nil || !ok {
		return nil, false, err
	}
	var meta clientMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, false, err
	}
	return &meta, true, nil
}

// clientKnown reports whether clientID is allowlisted or a live DCR client.
func (g *Gateway) clientKnown(ctx context.Context, clientID string) bool {
	if clientID == "" {
		return false
	}
	if g.cfg.AllowedClientIDs[clientID] {
		return true
	}
	_, ok, err := g.registeredClient(ctx, clientID)
	return err == nil && ok
}
