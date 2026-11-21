package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestPIIProjection verifies the owner decision (navikt/copilot#2529):
// people are projected to name and roles only; email, leave, employment dates,
// stale flags, resource types, teamPercent, NAVidents, contact addresses,
// changeStamps, links, and nested member lists are never exposed.
func TestPIIProjection(t *testing.T) {
	richMemberOla := map[string]any{
		"navIdent":    "Z111111",
		"description": "secret-member-note-ola",
		"roles":       []string{"DEVELOPER", "TECH_LEAD"},
		"teamPercent": 80,
		"startDate":   "2021-01-15",
		"endDate":     "2027-06-30",
		"resource": map[string]any{
			"navIdent":     "Z111111",
			"givenName":    "Ola",
			"familyName":   "Nordmann",
			"fullName":     "Ola Nordmann",
			"email":        "ola.nordmann@nav.no",
			"onLeave":      true,
			"resourceType": "INTERNAL",
			"startDate":    "2019-08-01",
			"endDate":      "2028-12-31",
			"stale":        true,
			"links":        map[string]any{"ui": "https://teamkatalog.nav.no/resource/Z111111"},
		},
	}
	richMemberKari := map[string]any{
		"navIdent":    "Z222222",
		"description": "secret-member-note-kari",
		"roles":       []string{"DESIGNER"},
		"teamPercent": 100,
		"startDate":   "2022-03-01",
		"endDate":     "2026-12-01",
		"resource": map[string]any{
			"navIdent":     "Z222222",
			"givenName":    "Kari",
			"familyName":   "Hansen",
			"fullName":     "",
			"email":        "kari.hansen@nav.no",
			"onLeave":      true,
			"resourceType": "EXTERNAL",
			"startDate":    "2022-03-01",
			"endDate":      "2026-12-01",
			"stale":        false,
		},
	}
	staleMemberNoResource := map[string]any{
		"navIdent": "Z333333",
		"roles":    []string{"PRODUCT_LEAD"},
	}

	richTeam := map[string]any{
		"id":                 testTeamID,
		"name":               "Team Copilot",
		"description":        "Platform team",
		"slackChannel":       "#copilot",
		"contactPersonIdent": "Z999999",
		"teamOwnerIdent":     "Z888888",
		"contactAddresses": []map[string]any{
			{"address": "contact.secret@nav.no", "type": "EPOST"},
		},
		"productAreaId": testAreaID,
		"avdelingNomId": "avd-1",
		"avdelingNavn":  "Utvikling",
		"clusterIds":    []string{testClusterID},
		"teamType":      "PLATFORM",
		"naisTeams":     []string{"copilot"},
		"tags":          []string{"ai"},
		"status":        "ACTIVE",
		"members":       []any{richMemberOla, richMemberKari, staleMemberNoResource},
		"changeStamp":   map[string]any{"lastModifiedBy": "Z666666 - Admin"},
		"links":         map[string]any{"ui": "https://teamkatalog.nav.no/team/" + testTeamID},
	}

	richArea := map[string]any{
		"id":                     testAreaID,
		"name":                   "Plattform",
		"avdelingNomId":          "avd-1",
		"avdelingNavn":           "Utvikling",
		"areaType":               "IT",
		"description":            "Plattformområde",
		"slackChannel":           "#plattform",
		"tags":                   []string{"nais"},
		"status":                 "ACTIVE",
		"isDefaultArea":          false,
		"ownerGroupNavidentList": []string{"Z777777"},
		"members":                []any{richMemberOla, richMemberKari},
		"changeStamp":            map[string]any{"lastModifiedBy": "Z666666 - Admin"},
		"links":                  map[string]any{"ui": "https://teamkatalog.nav.no/area/" + testAreaID},
	}

	richCluster := map[string]any{
		"id":            testClusterID,
		"name":          "KI-klynge",
		"description":   "KI",
		"slackChannel":  "#ki",
		"tags":          []string{"ki"},
		"productAreaId": testAreaID,
		"avdelingNomId": "avd-1",
		"avdelingNavn":  "Utvikling",
		"status":        "ACTIVE",
		"members":       []any{richMemberOla, richMemberKari},
		"changeStamp":   map[string]any{"lastModifiedBy": "Z666666 - Admin"},
		"links":         map[string]any{"ui": "https://teamkatalog.nav.no/cluster/" + testClusterID},
	}

	fake := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/team", "/team/search/copilot", "/team/naisteam/copilot":
			writeJSON(w, http.StatusOK, map[string]any{"content": []any{richTeam}})
		case "/team/" + testTeamID:
			writeJSON(w, http.StatusOK, richTeam)
		case "/productarea":
			writeJSON(w, http.StatusOK, map[string]any{"content": []any{richArea}})
		case "/productarea/" + testAreaID:
			writeJSON(w, http.StatusOK, richArea)
		case "/cluster":
			writeJSON(w, http.StatusOK, map[string]any{"content": []any{richCluster}})
		case "/cluster/" + testClusterID:
			writeJSON(w, http.StatusOK, richCluster)
		case "/member/membership/Z111111":
			writeJSON(w, http.StatusOK, map[string]any{
				"teams":        []any{richTeam},
				"productAreas": []any{richArea},
				"clusters":     []any{richCluster},
			})
		default:
			http.NotFound(w, r)
		}
	})

	srv := newTestServer(t, defaultTexasHandler("obo-tok"), fake)
	h := srv.routes()

	forbiddenStrings := []string{
		"ola.nordmann@nav.no",
		"kari.hansen@nav.no",
		"contact.secret@nav.no",
		"secret-member-note-ola",
		"secret-member-note-kari",
		"onLeave",
		"startDate",
		"endDate",
		"2021-01-15",
		"2027-06-30",
		"2019-08-01",
		"2028-12-31",
		"2022-03-01",
		"2026-12-01",
		"stale",
		"resourceType",
		"INTERNAL",
		"EXTERNAL",
		"teamPercent",
		"navIdent",
		"Z111111",
		"Z222222",
		"Z333333",
		"Z666666",
		"Z777777",
		"Z888888",
		"Z999999",
		"givenName",
		"familyName",
		"contactPersonIdent",
		"teamOwnerIdent",
		"ownerGroupNavidentList",
		"contactAddresses",
		"changeStamp",
		"lastModifiedBy",
		`"links"`,
	}

	assertNoForbidden := func(t *testing.T, txt string, extraForbidden ...string) {
		t.Helper()
		for _, f := range append(forbiddenStrings, extraForbidden...) {
			if strings.Contains(txt, f) {
				t.Errorf("output leaked forbidden PII/metadata %q in:\n%s", f, txt)
			}
		}
	}

	// 1. List/search/membership tools must never contain nested members lists.
	listAndMembershipCalls := []struct {
		name           string
		tool           string
		args           map[string]any
		extraForbidden []string
	}{
		{"list_teams", "list_teams", map[string]any{}, []string{`"members"`, "Ola Nordmann", "Kari Hansen"}},
		{"search_teams", "search_teams", map[string]any{"query": "copilot"}, []string{`"members"`, "Ola Nordmann", "Kari Hansen"}},
		{"get_teams_by_nais_team", "get_teams_by_nais_team", map[string]any{"naisTeam": "copilot"}, []string{`"members"`, "Ola Nordmann", "Kari Hansen"}},
		{"list_product_areas", "list_product_areas", map[string]any{}, []string{`"members"`, "Ola Nordmann", "Kari Hansen"}},
		{"list_clusters", "list_clusters", map[string]any{}, []string{`"members"`, "Ola Nordmann", "Kari Hansen"}},
		{"get_member_memberships", "get_member_memberships", map[string]any{"navIdent": "z111111"}, []string{`"members"`, "Kari Hansen"}},
	}

	for _, tc := range listAndMembershipCalls {
		t.Run(tc.name, func(t *testing.T) {
			_, out := callToolModern(t, h, "in", tc.tool, tc.args)
			txt, isErr := extractToolText(t, out)
			if isErr {
				t.Fatalf("unexpected tool error: %s", txt)
			}
			assertNoForbidden(t, txt, tc.extraForbidden...)
			if !strings.Contains(txt, `"memberCount"`) {
				t.Errorf("expected memberCount in %s output: %s", tc.name, txt)
			}
		})
	}

	// Verify get_member_memberships returns queried member's name and roles.
	t.Run("get_member_memberships keeps queried person name and roles", func(t *testing.T) {
		_, out := callToolModern(t, h, "in", "get_member_memberships", map[string]any{"navIdent": "Z111111"})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected error: %s", txt)
		}
		var res struct {
			Name         string           `json:"name"`
			Teams        []membershipUnit `json:"teams"`
			ProductAreas []membershipUnit `json:"productAreas"`
			Clusters     []membershipUnit `json:"clusters"`
		}
		if err := json.Unmarshal([]byte(txt), &res); err != nil {
			t.Fatal(err)
		}
		if res.Name != "Ola Nordmann" {
			t.Fatalf("name = %q, want Ola Nordmann", res.Name)
		}
		if len(res.Teams) != 1 || res.Teams[0].MemberCount != 3 || len(res.Teams[0].Roles) != 2 || res.Teams[0].Roles[0] != "DEVELOPER" || res.Teams[0].Roles[1] != "TECH_LEAD" {
			t.Fatalf("unexpected teams: %+v", res.Teams)
		}
		if len(res.ProductAreas) != 1 || res.ProductAreas[0].MemberCount != 2 {
			t.Fatalf("unexpected productAreas: %+v", res.ProductAreas)
		}
		if len(res.Clusters) != 1 || res.Clusters[0].MemberCount != 2 {
			t.Fatalf("unexpected clusters: %+v", res.Clusters)
		}
	})

	// 2. Detail tools must expose ONLY name and roles for each member.
	detailCalls := []struct {
		name        string
		tool        string
		args        map[string]any
		wantMembers int
	}{
		{"get_team", "get_team", map[string]any{"id": testTeamID}, 3},
		{"get_product_area", "get_product_area", map[string]any{"id": testAreaID}, 2},
		{"get_cluster", "get_cluster", map[string]any{"id": testClusterID}, 2},
	}

	for _, tc := range detailCalls {
		t.Run(tc.name, func(t *testing.T) {
			_, out := callToolModern(t, h, "in", tc.tool, tc.args)
			txt, isErr := extractToolText(t, out)
			if isErr {
				t.Fatalf("unexpected tool error: %s", txt)
			}
			assertNoForbidden(t, txt)

			var raw map[string]any
			if err := json.Unmarshal([]byte(txt), &raw); err != nil {
				t.Fatal(err)
			}
			members, ok := raw["members"].([]any)
			if !ok || len(members) != tc.wantMembers {
				t.Fatalf("members len = %d, want %d; body=%s", len(members), tc.wantMembers, txt)
			}
			for i, elem := range members {
				m, ok := elem.(map[string]any)
				if !ok {
					t.Fatalf("member[%d] is not an object", i)
				}
				if len(m) != 2 {
					t.Fatalf("member[%d] has %d keys (%v), want strictly 2 (name, roles)", i, len(m), m)
				}
				if _, ok := m["name"]; !ok {
					t.Fatalf("member[%d] missing name: %v", i, m)
				}
				if _, ok := m["roles"]; !ok {
					t.Fatalf("member[%d] missing roles: %v", i, m)
				}
			}
			// Verify givenName+familyName fallback produced "Kari Hansen" and fullName produced "Ola Nordmann".
			if !strings.Contains(txt, "Ola Nordmann") || !strings.Contains(txt, "Kari Hansen") {
				t.Fatalf("expected both member names in detail output: %s", txt)
			}
		})
	}
}
