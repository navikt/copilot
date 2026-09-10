package agentpakke

import (
	"strings"
	"testing"
)

// TestValidateRelease holds the published release metadata schema to the
// contract docs/README.agentpakke.md states. Discovery relies on it for every
// shape check, so a loosened pattern here is a loosened trust check there.
func TestValidateRelease(t *testing.T) {
	sha := strings.Repeat("a", 40)
	rec := func(version, sourceSha string) string {
		return `{"schemaVersion":1,"name":"grillmester","version":"` + version + `","sourceSha":"` + sourceSha + `"}`
	}
	tests := []struct {
		name    string
		record  string
		wantErr string // substring, or "" when the record must be accepted
	}{
		{"conforming", rec("0.4.1", sha), ""},
		{"zero version", rec("0.0.0", sha), ""},

		{"schemaVersion 2", `{"schemaVersion":2,"name":"grillmester","version":"0.4.1","sourceSha":"` + sha + `"}`, "schemaVersion"},
		{"missing sourceSha", `{"schemaVersion":1,"name":"grillmester","version":"0.4.1"}`, "sourceSha"},
		{"empty name", `{"schemaVersion":1,"name":"","version":"0.4.1","sourceSha":"` + sha + `"}`, "name"},
		{"extra field", `{"schemaVersion":1,"name":"grillmester","version":"0.4.1","sourceSha":"` + sha + `","repo":"evil/fork"}`, "'repo' not allowed"},
		{"v prefix", rec("v0.4.1", sha), "version"},
		{"prerelease", rec("0.4.1-rc.1", sha), "version"},
		{"build metadata", rec("0.4.1+build", sha), "version"},
		{"two parts", rec("0.4", sha), "version"},
		{"leading zero", rec("0.04.1", sha), "version"},
		{"short sha", rec("0.4.1", "aaaaaaa"), "sourceSha"},
		{"uppercase sha", rec("0.4.1", strings.ToUpper(sha)), "sourceSha"},
		{"not JSON", `{`, "not valid JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRelease([]byte(tt.record))
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateRelease = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateRelease accepted %s, want a refusal", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}
