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
// owning team of a server is its namespace. A person is keyed on the token's
// preferred_username, normalised by normEmail.
type teamDirectory interface {
	IsMember(ctx context.Context, team, email string) (bool, error)
	// Teams lists the teams email belongs to, sorted.
	Teams(ctx context.Context, email string) ([]string, error)
}

// normEmail trims and lowercases an email for comparison.
func normEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// staticTeams is a fixed team -> member email list, compared with normEmail.
// Members live in a Nais secret, never in the repo (it is public).
// shortcut: no lookup against the Nais teams API yet; replace with a client
// for it before teams outside copilot use the write API (#2625).
type staticTeams map[string][]string

func (s staticTeams) IsMember(_ context.Context, team, email string) (bool, error) {
	email = normEmail(email)
	if email == "" {
		return false, nil
	}
	return slices.ContainsFunc(s[team], func(m string) bool { return normEmail(m) == email }), nil
}

func (s staticTeams) Teams(ctx context.Context, email string) ([]string, error) {
	out := []string{}
	for team := range s {
		if ok, _ := s.IsMember(ctx, team, email); ok {
			out = append(out, team)
		}
	}
	slices.Sort(out)
	return out, nil
}

// devCluster reports whether cluster may run the write API with the static
// team list. Only dev-* counts; anything else, unset included, fails closed.
func devCluster(cluster string) bool {
	return strings.HasPrefix(cluster, "dev-")
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
	ident, email, err := a.user(ctx, header, adminClientID)
	if err != nil {
		return "", nil, err
	}
	list, err := teams.Teams(ctx, email)
	return ident, list, err
}

// teamsFromEnv reads FLEET_CONTROL_STATIC_TEAMS, a JSON object of team to
// member emails, from the fleet-control-static-teams secret in dev. Unset
// means no members, so every write is refused.
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
			// Do not echo the value: it is a person's email.
			if local, domain, ok := strings.Cut(normEmail(m), "@"); !ok || local == "" || domain == "" || strings.ContainsAny(domain, "@ ") {
				return nil, fmt.Errorf("FLEET_CONTROL_STATIC_TEAMS: team %q has an entry that is not an email", team)
			}
		}
	}
	return t, nil
}
