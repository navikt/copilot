package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/config"
	"github.com/navikt/copilot/fleet-gateway/internal/httpx"
	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
	"github.com/navikt/copilot/fleet-gateway/internal/oauth"
	"github.com/navikt/copilot/fleet-gateway/internal/oauth/entra"
	"github.com/navikt/copilot/fleet-gateway/internal/proxy"
	"github.com/navikt/copilot/fleet-gateway/internal/session"
)

// EntraStateCookieName prefixes the per-login cookie that binds the Entra state.
const EntraStateCookieName = "__Host-fleet_entra_state"

// MaxEntraStateCookies caps the state cookies a browser holds, so abandoned
// logins cannot grow the Cookie header until requests are refused (431).
const MaxEntraStateCookies = 8

// EntraStateCookieFor names the per-login state cookie, so parallel logins
// (several gateway routes authenticating at once) don't overwrite each other.
// The set is bounded by MaxAge (oauth.PendingTTL): cookies expire on their own,
// and the callback clears its own. Each is ~100 bytes.
func EntraStateCookieFor(state string) string {
	h := sha256.Sum256([]byte(state))
	return EntraStateCookieName + "_" + hex.EncodeToString(h[:4])
}

// ClientMetadata is a client's CIMD document or DCR registration.
type ClientMetadata struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// Authorize handles GET /authorize.
func (g *Server) Authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	var meta *ClientMetadata
	unverified := false
	if g.cfg.AllowedClientIDs[clientID] {
		m, err := g.FetchCIMD(r.Context(), clientID)
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
		// remembered consent can skip the page (Callback).
		g.redirectToEntra(w, r, &oauth.PendingAuth{
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
func (g *Server) redirectURIAllowlisted(redirectURI string) bool {
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

// CachedCIMD is a fetched CIMD document and when it was fetched.
type CachedCIMD struct {
	doc       *ClientMetadata
	fetchedAt time.Time
}

func retryableCIMDStatus(code int) bool {
	return code == http.StatusRequestTimeout ||
		code == http.StatusTooManyRequests ||
		(code >= http.StatusInternalServerError && code < 600)
}

// FetchCIMD returns the client metadata document for a URL client_id.
func (g *Server) FetchCIMD(ctx context.Context, clientID string) (*ClientMetadata, error) {
	stale, hasStale := g.cimdCache.Get(clientID)
	if hasStale && g.now().Before(stale.fetchedAt.Add(DefaultCIMDTTL)) {
		return stale.doc, nil
	}

	if err := config.ValidateHTTPURL(clientID, false); err != nil {
		return nil, fmt.Errorf("invalid client_id URL: %w", err)
	}

	fallbackOrErr := func(err error) (*ClientMetadata, error) {
		if hasStale && g.now().Before(stale.fetchedAt.Add(CIMDStaleMax)) {
			slog.Warn("CIMD fetch failed, serving stale document",
				"client_id", sanitizeLogValue(clientID),
				"age", g.now().Sub(stale.fetchedAt).Round(time.Second).String(),
				"error", sanitizeLogValue(err.Error()))
			return stale.doc, nil
		}
		return nil, err
	}

	evictAndErr := func(err error) (*ClientMetadata, error) {
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

	var doc ClientMetadata
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

	g.cimdCache.Set(clientID, CachedCIMD{doc: &doc, fetchedAt: g.now()}, CIMDStaleMax)
	return &doc, nil
}

func (g *Server) redirectToEntra(w http.ResponseWriter, r *http.Request, p *oauth.PendingAuth) {
	authzEndpoint, err := g.entra.AuthorizeEndpoint(r.Context())
	if err != nil {
		slog.Warn("Entra well-known metadata lookup failed", "error", err)
		metrics.IDPErrors.WithLabelValues("metadata").Inc()
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
	if err := g.PutPending(r.Context(), session.KindPendingEntra, entraState, p); err != nil {
		if errors.Is(err, session.ErrStoreFull) {
			httpx.MarkShed(w, "store_full")
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
		if strings.HasPrefix(c.Name, EntraStateCookieName+"_") {
			stale = append(stale, c.Name)
		}
	}
	if len(stale) >= MaxEntraStateCookies {
		for _, n := range stale {
			clearEntraStateCookie(w, n)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     EntraStateCookieFor(entraState),
		Value:    g.SignEntraState(entraState),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(oauth.PendingTTL.Seconds()),
	})

	v := url.Values{
		"client_id":             {g.cfg.AzureClientID},
		"response_type":         {"code"},
		"redirect_uri":          {g.cfg.BaseURL + "/oauth2/callback"},
		"response_mode":         {"query"},
		"state":                 {entraState},
		"scope":                 {g.cfg.EntraScope},
		"code_challenge":        {PKCES256(entraVerifier)},
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

// Callback handles GET /oauth2/callback from Entra.
func (g *Server) Callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	queryState := q.Get("state")

	name := EntraStateCookieFor(queryState)
	cookie, err := r.Cookie(name)
	if err != nil {
		// Rollout compat: logins started before the per-login cookie used a fixed name.
		name = EntraStateCookieName
		cookie, err = r.Cookie(name)
	}
	clearEntraStateCookie(w, EntraStateCookieFor(queryState))
	if err == nil && name == EntraStateCookieName {
		clearEntraStateCookie(w, EntraStateCookieName)
	}
	if err != nil || !g.VerifyEntraStateCookie(cookie.Value, queryState) {
		http.Error(w, "state binding cookie missing or invalid; start the login again", http.StatusForbidden)
		return
	}

	p, ok, err := g.PopPending(r.Context(), session.KindPendingEntra, queryState)
	legacy := false
	if err == nil && !ok {
		p, ok, err = g.PopPending(r.Context(), session.KindPendingEntraLegacy, queryState)
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
		slog.Info("audit login", "outcome", "entra_error", "error", TruncateLogValue(errCode, MaxLogErrorLen), "client_id", sanitizeLogValue(p.ClientID), "resource", sanitizeLogValue(p.Resource))
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

	out, status, err := g.entra.RedeemCode(r.Context(), entraCode, p.EntraVerifier)
	at, _ := out["access_token"].(string)
	idt, _ := out["id_token"].(string)
	logAccessTokenShape(at)
	missingToken := (at == "" && idt == "") || (at == "" && g.cfg.TeamTokenStrategy != StrategyRefresh)
	if err != nil || status != http.StatusOK || missingToken {
		errType, _ := out["error"].(string)
		// IdP fault: Entra unreachable (no status) or 5xx. A 4xx body that fails to parse,
		// or a client that went away (context.Canceled), is not Entra's fault.
		if (err != nil && status == 0 && !errors.Is(err, context.Canceled)) || status >= 500 {
			metrics.IDPErrors.WithLabelValues("code_redeem").Inc()
		}
		slog.Warn("audit login", "outcome", "code_redeem_failed", "status", status, "error", sanitizeLogValue(errType), "client_id", sanitizeLogValue(p.ClientID), "resource", sanitizeLogValue(p.Resource))
		back(url.Values{
			"error":             {"server_error"},
			"error_description": {safeEntraFailureMessage(out, status)},
		})
		return
	}

	// introspection mode always requires an id_token (#2594); legacy only when a nonce was sent.
	introspect := g.cfg.IdentitySource == IdentitySourceIntrospection
	if idt == "" && (introspect || p.EntraNonce != "") {
		slog.Warn("audit login", "outcome", "missing_id_token", "client_id", p.ClientID, "resource", p.Resource)
		back(url.Values{
			"error":             {"server_error"},
			"error_description": {"upstream identity token missing"},
		})
		return
	}
	var idClaims map[string]any
	var parseErr error
	if idt != "" {
		idClaims, parseErr = entra.ParseJWTClaims(idt)
		if parseErr == nil {
			parseErr = entra.ValidateIDTokenClaims(idClaims, g.cfg.AzureIssuer, g.cfg.AzureClientID, g.now())
		}
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

	navIdent := ""
	var introErr error
	if introspect {
		// The token response is hostile: NAVident comes only from Texas
		// introspection of the access token, never from an unverified decode (#2594).
		navIdent, introErr = g.introspectNAVident(r.Context(), at)
	} else {
		// legacy: unchanged main behaviour; removed in the follow-up to #2594.
		extractIdentityFromClaims(idClaims, &navIdent)
		if at != "" {
			if atClaims, atErr := entra.ParseJWTClaims(at); atErr == nil {
				extractIdentityFromClaims(atClaims, &navIdent)
			}
		}
	}
	if introErr != nil {
		metrics.IDPErrors.WithLabelValues("introspection").Inc()
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
	if g.cfg.RTStorage == RTStorageClientEncrypted {
		// Never persist the Entra refresh token in the session store in client-encrypted
		// mode; carry it transiently on the 60s single-use code until /token seals it.
		sessRT = ""
		entraRT = rt
	}
	sess := oauth.Session{
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

	issued := &oauth.IssuedCode{
		ClientID:    p.ClientID,
		RedirectURI: p.RedirectURI,
		Challenge:   p.Challenge,
		Resource:    p.Resource,
		Session:     sess,
		EntraRT:     entraRT,
		Created:     issuedAt,
	}
	remembered, err := g.HasConsent(r.Context(), navIdent, p.ClientID, p.Resource)
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
	if err := g.PutPending(r.Context(), session.KindPendingConsent, consentID, p); err != nil {
		if errors.Is(err, session.ErrStoreFull) {
			httpx.MarkShed(w, "store_full")
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
func (g *Server) issueCode(w http.ResponseWriter, r *http.Request, p *oauth.PendingAuth, gwCode string, issued *oauth.IssuedCode) {
	if err := g.PutCode(r.Context(), gwCode, issued); err != nil {
		if errors.Is(err, session.ErrStoreFull) {
			httpx.MarkShed(w, "store_full")
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

func (g *Server) redirectToClient(w http.ResponseWriter, r *http.Request, p *oauth.PendingAuth, v url.Values) {
	target, err := url.Parse(p.RedirectURI)
	if err != nil {
		http.Error(w, "invalid stored redirect_uri", http.StatusInternalServerError)
		return
	}
	v.Set("state", p.State)
	v.Set("iss", g.cfg.BaseURL)
	redirectWithValues(w, r, target, v)
}

// SignEntraState signs state for the Entra state cookie.
func (g *Server) SignEntraState(state string) string {
	return entra.SignState(g.cfg.CookieSecret, state)
}

// VerifyEntraStateCookie checks a state cookie against the query state.
func (g *Server) VerifyEntraStateCookie(cookieVal, queryState string) bool {
	return entra.VerifyState(append([][]byte{g.cfg.CookieSecret}, g.cfg.CookieVerifyKeys...), cookieVal, queryState)
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

// redirectWithValues redirects to base with v added to its query.
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
		maxAge = proxy.DefaultSessionMaxAge
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
	if len(claims) > MaxClaimsBytes {
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

func extractIdentityFromClaims(claims map[string]any, navIdent *string) {
	if claims == nil {
		return
	}
	if *navIdent == "" {
		if v, ok := claims["NAVident"].(string); ok {
			*navIdent = strings.TrimSpace(v)
		}
	}
}

// introspectNAVident validates the Entra access token at Texas and returns
// its NAVident. Any failure, an inactive token or a missing NAVident returns
// "" or an error; the caller rejects the login.
func (g *Server) introspectNAVident(ctx context.Context, accessToken string) (string, error) {
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, httpx.MaxUpstreamBodyBytes)).Decode(&out); err != nil {
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
