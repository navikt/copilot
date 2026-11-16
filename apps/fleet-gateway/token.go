package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"time"
)

type gatewaySession struct {
	clientID     string
	resource     string
	entraToken   string
	refreshToken string
	expiry       time.Time
}

type issuedCode struct {
	ClientID    string
	RedirectURI string
	Challenge   string
	Resource    string
	Session     *gatewaySession
	Created     time.Time
}

func (g *Gateway) tokenHandler(w http.ResponseWriter, r *http.Request) {
	bad := func(status int, errCode, desc string) {
		writeJSON(w, status, map[string]string{
			"error":             errCode,
			"error_description": desc,
		})
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
	if err := r.ParseForm(); err != nil {
		bad(http.StatusBadRequest, "invalid_request", "invalid form body")
		return
	}

	if r.PostFormValue("grant_type") != "authorization_code" {
		bad(http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
		return
	}

	code := r.PostFormValue("code")
	if code == "" {
		bad(http.StatusBadRequest, "invalid_request", "code is required")
		return
	}

	// Single-use: Pop deletes the code even if subsequent binding checks fail.
	issued, ok := g.codes.Pop(code)
	if !ok {
		bad(http.StatusBadRequest, "invalid_grant", "unknown, used, or expired code")
		return
	}

	if r.PostFormValue("client_id") != issued.ClientID {
		bad(http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}
	if r.PostFormValue("redirect_uri") != issued.RedirectURI {
		bad(http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}
	if reqResource := r.PostFormValue("resource"); reqResource != "" && reqResource != issued.Resource {
		bad(http.StatusBadRequest, "invalid_target", "resource mismatch")
		return
	}

	verifier := r.PostFormValue("code_verifier")
	if !isValidCodeVerifier(verifier) ||
		subtle.ConstantTimeCompare([]byte(pkceS256(verifier)), []byte(issued.Challenge)) != 1 {
		bad(http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	ttl := issued.Session.expiry.Sub(g.now())
	if ttl <= 0 {
		bad(http.StatusBadRequest, "invalid_grant", "session expired")
		return
	}

	accessToken, err := randomToken()
	if err != nil {
		bad(http.StatusInternalServerError, "server_error", "failed to issue token")
		return
	}
	if !g.sessions.Set(accessToken, issued.Session, ttl) {
		bad(http.StatusServiceUnavailable, "temporarily_unavailable", "session store capacity reached")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(ttl.Seconds()),
	})
}

func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func isValidCodeVerifier(verifier string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	for i := 0; i < len(verifier); i++ {
		c := verifier[i]
		isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := c >= '0' && c <= '9'
		isUnreserved := c == '-' || c == '.' || c == '_' || c == '~'
		if !isAlpha && !isDigit && !isUnreserved {
			return false
		}
	}
	return true
}
