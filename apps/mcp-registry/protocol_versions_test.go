package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProtocolVersions_BothPathsShareOneValidator feeds the same value to
// allowlist.json and the write API; a rejection must carry
// validateProtocolVersions' own message in both.
func TestProtocolVersions_BothPathsShareOneValidator(t *testing.T) {
	cases := []struct {
		name, value string
		ok          bool
	}{
		{"absent", ``, true},
		{"one", `["2026-07-28"]`, true},
		{"several", `["2026-07-28","2025-11-25","2025-06-18"]`, true},
		{"max entries", `[` + manyRevs(maxProtocolVersions) + `]`, true},
		{"empty", `[]`, false},
		{"too many", `[` + manyRevs(maxProtocolVersions+1) + `]`, false},
		{"not a date", `["latest"]`, false},
		{"bad month", `["2026-13-01"]`, false},
		{"no dashes", `["20260728"]`, false},
		{"empty string", `[""]`, false},
		{"duplicate", `["2026-07-28","2026-07-28"]`, false},
		{"wrong type", `"2026-07-28"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field := ""
			if tc.value != "" {
				field = `,"protocolVersions":` + tc.value
			}
			staticErr := validateAllowlistBytes([]byte(strings.Replace(audienceStatic, "%s", field, 1)))
			_, _, subErr := ValidateSubmission([]byte(strings.Replace(audienceSub, "%s", field, 1)))
			if (staticErr == nil) != tc.ok || (subErr == nil) != tc.ok {
				t.Fatalf("want ok=%v, allowlist err=%v, submission err=%v", tc.ok, staticErr, subErr)
			}
			if tc.ok {
				return
			}
			var v []string
			if json.Unmarshal([]byte(tc.value), &v) != nil {
				return // type errors fail in JSON decode, before validation
			}
			want := validateProtocolVersions(v)
			if want == nil {
				t.Fatalf("validateProtocolVersions accepted %s that both paths rejected", tc.value)
			}
			if !strings.HasSuffix(staticErr.Error(), want.Error()) || !strings.HasSuffix(subErr.Error(), want.Error()) {
				t.Fatalf("paths did not reject through validateProtocolVersions (%q):\nallowlist: %v\nsubmission: %v", want, staticErr, subErr)
			}
		})
	}
}

func manyRevs(n int) string {
	s := make([]string, n)
	for i := range s {
		s[i] = `"2025-01-` + string(rune('1'+i/10)) + string(rune('0'+i%10)) + `"`
	}
	return strings.Join(s, ",")
}

// TestProtocolVersions_ServedUnchanged checks both sources serve the list as
// given under _meta."io.github.navikt/registry".
func TestProtocolVersions_ServedUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowlist.json")
	static := `{"servers":[
		{"name":"io.github.navikt/pv-static","description":"Static with versions","version":"1","protocolVersions":["2026-07-28","2025-11-25"],"remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]},
		{"name":"io.github.navikt/plain-static","description":"Static without","version":"1","tools":["x"],"remotes":[{"type":"streamable-http","url":"https://mcp.example.com/mcp"}]}]}`
	if err := os.WriteFile(path, []byte(static), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := func(context.Context) ([]DBEntry, error) {
		return []DBEntry{
			{Name: "io.github.navikt/pv-db", Entry: []byte(`{"name":"io.github.navikt/pv-db","description":"DB with versions","version":"1","owner":"team-a","protocolVersions":["2026-07-28"],"remotes":[{"type":"sse","url":"https://mcp.example.com"}]}`)},
			{Name: "io.github.navikt/plain-db", Entry: subJSON("io.github.navikt/plain-db", "DB without", "1")},
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
		"io.github.navikt/pv-static":    `["2026-07-28","2025-11-25"]`,
		"io.github.navikt/pv-db":        `["2026-07-28"]`,
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
		if pv := string(got.Meta["io.github.navikt/registry"]["protocolVersions"]); pv != w {
			t.Errorf("%s: served protocolVersions %q, want %q", s.Server.Name, pv, w)
		}
	}
	if len(want) != 0 {
		t.Errorf("servers missing from response: %v", want)
	}
}
