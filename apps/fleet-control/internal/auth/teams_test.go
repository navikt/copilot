package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTeamsFromEnv(t *testing.T) {
	// Emails are compared trimmed and case-insensitively.
	t.Setenv("FLEET_CONTROL_STATIC_TEAMS", `{"team-a":[" Kari.Nordmann@Example.com "],"team-b":["kari.nordmann@example.com"]}`)
	if d, err := TeamsFromEnv("dev-gcp"); err != nil {
		t.Fatal(err)
	} else if ok, _ := d.IsMember(context.Background(), "team-a", "KARI.nordmann@example.COM"); !ok {
		t.Error("want member")
	} else if ok, _ := d.IsMember(context.Background(), "team-a", ""); ok {
		t.Error("empty email must not be a member")
	} else if list, _ := d.Teams(context.Background(), "kari.nordmann@example.com"); strings.Join(list, ",") != "team-a,team-b" {
		t.Errorf("teams: %v", list)
	}
	// The static list must never run outside dev, and an unknown cluster is not dev.
	for _, cluster := range []string{"prod-gcp", "prod-fss", "management", "dev", ""} {
		if _, err := TeamsFromEnv(cluster); err == nil {
			t.Errorf("static teams in cluster %q: want error", cluster)
		}
	}
	ident, list, err := resolvePerson(context.Background(), testAuthz(t), "Bearer "+userTok("A111111"), adminID, testTeams)
	if err != nil || ident != "A111111" || strings.Join(list, ",") != "team-a" {
		t.Errorf("resolvePerson: %q %v %v", ident, list, err)
	}
	if _, _, err = resolvePerson(context.Background(), testAuthz(t), "Bearer "+adminID, adminID, testTeams); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("resolvePerson with an app token: %v", err)
	}
	// A mixed-case preferred_username in the token resolves to its teams.
	_, list, err = resolvePerson(context.Background(), testAuthz(t), "Bearer "+userTok("A111111")+":Kari.Nordmann@Example.com", adminID, staticTeams{"team-a": {"kari.nordmann@example.com"}})
	if err != nil || strings.Join(list, ",") != "team-a" {
		t.Errorf("resolvePerson by email: %v %v", list, err)
	}
	// No preferred_username: refused, not resolved to an empty team list.
	if _, _, err = resolvePerson(context.Background(), testAuthz(t), "Bearer "+userTok("A111111")+":", adminID, testTeams); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("resolvePerson without preferred_username: %v", err)
	}
	for _, bad := range []string{`{"Team":["a@example.com"]}`, `{"team-a":["x"]}`, `{"team-a":["@example.com"]}`, `[]`} {
		t.Setenv("FLEET_CONTROL_STATIC_TEAMS", bad)
		if _, err := TeamsFromEnv("dev-gcp"); err == nil {
			t.Errorf("%s: want error", bad)
		}
	}
}

const adminID = "44444444-4444-4444-4444-444444444444"

var testTeams = staticTeams{"team-a": {"A111111", "A222222"}, "team-b": {"B111111"}}

func userTok(ident string) string { return "user:" + adminID + ":" + ident }
