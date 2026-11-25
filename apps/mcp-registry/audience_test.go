package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
		{"max length slug", `{"teams":["` + strings.Repeat("a", maxTeamSlugLen) + `"]}`, true},
		{"empty list", `{"teams":[]}`, false},
		{"missing teams", `{}`, false},
		{"uppercase slug", `{"teams":["Team-A"]}`, false},
		{"slug with space", `{"teams":["team a"]}`, false},
		{"slug with underscore", `{"teams":["team_a"]}`, false},
		{"empty slug", `{"teams":[""]}`, false},
		{"too long slug", `{"teams":["` + strings.Repeat("a", maxTeamSlugLen+1) + `"]}`, false},
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

// TestAudience_ServedUnchanged checks both sources serve audience as given
// under _meta."io.github.navikt/registry", and that entries without one have
// no audience key.
func TestAudience_ServedUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowlist.json")
	static := `{"servers":[
		{"name":"io.github.navikt/aud-static","description":"Static with audience","version":"1","audience":{"teams":["team-b","team-a"]},"remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]},
		{"name":"io.github.navikt/plain-static","description":"Static without audience","version":"1","tools":["x"],"remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]}]}`
	if err := os.WriteFile(path, []byte(static), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := func(context.Context) ([]DBEntry, error) {
		return []DBEntry{
			{Name: "io.github.navikt/aud-db", Entry: []byte(`{"name":"io.github.navikt/aud-db","description":"DB with audience","version":"1","owner":"team-a","audience":{"teams":["team-c"]},"remotes":[{"type":"sse","url":"https://mcp.example.com"}]}`)},
			{Name: "io.github.navikt/plain-db", Entry: subJSON("io.github.navikt/plain-db", "DB without audience", "1")},
		}, nil
	}
	store := NewStore(testConfig(), path, loader)
	if err := store.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	servers, err := store.GetServers()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"io.github.navikt/aud-static":   `{"teams":["team-b","team-a"]}`,
		"io.github.navikt/aud-db":       `{"teams":["team-c"]}`,
		"io.github.navikt/plain-static": ``,
		"io.github.navikt/plain-db":     ``,
	}
	for _, s := range servers {
		w, ok := want[s.Server.Name]
		if !ok {
			continue
		}
		delete(want, s.Server.Name)
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Meta map[string]map[string]json.RawMessage `json:"_meta"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if aud := string(got.Meta["io.github.navikt/registry"]["audience"]); aud != w {
			t.Errorf("%s: served audience %q, want %q", s.Server.Name, aud, w)
		}
	}
	if len(want) != 0 {
		t.Errorf("servers missing from response: %v", want)
	}
}
