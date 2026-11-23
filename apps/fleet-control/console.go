package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

// consoleTeamsURL is the Nais Console GraphQL API (beta), see
// https://doc.nais.io/operate/console/api.
const consoleTeamsURL = "https://console.nav.cloud.nais.io/graphql"

// consoleTeamsTTL bounds how long a membership answer is reused. One minute
// keeps a removed member's access short while a burst of UI and write calls
// costs one lookup. Errors are never cached.
const consoleTeamsTTL = time.Minute

const userTeamsQuery = `query($email: String!, $after: Cursor) {
  user(email: $email) {
    teams(first: 100, after: $after) {
      nodes { team { slug } }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

// consoleTeams looks up team membership in the Nais Console API with the
// workload's service account token. Every failure is an error, so callers
// deny (503); a user with no teams is a member of none (403).
type consoleTeams struct {
	url       string
	tokenPath string // re-read before each call: the file rotates in place
	client    *http.Client
	now       func() time.Time

	mu    sync.Mutex
	cache map[string]cachedTeams
}

type cachedTeams struct {
	teams []string
	at    time.Time
}

func newConsoleTeams(url, tokenPath string) *consoleTeams {
	return &consoleTeams{url: url, tokenPath: tokenPath, client: &http.Client{Timeout: 5 * time.Second}, now: time.Now, cache: map[string]cachedTeams{}}
}

func (c *consoleTeams) IsMember(ctx context.Context, team, email string) (bool, error) {
	if normEmail(email) == "" {
		return false, nil
	}
	teams, err := c.Teams(ctx, email)
	if err != nil {
		return false, err
	}
	return slices.Contains(teams, team), nil
}

func (c *consoleTeams) Teams(ctx context.Context, email string) ([]string, error) {
	email = normEmail(email)
	if email == "" {
		return []string{}, nil
	}
	c.mu.Lock()
	hit, ok := c.cache[email]
	c.mu.Unlock()
	if ok && c.now().Sub(hit.at) < consoleTeamsTTL {
		return slices.Clone(hit.teams), nil
	}
	teams := []string{}
	var after *string
	for range 20 { // 2000 teams is far past any real person
		page, err := c.page(ctx, email, after)
		if err != nil {
			return nil, err
		}
		for _, n := range page.Nodes {
			if !dnsLabelRegex.MatchString(n.Team.Slug) {
				return nil, fmt.Errorf("nais console: invalid team slug in answer")
			}
			teams = append(teams, n.Team.Slug)
		}
		if !page.PageInfo.HasNextPage {
			slices.Sort(teams)
			c.mu.Lock()
			c.cache[email] = cachedTeams{teams, c.now()}
			c.mu.Unlock()
			return slices.Clone(teams), nil
		}
		if page.PageInfo.EndCursor == nil || *page.PageInfo.EndCursor == "" {
			return nil, errors.New("nais console: next page without cursor")
		}
		after = page.PageInfo.EndCursor
	}
	return nil, errors.New("nais console: too many team pages")
}

type teamsPage struct {
	Nodes []struct {
		Team struct {
			Slug string `json:"slug"`
		} `json:"team"`
	} `json:"nodes"`
	PageInfo struct {
		HasNextPage bool    `json:"hasNextPage"`
		EndCursor   *string `json:"endCursor"`
	} `json:"pageInfo"`
}

func (c *consoleTeams) page(ctx context.Context, email string, after *string) (*teamsPage, error) {
	raw, err := os.ReadFile(c.tokenPath)
	if err != nil {
		return nil, fmt.Errorf("nais console: read service account token: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return nil, errors.New("nais console: empty service account token")
	}
	body, _ := json.Marshal(map[string]any{"query": userTeamsQuery, "variables": map[string]any{"email": email, "after": after}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req) //nolint:gosec // fixed Console URL; tests pass httptest URLs
	if err != nil {
		return nil, fmt.Errorf("nais console: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nais console: status %d", resp.StatusCode)
	}
	var out struct {
		Data *struct {
			User *struct {
				Teams *teamsPage `json:"teams"`
			} `json:"user"`
		} `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("nais console: decode: %w", err)
	}
	// Do not log the error bodies: they may echo the email.
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("nais console: %d graphql errors", len(out.Errors))
	}
	if out.Data == nil || out.Data.User == nil || out.Data.User.Teams == nil {
		return nil, errors.New("nais console: empty answer")
	}
	return out.Data.User.Teams, nil
}
