package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// githubUserInfo is the subset of GitHub's GET /user response nav-pilot needs
// to confirm a stored token is still valid and show the developer who they're
// logged in as.
type githubUserInfo struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// fetchGitHubUser validates a token by calling GET api.github.com/user.
// Returns an error if the token is invalid/expired or the request fails.
func fetchGitHubUser(ctx context.Context, token string) (*githubUserInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("building /user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling GitHub /user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("token is invalid or expired")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub /user returned status %d", resp.StatusCode)
	}

	var user githubUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding GitHub /user response: %w", err)
	}
	return &user, nil
}

// checkOrgMembership reports whether the token's owner is a member of org.
func checkOrgMembership(ctx context.Context, token, org, username string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/orgs/%s/members/%s", org, username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("building org membership request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("calling GitHub org membership: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("org membership check returned status %d", resp.StatusCode)
	}
}
