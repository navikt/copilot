package registry

import (
	"encoding/json"
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
