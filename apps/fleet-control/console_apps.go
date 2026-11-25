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
)

// appLister lists a team's Nais applications. Only consoleTeams has one: the
// dev fallback (staticTeams) answers 503, and /fleet falls back to a text field.
type appLister interface {
	Apps(ctx context.Context, team string) ([]TeamApp, error)
}

// TeamApp is one Nais application in one environment.
type TeamApp struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

const teamAppsQuery = `query($slug: Slug!, $after: Cursor) {
  team(slug: $slug) {
    applications(first: 100, after: $after) {
      nodes { name teamEnvironment { environment { name } } }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

type appsPage struct {
	Nodes []struct {
		Name            string `json:"name"`
		TeamEnvironment struct {
			Environment struct {
				Name string `json:"name"`
			} `json:"environment"`
		} `json:"teamEnvironment"`
	} `json:"nodes"`
	PageInfo struct {
		HasNextPage bool    `json:"hasNextPage"`
		EndCursor   *string `json:"endCursor"`
	} `json:"pageInfo"`
}

// Apps lists the team's applications, sorted by name then environment. Not
// cached: it is read once per wizard step. Every failure is an error.
func (c *consoleTeams) Apps(ctx context.Context, team string) ([]TeamApp, error) {
	if !dnsLabelRegex.MatchString(team) {
		return nil, errors.New("nais console: invalid team")
	}
	apps := []TeamApp{}
	var after *string
	for range 10 { // 1000 apps is far past any real team
		var out struct {
			Data *struct {
				Team *struct {
					Applications *appsPage `json:"applications"`
				} `json:"team"`
			} `json:"data"`
			Errors []json.RawMessage `json:"errors"`
		}
		if err := c.post(ctx, teamAppsQuery, map[string]any{"slug": team, "after": after}, &out); err != nil {
			return nil, err
		}
		if len(out.Errors) > 0 {
			return nil, fmt.Errorf("nais console: %d graphql errors", len(out.Errors))
		}
		if out.Data == nil || out.Data.Team == nil || out.Data.Team.Applications == nil {
			return nil, errors.New("nais console: empty answer")
		}
		page := out.Data.Team.Applications
		for _, n := range page.Nodes {
			env := n.TeamEnvironment.Environment.Name
			if !dnsLabelRegex.MatchString(n.Name) || !dnsLabelRegex.MatchString(env) {
				return nil, errors.New("nais console: invalid application in answer")
			}
			apps = append(apps, TeamApp{Name: n.Name, Environment: env})
		}
		if !page.PageInfo.HasNextPage {
			slices.SortFunc(apps, func(a, b TeamApp) int {
				return strings.Compare(a.Name+"\x00"+a.Environment, b.Name+"\x00"+b.Environment)
			})
			return apps, nil
		}
		if page.PageInfo.EndCursor == nil || *page.PageInfo.EndCursor == "" {
			return nil, errors.New("nais console: next page without cursor")
		}
		after = page.PageInfo.EndCursor
	}
	return nil, errors.New("nais console: too many application pages")
}

// post sends one GraphQL query with the service account token and decodes
// the answer (at most 1 MiB) into out.
func (c *consoleTeams) post(ctx context.Context, query string, vars map[string]any, out any) error {
	raw, err := os.ReadFile(c.tokenPath)
	if err != nil {
		return fmt.Errorf("nais console: read service account token: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return errors.New("nais console: empty service account token")
	}
	body, _ := json.Marshal(map[string]any{"query": query, "variables": vars})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req) //nolint:gosec // fixed Console URL; tests pass httptest URLs
	if err != nil {
		return fmt.Errorf("nais console: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nais console: status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("nais console: decode: %w", err)
	}
	return nil
}

// listTeamApps answers the team's Nais applications to its members. A
// non-member gets 404 like every other team route; no Console client is 503.
func listTeamApps(teams teamDirectory) apisHandler {
	return func(w http.ResponseWriter, r *http.Request, c caller, _ *DB) error {
		ns := r.PathValue("namespace")
		if err := memberOf(c, ns); err != nil {
			return err
		}
		l, ok := teams.(appLister)
		if !ok {
			return stErr(http.StatusServiceUnavailable, "ServiceUnavailable", "the Nais application list is not available here")
		}
		apps, err := l.Apps(r.Context(), ns)
		if err != nil {
			return fmt.Errorf("list apps: %w", err)
		}
		writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "TeamAppList", "items": apps})
		return nil
	}
}
