package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/navikt/copilot/fleet-gateway/internal/config"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

const entraStateCookieName = "__Host-fleet_entra_state"

// maxEntraStateCookies caps the state cookies a browser holds, so abandoned
// logins cannot grow the Cookie header until requests are refused (431).
const maxEntraStateCookies = 8

// entraStateCookieFor names the per-login state cookie, so parallel logins
// (several gateway routes authenticating at once) don't overwrite each other.
// The set is bounded by MaxAge (defaultPendingTTL): cookies expire on their own,
// and the callback clears its own. Each is ~100 bytes.
func entraStateCookieFor(state string) string {
	h := sha256.Sum256([]byte(state))
	return entraStateCookieName + "_" + hex.EncodeToString(h[:4])
}

type pendingAuth struct {
	ClientID      string    `json:"client_id"`
	ClientName    string    `json:"client_name"`
	Unverified    bool      `json:"unverified,omitempty"`
	RedirectURI   string    `json:"redirect_uri"`
	State         string    `json:"state"`
	Challenge     string    `json:"challenge"`
	Resource      string    `json:"resource"`
	Prompt        string    `json:"prompt,omitempty"`
	Claims        string    `json:"claims,omitempty"`
	EntraVerifier string    `json:"entra_verifier,omitempty"`
	EntraNonce    string    `json:"entra_nonce,omitempty"`
	CSRF          string    `json:"csrf"`
	Created       time.Time `json:"created"`
	// Issued is set when consent is asked after the Entra login: the code is
	// held here until the user approves.
	Issued *issuedCode `json:"issued,omitempty"`
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
	redirectURI := q.Get("redirect_uri")
	var meta *clientMetadata
	unverified := false
	if g.cfg.AllowedClientIDs[clientID] {
		m, err := g.fetchCIMD(r.Context(), clientID)
		if err != nil {
			slog.Warn("CIMD fetch or validation failed", "client_id", sanitizeLogValue(clientID), "error", sanitizeLogValue(err.Error()))
			http.Error(w, "unable to validate client metadata document", http.StatusBadRequest)
			return
		}
		if !g.redirectURIAllowlisted(redirectURI) {
			http.Error(w, "redirect_uri does not match allowlisted client metadata", http.StatusBadRequest)
			return
		}
		meta = m
	} else {
		m, ok, err := g.registeredClient(r.Context(), clientID)
		if err != nil {
			slog.Warn("load registered client failed", "error", err)
			http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
			return
		}
		if !ok {
			renderErrorPage(w, http.StatusBadRequest, "Verktøyet har ikke tilgang", "Verktøyet du logget inn fra, er ikke godkjent for denne gatewayen, eller registreringen er utløpt.", "Start innloggingen på nytt fra verktøyet. Hvis feilen gjentar seg, kontakt teamet som drifter gatewayen.")
			return
		}
		if !slices.Contains(m.GrantTypes, "authorization_code") {
			http.Error(w, "unauthorized_client: authorization_code grant not registered for client", http.StatusBadRequest)
			return
		}
		meta, unverified = m, true
	}

	if redirectURI == "" ||
		!slices.ContainsFunc(meta.RedirectURIs, func(reg string) bool { return redirectURIMatches(reg, redirectURI) }) {
		http.Error(w, "redirect_uri does not match client metadata", http.StatusBadRequest)
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
	prompt := q.Get("prompt")
	claims := q.Get("claims")

	switch {
	case q.Get("response_type") != "code":
		failRedirect("unsupported_response_type", "response_type must be code")
	case challenge == "" || q.Get("code_challenge_method") != "S256" || !isValidCodeChallenge(challenge):
		failRedirect("invalid_request", "PKCE with S256 is required")
	case state == "":
		failRedirect("invalid_request", "state is required")
	case resource == "" || !g.table().AllowedResources[resource]:
		failRedirect("invalid_target", "missing or unsupported resource")
	case !isValidPrompt(prompt):
		failRedirect("invalid_request", "unsupported prompt value")
	case validateClaimsParam(claims) != nil:
		failRedirect("invalid_request", "claims must be a valid JSON object within size limit")
	default:
		// Consent is asked after the Entra login, once the user is known, so a
		// remembered consent can skip the page (callbackHandler).
		g.redirectToEntra(w, r, &pendingAuth{
			ClientID:    clientID,
			ClientName:  meta.ClientName,
			Unverified:  unverified,
			RedirectURI: redirectURI,
			State:       state,
			Challenge:   challenge,
			Resource:    resource,
			Prompt:      prompt,
			Claims:      claims,
			Created:     g.now(),
		})
	}
}

// redirectURIAllowlisted reports whether redirectURI matches an
// ALLOWED_REDIRECT_URIS entry under the same rules as client metadata.
func (g *Gateway) redirectURIAllowlisted(redirectURI string) bool {
	if g.cfg.AllowedRedirectURIs[redirectURI] {
		return true
	}
	for reg := range g.cfg.AllowedRedirectURIs {
		if redirectURIMatches(reg, redirectURI) {
			return true
		}
	}
	return false
}

// redirectURIMatches compares a registered redirect URI with a requested one.
// Exact string match, except for http loopback IP literals (127.0.0.1, [::1]):
// there the port may differ, as RFC 8252 section 7.3 requires for native
// clients that bind an ephemeral port. Scheme, host, path and query must still
// match exactly. The "localhost" name never gets this treatment (RFC 8252 8.3).
func redirectURIMatches(registered, requested string) bool {
	if registered == requested {
		return true
	}
	reg, err := url.Parse(registered)
	if err != nil {
		return false
	}
	req, err := url.Parse(requested)
	if err != nil {
		return false
	}
	return isLoopbackRedirect(reg) && isLoopbackRedirect(req) &&
		reg.Hostname() == req.Hostname() &&
		reg.EscapedPath() == req.EscapedPath() &&
		reg.RawQuery == req.RawQuery
}

// isLoopbackRedirect reports whether u is http on a loopback IP literal, with
// no userinfo, fragment or opaque part, and a numeric port if any.
func isLoopbackRedirect(u *url.URL) bool {
	if u.Scheme != "http" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	switch u.Host {
	case "127.0.0.1", "[::1]":
	default:
		h, port, err := net.SplitHostPort(u.Host)
		if err != nil || (h != "127.0.0.1" && h != "::1") {
			return false
		}
		if !isASCIIDigits(port) {
			return false
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}

func isASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

type cachedCIMD struct {
	doc       *clientMetadata
	fetchedAt time.Time
}

func retryableCIMDStatus(code int) bool {
	return code == http.StatusRequestTimeout ||
		code == http.StatusTooManyRequests ||
		(code >= http.StatusInternalServerError && code < 600)
}

func (g *Gateway) fetchCIMD(ctx context.Context, clientID string) (*clientMetadata, error) {
	stale, hasStale := g.cimdCache.Get(clientID)
	if hasStale && g.now().Before(stale.fetchedAt.Add(defaultCIMDTTL)) {
		return stale.doc, nil
	}

	if err := config.ValidateHTTPURL(clientID, false); err != nil {
		return nil, fmt.Errorf("invalid client_id URL: %w", err)
	}

	fallbackOrErr := func(err error) (*clientMetadata, error) {
		if hasStale && g.now().Before(stale.fetchedAt.Add(cimdStaleMax)) {
			slog.Warn("CIMD fetch failed, serving stale document",
				"client_id", sanitizeLogValue(clientID),
				"age", g.now().Sub(stale.fetchedAt).Round(time.Second).String(),
				"error", sanitizeLogValue(err.Error()))
			return stale.doc, nil
		}
		return nil, err
	}

	evictAndErr := func(err error) (*clientMetadata, error) {
		g.cimdCache.Pop(clientID)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return fallbackOrErr(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fleet-gateway/1.0")

	resp, err := g.cfg.HTTPClient.Do(req) //nolint:gosec // URL validated against AllowedClientIDs allowlist
	if err != nil {
		if resp != nil {
			_ = resp.Body.Close()
			if !retryableCIMDStatus(resp.StatusCode) {
				return evictAndErr(err)
			}
		}
		return fallbackOrErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected CIMD status %d", resp.StatusCode)
		if retryableCIMDStatus(resp.StatusCode) {
			return fallbackOrErr(err)
		}
		return evictAndErr(err)
	}

	var doc clientMetadata
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCIMDBodyBytes)).Decode(&doc); err != nil {
		return evictAndErr(fmt.Errorf("decode CIMD: %w", err))
	}
	if doc.ClientID != clientID {
		return evictAndErr(errors.New("CIMD client_id does not match request URL"))
	}
	if doc.TokenEndpointAuthMethod != "" && doc.TokenEndpointAuthMethod != "none" {
		return evictAndErr(fmt.Errorf("unsupported token_endpoint_auth_method %q", doc.TokenEndpointAuthMethod))
	}
	if len(doc.RedirectURIs) == 0 {
		return evictAndErr(errors.New("CIMD redirect_uris is empty"))
	}
	if doc.ClientName == "" {
		doc.ClientName = doc.ClientID
	}

	g.cimdCache.Set(clientID, cachedCIMD{doc: &doc, fetchedAt: g.now()}, cimdStaleMax)
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
	entraVerifier, err := randomToken()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	entraNonce := ""
	if hasScope(g.cfg.EntraScope, "openid") {
		entraNonce, err = randomToken()
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	p.EntraVerifier = entraVerifier
	p.EntraNonce = entraNonce
	p.Created = g.now()
	if err := g.putPending(r.Context(), kindPendingEntra, entraState, p); err != nil {
		if errors.Is(err, errStoreFull) {
			markShed(w, "store_full")
			http.Error(w, "too many pending authorizations", http.StatusServiceUnavailable)
			return
		}
		slog.Warn("store pending entra state failed", "error", err)
		http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
		return
	}

	// shortcut: cookies carry no timestamp, so at the cap all earlier state cookies
	// are dropped (in-flight logins beyond the cap must restart); upgrade by signing issuedAt.
	var stale []string
	for _, c := range r.Cookies() {
		if strings.HasPrefix(c.Name, entraStateCookieName+"_") {
			stale = append(stale, c.Name)
		}
	}
	if len(stale) >= maxEntraStateCookies {
		for _, n := range stale {
			clearEntraStateCookie(w, n)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     entraStateCookieFor(entraState),
		Value:    g.signEntraState(entraState),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(defaultPendingTTL.Seconds()),
	})

	v := url.Values{
		"client_id":             {g.cfg.AzureClientID},
		"response_type":         {"code"},
		"redirect_uri":          {g.cfg.BaseURL + "/oauth2/callback"},
		"response_mode":         {"query"},
		"state":                 {entraState},
		"scope":                 {g.cfg.EntraScope},
		"code_challenge":        {pkceS256(entraVerifier)},
		"code_challenge_method": {"S256"},
	}
	if entraNonce != "" {
		v.Set("nonce", entraNonce)
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

	g.wellKnownMu.RLock()
	cached := g.cachedAuthzEndpoint
	g.wellKnownMu.RUnlock()
	if cached != "" {
		return cached, nil
	}

	ch := g.wellKnownGroup.DoChan("authz", func() (any, error) {
		g.wellKnownMu.RLock()
		if g.cachedAuthzEndpoint != "" {
			ep := g.cachedAuthzEndpoint
			g.wellKnownMu.RUnlock()
			return ep, nil
		}
		g.wellKnownMu.RUnlock()

		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultOutboundTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, g.cfg.AzureWellKnownURL, nil)
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
		if err := config.ValidateHTTPURL(metadata.AuthorizationEndpoint, false); err != nil {
			return "", fmt.Errorf("invalid authorization_endpoint in well-known: %w", err)
		}

		g.wellKnownMu.Lock()
		g.cachedAuthzEndpoint = metadata.AuthorizationEndpoint
		g.wellKnownMu.Unlock()
		return metadata.AuthorizationEndpoint, nil
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}
		ep, _ := res.Val.(string)
		return ep, nil
	}
}

func (g *Gateway) callbackHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	queryState := q.Get("state")

	name := entraStateCookieFor(queryState)
	cookie, err := r.Cookie(name)
	if err != nil {
		// Rollout compat: logins started before the per-login cookie used a fixed name.
		name = entraStateCookieName
		cookie, err = r.Cookie(name)
	}
	clearEntraStateCookie(w, entraStateCookieFor(queryState))
	if err == nil && name == entraStateCookieName {
		clearEntraStateCookie(w, entraStateCookieName)
	}
	if err != nil || !g.verifyEntraStateCookie(cookie.Value, queryState) {
		http.Error(w, "state binding cookie missing or invalid; start the login again", http.StatusForbidden)
		return
	}

	p, ok, err := g.popPending(r.Context(), kindPendingEntra, queryState)
	legacy := false
	if err == nil && !ok {
		p, ok, err = g.popPending(r.Context(), kindPendingEntraLegacy, queryState)
		legacy = ok
	}
	if err != nil {
		slog.Warn("pop pending entra state failed", "error", err)
		http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
		return
	}
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
		slog.Info("audit login", "outcome", "entra_error", "error", truncateLogValue(errCode, maxLogErrorLen), "client_id", sanitizeLogValue(p.ClientID), "resource", sanitizeLogValue(p.Resource))
		back(url.Values{
			"error":             {errCode},
			"error_description": {sanitizeUpstreamDesc(q.Get("error_description"))},
		})
		return
	}

	entraCode := q.Get("code")
	if entraCode == "" {
		slog.Info("audit login", "outcome", "missing_code", "client_id", sanitizeLogValue(p.ClientID), "resource", sanitizeLogValue(p.Resource))
		back(url.Values{
			"error":             {"invalid_request"},
			"error_description": {"missing authorization code from upstream"},
		})
		return
	}

	out, status, err := g.redeemEntraCode(r.Context(), entraCode, p.EntraVerifier)
	at, _ := out["access_token"].(string)
	idt, _ := out["id_token"].(string)
	logAccessTokenShape(at)
	missingToken := (at == "" && idt == "") || (at == "" && g.cfg.TeamTokenStrategy != strategyRefresh)
	if err != nil || status != http.StatusOK || missingToken {
		errType, _ := out["error"].(string)
		slog.Warn("audit login", "outcome", "code_redeem_failed", "status", status, "error", sanitizeLogValue(errType), "client_id", sanitizeLogValue(p.ClientID), "resource", sanitizeLogValue(p.Resource))
		back(url.Values{
			"error":             {"server_error"},
			"error_description": {safeEntraFailureMessage(out, status)},
		})
		return
	}

	// Identity comes only from a validated id_token; the access token is never
	// decoded for identity, so a missing id_token rejects the login (#2594).
	if idt == "" {
		slog.Warn("audit login", "outcome", "missing_id_token", "client_id", p.ClientID, "resource", p.Resource)
		back(url.Values{
			"error":             {"server_error"},
			"error_description": {"upstream identity token missing"},
		})
		return
	}
	idClaims, parseErr := parseJWTClaims(idt)
	if parseErr == nil {
		parseErr = validateIDTokenClaims(idClaims, g.cfg.AzureIssuer, g.cfg.AzureClientID, g.now())
	}
	if parseErr != nil {
		slog.Warn("audit login", "outcome", "invalid_id_token", "reason", parseErr.Error(), "client_id", p.ClientID, "resource", p.Resource)
		back(url.Values{
			"error":             {"server_error"},
			"error_description": {"upstream identity token validation failed"},
		})
		return
	}

	if p.EntraNonce != "" {
		nonceClaim, _ := idClaims["nonce"].(string)
		if nonceClaim == "" || subtle.ConstantTimeCompare([]byte(nonceClaim), []byte(p.EntraNonce)) != 1 {
			slog.Warn("audit login", "outcome", "nonce_mismatch", "client_id", p.ClientID, "resource", p.Resource)
			back(url.Values{
				"error":             {"server_error"},
				"error_description": {"upstream identity token nonce validation failed"},
			})
			return
		}
	}

	// The token response is hostile: NAVident comes only from Texas
	// introspection of the access token, never from an unverified decode (#2594).
	navIdent, introErr := g.introspectNAVident(r.Context(), at)
	if introErr != nil {
		slog.Warn("audit login", "outcome", "introspection_failed", "reason", introErr.Error(), "client_id", p.ClientID, "resource", p.Resource)
		back(url.Values{
			"error":             {"server_error"},
			"error_description": {"upstream access token validation failed"},
		})
		return
	}
	if navIdent == "" {
		slog.Warn("audit login", "outcome", "missing_navident", "client_id", p.ClientID, "resource", p.Resource)
		back(url.Values{
			"error":             {"access_denied"},
			"error_description": {"missing required NAVident claim"},
		})
		return
	}

	issuedAt := g.now()
	sessionTTL := computeSessionTTL(out["expires_in"], g.cfg.SessionMaxAge)
	rt, _ := out["refresh_token"].(string)
	sessRT := rt
	entraRT := ""
	if g.cfg.RTStorage == rtStorageClientEncrypted {
		// Never persist the Entra refresh token in the session store in client-encrypted
		// mode; carry it transiently on the 60s single-use code until /token seals it.
		sessRT = ""
		entraRT = rt
	}
	sess := gatewaySession{
		ClientID:     p.ClientID,
		Resource:     p.Resource,
		EntraToken:   at,
		RefreshToken: sessRT,
		NAVIdent:     navIdent,
		IssuedAt:     issuedAt,
		Expiry:       issuedAt.Add(sessionTTL),
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
		EntraRT:     entraRT,
		Created:     issuedAt,
	}
	remembered, err := g.hasConsent(r.Context(), navIdent, p.ClientID, p.Resource)
	if err != nil {
		slog.Warn("consent lookup failed", "error", err)
	}
	if remembered && p.Prompt != "consent" && !legacy {
		g.issueCode(w, r, p, gwCode, issued)
		return
	}
	if p.Prompt == "none" {
		slog.Info("audit login", "outcome", "consent_required", "navident", navIdent, "client_id", p.ClientID, "resource", p.Resource)
		back(url.Values{"error": {"consent_required"}})
		return
	}

	consentID, err := randomToken()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if p.CSRF, err = randomToken(); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	p.Issued = issued
	if err := g.putPending(r.Context(), kindPendingConsent, consentID, p); err != nil {
		if errors.Is(err, errStoreFull) {
			markShed(w, "store_full")
			http.Error(w, "too many pending authorizations", http.StatusServiceUnavailable)
			return
		}
		slog.Warn("store pending consent failed", "error", err)
		http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
		return
	}
	g.renderConsentPage(w, consentID, p)
}

// issueCode stores the gateway authorization code and sends the browser back
// to the client.
func (g *Gateway) issueCode(w http.ResponseWriter, r *http.Request, p *pendingAuth, gwCode string, issued *issuedCode) {
	if err := g.putCode(r.Context(), gwCode, issued); err != nil {
		if errors.Is(err, errStoreFull) {
			markShed(w, "store_full")
			http.Error(w, "too many active authorization codes", http.StatusServiceUnavailable)
			return
		}
		slog.Warn("store authorization code failed", "error", err)
		http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
		return
	}

	slog.Info("audit login", "outcome", "ok", "navident", issued.Session.NAVIdent, "client_id", p.ClientID, "resource", p.Resource, "refresh_token_received", issued.EntraRT != "" || issued.Session.RefreshToken != "")
	g.redirectToClient(w, r, p, url.Values{"code": {gwCode}})
}

func (g *Gateway) redirectToClient(w http.ResponseWriter, r *http.Request, p *pendingAuth, v url.Values) {
	target, err := url.Parse(p.RedirectURI)
	if err != nil {
		http.Error(w, "invalid stored redirect_uri", http.StatusInternalServerError)
		return
	}
	v.Set("state", p.State)
	v.Set("iss", g.cfg.BaseURL)
	redirectWithValues(w, r, target, v)
}

func (g *Gateway) redeemEntraCode(ctx context.Context, code, codeVerifier string) (map[string]any, int, error) {
	form := url.Values{
		"client_id":     {g.cfg.AzureClientID},
		"client_secret": {g.cfg.AzureClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {g.cfg.BaseURL + "/oauth2/callback"},
		"scope":         {g.cfg.EntraScope},
	}
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}

	body, status, err := g.postEntraToken(ctx, form)
	if err != nil {
		return nil, status, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, status, err
	}
	return out, status, nil
}

func (g *Gateway) postEntraToken(ctx context.Context, form url.Values) ([]byte, int, error) {
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBodyBytes))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (g *Gateway) signEntraState(state string) string {
	return signEntraStateWith(g.cfg.CookieSecret, state)
}

func signEntraStateWith(key []byte, state string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("entra-state:"))
	mac.Write([]byte(state))
	return state + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (g *Gateway) verifyEntraStateCookie(cookieVal, queryState string) bool {
	if queryState == "" || cookieVal == "" {
		return false
	}
	parts := strings.Split(cookieVal, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	valid := false
	for _, key := range append([][]byte{g.cfg.CookieSecret}, g.cfg.CookieVerifyKeys...) {
		if subtle.ConstantTimeCompare([]byte(cookieVal), []byte(signEntraStateWith(key, parts[0]))) == 1 {
			valid = true
		}
	}
	if !valid {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[0]), []byte(queryState)) == 1
}

func clearEntraStateCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func redirectWithValues(w http.ResponseWriter, r *http.Request, base *url.URL, v url.Values) {
	target := *base
	q := target.Query()
	for k, vals := range v {
		q[k] = vals
	}
	target.RawQuery = q.Encode()
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

func computeSessionTTL(expiresIn any, maxAge time.Duration) time.Duration {
	if maxAge <= 0 {
		maxAge = defaultSessionMaxAge
	}
	secs := parseNumeric(expiresIn)
	if secs <= 0 {
		return min(defaultSessionTTL, maxAge)
	}
	d := time.Duration(secs) * time.Second
	if d > maxAge {
		return maxAge
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

func isValidPrompt(prompt string) bool {
	switch prompt {
	case "", "none", "login", "consent", "select_account":
		return true
	default:
		return false
	}
}

func validateClaimsParam(claims string) error {
	if claims == "" {
		return nil
	}
	if len(claims) > maxClaimsBytes {
		return errors.New("claims parameter exceeds maximum size")
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(claims), &obj); err != nil || obj == nil {
		return errors.New("claims must be a valid JSON object")
	}
	return nil
}

func hasScope(scopes, target string) bool {
	for _, s := range strings.Fields(scopes) {
		if s == target {
			return true
		}
	}
	return false
}

func parseJWTClaims(rawJWT string) (map[string]any, error) {
	parts := strings.Split(rawJWT, ".")
	if len(parts) != 3 || parts[1] == "" {
		return nil, errors.New("malformed JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil || claims == nil {
		return nil, errors.New("invalid JWT claims JSON")
	}
	return claims, nil
}

// idTokenSkew is the clock skew tolerated on exp, nbf and iat.
const idTokenSkew = 2 * time.Minute

// validateIDTokenClaims checks iss, aud/azp and exp/nbf/iat of the Entra
// id_token (OIDC Core 3.1.3.7, steps 2-5 and 9-10). The signature is not
// checked: the token comes straight from the configured Entra token endpoint
// over TLS in the back-channel code redemption, which step 6 allows to stand
// in for signature validation. Every claim is still treated as hostile, and a
// missing claim or empty configuration fails closed.
func validateIDTokenClaims(claims map[string]any, issuer, clientID string, now time.Time) error {
	if issuer == "" || clientID == "" {
		return errors.New("id_token validation not configured")
	}
	if iss, _ := claims["iss"].(string); iss != issuer {
		return errors.New("id_token iss mismatch")
	}
	var auds []string
	switch v := claims["aud"].(type) {
	case string:
		auds = []string{v}
	case []any:
		for _, a := range v {
			s, ok := a.(string)
			if !ok {
				return errors.New("id_token aud malformed")
			}
			auds = append(auds, s)
		}
	}
	if !slices.Contains(auds, clientID) {
		return errors.New("id_token aud mismatch")
	}
	if azp, ok := claims["azp"]; (len(auds) > 1 || ok) && azp != clientID {
		return errors.New("id_token azp mismatch")
	}
	numeric := func(name string) (time.Time, bool) {
		f, ok := claims[name].(float64)
		// Upper bound (year 9999) keeps int64 conversion and time math from overflowing.
		if !ok || f <= 0 || f > 253402300799 {
			return time.Time{}, false
		}
		return time.Unix(int64(f), 0), true
	}
	exp, ok := numeric("exp")
	if !ok || !now.Before(exp.Add(idTokenSkew)) {
		return errors.New("id_token expired or exp missing")
	}
	iat, ok := numeric("iat")
	if !ok || iat.After(now.Add(idTokenSkew)) {
		return errors.New("id_token iat missing or in the future")
	}
	if _, present := claims["nbf"]; present {
		nbf, ok := numeric("nbf")
		if !ok || nbf.After(now.Add(idTokenSkew)) {
			return errors.New("id_token not yet valid")
		}
	}
	return nil
}

// introspectNAVident validates the Entra access token at Texas and returns
// its NAVident. Any failure, an inactive token or a missing NAVident returns
// "" or an error; the caller rejects the login.
func (g *Gateway) introspectNAVident(ctx context.Context, accessToken string) (string, error) {
	if accessToken == "" {
		return "", errors.New("access token missing")
	}
	payload, err := json.Marshal(map[string]string{"identity_provider": "entra_id", "token": accessToken})
	if err != nil {
		return "", errors.New("encode introspection request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.TexasIntrospectEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", errors.New("build introspection request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := g.cfg.HTTPClient.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", errors.New("introspection request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("introspection status %d", resp.StatusCode)
	}
	var out struct {
		Active   bool   `json:"active"`
		NAVident string `json:"NAVident"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxUpstreamBodyBytes)).Decode(&out); err != nil {
		return "", errors.New("invalid introspection response")
	}
	if !out.Active {
		return "", errors.New("access token inactive")
	}
	return strings.TrimSpace(out.NAVident), nil
}

// logAccessTokenShape logs only non-secret claims (aud, ver, azp, scp) of the
// Entra access token, to diagnose OBO audience errors. Never the token.
func logAccessTokenShape(at string) {
	parts := strings.Split(at, ".")
	if len(parts) != 3 {
		return
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return
	}
	var c struct {
		Aud any    `json:"aud"`
		Ver string `json:"ver"`
		Azp string `json:"azp"`
		Scp string `json:"scp"`
	}
	if json.Unmarshal(raw, &c) != nil {
		return
	}
	slog.Debug("entra access token shape", "aud", fmt.Sprint(c.Aud), "ver", c.Ver, "azp", c.Azp, "scp", c.Scp)
}
