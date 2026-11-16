package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const entraStateCookieName = "fleet_entra_state"

type pendingAuth struct {
	ClientID    string
	ClientName  string
	RedirectURI string
	State       string
	Challenge   string
	Resource    string
	Prompt      string
	Claims      string
	CSRF        string
	Created     time.Time
}

type clientMetadata struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func (g *Gateway) authorizeHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	if !g.cfg.AllowedClientIDs[clientID] {
		http.Error(w, "unknown client_id: client is not allowlisted", http.StatusBadRequest)
		return
	}

	meta, err := g.fetchCIMD(r.Context(), clientID)
	if err != nil {
		slog.Warn("CIMD fetch or validation failed", "client_id", clientID, "error", err)
		http.Error(w, "unable to validate client metadata document", http.StatusBadRequest)
		return
	}

	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" ||
		!slices.Contains(meta.RedirectURIs, redirectURI) ||
		!g.cfg.AllowedRedirectURIs[redirectURI] {
		http.Error(w, "redirect_uri does not match allowlisted client metadata", http.StatusBadRequest)
		return
	}
	parsedRedirect, err := url.Parse(redirectURI)
	if err != nil || parsedRedirect.Fragment != "" {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}

	state := q.Get("state")
	failRedirect := func(errCode, desc string) {
		v := url.Values{
			"error":             {errCode},
			"error_description": {desc},
			"iss":               {g.cfg.BaseURL},
		}
		if state != "" {
			v.Set("state", state)
		}
		redirectWithValues(w, r, parsedRedirect, v)
	}

	challenge := q.Get("code_challenge")
	resource := q.Get("resource")

	switch {
	case q.Get("response_type") != "code":
		failRedirect("unsupported_response_type", "response_type must be code")
	case challenge == "" || q.Get("code_challenge_method") != "S256" || !isValidCodeChallenge(challenge):
		failRedirect("invalid_request", "PKCE with S256 is required")
	case state == "":
		failRedirect("invalid_request", "state is required")
	case resource == "" || !g.cfg.AllowedResources[resource]:
		failRedirect("invalid_target", "missing or unsupported resource")
	default:
		consentID, err := randomToken()
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		csrf, err := randomToken()
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		p := &pendingAuth{
			ClientID:    clientID,
			ClientName:  meta.ClientName,
			RedirectURI: redirectURI,
			State:       state,
			Challenge:   challenge,
			Resource:    resource,
			Prompt:      q.Get("prompt"),
			Claims:      q.Get("claims"),
			CSRF:        csrf,
			Created:     g.now(),
		}

		// Step 1 never remembers prior consent: always ask before redirecting to Entra.
		if !g.pendingConsent.Set(consentID, p, defaultPendingTTL) {
			http.Error(w, "too many pending authorizations", http.StatusServiceUnavailable)
			return
		}

		g.renderConsentPage(w, consentID, p)
	}
}

func (g *Gateway) fetchCIMD(ctx context.Context, clientID string) (*clientMetadata, error) {
	if cached, ok := g.cimdCache.Get(clientID); ok {
		return cached, nil
	}

	if err := validateHTTPURL(clientID, false); err != nil {
		return nil, fmt.Errorf("invalid client_id URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fleet-gateway/1.0")

	resp, err := g.cfg.HTTPClient.Do(req) //nolint:gosec // URL validated against AllowedClientIDs allowlist
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected CIMD status %d", resp.StatusCode)
	}

	var doc clientMetadata
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCIMDBodyBytes)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode CIMD: %w", err)
	}
	if doc.ClientID != clientID {
		return nil, errors.New("CIMD client_id does not match request URL")
	}
	if doc.TokenEndpointAuthMethod != "" && doc.TokenEndpointAuthMethod != "none" {
		return nil, fmt.Errorf("unsupported token_endpoint_auth_method %q", doc.TokenEndpointAuthMethod)
	}
	if len(doc.RedirectURIs) == 0 {
		return nil, errors.New("CIMD redirect_uris is empty")
	}
	if doc.ClientName == "" {
		doc.ClientName = doc.ClientID
	}

	g.cimdCache.Set(clientID, &doc, defaultCIMDTTL)
	return &doc, nil
}

func (g *Gateway) redirectToEntra(w http.ResponseWriter, r *http.Request, p *pendingAuth) {
	authzEndpoint, err := g.entraAuthorizeEndpoint(r.Context())
	if err != nil {
		slog.Warn("Entra well-known metadata lookup failed", "error", err)
		http.Error(w, "upstream identity provider unavailable", http.StatusBadGateway)
		return
	}

	entraState, err := randomToken()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	p.Created = g.now()
	if !g.pendingEntra.Set(entraState, p, defaultPendingTTL) {
		http.Error(w, "too many pending authorizations", http.StatusServiceUnavailable)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     entraStateCookieName,
		Value:    g.signEntraState(entraState),
		Path:     "/oauth2/callback",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(defaultPendingTTL.Seconds()),
	})

	v := url.Values{
		"client_id":     {g.cfg.AzureClientID},
		"response_type": {"code"},
		"redirect_uri":  {g.cfg.BaseURL + "/oauth2/callback"},
		"response_mode": {"query"},
		"state":         {entraState},
		"scope":         {g.cfg.EntraScope},
	}
	if p.Prompt != "" {
		v.Set("prompt", p.Prompt)
	}
	if p.Claims != "" {
		v.Set("claims", p.Claims)
	}

	http.Redirect(w, r, authzEndpoint+"?"+v.Encode(), http.StatusFound)
}

func (g *Gateway) entraAuthorizeEndpoint(ctx context.Context) (string, error) {
	if g.cfg.AzureAuthorizeEndpoint != "" {
		return g.cfg.AzureAuthorizeEndpoint, nil
	}

	g.wellKnownMu.Lock()
	cached := g.cachedAuthzEndpoint
	g.wellKnownMu.Unlock()
	if cached != "" {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.cfg.AzureWellKnownURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fleet-gateway/1.0")

	resp, err := g.cfg.HTTPClient.Do(req) //nolint:gosec // URL validated at startup
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("well-known returned status %d", resp.StatusCode)
	}

	var metadata struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCIMDBodyBytes)).Decode(&metadata); err != nil {
		return "", fmt.Errorf("decode well-known: %w", err)
	}
	if err := validateHTTPURL(metadata.AuthorizationEndpoint, false); err != nil {
		return "", fmt.Errorf("invalid authorization_endpoint in well-known: %w", err)
	}

	g.wellKnownMu.Lock()
	g.cachedAuthzEndpoint = metadata.AuthorizationEndpoint
	g.wellKnownMu.Unlock()
	return metadata.AuthorizationEndpoint, nil
}

func (g *Gateway) callbackHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	queryState := q.Get("state")

	clearEntraStateCookie(w)

	cookie, err := r.Cookie(entraStateCookieName)
	if err != nil || !g.verifyEntraStateCookie(cookie.Value, queryState) {
		http.Error(w, "state binding cookie missing or invalid; start the login again", http.StatusForbidden)
		return
	}

	p, ok := g.pendingEntra.Pop(queryState)
	if !ok {
		http.Error(w, "unknown or expired login; start again", http.StatusBadRequest)
		return
	}

	redirectParsed, err := url.Parse(p.RedirectURI)
	if err != nil {
		http.Error(w, "invalid stored redirect_uri", http.StatusInternalServerError)
		return
	}

	back := func(v url.Values) {
		v.Set("state", p.State)
		v.Set("iss", g.cfg.BaseURL)
		redirectWithValues(w, r, redirectParsed, v)
	}

	if errCode := q.Get("error"); errCode != "" {
		slog.Info("audit login", "outcome", "entra_error", "error", errCode, "client_id", p.ClientID, "resource", p.Resource)
		back(url.Values{
			"error":             {errCode},
			"error_description": {sanitizeUpstreamDesc(q.Get("error_description"))},
		})
		return
	}

	entraCode := q.Get("code")
	if entraCode == "" {
		slog.Info("audit login", "outcome", "missing_code", "client_id", p.ClientID, "resource", p.Resource)
		back(url.Values{
			"error":             {"invalid_request"},
			"error_description": {"missing authorization code from upstream"},
		})
		return
	}

	out, status, err := g.redeemEntraCode(r.Context(), entraCode)
	at, _ := out["access_token"].(string)
	idt, _ := out["id_token"].(string)
	if err != nil || status != http.StatusOK || (at == "" && idt == "") {
		errType, _ := out["error"].(string)
		slog.Warn("audit login", "outcome", "code_redeem_failed", "status", status, "error", errType, "client_id", p.ClientID, "resource", p.Resource)
		back(url.Values{
			"error":             {"server_error"},
			"error_description": {safeEntraFailureMessage(out, status)},
		})
		return
	}

	sessionTTL := computeSessionTTL(out["expires_in"])
	rt, _ := out["refresh_token"].(string)
	sess := &gatewaySession{
		clientID:     p.ClientID,
		resource:     p.Resource,
		entraToken:   at,
		refreshToken: rt,
		expiry:       g.now().Add(sessionTTL),
	}

	gwCode, err := randomToken()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	issued := &issuedCode{
		ClientID:    p.ClientID,
		RedirectURI: p.RedirectURI,
		Challenge:   p.Challenge,
		Resource:    p.Resource,
		Session:     sess,
		Created:     g.now(),
	}
	if !g.codes.Set(gwCode, issued, defaultCodeTTL) {
		http.Error(w, "too many active authorization codes", http.StatusServiceUnavailable)
		return
	}

	slog.Info("audit login", "outcome", "ok", "client_id", p.ClientID, "resource", p.Resource, "refresh_token_received", rt != "")
	back(url.Values{"code": {gwCode}})
}

func (g *Gateway) redeemEntraCode(ctx context.Context, code string) (map[string]any, int, error) {
	form := url.Values{
		"client_id":     {g.cfg.AzureClientID},
		"client_secret": {g.cfg.AzureClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {g.cfg.BaseURL + "/oauth2/callback"},
		"scope":         {g.cfg.EntraScope},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.AzureTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fleet-gateway/1.0")

	resp, err := g.cfg.HTTPClient.Do(req) //nolint:gosec // URL validated at startup
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var out map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxUpstreamBodyBytes)).Decode(&out); err != nil {
		return nil, resp.StatusCode, err
	}
	return out, resp.StatusCode, nil
}

func (g *Gateway) signEntraState(state string) string {
	mac := hmac.New(sha256.New, g.cfg.CookieSecret)
	mac.Write([]byte("entra-state:"))
	mac.Write([]byte(state))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return state + "." + sig
}

func (g *Gateway) verifyEntraStateCookie(cookieVal, queryState string) bool {
	if queryState == "" || cookieVal == "" {
		return false
	}
	parts := strings.Split(cookieVal, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	expected := g.signEntraState(parts[0])
	if subtle.ConstantTimeCompare([]byte(cookieVal), []byte(expected)) != 1 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[0]), []byte(queryState)) == 1
}

func clearEntraStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     entraStateCookieName,
		Value:    "",
		Path:     "/oauth2/callback",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func redirectWithValues(w http.ResponseWriter, r *http.Request, base *url.URL, v url.Values) {
	target := *base
	target.RawQuery = v.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func isValidCodeChallenge(challenge string) bool {
	if len(challenge) < 43 || len(challenge) > 128 {
		return false
	}
	for i := 0; i < len(challenge); i++ {
		c := challenge[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

func computeSessionTTL(expiresIn any) time.Duration {
	secs := parseNumeric(expiresIn)
	if secs <= 0 {
		return defaultSessionTTL
	}
	d := time.Duration(secs) * time.Second
	if d > defaultSessionMaxTTL {
		return defaultSessionMaxTTL
	}
	return d
}

func parseNumeric(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	case string:
		var f float64
		_ = json.Unmarshal([]byte(n), &f)
		return int64(f)
	default:
		return 0
	}
}

func sanitizeUpstreamDesc(desc string) string {
	if strings.Contains(desc, "AADSTS65001") {
		return "AADSTS65001: missing access to downstream app"
	}
	if len(desc) > 200 {
		return desc[:200]
	}
	return desc
}

func safeEntraFailureMessage(out map[string]any, status int) string {
	errCode, _ := out["error"].(string)
	desc, _ := out["error_description"].(string)
	if strings.Contains(desc, "AADSTS65001") {
		return "AADSTS65001: missing access to downstream app"
	}
	if errCode != "" {
		return fmt.Sprintf("upstream token exchange failed (%s, status %d)", errCode, status)
	}
	return fmt.Sprintf("upstream token exchange failed (status %d)", status)
}
