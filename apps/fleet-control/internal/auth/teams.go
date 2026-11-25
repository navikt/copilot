package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

// TeamDirectory answers whether a person is a member of a Nais team. The
// owning team of a server is its namespace. A person is keyed on the token's
// preferred_username, normalised by normEmail.
type TeamDirectory interface {
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

// IsMember reports whether email is listed under team.
func (s staticTeams) IsMember(_ context.Context, team, email string) (bool, error) {
	email = normEmail(email)
	if email == "" {
		return false, nil
	}
	return slices.ContainsFunc(s[team], func(m string) bool { return normEmail(m) == email }), nil
}

// Teams lists the teams email is listed under.
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

// DevCluster reports whether cluster may run the write API with the static
// team list. Only dev-* counts; anything else, unset included, fails closed.
func DevCluster(cluster string) bool {
	return strings.HasPrefix(cluster, "dev-")
}

// PilotTeamsFromEnv reads FLEET_CONTROL_PILOT_TEAMS, the comma-separated
// teams that may write outside dev-*. Entries are trimmed and must be
// lowercase DNS labels, the exact form of a namespace path value; anything
// else refuses startup rather than being folded into a match. Empty or unset
// gives nil: every write outside dev is refused.
func PilotTeamsFromEnv() ([]string, error) {
	var out []string
	for _, t := range strings.Split(os.Getenv("FLEET_CONTROL_PILOT_TEAMS"), ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !dnsLabelRegex.MatchString(t) {
			return nil, fmt.Errorf("FLEET_CONTROL_PILOT_TEAMS: invalid team %q", t)
		}
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out, nil
}

// resolvePerson returns the NAVident behind a write request and its teams,
// for callers that need the list rather than one membership check.
func resolvePerson(ctx context.Context, a *Authz, header, adminClientID string, teams TeamDirectory) (string, []string, error) {
	ident, email, err := a.User(ctx, header, adminClientID)
	if err != nil {
		return "", nil, err
	}
	list, err := teams.Teams(ctx, email)
	return ident, list, err
}

// errNoServiceAccount: no Console service account is bound to this workload yet.
var errNoServiceAccount = errors.New("nais console: no service account bound (NAIS_SERVICE_ACCOUNT_TOKEN_PATH unset)")

// UnboundTeams answers every lookup with errNoServiceAccount.
type UnboundTeams struct{}

// IsMember always fails with errNoServiceAccount.
func (UnboundTeams) IsMember(context.Context, string, string) (bool, error) {
	return false, errNoServiceAccount
}

// Teams always fails with errNoServiceAccount.
func (UnboundTeams) Teams(context.Context, string) ([]string, error) { return nil, errNoServiceAccount }

// TeamsFromEnv picks the team directory. With a Nais service account binding
// (NAIS_SERVICE_ACCOUNT_TOKEN_PATH) it is the Nais Console API. Otherwise it
// reads FLEET_CONTROL_STATIC_TEAMS, a JSON object of team to member emails,
// from the fleet-control-static-teams secret, in dev only. Neither set means
// no members in dev-*, and unboundTeams elsewhere.
func TeamsFromEnv(cluster string) (TeamDirectory, error) {
	raw := os.Getenv("FLEET_CONTROL_STATIC_TEAMS")
	if path := strings.TrimSpace(os.Getenv("NAIS_SERVICE_ACCOUNT_TOKEN_PATH")); path != "" {
		if raw != "" && !DevCluster(cluster) {
			return nil, fmt.Errorf("FLEET_CONTROL_STATIC_TEAMS is for dev only, refused in cluster %q", cluster)
		}
		return newConsoleTeams(consoleTeamsURL, path), nil
	}
	if raw == "" && !DevCluster(cluster) {
		// Prod before the Console binding exists: start, but fail every lookup closed (503).
		return UnboundTeams{}, nil
	}
	t := staticTeams{}
	if raw == "" {
		return t, nil
	}
	if !DevCluster(cluster) {
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

// Same patterns as package main's; duplicated so auth does not import main.
var (
	dnsLabelRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	uuidRe        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)
