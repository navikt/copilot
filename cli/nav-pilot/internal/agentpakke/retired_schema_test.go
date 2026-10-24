package agentpakke

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateRetired tests the validator directly, because going through
// findRetiredOrphans cannot: every malformed record there also fails to match a
// real file for unrelated reasons, so that test stays green with validation
// removed. The first version of this coverage did exactly that and proved
// nothing.
func TestValidateRetired(t *testing.T) {
	blob := strings.Repeat("a", 40)
	tests := []struct {
		name    string
		record  string
		wantErr string // substring, or "" when the record must be accepted
	}{
		{"conforming", `{"paths":{"agents/a.agent.md":["` + blob + `"]}}`, ""},
		{"a declared layout's nested path", `{"paths":{"content/agents/a.agent.md":["` + blob + `"]}}`, ""},
		{"empty paths says nothing is retired", `{"paths":{}}`, ""},
		{"comment is ignored", `{"_comment":"generated","paths":{}}`, ""},

		{"paths is required", `{"_comment":"x"}`, "paths"},
		{"hash is not a blob id", `{"paths":{"agents/a.agent.md":["deadbeef"]}}`, "not a git blob id"},
		{"hash is uppercase", `{"paths":{"agents/a.agent.md":["` + strings.ToUpper(blob) + `"]}}`, "not a git blob id"},
		{"no hashes at all", `{"paths":{"agents/a.agent.md":[]}}`, "minItems"},

		// The path rules are the ones that decide which file gets removed.
		{"path escapes the repo", `{"paths":{"../../etc/passwd":["` + blob + `"]}}`, "not a repo-relative path"},
		{"absolute path", `{"paths":{"/etc/passwd":["` + blob + `"]}}`, "not a repo-relative path"},
		{"home-relative path", `{"paths":{"~/secrets":["` + blob + `"]}}`, "not a repo-relative path"},
		{"not an object", `["agents/a.agent.md"]`, "got array"},

		{"replacement", `{"paths":{},"replacements":{"skills/old":"new-skill"}}`, ""},
		{"empty replacement", `{"paths":{},"replacements":{"skills/old":""}}`, "minLength"},
		{"replacement with a control character", `{"paths":{},"replacements":{"skills/old":"a\u001b[2Jb"}}`, "replacements"},
		{"replacement key escapes the repo", `{"paths":{},"replacements":{"../x":"new"}}`, "not a repo-relative path"},
		{"replacement with a C1 CSI", `{"paths":{},"replacements":{"skills/old":"a\u009bb"}}`, "replacements"},
		{"replacement with a bidi override", `{"paths":{},"replacements":{"skills/old":"a\u202eb"}}`, "replacements"},
		{"replacement too long", `{"paths":{},"replacements":{"skills/old":"` + strings.Repeat("x", 200) + `"}}`, "maxLength"},
		{"replacement is not a string", `{"paths":{},"replacements":{"skills/old":1}}`, "replacements"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRetired([]byte(tt.record))
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateRetired = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateRetired accepted %s, want a refusal", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %q", err, tt.wantErr)
			}
		})
	}
}

// The record this repo publishes must pass the validator sync applies, or sync
// silently ignores it.
func TestCommittedRetiredRecordValidates(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", RetiredRecordPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRetired(data); err != nil {
		t.Error(err)
	}
}
