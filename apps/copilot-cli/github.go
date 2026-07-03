package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const githubAPIBaseURL = "https://api.github.com"

// GitHubClient validates GitHub tokens and checks org membership on behalf
// of the requesting developer. copilot-cli never stores or sees a GitHub
// App token here — it simply forwards the user's own token to GitHub.
type GitHubClient struct {
	httpClient *http.Client
	baseURL    string
}

func newGitHubClient() *GitHubClient {
	return &GitHubClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    githubAPIBaseURL,
	}
}

type githubUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	ID    int64  `json:"id"`
}

// AuthenticatedUser identifies the developer behind a request, resolved via
// their own GitHub token.
type AuthenticatedUser struct {
	Login string
	Name  string
}

// resolveUser validates the given GitHub token by calling GET /user, which
// implicitly proves the token is live and belongs to the caller.
func (c *GitHubClient) resolveUser(ctx context.Context, token string) (*AuthenticatedUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/user", nil)
	if err != nil {
		return nil, fmt.Errorf("building /user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling GitHub /user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errInvalidToken
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub /user returned status %d", resp.StatusCode)
	}

	var user githubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding GitHub /user response: %w", err)
	}
	if user.Login == "" {
		return nil, fmt.Errorf("GitHub /user response missing login")
	}

	return &AuthenticatedUser{Login: user.Login, Name: user.Name}, nil
}

// isOrgMember checks whether the given user is a member of org, using the
// caller's own token. GitHub returns 204 for members and 404 otherwise.
func (c *GitHubClient) isOrgMember(ctx context.Context, token, org, username string) (bool, error) {
	url := fmt.Sprintf("%s/orgs/%s/members/%s", c.baseURL, org, username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("building org membership request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("calling GitHub org membership: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	case http.StatusUnauthorized:
		return false, errInvalidToken
	default:
		return false, fmt.Errorf("GitHub org membership check returned status %d", resp.StatusCode)
	}
}

var errInvalidToken = fmt.Errorf("invalid or expired GitHub token")
