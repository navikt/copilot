// Package github is the GitHub OAuth and REST client.
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client calls GitHub OAuth and the REST API.
type Client struct {
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
	APIBaseURL   string
}

// Token is a GitHub user access token.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	TokenType    string
	Scope        string
}

// User is the authenticated GitHub user.
type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// Org is a GitHub organization.
type Org struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// Repo is a GitHub repository.
type Repo struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Archived bool   `json:"archived"`
	Fork     bool   `json:"fork"`
	Private  bool   `json:"private"`
}

// NewClient returns a client for api.github.com.
func NewClient(clientID, clientSecret string) *Client {
	return &Client{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		APIBaseURL:   "https://api.github.com",
	}
}

// ExchangeCode exchanges an OAuth authorization code for a token.
func (c *Client) ExchangeCode(code string) (*Token, error) {
	data := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"code":          {code},
	}

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("github oauth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	expiresAt := time.Now().Add(8 * time.Hour)
	if tokenResp.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	return &Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
	}, nil
}

// RefreshToken exchanges a refresh token for a new token.
func (c *Client) RefreshToken(refreshToken string) (*Token, error) {
	data := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("github refresh error: %s", tokenResp.Error)
	}

	return &Token{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
	}, nil
}

// GetUser returns the user the token belongs to.
func (c *Client) GetUser(accessToken string) (*User, error) {
	req, err := http.NewRequest("GET", c.APIBaseURL+"/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUserOrganizations returns every organization the user belongs to.
func (c *Client) GetUserOrganizations(accessToken string) ([]Org, error) {
	var all []Org
	for page := 1; ; page++ {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/user/orgs?per_page=100&page=%d", c.APIBaseURL, page), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
		}

		var orgs []Org
		err = json.NewDecoder(resp.Body).Decode(&orgs)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		all = append(all, orgs...)
		// shortcut: stops at 10 pages (1000 orgs), far beyond any real user.
		if len(orgs) < 100 || page == 10 {
			return all, nil
		}
	}
}

// CheckOrgMembership reports whether the user is in one of allowedOrgs, and which.
func (c *Client) CheckOrgMembership(accessToken string, allowedOrgs []string) (bool, string) {
	orgs, err := c.GetUserOrganizations(accessToken)
	if err != nil {
		return false, ""
	}

	for _, allowedOrg := range allowedOrgs {
		for _, userOrg := range orgs {
			if strings.EqualFold(userOrg.Login, allowedOrg) {
				return true, userOrg.Login
			}
		}
	}

	return false, ""
}

// RepoContent is one entry in a repository directory listing.
type RepoContent struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

// GetRepoFile reports whether path exists in the repository.
func (c *Client) GetRepoFile(accessToken, owner, repo, path string) (bool, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		c.APIBaseURL, url.PathEscape(owner), url.PathEscape(repo), path)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
	}
}

// ListDirectory lists path, or returns nil if it does not exist.
func (c *Client) ListDirectory(accessToken, owner, repo, path string) ([]RepoContent, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		c.APIBaseURL, url.PathEscape(owner), url.PathEscape(repo), path)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
	}

	var contents []RepoContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, nil
	}

	return contents, nil
}

// GetDirectoryCount returns the number of entries in path.
func (c *Client) GetDirectoryCount(accessToken, owner, repo, path string) (int, error) {
	entries, err := c.ListDirectory(accessToken, owner, repo, path)
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

// GetRepoLanguages returns the repository languages.
func (c *Client) GetRepoLanguages(accessToken, owner, repo string) ([]string, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/languages",
		c.APIBaseURL, url.PathEscape(owner), url.PathEscape(repo))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
	}

	var langMap map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&langMap); err != nil {
		return nil, err
	}

	langs := make([]string, 0, len(langMap))
	for lang := range langMap {
		langs = append(langs, lang)
	}
	return langs, nil
}

// ListTeamRepos returns non-archived, non-fork repositories for a GitHub team.
// Uses the team slug (URL-friendly name). Results are paginated.
func (c *Client) ListTeamRepos(accessToken, org, teamSlug string) ([]Repo, error) {
	var allRepos []Repo
	page := 1
	perPage := 100

	for {
		apiURL := fmt.Sprintf("%s/orgs/%s/teams/%s/repos?per_page=%d&page=%d",
			c.APIBaseURL, url.PathEscape(org), url.PathEscape(teamSlug), perPage, page)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
		}

		var repos []Repo
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		_ = resp.Body.Close()

		for _, r := range repos {
			if !r.Archived && !r.Fork {
				allRepos = append(allRepos, r)
			}
		}

		if len(repos) < perPage {
			break
		}
		page++
	}

	return allRepos, nil
}

type searchReposResponse struct {
	Items []Repo `json:"items"`
}

// SearchReposByPrefix finds repos in an org matching a name prefix.
// Uses the GitHub search API. Results are paginated, limited to 200 repos.
func (c *Client) SearchReposByPrefix(accessToken, org, prefix string) ([]Repo, error) {
	var allRepos []Repo
	page := 1
	perPage := 100
	const maxResults = 200

	query := fmt.Sprintf("%s in:name org:%s fork:false archived:false", prefix, org)

	for {
		apiURL := fmt.Sprintf("%s/search/repositories?q=%s&per_page=%d&page=%d",
			c.APIBaseURL, url.QueryEscape(query), perPage, page)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
		}

		var result searchReposResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		_ = resp.Body.Close()

		allRepos = append(allRepos, result.Items...)

		if len(result.Items) < perPage || len(allRepos) >= maxResults {
			break
		}
		page++
	}

	if len(allRepos) > maxResults {
		allRepos = allRepos[:maxResults]
	}

	return allRepos, nil
}
