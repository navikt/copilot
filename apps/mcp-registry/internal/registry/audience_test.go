package registry

import (
	"encoding/json"
	"strings"
	"testing"
)

const audienceStatic = `{"servers":[{"name":"io.github.navikt/aud","description":"Audience test server","version":"1","remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]%s}]}`
const audienceSub = `{"name":"io.github.navikt/aud","description":"Audience test server","version":"1","owner":"team-a","remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]%s}`

// validateAllowlistBytes runs the allowlist.json path: the same decode and
// validation as validateAllowListFile and Store.Reload.
func validateAllowlistBytes(b []byte) error {
	data, err := DecodeStaticRegistry(b)
	if err != nil {
		return err
	}
	return ValidateRegistry(&data)
}

// TestAudience_BothPathsShareOneValidator feeds the same audience to
// allowlist.json and the write API. Each must accept or reject alike, and a
// rejection must carry validateAudience's own message, proving both call it.
func TestAudience_BothPathsShareOneValidator(t *testing.T) {
	cases := []struct {
		name, audience string
		ok             bool
	}{
		{"absent", ``, true},
		{"one team", `{"teams":["team-a"]}`, true},
		{"several teams", `{"teams":["team-a","nais","b2"]}`, true},
		{"max length slug", `{"teams":["` + strings.Repeat("a", MaxTeamSlugLen) + `"]}`, true},
		{"empty list", `{"teams":[]}`, false},
		{"missing teams", `{}`, false},
		{"uppercase slug", `{"teams":["Team-A"]}`, false},
		{"slug with space", `{"teams":["team a"]}`, false},
		{"slug with underscore", `{"teams":["team_a"]}`, false},
		{"empty slug", `{"teams":[""]}`, false},
		{"too long slug", `{"teams":["` + strings.Repeat("a", MaxTeamSlugLen+1) + `"]}`, false},
		{"duplicate", `{"teams":["team-a","team-a"]}`, false},
		{"too many teams", `{"teams":[` + manyTeams(maxAudienceTeams+1) + `]}`, false},
		{"teams wrong type", `{"teams":"team-a"}`, false},
		{"slug wrong type", `{"teams":[1]}`, false},
		{"audience wrong type", `["team-a"]`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field := ""
			if tc.audience != "" {
				field = `,"audience":` + tc.audience
			}
			staticErr := validateAllowlistBytes([]byte(strings.Replace(audienceStatic, "%s", field, 1)))
			_, _, subErr := ValidateSubmission([]byte(strings.Replace(audienceSub, "%s", field, 1)))
			if (staticErr == nil) != tc.ok || (subErr == nil) != tc.ok {
				t.Fatalf("want ok=%v, allowlist err=%v, submission err=%v", tc.ok, staticErr, subErr)
			}
			if tc.ok {
				return
			}
			var a Audience
			if json.Unmarshal([]byte(tc.audience), &a) != nil {
				return // a type error fails in each path's JSON decode, before validation
			}
			want := validateAudience(&a)
			if want == nil {
				t.Fatalf("validateAudience accepted %s that both paths rejected", tc.audience)
			}
			if !strings.HasSuffix(staticErr.Error(), want.Error()) || !strings.HasSuffix(subErr.Error(), want.Error()) {
				t.Fatalf("paths did not reject through validateAudience (%q):\nallowlist: %v\nsubmission: %v", want, staticErr, subErr)
			}
		})
	}
}

func manyTeams(n int) string {
	s := make([]string, n)
	for i := range s {
		s[i] = `"t` + strings.Repeat("x", i%5) + string(rune('a'+i%26)) + string(rune('a'+i/26)) + `"`
	}
	return strings.Join(s, ",")
}
