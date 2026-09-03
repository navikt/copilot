package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OAuthServer struct {
	BaseURL             string
	GitHubClient        *GitHubClient
	Store               *TokenStore
	AllowedOrganization string
	clientIDKey         []byte
}

// ClientRegistration is the RFC 7591 response to a Dynamic Client Registration
// request. Nothing here is stored: ClientID carries the registration itself
// (see clientid.go). No client_secret is issued, so client_secret and
// client_secret_expires_at are both absent, per RFC 7591 section 3.2.1.
type ClientRegistration struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
}

type AuthorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

type ProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
}

func NewOAuthServer(baseURL string, githubClient *GitHubClient, store *TokenStore, allowedOrganization string) *OAuthServer {
	return &OAuthServer{
		BaseURL:             baseURL,
		GitHubClient:        githubClient,
		Store:               store,
		AllowedOrganization: allowedOrganization,
		clientIDKey:         deriveClientIDKey(githubClient.ClientSecret),
	}
}

func (s *OAuthServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.handleAuthServerMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.handleProtectedResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", s.handleProtectedResourceMetadata)
	mux.HandleFunc("GET /oauth/authorize", s.handleAuthorize)
	mux.HandleFunc("GET /oauth/callback", s.handleCallback)
	mux.HandleFunc("POST /oauth/token", s.handleToken)
	mux.HandleFunc("OPTIONS /oauth/token", s.handleTokenOptions)
	mux.HandleFunc("POST /register", s.handleRegister)
	mux.HandleFunc("OPTIONS /register", s.handleRegisterOptions)
}

func (s *OAuthServer) handleAuthServerMetadata(w http.ResponseWriter, _ *http.Request) {
	slog.Debug("serving authorization server metadata", "base_url", s.BaseURL)
	metadata := AuthorizationServerMetadata{
		Issuer:                            s.BaseURL,
		AuthorizationEndpoint:             s.BaseURL + "/oauth/authorize",
		TokenEndpoint:                     s.BaseURL + "/oauth/token",
		RegistrationEndpoint:              s.BaseURL + "/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
	}

	s.setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metadata)
}

func (s *OAuthServer) handleProtectedResourceMetadata(w http.ResponseWriter, _ *http.Request) {
	metadata := ProtectedResourceMetadata{
		Resource:             s.BaseURL + "/mcp",
		AuthorizationServers: []string{s.BaseURL},
	}

	s.setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metadata)
}

func (s *OAuthServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	clientState := r.URL.Query().Get("state")
	redirectURI := r.URL.Query().Get("redirect_uri")
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")

	slog.Debug("authorize request received",
		"client_id", logSafe(clientID),
		"redirect_uri", logSafe(redirectURI),
		"has_state", clientState != "",
		"has_pkce", codeChallenge != "",
		"code_challenge_method", logSafe(codeChallengeMethod),
		"user_agent", logSafe(r.UserAgent()),
	)

	if clientID == "" {
		http.Error(w, "Missing required parameter: client_id", http.StatusBadRequest)
		return
	}

	reg, err := verifyClientID(s.clientIDKey, clientID)
	if err != nil {
		// The client_id is not one this server signed: forged, tampered with,
		// or issued under a signing key that has since rotated. Either way its
		// redirect_uri cannot be trusted (GHSA-7hwf-488h-59x8); tell the client
		// to re-register.
		slog.Warn("unknown client_id", "client_id", logSafe(clientID))
		recordOAuthFlow("authorize", "invalid_client")
		writeUnknownClient(w, r)
		return
	}

	if !isRegisteredRedirectURI(reg.RedirectURIs, redirectURI) { // also rejects ""
		slog.Warn("redirect_uri not registered",
			"client_id", logSafe(clientID),
			"redirect_uri", logSafe(redirectURI),
		)
		http.Error(w, "redirect_uri does not match registered URIs", http.StatusBadRequest)
		return
	}

	if codeChallengeMethod != "" && codeChallengeMethod != "S256" {
		http.Error(w, "Only S256 code challenge method supported", http.StatusBadRequest)
		return
	}

	internalState := generateSecureToken(32)

	session := &AuthSession{
		ClientID:            clientID,
		ClientState:         clientState,
		RedirectURI:         redirectURI,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		CreatedAt:           time.Now(),
	}
	s.Store.SaveAuthSession(internalState, session)

	slog.Info("starting oauth flow",
		"client_id", logSafe(clientID),
		"redirect_uri", logSafe(redirectURI),
		"has_pkce", codeChallenge != "",
	)
	recordOAuthFlow("authorize", "started")

	githubURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=%s",
		s.GitHubClient.ClientID,
		url.QueryEscape(s.BaseURL+"/oauth/callback"),
		internalState,
		url.QueryEscape("read:user read:org user:email"),
	)

	http.Redirect(w, r, githubURL, http.StatusFound)
}

// unknownClientPage is what a person sees when their browser lands on the
// invalid_client error from /oauth/authorize.
//
// A signed client_id survives restarts, so this is now the rare case: a forged
// or corrupted client_id, or one issued before the signing key rotated. The
// editor extension sits blocked on its loopback callback without ever reading
// the response body, so the browser tab is the only place a human is told what
// happened and it has to say how to recover.
const unknownClientPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Unknown client - sign in again</title></head>
<body>
<h1>This server does not recognise your editor's registration</h1>
<p>Your editor is sending a <code>client_id</code> that this server did not issue, or issued
before its signing key was rotated, so the server cannot accept it
(<code>invalid_client</code>).</p>
<h2>How to recover</h2>
<p>Remove the cached MCP registration and tokens for this server in your editor, then sign in
again. The editor registers itself anew and the sign-in goes through.</p>
<ul>
<li>VS Code: open the Accounts menu, sign out of this MCP server, and reconnect it.</li>
<li>Other clients: delete the stored OAuth registration for this server and reconnect.</li>
</ul>
<p>Nothing is wrong with your account, and you do not need to report this.</p>
</body>
</html>
`

// writeUnknownClient answers an unregistered client_id. Machine clients that
// parse the body keep the RFC 6749 JSON error; a browser gets a page it can act
// on, because it is the browser that the person is looking at.
func writeUnknownClient(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(unknownClientPage))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             "invalid_client",
		"error_description": "Unknown client_id. Clear the cached MCP registration and tokens for this server, then sign in again to re-register.",
	})
}

func (s *OAuthServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		slog.Error("github oauth error", "error", errorParam, "description", errorDesc)
		recordOAuthFlow("callback", "github_error")
		http.Error(w, fmt.Sprintf("GitHub OAuth error: %s - %s", errorParam, errorDesc), http.StatusBadRequest)
		return
	}

	session, err := s.Store.GetAuthSession(state)
	if err != nil {
		slog.Error("invalid state", "error", err)
		http.Error(w, "Invalid or expired state", http.StatusBadRequest)
		return
	}
	s.Store.DeleteAuthSession(state)

	githubToken, err := s.GitHubClient.ExchangeCode(code)
	if err != nil {
		slog.Error("failed to exchange code", "error", err)
		http.Error(w, "Failed to exchange code with GitHub", http.StatusInternalServerError)
		return
	}

	user, err := s.GitHubClient.GetUser(githubToken.AccessToken)
	if err != nil {
		slog.Error("failed to get user", "error", err)
		http.Error(w, "Failed to get GitHub user", http.StatusInternalServerError)
		return
	}

	// Check organization membership
	if s.AllowedOrganization != "" {
		isMember, matchedOrg := s.GitHubClient.CheckOrgMembership(githubToken.AccessToken, []string{s.AllowedOrganization})
		if !isMember {
			slog.Warn("user not member of allowed organization",
				"user", user.Login,
				"allowed_org", s.AllowedOrganization,
			)
			recordOAuthFlow("callback", "org_denied")
			http.Error(w, fmt.Sprintf("Access denied: You must be a member of the %s organization", s.AllowedOrganization), http.StatusForbidden)
			return
		}
		slog.Info("user authorized",
			"login", user.Login,
			"id", user.ID,
			"org", matchedOrg,
		)
	} else {
		slog.Info("user authenticated", "login", user.Login, "id", user.ID)
	}

	mcpCode := generateSecureToken(32)
	s.Store.SaveAuthCode(mcpCode, &AuthCode{
		ClientID:           session.ClientID,
		GitHubAccessToken:  githubToken.AccessToken,
		GitHubRefreshToken: githubToken.RefreshToken,
		GitHubExpiresAt:    githubToken.ExpiresAt,
		CodeChallenge:      session.CodeChallenge,
		RedirectURI:        session.RedirectURI,
		UserLogin:          user.Login,
		UserID:             user.ID,
		CreatedAt:          time.Now(),
	})

	callbackURL := fmt.Sprintf("%s?code=%s&state=%s",
		session.RedirectURI,
		url.QueryEscape(mcpCode),
		url.QueryEscape(session.ClientState),
	)

	recordOAuthFlow("callback", "success")
	recordAuthentication()
	http.Redirect(w, r, callbackURL, http.StatusFound)
}

// handleTokenOptions answers the CORS preflight without granting any origin.
//
// /oauth/token deliberately sends no Access-Control-Allow-Origin. Every
// supported client (VS Code and the other IDE extensions) redeems the code from
// a native process rather than a web page, so none of them needs the header. A
// wildcard grant let any page read a token response in the victim's browser
// (GHSA-7hwf-488h-59x8).
func (s *OAuthServer) handleTokenOptions(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *OAuthServer) handleToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// RFC 6749 §5.1: a token response carries credentials and must not be
	// stored by any intermediary or by the browser's back/forward cache.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	if err := r.ParseForm(); err != nil {
		s.writeTokenError(w, "invalid_request", "Failed to parse form")
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
	case "refresh_token":
		s.handleRefreshTokenGrant(w, r)
	default:
		s.writeTokenError(w, "unsupported_grant_type", "Grant type not supported")
	}
}

// handleAuthorizationCodeGrant processes the authorization_code grant type.
// Body size is already limited by handleToken via http.MaxBytesReader.
func (s *OAuthServer) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")                  //nolint:gosec // body limited in handleToken
	codeVerifier := r.FormValue("code_verifier") //nolint:gosec // body limited in handleToken
	redirectURI := r.FormValue("redirect_uri")   //nolint:gosec // body limited in handleToken
	clientID := r.FormValue("client_id")         //nolint:gosec // body limited in handleToken

	authCode, err := s.Store.GetAuthCode(code)
	if err != nil {
		slog.Error("invalid auth code", "error", err)
		s.writeTokenError(w, "invalid_grant", "Invalid or expired authorization code")
		return
	}
	s.Store.DeleteAuthCode(code)

	slog.Debug("token exchange attempt",
		"client_id", clientID,
		"redirect_uri", redirectURI,
		"stored_redirect_uri", authCode.RedirectURI,
		"has_code_verifier", codeVerifier != "",
		"has_stored_code_challenge", authCode.CodeChallenge != "",
		"user", authCode.UserLogin,
	)

	if time.Since(authCode.CreatedAt) > 10*time.Minute {
		slog.Debug("auth code expired", "age", time.Since(authCode.CreatedAt))
		s.writeTokenError(w, "invalid_grant", "Authorization code expired")
		return
	}

	if clientID == "" || clientID != authCode.ClientID {
		slog.Warn("client_id missing or mismatched in token exchange",
			"expected", authCode.ClientID,
			"got", clientID,
		)
		s.writeTokenError(w, "invalid_client", "client_id missing or does not match the authorization code")
		return
	}

	if authCode.RedirectURI != redirectURI {
		slog.Warn("redirect_uri mismatch in token exchange",
			"stored", authCode.RedirectURI,
			"received", redirectURI,
		)
		s.writeTokenError(w, "invalid_grant", "Redirect URI mismatch")
		return
	}

	if authCode.CodeChallenge != "" {
		if !VerifyPKCE(codeVerifier, authCode.CodeChallenge) {
			slog.Warn("PKCE verification failed", "user", authCode.UserLogin)
			s.writeTokenError(w, "invalid_grant", "PKCE verification failed")
			return
		}
	}

	accessToken := generateSecureToken(64)
	refreshToken := generateSecureToken(64)
	expiresIn := 3600

	s.Store.SaveToken(accessToken, &TokenData{
		GitHubAccessToken:  authCode.GitHubAccessToken,
		GitHubRefreshToken: authCode.GitHubRefreshToken,
		GitHubExpiresAt:    authCode.GitHubExpiresAt,
		UserLogin:          authCode.UserLogin,
		UserID:             authCode.UserID,
		ExpiresAt:          time.Now().Add(time.Duration(expiresIn) * time.Second),
	})

	s.Store.SaveRefreshToken(refreshToken, &RefreshTokenData{
		ClientID:           authCode.ClientID,
		GitHubRefreshToken: authCode.GitHubRefreshToken,
		UserLogin:          authCode.UserLogin,
		UserID:             authCode.UserID,
		CreatedAt:          time.Now(),
	})

	slog.Info("token issued", "user", authCode.UserLogin, "expires_in", expiresIn)
	recordOAuthFlow("token_exchange", "success")

	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    expiresIn,
		"refresh_token": refreshToken,
	}
	_ = json.NewEncoder(w).Encode(response)
}

// handleRefreshTokenGrant processes the refresh_token grant type.
// Body size is already limited by handleToken via http.MaxBytesReader.
func (s *OAuthServer) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token") //nolint:gosec // body limited in handleToken
	clientID := r.FormValue("client_id")         //nolint:gosec // body limited in handleToken

	rtData, err := s.Store.GetRefreshToken(refreshToken)
	if err != nil {
		s.writeTokenError(w, "invalid_grant", "Invalid refresh token")
		return
	}

	// RFC 6749 §6 requires the authorization server to ensure a refresh token
	// was issued to the client redeeming it. This is a public client
	// (token_endpoint_auth_method "none"), so client_id is that check, the same
	// one the authorization_code grant already makes. Without it a stolen
	// refresh token mints access tokens for anyone holding it, for the full
	// 30-day lifetime (GHSA-7hwf-488h-59x8).
	//
	// An empty rtData.ClientID means a token issued before the binding existed.
	// It is rejected rather than grandfathered: the store is in-memory, so a
	// restart drops every refresh token anyway, and trusting an unbound token
	// is the hole itself.
	if clientID == "" || clientID != rtData.ClientID {
		slog.Warn("client_id missing or mismatched in refresh grant",
			"user", rtData.UserLogin,
			"bound", rtData.ClientID != "",
		)
		recordOAuthFlow("token_refresh", "invalid_client")
		s.writeTokenError(w, "invalid_client", "client_id missing or does not match the refresh token")
		return
	}

	newGitHubToken, err := s.GitHubClient.RefreshToken(rtData.GitHubRefreshToken)
	if err != nil {
		slog.Error("failed to refresh github token", "error", err, "user", rtData.UserLogin)
		s.writeTokenError(w, "invalid_grant", "Failed to refresh GitHub token")
		return
	}

	accessToken := generateSecureToken(64)
	newRefreshToken := generateSecureToken(64)
	expiresIn := 3600

	s.Store.SaveToken(accessToken, &TokenData{
		GitHubAccessToken:  newGitHubToken.AccessToken,
		GitHubRefreshToken: newGitHubToken.RefreshToken,
		GitHubExpiresAt:    newGitHubToken.ExpiresAt,
		UserLogin:          rtData.UserLogin,
		UserID:             rtData.UserID,
		ExpiresAt:          time.Now().Add(time.Duration(expiresIn) * time.Second),
	})

	s.Store.DeleteRefreshToken(refreshToken)
	s.Store.SaveRefreshToken(newRefreshToken, &RefreshTokenData{
		ClientID:           rtData.ClientID,
		GitHubRefreshToken: newGitHubToken.RefreshToken,
		UserLogin:          rtData.UserLogin,
		UserID:             rtData.UserID,
		CreatedAt:          time.Now(),
	})

	slog.Info("token refreshed", "user", rtData.UserLogin)
	recordOAuthFlow("token_refresh", "success")

	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    expiresIn,
		"refresh_token": newRefreshToken,
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *OAuthServer) writeTokenError(w http.ResponseWriter, code, description string) {
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             code,
		"error_description": description,
	})
}

func (s *OAuthServer) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
}

func (s *OAuthServer) handleRegisterOptions(w http.ResponseWriter, _ *http.Request) {
	s.setCORSHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *OAuthServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	s.setCORSHeaders(w)

	// Registrations are not stored, so there is nothing to exhaust and no
	// registration cap. The body is still bounded, and so is the number of
	// redirect_uris below — but neither bounds the length of a single
	// redirect_uri, so the minted client_id is checked against maxClientIDLen
	// before it is handed out.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	var req struct {
		ClientName              string   `json:"client_name"`
		RedirectURIs            []string `json:"redirect_uris"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_client_metadata",
			"error_description": "Failed to parse request body",
		})
		return
	}

	if len(req.RedirectURIs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_client_metadata",
			"error_description": "redirect_uris is required and must not be empty",
		})
		return
	}

	if len(req.RedirectURIs) > 10 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_client_metadata",
			"error_description": "At most 10 redirect_uris are supported",
		})
		return
	}

	for _, uri := range req.RedirectURIs {
		if !isValidRedirectURI(uri) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":             "invalid_redirect_uri",
				"error_description": "redirect_uri must use http://127.0.0.1 or https scheme: " + uri,
			})
			return
		}
	}

	if len(req.GrantTypes) == 0 {
		req.GrantTypes = []string{"authorization_code"}
	}
	for _, gt := range req.GrantTypes {
		if gt != "authorization_code" && gt != "refresh_token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":             "invalid_client_metadata",
				"error_description": "Unsupported grant_type: " + gt,
			})
			return
		}
	}

	if len(req.ResponseTypes) == 0 {
		req.ResponseTypes = []string{"code"}
	}

	if req.TokenEndpointAuthMethod == "" {
		req.TokenEndpointAuthMethod = "none"
	}
	if req.TokenEndpointAuthMethod != "none" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_client_metadata",
			"error_description": "Only token_endpoint_auth_method 'none' is supported (public clients)",
		})
		return
	}

	issuedAt := time.Now().Unix()
	clientID := mintClientID(s.clientIDKey, clientIDInfo{
		RedirectURIs: req.RedirectURIs,
		IssuedAt:     issuedAt,
		Nonce:        generateSecureToken(8),
	})

	// The caps above bound the request, not the length of one redirect_uri, so
	// a single long URI can still produce a client_id that verifyClientID would
	// refuse to decode. Reject the registration rather than issue one that can
	// never authorise.
	if len(clientID) > maxClientIDLen {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_client_metadata",
			"error_description": "redirect_uris are too long to fit in a client_id",
		})
		return
	}

	reg := &ClientRegistration{
		ClientID:                clientID,
		ClientIDIssuedAt:        issuedAt,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              req.GrantTypes,
		ResponseTypes:           req.ResponseTypes,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
	}

	slog.Info("client registered",
		"client_id", logSafe(clientID),
		"client_name", logSafe(req.ClientName),
		// Sanitised for consistency and for whoever loosens the validation
		// above. Not reachable today: every one of these is validated before
		// this line, so a control character cannot survive to be logged.
		"redirect_uris", logSafeAll(req.RedirectURIs),
		"grant_types", logSafeAll(req.GrantTypes),
		"token_endpoint_auth_method", logSafe(req.TokenEndpointAuthMethod),
	)
	recordOAuthFlow("client_registration", "success")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(reg)
}

func isValidRedirectURI(uri string) bool {
	parsed, err := url.Parse(uri)
	if err != nil {
		return false
	}
	if parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1" {
		return true
	}
	if parsed.Scheme == "http" && parsed.Hostname() == "localhost" {
		return true
	}
	if parsed.Scheme == "https" {
		return true
	}
	return false
}

func isRegisteredRedirectURI(registered []string, uri string) bool {
	for _, r := range registered {
		if r == uri {
			return true
		}
		if matchesLoopbackIgnoringPort(r, uri) {
			return true
		}
	}
	return false
}

// matchesLoopbackIgnoringPort implements RFC 8252 Section 7.3:
// for loopback IP redirect URIs, the port must be excluded from the comparison.
func matchesLoopbackIgnoringPort(registered, requested string) bool {
	reg, err := url.Parse(registered)
	if err != nil {
		return false
	}
	req, err := url.Parse(requested)
	if err != nil {
		return false
	}
	if reg.Scheme != "http" || req.Scheme != "http" {
		return false
	}
	regHost := reg.Hostname()
	reqHost := req.Hostname()
	if !isLoopback(regHost) || !isLoopback(reqHost) {
		return false
	}
	return regHost == reqHost && reg.Path == req.Path
}

func isLoopback(host string) bool {
	return host == "127.0.0.1" || host == "localhost"
}

func generateSecureToken(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
