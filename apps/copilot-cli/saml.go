package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// samlLookup finds the Nav e-mail behind a navikt GitHub account: the nameId
// of the member's SAML SSO identity in the org, read with the nav-pilot
// GitHub App's installation token. It is the input to the survey's
// participation hash and nothing else: no cache, no log.
//
// The App needs the organization permission that lets it read SAML
// identities (the same one copilot-api's App uses for the reverse lookup) and
// must be installed on navikt.
type samlLookup struct {
	httpClient     *http.Client
	baseURL        string
	org            string
	appID          string
	installationID string
	key            *rsa.PrivateKey

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func newSAMLLookup(org, appID, installationID, privateKeyPEM string) (*samlLookup, error) {
	if appID == "" || installationID == "" || privateKeyPEM == "" {
		return nil, nil
	}
	block, _ := pem.Decode([]byte(strings.ReplaceAll(privateKeyPEM, `\n`, "\n")))
	if block == nil {
		return nil, errors.New("GITHUB_APP_PRIVATE_KEY is not PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		k, err8 := x509.ParsePKCS8PrivateKey(block.Bytes)
		var ok bool
		if key, ok = k.(*rsa.PrivateKey); err8 != nil || !ok {
			return nil, errors.New("GITHUB_APP_PRIVATE_KEY is not an RSA key")
		}
	}
	return &samlLookup{
		httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: githubAPIBaseURL,
		org: org, appID: appID, installationID: installationID, key: key,
	}, nil
}

func (s *samlLookup) installationToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Before(s.tokenExp) {
		return s.token, nil
	}
	now := time.Now()
	appJWT, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
		Issuer:    s.appID,
	}).SignedString(s.key)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/app/installations/"+s.installationID+"/access_tokens", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("installation token: status %d", resp.StatusCode)
	}
	var body struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil || body.Token == "" {
		return "", errors.New("installation token: bad response")
	}
	s.token, s.tokenExp = body.Token, body.ExpiresAt.Add(-5*time.Minute)
	return s.token, nil
}

// email returns the SAML nameId of login's identity in the org, or
// errNoNavIdentity when the account has none.
func (s *samlLookup) email(ctx context.Context, login string) (string, error) {
	token, err := s.installationToken(ctx)
	if err != nil {
		return "", err
	}
	query := `query($org: String!, $login: String!) {
		organization(login: $org) { samlIdentityProvider { externalIdentities(first: 1, login: $login) {
			nodes { samlIdentity { nameId } user { login } } } } } }`
	payload, _ := json.Marshal(map[string]any{"query": query, "variables": map[string]string{"org": s.org, "login": login}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/graphql", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("saml lookup: status %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			Organization struct {
				SAMLIdentityProvider *struct {
					ExternalIdentities struct {
						Nodes []struct {
							SAMLIdentity *struct {
								NameID string `json:"nameId"`
							} `json:"samlIdentity"`
							User *struct {
								Login string `json:"login"`
							} `json:"user"`
						} `json:"nodes"`
					} `json:"externalIdentities"`
				} `json:"samlIdentityProvider"`
			} `json:"organization"`
		} `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", errors.New("saml lookup: bad response")
	}
	if len(out.Errors) > 0 || out.Data.Organization.SAMLIdentityProvider == nil {
		// The App cannot see SAML identities: a server fault, not the user's.
		return "", errors.New("saml lookup: no access to the org's SAML identities")
	}
	for _, n := range out.Data.Organization.SAMLIdentityProvider.ExternalIdentities.Nodes {
		if n.User != nil && strings.EqualFold(n.User.Login, login) && n.SAMLIdentity != nil && n.SAMLIdentity.NameID != "" {
			return n.SAMLIdentity.NameID, nil
		}
	}
	return "", errNoNavIdentity
}

// emailFor is surveyAPI.emailFor: the Entra token's e-mail, or the SAML
// lookup for a GitHub sign-in. A nil lookup turns the GitHub path off.
func emailFor(lookup *samlLookup) func(context.Context, *AuthenticatedUser) (string, error) {
	return func(ctx context.Context, u *AuthenticatedUser) (string, error) {
		switch {
		case u.Issuer == issuerEntra && u.email != "":
			return u.email, nil
		case u.Issuer == issuerGitHub && lookup != nil:
			return lookup.email(ctx, u.Login)
		}
		return "", errors.New("no e-mail for this sign-in")
	}
}
