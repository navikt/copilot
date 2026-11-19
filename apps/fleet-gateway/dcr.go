package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"
)

// dcrClientIDPrefix marks gateway-issued client_ids. The rest is random and
// carries no meaning; it only keeps them apart from CIMD URLs in logs.
const (
	dcrClientIDPrefix  = "dcr-"
	maxDCRClientName   = 64
	maxDCRRedirectURIs = 5
)

// dcrClient is the stored record. IssuedAt bounds the sliding TTL by
// DCRClientMaxAge; RenewedAt throttles renewal writes.
type dcrClient struct {
	clientMetadata
	IssuedAt  time.Time `json:"issued_at"`
	RenewedAt time.Time `json:"renewed_at"`
}

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
	allowed, err := g.allowRegistration(r.Context(), ip)
	if err != nil {
		slog.Warn("dcr rate limit failed", "error", err)
		fail(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
		return
	}
	if !allowed {
		observeRateLimited(rateLimitEndpointRegister)
		if g.rateLimitLog == nil || g.rateLimitLog.Allow(rateLimitEndpointRegister+"\x00"+ip) {
			slog.Warn("rate limited", "endpoint", rateLimitEndpointRegister)
		}
		w.Header().Set("Retry-After", "3600")
		fail(http.StatusTooManyRequests, "temporarily_unavailable", "registration rate limit exceeded")
		return
	}

	var req dcrRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxDCRBodyBytes))
	if err = dec.Decode(&req); err != nil {
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
	// Clients bootstrap only through authorization codes, so authorization_code is
	// required; refresh_token is the only optional extra.
	mayRefresh := slices.Contains(grants, "refresh_token")
	if !slices.Contains(grants, "authorization_code") ||
		slices.ContainsFunc(grants, func(gt string) bool { return gt != "authorization_code" && gt != "refresh_token" }) {
		fail(http.StatusBadRequest, "invalid_client_metadata", "grant_types must be [authorization_code] or [authorization_code, refresh_token]")
		return
	}
	grants = []string{"authorization_code"}
	if mayRefresh {
		grants = append(grants, "refresh_token")
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
	now := g.now()
	raw, err := json.Marshal(dcrClient{clientMetadata: meta, IssuedAt: now, RenewedAt: now})
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	// Full: drop the client with the earliest expiry. Expiry slides only on a
	// successful token exchange, so this is the least recently renewed client.
	evicted, err := g.store.PutEvict(r.Context(), kindDCRClient, meta.ClientID, raw, g.cfg.DCRClientTTL)
	if evicted {
		slog.Info("audit dcr", "outcome", "evicted_oldest")
	}
	if err != nil {
		if errors.Is(err, errStoreFull) {
			w.Header().Set("Retry-After", "3600")
			markShed(w, "store_full")
			fail(http.StatusServiceUnavailable, "temporarily_unavailable", "client registration capacity reached")
			return
		}
		slog.Warn("store dcr client failed", "error", err)
		fail(http.StatusServiceUnavailable, "temporarily_unavailable", "session store unavailable")
		return
	}
	slog.Info("audit dcr", "outcome", "registered", "client_id", meta.ClientID, "client_name", sanitizeLogValue(meta.ClientName))

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

// allowRegistration applies the per-IP and global registration limits through
// the session store, so every replica shares them. The global counter only
// counts requests the per-IP limit let through: one IP cannot use up more of
// the global budget than its own limit.
func (g *Gateway) allowRegistration(ctx context.Context, ip string) (bool, error) {
	n, err := g.store.Incr(ctx, "dcr_ip:"+ip, defaultDCRRateLimitWindow)
	if err != nil || n > g.cfg.DCRRateLimitRequests {
		return false, err
	}
	n, err = g.store.Incr(ctx, "dcr_global", defaultDCRRateLimitWindow)
	if err != nil || n > g.cfg.DCRRateLimitGlobal {
		return false, err
	}
	return true, nil
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
	if err != nil {
		if errors.Is(err, errSealUnknownKID) {
			slog.Warn("dcr client row sealed with unknown key id, treating as missing")
			return nil, false, nil
		}
		if errors.Is(err, errSealDecrypt) || errors.Is(err, errSealMalformed) {
			slog.Warn("unreadable dcr client row, treating as missing", "reason", err.Error())
			_ = g.store.Delete(ctx, kindDCRClient, clientID)
			return nil, false, nil
		}
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	var rec dcrClient
	if err := json.Unmarshal(raw, &rec); err != nil {
		slog.Warn("unreadable dcr client row, treating as missing", "reason", err.Error())
		_ = g.store.Delete(ctx, kindDCRClient, clientID)
		return nil, false, nil
	}
	return &rec.clientMetadata, true, nil
}

// renewClient slides a DCR client's TTL after a successful token exchange, at
// most once per TTL/30, and never past IssuedAt+DCRClientMaxAge. It is not
// called from /authorize: unauthenticated requests must not keep a client
// alive. A failed renewal only logs: the client is live until its expiry.
func (g *Gateway) renewClient(ctx context.Context, clientID string) {
	if !g.cfg.DCREnabled || !strings.HasPrefix(clientID, dcrClientIDPrefix) {
		return
	}
	raw, ok, err := g.store.Get(ctx, kindDCRClient, clientID)
	if err != nil || !ok {
		return
	}
	var rec dcrClient
	if json.Unmarshal(raw, &rec) != nil {
		return
	}
	now := g.now()
	if rec.IssuedAt.IsZero() {
		rec.IssuedAt = now // registered before renewal existed
	}
	if now.Sub(rec.RenewedAt) < g.cfg.DCRClientTTL/30 {
		return
	}
	ttl := min(g.cfg.DCRClientTTL, rec.IssuedAt.Add(g.cfg.DCRClientMaxAge).Sub(now))
	if ttl <= 0 {
		return
	}
	rec.RenewedAt = now
	raw, err = json.Marshal(rec)
	if err == nil {
		err = g.store.Put(ctx, kindDCRClient, rec.ClientID, raw, ttl)
	}
	if err != nil {
		slog.Warn("renew dcr client failed", "error", err)
	}
}

// clientGrants loads a client once: known is true for allowlisted or live DCR
// clients, mayRefresh honours the grant_types a DCR client registered (RFC 7591
// 2: default authorization_code only). Allowlisted CIMD clients may refresh. A
// store error is returned so callers answer 503, not a client error.
func (g *Gateway) clientGrants(ctx context.Context, clientID string) (known, mayRefresh bool, err error) {
	if clientID == "" {
		return false, false, nil
	}
	if g.cfg.AllowedClientIDs[clientID] {
		return true, true, nil
	}
	meta, ok, err := g.registeredClient(ctx, clientID)
	if err != nil || !ok {
		return false, false, err
	}
	return true, slices.Contains(meta.GrantTypes, "refresh_token"), nil
}
