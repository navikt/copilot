// Package main implements an MCP server with GitHub OAuth authentication.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GitHubClient struct {
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
	APIBaseURL   string
}

type GitHubToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	TokenType    string
	Scope        string
}

type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type GitHubOrg struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type GitHubRepo struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Archived bool   `json:"archived"`
	Fork     bool   `json:"fork"`
	Private  bool   `json:"private"`
}

func NewGitHubClient(clientID, clientSecret string) *GitHubClient {
	return &GitHubClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		APIBaseURL:   "https://api.github.com",
	}
}

func (c *GitHubClient) ExchangeCode(code string) (*GitHubToken, error) {
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

	return &GitHubToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
	}, nil
}

func (c *GitHubClient) RefreshToken(refreshToken string) (*GitHubToken, error) {
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

	return &GitHubToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		TokenType:    tokenResp.TokenType,
		Scope:        tokenResp.Scope,
	}, nil
}

func (c *GitHubClient) GetUser(accessToken string) (*GitHubUser, error) {
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

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (c *GitHubClient) GetUserOrganizations(accessToken string) ([]GitHubOrg, error) {
	req, err := http.NewRequest("GET", c.APIBaseURL+"/user/orgs", nil)
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

	var orgs []GitHubOrg
	if err := json.NewDecoder(resp.Body).Decode(&orgs); err != nil {
		return nil, err
	}

	return orgs, nil
}

func (c *GitHubClient) CheckOrgMembership(accessToken string, allowedOrgs []string) (bool, string) {
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

type RepoContent struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

func (c *GitHubClient) GetRepoFile(accessToken, owner, repo, path string) (bool, error) {
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
	_, _ = io.ReadAll(resp.Body)

	return resp.StatusCode == http.StatusOK, nil
}

func (c *GitHubClient) GetDirectoryCount(accessToken, owner, repo, path string) (int, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		c.APIBaseURL, url.PathEscape(owner), url.PathEscape(repo), path)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return 0, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
	}

	var contents []RepoContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return 0, nil
	}

	return len(contents), nil
}

func (c *GitHubClient) GetRepoLanguages(accessToken, owner, repo string) ([]string, error) {
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

// GetRepoFileContent returns the decoded content of a file from a GitHub repository.
// Returns empty string and nil error if the file does not exist.
func (c *GitHubClient) GetRepoFileContent(accessToken, owner, repo, path string) (string, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		c.APIBaseURL, url.PathEscape(owner), url.PathEscape(repo), path)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.raw+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api error: %d - %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

// ListTeamRepos returns non-archived, non-fork repositories for a GitHub team.
// Uses the team slug (URL-friendly name). Results are paginated.
func (c *GitHubClient) ListTeamRepos(accessToken, org, teamSlug string) ([]GitHubRepo, error) {
	var allRepos []GitHubRepo
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

		var repos []GitHubRepo
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
	Items []GitHubRepo `json:"items"`
}

// SearchReposByPrefix finds repos in an org matching a name prefix.
// Uses the GitHub search API. Results are paginated, limited to 200 repos.
func (c *GitHubClient) SearchReposByPrefix(accessToken, org, prefix string) ([]GitHubRepo, error) {
	var allRepos []GitHubRepo
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
