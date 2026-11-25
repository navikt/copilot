package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ConsoleURL is the Nais Console GraphQL API, the membership source
// fleet-control uses (apps/fleet-control/console.go).
const ConsoleURL = "https://console.nav.cloud.nais.io/graphql"

const userTeamsQuery = `query($email: String!, $after: Cursor) {
  user(email: $email) {
    teams(first: 100, after: $after) {
      nodes { team { slug } }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

// ConsoleTeams looks up membership with the workload's Nais service account
// token. Every failure is an error, so the probe answers 503, never allow.
// shortcut: no cache, unlike fleet-control; add one if probe volume grows.
type ConsoleTeams struct {
	URL       string
	TokenPath string // re-read on each call: the file rotates in place
	Client    *http.Client
}

// NewConsoleTeams returns a ConsoleTeams for the Nais Console API.
func NewConsoleTeams(tokenPath string) *ConsoleTeams {
	return &ConsoleTeams{URL: ConsoleURL, TokenPath: tokenPath, Client: &http.Client{Timeout: 5 * time.Second}}
}

// normEmail lowercases an ASCII email; non-ASCII gives "" (deny), as in fleet-control.
func normEmail(s string) string {
	s = strings.TrimSpace(s)
	for i := range len(s) {
		if s[i] >= 0x80 {
			return ""
		}
	}
	return strings.ToLower(s)
}

// IsMember reports whether email belongs to team.
func (c *ConsoleTeams) IsMember(ctx context.Context, team, email string) (bool, error) {
	email = normEmail(email)
	if email == "" {
		return false, nil
	}
	var after *string
	for range 20 { // 2000 teams is far past any real person
		page, err := c.page(ctx, email, after)
		if err != nil {
			return false, err
		}
		for _, n := range page.Nodes {
			if n.Team.Slug == team {
				return true, nil
			}
		}
		if !page.PageInfo.HasNextPage {
			return false, nil
		}
		if page.PageInfo.EndCursor == nil || *page.PageInfo.EndCursor == "" {
			return false, errors.New("nais console: next page without cursor")
		}
		after = page.PageInfo.EndCursor
	}
	return false, errors.New("nais console: too many team pages")
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

func (c *ConsoleTeams) page(ctx context.Context, email string, after *string) (*teamsPage, error) {
	raw, err := os.ReadFile(c.TokenPath) //nolint:gosec // operator config, the Nais service account mount
	if err != nil {
		return nil, fmt.Errorf("nais console: read service account token: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return nil, errors.New("nais console: empty service account token")
	}
	body, _ := json.Marshal(map[string]any{"query": userTeamsQuery, "variables": map[string]any{"email": email, "after": after}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req) //nolint:gosec // fixed Console URL; tests pass httptest URLs
	if err != nil {
		return nil, fmt.Errorf("nais console: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nais console: status %d", resp.StatusCode)
	}
	var out struct {
		Data *struct {
			User *struct {
				Teams *teamsPage `json:"teams"`
			} `json:"user"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
			Path    []any  `json:"path"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("nais console: decode: %w", err)
	}
	// A person not in Nais is in no team; only on the first page, as in fleet-control.
	if after == nil && len(out.Errors) == 1 && out.Errors[0].Message == "The specified user was not found." &&
		len(out.Errors[0].Path) == 1 && out.Errors[0].Path[0] == "user" {
		return &teamsPage{}, nil
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("nais console: %d graphql errors", len(out.Errors))
	}
	if out.Data == nil || out.Data.User == nil || out.Data.User.Teams == nil {
		return nil, errors.New("nais console: empty answer")
	}
	return out.Data.User.Teams, nil
}
