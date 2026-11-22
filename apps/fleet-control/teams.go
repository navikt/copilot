package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// teamDirectory answers whether a person is a member of a Nais team. The
// owning team of a server is its namespace.
type teamDirectory interface {
	IsMember(ctx context.Context, team, navIdent string) (bool, error)
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

// teamsFromEnv reads FLEET_CONTROL_STATIC_TEAMS, a JSON object of team to
// NAVidents. Unset means no members, so every write is refused.
func teamsFromEnv() (teamDirectory, error) {
	raw := os.Getenv("FLEET_CONTROL_STATIC_TEAMS")
	t := staticTeams{}
	if raw == "" {
		return t, nil
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
