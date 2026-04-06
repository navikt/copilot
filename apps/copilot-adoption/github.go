package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

type GitHubClient struct {
	httpClient  *http.Client
	org         string
	batchSize   int
	concurrency int
}

func NewGitHubClient(cfg *Config) (*GitHubClient, error) {
	transport, err := ghinstallation.New(
		http.DefaultTransport,
		cfg.GitHubAppID,
		cfg.GitHubAppInstallationID,
		[]byte(cfg.GitHubAppPrivateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	return &GitHubClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		org:         cfg.OrganizationSlug,
		batchSize:   cfg.GraphQLBatchSize,
		concurrency: cfg.ScanConcurrency,
	}, nil
}

// --- REST: List Repositories ---

type restRepo struct {
	Name          string   `json:"name"`
	DefaultBranch string   `json:"default_branch"`
	Language      string   `json:"language"`
	Archived      bool     `json:"archived"`
	Fork          bool     `json:"fork"`
	Visibility    string   `json:"visibility"`
	CreatedAt     string   `json:"created_at"`
	PushedAt      string   `json:"pushed_at"`
	Topics        []string `json:"topics"`
}

func (c *GitHubClient) ListRepos(ctx context.Context, org string) ([]RepoInfo, error) {
	var allRepos []RepoInfo
	page := 1

	for {
		url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100&page=%d&type=all", org, page)
		repos, err := c.fetchRepoPage(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("failed to list repos page %d: %w", page, err)
		}
		if len(repos) == 0 {
			break
		}

		for _, r := range repos {
			info := RepoInfo{
				Name:            r.Name,
				DefaultBranch:   r.DefaultBranch,
				PrimaryLanguage: r.Language,
				IsArchived:      r.Archived,
				IsFork:          r.Fork,
				Visibility:      r.Visibility,
				Topics:          r.Topics,
			}
			if t, err := time.Parse(time.RFC3339, r.CreatedAt); err == nil {
				info.CreatedAt = t
			}
			if t, err := time.Parse(time.RFC3339, r.PushedAt); err == nil {
				info.PushedAt = t
			}
			allRepos = append(allRepos, info)
		}

		if len(repos) < 100 {
			break
		}
		page++
	}

	slog.Info("Listed repositories", "org", org, "total", len(allRepos))
	return allRepos, nil
}

func (c *GitHubClient) fetchRepoPage(ctx context.Context, url string) ([]restRepo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var repos []restRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("failed to decode repos: %w", err)
	}
	return repos, nil
}

// --- REST: Team Mapping (org-side traversal) ---

type restTeam struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type restTeamRepo struct {
	Name        string          `json:"name"`
	Permissions map[string]bool `json:"permissions"`
}

func (c *GitHubClient) BuildTeamMap(ctx context.Context, org string) (map[string][]TeamAccess, error) {
	teams, err := c.listTeams(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	slog.Info("Listed teams", "org", org, "count", len(teams))

	teamMap := make(map[string][]TeamAccess)
	for _, team := range teams {
		repos, err := c.listTeamRepos(ctx, org, team.Slug)
		if err != nil {
			slog.Warn("Failed to list repos for team, skipping", "team", team.Slug, "error", err)
			continue
		}

		for _, repo := range repos {
			perm := highestPermission(repo.Permissions)
			teamMap[repo.Name] = append(teamMap[repo.Name], TeamAccess{
				Slug:       team.Slug,
				Name:       team.Name,
				Permission: perm,
			})
		}
	}

	slog.Info("Built team map", "repos_with_teams", len(teamMap))
	return teamMap, nil
}

func highestPermission(perms map[string]bool) string {
	for _, p := range []string{"admin", "maintain", "push", "triage", "pull"} {
		if perms[p] {
			return p
		}
	}
	return "none"
}

func (c *GitHubClient) listTeams(ctx context.Context, org string) ([]restTeam, error) {
	var all []restTeam
	page := 1
	for {
		url := fmt.Sprintf("https://api.github.com/orgs/%s/teams?per_page=100&page=%d", org, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.doWithRetry(ctx, req)
		if err != nil {
			return nil, err
		}

		var teams []restTeam
		if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		_ = resp.Body.Close()

		all = append(all, teams...)
		if len(teams) < 100 {
			break
		}
		page++
	}
	return all, nil
}

func (c *GitHubClient) listTeamRepos(ctx context.Context, org, teamSlug string) ([]restTeamRepo, error) {
	var all []restTeamRepo
	page := 1
	for {
		url := fmt.Sprintf("https://api.github.com/orgs/%s/teams/%s/repos?per_page=100&page=%d", org, teamSlug, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.doWithRetry(ctx, req)
		if err != nil {
			return nil, err
		}

		var repos []restTeamRepo
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		_ = resp.Body.Close()

		all = append(all, repos...)
		if len(repos) < 100 {
			break
		}
		page++
	}
	return all, nil
}

// --- GraphQL: Customization File Scanning ---

// graphqlRequest is the payload sent to GitHub's GraphQL API.
type graphqlRequest struct {
	Query string `json:"query"`
}

// ScanRepos checks multiple repositories for customization files using batched GraphQL queries.
// Uses concurrency workers to parallelize batch execution.
// Returns ScanOutput with customizations per repo and last commit dates.
func (c *GitHubClient) ScanRepos(ctx context.Context, org string, repos []RepoInfo, criteria []SearchCriteria) (*ScanOutput, error) {
	type batchJob struct {
		index int
		repos []RepoInfo
	}

	// Build batch jobs
	var jobs []batchJob
	for i := 0; i < len(repos); i += c.batchSize {
		end := i + c.batchSize
		if end > len(repos) {
			end = len(repos)
		}
		jobs = append(jobs, batchJob{index: i, repos: repos[i:end]})
	}

	output := &ScanOutput{
		Customizations: make(map[string]map[string]SearchResult),
		LastCommits:    make(map[string]*time.Time),
	}
	var mu sync.Mutex

	workers := c.concurrency
	if workers < 1 {
		workers = 1
	}

	jobCh := make(chan batchJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var wg sync.WaitGroup
	scanned := 0

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				batchCustomizations, batchCommits, err := c.scanBatch(ctx, org, job.repos, criteria)

				mu.Lock()
				if err != nil {
					slog.Warn("GraphQL batch failed, recording nil results to track failures",
						"batch_start", job.index,
						"batch_size", len(job.repos),
						"error", err,
					)
					// Use nil to indicate failure (vs emptyResults for "no customizations")
					for _, repo := range job.repos {
						output.Customizations[repo.Name] = nil
					}
				} else {
					for name, res := range batchCustomizations {
						output.Customizations[name] = res
					}
					for name, t := range batchCommits {
						output.LastCommits[name] = t
					}
				}
				scanned += len(job.repos)
				if scanned%300 < c.batchSize {
					slog.Info("Scan progress", "repos_scanned", scanned, "total", len(repos))
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return output, nil
}

func emptyResults(criteria []SearchCriteria) map[string]SearchResult {
	m := make(map[string]SearchResult, len(criteria))
	for _, c := range criteria {
		m[c.Category] = SearchResult{Exists: false}
	}
	return m
}

// scanBatch executes a single batched GraphQL query for multiple repos.
func (c *GitHubClient) scanBatch(ctx context.Context, org string, repos []RepoInfo, criteria []SearchCriteria) (map[string]map[string]SearchResult, map[string]*time.Time, error) {
	query := buildGraphQLQuery(org, repos, criteria)

	body, err := json.Marshal(graphqlRequest{Query: query})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.github.com/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GraphQL request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("GraphQL request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var gqlResp graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, nil, fmt.Errorf("failed to decode GraphQL response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		slog.Debug("GraphQL response had errors", "errors", gqlResp.Errors)
	}

	customizations, lastCommits := parseGraphQLResponse(gqlResp.Data, repos, criteria)
	return customizations, lastCommits, nil
}

// buildGraphQLQuery constructs a batched query checking all criteria across multiple repos.
func buildGraphQLQuery(org string, repos []RepoInfo, criteria []SearchCriteria) string {
	var b strings.Builder
	b.WriteString("query {\n")

	for i, repo := range repos {
		ref := repo.DefaultBranch
		if ref == "" {
			ref = "HEAD"
		}

		fmt.Fprintf(&b, "  repo%d: repository(owner: %q, name: %q) {\n", i, org, repo.Name)
		b.WriteString("    defaultBranchRef { target { ... on Commit { committedDate } } }\n")

		for _, c := range criteria {
			alias := c.GraphQLAlias()
			expression := fmt.Sprintf("%s:%s", ref, c.TreePath)

			switch c.CheckType {
			case CheckFile:
				fmt.Fprintf(&b, "    %s: object(expression: %q) { __typename ... on Blob { oid } }\n", alias, expression)
			case CheckDirectory:
				fmt.Fprintf(&b, "    %s: object(expression: %q) {\n", alias, expression)
				b.WriteString("      __typename\n")
				b.WriteString("      ... on Tree { entries { name type object { oid } } }\n")
				b.WriteString("    }\n")
			}
		}

		b.WriteString("  }\n")
	}

	b.WriteString("}\n")
	return b.String()
}

// --- GraphQL response parsing ---

type graphqlResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []graphqlError             `json:"errors"`
}

type graphqlError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type objectResponse struct {
	TypeName string      `json:"__typename"`
	OID      string      `json:"oid"`
	Entries  []treeEntry `json:"entries"`
}

type treeEntry struct {
	Name   string       `json:"name"`
	Type   string       `json:"type"` // "blob" or "tree"
	Object *entryObject `json:"object"`
}

type entryObject struct {
	OID string `json:"oid"`
}

type defaultBranchRefResponse struct {
	Target struct {
		CommittedDate string `json:"committedDate"`
	} `json:"target"`
}

func parseGraphQLResponse(data map[string]json.RawMessage, repos []RepoInfo, criteria []SearchCriteria) (map[string]map[string]SearchResult, map[string]*time.Time) {
	results := make(map[string]map[string]SearchResult, len(repos))
	lastCommits := make(map[string]*time.Time, len(repos))

	for i, repo := range repos {
		key := fmt.Sprintf("repo%d", i)
		repoData, ok := data[key]
		if !ok || string(repoData) == "null" {
			results[repo.Name] = emptyResults(criteria)
			continue
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(repoData, &fields); err != nil {
			slog.Warn("Failed to parse repo data", "repo", repo.Name, "error", err)
			results[repo.Name] = emptyResults(criteria)
			continue
		}

		repoResults := make(map[string]SearchResult, len(criteria))

		// Extract last commit date from defaultBranchRef
		if branchData, ok := fields["defaultBranchRef"]; ok && string(branchData) != "null" {
			var branchRef defaultBranchRefResponse
			if err := json.Unmarshal(branchData, &branchRef); err == nil && branchRef.Target.CommittedDate != "" {
				if t, err := time.Parse(time.RFC3339, branchRef.Target.CommittedDate); err == nil {
					lastCommits[repo.Name] = &t
				}
			}
		}

		for _, c := range criteria {
			alias := c.GraphQLAlias()
			fieldData, ok := fields[alias]
			if !ok || string(fieldData) == "null" {
				repoResults[c.Category] = SearchResult{Exists: false}
				continue
			}

			var obj objectResponse
			if err := json.Unmarshal(fieldData, &obj); err != nil {
				repoResults[c.Category] = SearchResult{Exists: false}
				continue
			}

			switch c.CheckType {
			case CheckFile:
				repoResults[c.Category] = SearchResult{Exists: obj.TypeName == "Blob", Oids: blobOids(obj)}
			case CheckDirectory:
				if obj.TypeName != "Tree" || len(obj.Entries) == 0 {
					repoResults[c.Category] = SearchResult{Exists: false}
					continue
				}
				var names []string
				for _, e := range obj.Entries {
					names = append(names, e.Name)
				}
				matched := c.MatchFiles(names)
				repoResults[c.Category] = SearchResult{
					Exists: len(matched) > 0,
					Files:  matched,
					Oids:   matchedOids(obj.Entries, matched),
				}
			}
		}
		results[repo.Name] = repoResults
	}

	return results, lastCommits
}

// blobOids returns the OID for a file-level check (single blob).
func blobOids(obj objectResponse) []string {
	if obj.OID != "" {
		return []string{obj.OID}
	}
	return nil
}

// matchedOids returns blob OIDs in the same order as matched file names.
func matchedOids(entries []treeEntry, matched []string) []string {
	entryMap := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.Object != nil && e.Object.OID != "" {
			entryMap[e.Name] = e.Object.OID
		}
	}

	oids := make([]string, len(matched))
	for i, name := range matched {
		oids[i] = entryMap[name]
	}
	return oids
}

// --- HTTP retry with rate limit handling ---

const maxRetries = 3

// doWithRetry executes an HTTP request with retry logic for rate limits and server errors.
// Returns the response on success (caller must close the body).
// Does NOT retry 4xx client errors (except 429 rate limit).
func (c *GitHubClient) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Save the body for retries (requests with body like GraphQL POST)
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		_ = req.Body.Close()
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * time.Second // 2s, 4s
			slog.Debug("Retrying request", "attempt", attempt+1, "backoff", backoff, "url", req.URL.String())
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Reset body for retry
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Log rate limit state
		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
			if rem, err := strconv.Atoi(remaining); err == nil && rem < 100 {
				slog.Warn("GitHub API rate limit low",
					"remaining", rem,
					"limit", resp.Header.Get("X-RateLimit-Limit"),
					"reset", resp.Header.Get("X-RateLimit-Reset"),
				)
			}
		}

		// Success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		// Rate limited — honor Retry-After header or X-RateLimit-Reset
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
			// Check Retry-After header first
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					wait := time.Duration(seconds) * time.Second
					slog.Warn("Rate limited, waiting", "retry_after", wait, "status", resp.StatusCode)
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(wait):
					}
					lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
					continue
				}
			}

			// Check X-RateLimit headers (GitHub's primary rate limiting)
			remaining := resp.Header.Get("X-RateLimit-Remaining")
			resetHeader := resp.Header.Get("X-RateLimit-Reset")
			if remaining == "0" && resetHeader != "" {
				if resetUnix, err := strconv.ParseInt(resetHeader, 10, 64); err == nil {
					resetTime := time.Unix(resetUnix, 0)
					wait := time.Until(resetTime) + 5*time.Second // Add buffer
					if wait > 0 && wait < 15*time.Minute {        // Sanity check
						slog.Warn("Rate limited, waiting until reset", "reset_at", resetTime, "wait", wait)
						select {
						case <-ctx.Done():
							return nil, ctx.Err()
						case <-time.After(wait):
						}
						lastErr = fmt.Errorf("status %d: rate limit exceeded", resp.StatusCode)
						continue
					}
				}
			}

			// 403 without rate limit indicators is a permission error — don't retry
			if resp.StatusCode == http.StatusForbidden {
				return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			}
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			continue
		}

		// Server error — retry
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			continue
		}

		// Other client errors (400, 401, 404, etc.) — don't retry
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
}

// sourceRepo is the canonical repo where customization files are maintained.
const sourceRepo = "copilot"

// ResolveSourceOIDs fetches blob OIDs for all customization files in the source repo.
// Returns a map of category → (filename → OID). For file-type checks, the filename
// is the basename of the TreePath.
func (c *GitHubClient) ResolveSourceOIDs(ctx context.Context, criteria []SearchCriteria) (SourceOIDs, error) {
	repo := RepoInfo{Name: sourceRepo, DefaultBranch: "main"}
	batchCustomizations, _, err := c.scanBatch(ctx, c.org, []RepoInfo{repo}, criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to scan source repo %s/%s: %w", c.org, sourceRepo, err)
	}

	repoResults, ok := batchCustomizations[sourceRepo]
	if !ok {
		return nil, fmt.Errorf("source repo %s not found in scan results", sourceRepo)
	}

	source := make(SourceOIDs, len(criteria))
	for _, c := range criteria {
		sr, ok := repoResults[c.Category]
		if !ok || !sr.Exists {
			continue
		}

		oidMap := make(map[string]string)
		switch c.CheckType {
		case CheckFile:
			// File checks: use TreePath basename as key
			if len(sr.Oids) > 0 {
				base := c.TreePath
				if idx := strings.LastIndex(c.TreePath, "/"); idx >= 0 {
					base = c.TreePath[idx+1:]
				}
				oidMap[base] = sr.Oids[0]
			}
		case CheckDirectory:
			// Directory checks: Files and Oids are parallel arrays
			for i, name := range sr.Files {
				if i < len(sr.Oids) && sr.Oids[i] != "" {
					oidMap[name] = sr.Oids[i]
				}
			}
		}

		if len(oidMap) > 0 {
			source[c.Category] = oidMap
		}
	}

	slog.Info("Resolved source OIDs", "categories", len(source))
	return source, nil
}
