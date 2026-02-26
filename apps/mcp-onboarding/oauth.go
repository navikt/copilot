package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

type OAuthServer struct {
	BaseURL             string
	GitHubClient        *GitHubClient
	Store               *TokenStore
	AllowedOrganization string
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

	if clientID == "" {
		http.Error(w, "Missing required parameter: client_id", http.StatusBadRequest)
		return
	}

	reg, _ := s.Store.GetClientRegistration(clientID)
	if reg != nil && redirectURI != "" && !isRegisteredRedirectURI(reg.RedirectURIs, redirectURI) {
		slog.Warn("redirect_uri not registered",
			"client_id", clientID,
			"redirect_uri", redirectURI,
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
		"client_id", clientID,
		"redirect_uri", redirectURI,
		"has_pkce", codeChallenge != "",
	)

	githubURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=%s",
		s.GitHubClient.ClientID,
		url.QueryEscape(s.BaseURL+"/oauth/callback"),
		internalState,
		url.QueryEscape("read:user read:org user:email"),
	)

	http.Redirect(w, r, githubURL, http.StatusFound)
}

func (s *OAuthServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		slog.Error("github oauth error", "error", errorParam, "description", errorDesc)
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
		mcpCode,
		session.ClientState,
	)

	http.Redirect(w, r, callbackURL, http.StatusFound)
}

func (s *OAuthServer) handleTokenOptions(w http.ResponseWriter, _ *http.Request) {
	s.setCORSHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *OAuthServer) handleToken(w http.ResponseWriter, r *http.Request) {
	s.setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")

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

func (s *OAuthServer) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	codeVerifier := r.FormValue("code_verifier")
	redirectURI := r.FormValue("redirect_uri")
	clientID := r.FormValue("client_id")

	authCode, err := s.Store.GetAuthCode(code)
	if err != nil {
		slog.Error("invalid auth code", "error", err)
		s.writeTokenError(w, "invalid_grant", "Invalid or expired authorization code")
		return
	}
	s.Store.DeleteAuthCode(code)

	if time.Since(authCode.CreatedAt) > 10*time.Minute {
		s.writeTokenError(w, "invalid_grant", "Authorization code expired")
		return
	}

	if clientID != "" && authCode.ClientID != "" && authCode.ClientID != clientID {
		slog.Warn("client_id mismatch in token exchange",
			"expected", authCode.ClientID,
			"got", clientID,
		)
		s.writeTokenError(w, "invalid_client", "client_id mismatch")
		return
	}

	if authCode.RedirectURI != redirectURI {
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
		GitHubRefreshToken: authCode.GitHubRefreshToken,
		UserLogin:          authCode.UserLogin,
		UserID:             authCode.UserID,
		CreatedAt:          time.Now(),
	})

	slog.Info("token issued", "user", authCode.UserLogin, "expires_in", expiresIn)

	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    expiresIn,
		"refresh_token": refreshToken,
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *OAuthServer) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")

	rtData, err := s.Store.GetRefreshToken(refreshToken)
	if err != nil {
		s.writeTokenError(w, "invalid_grant", "Invalid refresh token")
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
		GitHubRefreshToken: newGitHubToken.RefreshToken,
		UserLogin:          rtData.UserLogin,
		UserID:             rtData.UserID,
		CreatedAt:          time.Now(),
	})

	slog.Info("token refreshed", "user", rtData.UserLogin)

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

	if s.Store.CountClientRegistrations() > 1000 {
		slog.Warn("too many client registrations")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "too_many_requests",
			"error_description": "Too many client registrations",
		})
		return
	}

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

	clientID := generateSecureToken(16)

	reg := &ClientRegistration{
		ClientID:                clientID,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              req.GrantTypes,
		ResponseTypes:           req.ResponseTypes,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
		CreatedAt:               time.Now(),
	}
	s.Store.SaveClientRegistration(reg)

	slog.Info("client registered",
		"client_id", clientID,
		"client_name", req.ClientName,
		"redirect_uris", req.RedirectURIs,
	)

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
	}
	return false
}

func generateSecureToken(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
