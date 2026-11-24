package main

import (
	"context"
	"encoding/json"
	"errors"
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

// normEmail trims and lowercases an email for comparison. Non-ASCII gives ""
// (deny): Unicode case folding maps e.g. U+212A KELVIN SIGN to "k", so two
// different addresses could compare equal.
func normEmail(s string) string {
	s = strings.TrimSpace(s)
	for i := range len(s) {
		if s[i] >= 0x80 {
			return ""
		}
	}
	return strings.ToLower(s)
}

// staticTeams is a fixed team -> member email list, compared with normEmail.
// Members live in a Nais secret, never in the repo (it is public).
// It is the dev fallback when no Nais service account is bound; see
// consoleTeams for the real lookup.
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
// outside dev. shortcut: write routes are dev-only until fleet-control
// has a prod manifest and a service account bound there.
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

// errNoServiceAccount: no Console service account is bound to this workload yet.
var errNoServiceAccount = errors.New("nais console: no service account bound (NAIS_SERVICE_ACCOUNT_TOKEN_PATH unset)")

// unboundTeams answers every lookup with errNoServiceAccount.
type unboundTeams struct{}

func (unboundTeams) IsMember(context.Context, string, string) (bool, error) {
	return false, errNoServiceAccount
}

func (unboundTeams) Teams(context.Context, string) ([]string, error) { return nil, errNoServiceAccount }

// teamsFromEnv picks the team directory. With a Nais service account binding
// (NAIS_SERVICE_ACCOUNT_TOKEN_PATH) it is the Nais Console API. Otherwise it
// reads FLEET_CONTROL_STATIC_TEAMS, a JSON object of team to member emails,
// from the fleet-control-static-teams secret, in dev only. Neither set means
// no members in dev-*, and unboundTeams elsewhere.
func teamsFromEnv(cluster string) (teamDirectory, error) {
	raw := os.Getenv("FLEET_CONTROL_STATIC_TEAMS")
	if path := strings.TrimSpace(os.Getenv("NAIS_SERVICE_ACCOUNT_TOKEN_PATH")); path != "" {
		if raw != "" && !devCluster(cluster) {
			return nil, fmt.Errorf("FLEET_CONTROL_STATIC_TEAMS is for dev only, refused in cluster %q", cluster)
		}
		return newConsoleTeams(consoleTeamsURL, path), nil
	}
	if raw == "" && !devCluster(cluster) {
		// Prod before the Console binding exists: start, but fail every lookup closed (503).
		return unboundTeams{}, nil
	}
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
