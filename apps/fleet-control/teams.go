package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// teamDirectory answers whether a person is a member of a Nais team. The
// owning team of a server is its namespace.
type teamDirectory interface {
	IsMember(ctx context.Context, team, navIdent string) (bool, error)
	// Teams lists the teams navIdent belongs to, sorted.
	Teams(ctx context.Context, navIdent string) ([]string, error)
}

// staticTeams is a fixed team -> NAVident list.
// shortcut: no lookup against the Nais teams API yet; replace with a client
// for it before teams outside copilot use the write API (#2625).
type staticTeams map[string][]string

func (s staticTeams) IsMember(_ context.Context, team, navIdent string) (bool, error) {
	for _, m := range s[team] {
		if m == navIdent {
			return true, nil
		}
	}
	return false, nil
}

func (s staticTeams) Teams(_ context.Context, navIdent string) ([]string, error) {
	out := []string{}
	for team, members := range s {
		if slices.Contains(members, navIdent) {
			out = append(out, team)
		}
	}
	slices.Sort(out)
	return out, nil
}

// devCluster reports whether cluster may run the write API with the static
// team list. Unset or prod-* is not dev: fail closed.
func devCluster(cluster string) bool {
	return cluster != "" && !strings.HasPrefix(cluster, "prod")
}

// adminClientFor returns the client id that mounts the write routes, or ""
// outside dev. shortcut: write routes are dev-only until a Nais teams API
// client replaces staticTeams (#2625).
func adminClientFor(cluster, clientID string) string {
	if !devCluster(cluster) {
		return ""
	}
	return clientID
}

// resolvePerson returns the NAVident behind a write request and its teams,
// for callers that need the list rather than one membership check.
func resolvePerson(ctx context.Context, a *authz, header, adminClientID string, teams teamDirectory) (string, []string, error) {
	ident, err := a.user(ctx, header, adminClientID)
	if err != nil {
		return "", nil, err
	}
	list, err := teams.Teams(ctx, ident)
	return ident, list, err
}

// teamsFromEnv reads FLEET_CONTROL_STATIC_TEAMS, a JSON object of team to
// NAVidents. Unset means no members, so every write is refused.
func teamsFromEnv(cluster string) (teamDirectory, error) {
	raw := os.Getenv("FLEET_CONTROL_STATIC_TEAMS")
	t := staticTeams{}
	if raw == "" {
		return t, nil
	}
	if !devCluster(cluster) {
		return nil, fmt.Errorf("FLEET_CONTROL_STATIC_TEAMS is for dev only, refused in cluster %q", cluster)
	}
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return nil, fmt.Errorf("FLEET_CONTROL_STATIC_TEAMS: %w", err)
	}
	for team, members := range t {
		if !dnsLabelRegex.MatchString(team) {
			return nil, fmt.Errorf("FLEET_CONTROL_STATIC_TEAMS: invalid team %q", team)
		}
		for _, m := range members {
			if !navIdentRegex.MatchString(m) {
				return nil, fmt.Errorf("FLEET_CONTROL_STATIC_TEAMS: invalid NAVident %q", m)
			}
		}
	}
	return t, nil
}
