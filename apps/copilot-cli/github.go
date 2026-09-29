package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const githubAPIBaseURL = "https://api.github.com"

// GitHubClient validates GitHub user tokens. Org membership is checked by
// copilot-api (see upstream.isOrgMember), so the tokens need no permissions.
//
// A token is accepted only when GitHub confirms, through this app's own
// credentials, that it was issued to this app (POST
// /applications/{client_id}/token). That is the audience check a GitHub
// token otherwise lacks: a token another app obtained for the same user (the
// gh CLI's, an IDE's) is refused, so no other app's token can be replayed
// here.
type GitHubClient struct {
	httpClient   *http.Client
	baseURL      string
	clientID     string
	clientSecret string
}

func newGitHubClient(clientID, clientSecret string) *GitHubClient {
	return &GitHubClient{
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		baseURL:      githubAPIBaseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

func (c *GitHubClient) configured() bool { return c.clientID != "" && c.clientSecret != "" }

var (
	errInvalidToken  = errors.New("invalid or expired token")
	errIssuerOffline = errors.New("this sign-in method is not configured")
)

// resolveUser asks GitHub whether token was issued to this app, and to whom.
// 404 and 422 mean the token is not valid for this app: revoked, expired, or
// another app's.
func (c *GitHubClient) resolveUser(ctx context.Context, token string) (*AuthenticatedUser, error) {
	if !c.configured() {
		return nil, errIssuerOffline
	}
	resp, err := c.appTokenRequest(ctx, http.MethodPost, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return nil, errInvalidToken
	default:
		// 401 here means this app's credentials are wrong, not the
		// caller's token: a server fault.
		return nil, fmt.Errorf("GitHub token check returned status %d", resp.StatusCode)
	}

	var check struct {
		ExpiresAt *time.Time `json:"expires_at"`
		App       struct {
			ClientID string `json:"client_id"`
		} `json:"app"`
		User struct {
			Login string `json:"login"`
			ID    int64  `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&check); err != nil {
		return nil, fmt.Errorf("decoding GitHub token check: %w", err)
	}
	if check.App.ClientID != c.clientID {
		return nil, errInvalidToken
	}
	if check.ExpiresAt != nil && time.Now().After(*check.ExpiresAt) {
		return nil, errInvalidToken
	}
	if check.User.Login == "" || check.User.ID == 0 {
		return nil, errors.New("GitHub token check response missing user")
	}
	user := &AuthenticatedUser{
		Issuer:  issuerGitHub,
		Subject: strconv.FormatInt(check.User.ID, 10),
		Login:   check.User.Login,
	}
	if check.ExpiresAt != nil {
		user.expiresAt = *check.ExpiresAt
	}
	return user, nil
}

// revoke revokes token, which must have been issued to this app: GitHub
// answers 404 or 422 for any other token, so nothing but our own tokens can
// be revoked through this.
func (c *GitHubClient) revoke(ctx context.Context, token string) error {
	if !c.configured() {
		return errIssuerOffline
	}
	resp, err := c.appTokenRequest(ctx, http.MethodDelete, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound, http.StatusUnprocessableEntity:
		return errInvalidToken
	default:
		return fmt.Errorf("GitHub token revoke returned status %d", resp.StatusCode)
	}
}

// appTokenRequest calls /applications/{client_id}/token with this app's
// credentials, about token.
func (c *GitHubClient) appTokenRequest(ctx context.Context, method, token string) (*http.Response, error) {
	body, err := json.Marshal(map[string]string{"access_token": token})
	if err != nil {
		return nil, fmt.Errorf("encoding token request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method,
		c.baseURL+"/applications/"+url.PathEscape(c.clientID)+"/token", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building token request: %w", err)
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling GitHub %s token: %w", method, err)
	}
	return resp, nil
}
