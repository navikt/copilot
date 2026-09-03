package cli

import "testing"

func TestCpltPresetFromConfigGet(t *testing.T) {
	tests := []struct {
		name, out, want string
	}{
		{"value with annotation", "standard\n[cplt] (default — not set in config file)\n", "standard"},
		{"strict", "strict\n[cplt] (set in config file)\n", "strict"},
		{"bare value", "permissive\n", "permissive"},
		{"padded", "  full-trust  \n", "full-trust"},
		{"empty (command failed, nothing on stdout)", "", ""},
		{"only whitespace", "\n\n", ""},
		{"unknown value", "paranoid\n", ""},
		{"error text", "[cplt] unknown config key 'sandbox.preset'\n", ""},
	}
	for _, tc := range tests {
		if got := cpltPresetFromConfigGet(tc.out); got != tc.want {
			t.Errorf("%s: cpltPresetFromConfigGet(%q) = %q, want %q", tc.name, tc.out, got, tc.want)
		}
	}
}

func TestCpltRecommendStrict(t *testing.T) {
	tests := []struct {
		preset string
		want   bool
	}{
		{"strict", false},
		{"standard", true},
		{"permissive", true},
		{"full-trust", true},
		{"", false}, // unknown: skip the recommendation rather than guess
	}
	for _, tc := range tests {
		if got := cpltRecommendStrict(tc.preset); got != tc.want {
			t.Errorf("cpltRecommendStrict(%q) = %v, want %v", tc.preset, got, tc.want)
		}
	}
}

// realCpltCheckBattery is the head of a real `cplt check --json` battery report,
// captured from cplt 2026.09.02-164136-a480712 in this repository. Trimmed to
// two items; the fields doctor reads are verbatim.
const realCpltCheckBattery = `{
  "agent": "Copilot",
  "platform": "macos (Seatbelt)",
  "enforcing": false,
  "verified": 2,
  "battery": true,
  "items": [
    {
      "name": "read project dir",
      "category": "filesystem",
      "target": "/repo",
      "decision": "allowed",
      "expected": "allowed",
      "reason": "covered by the project-dir rule (read+write+execute)."
    },
    {
      "name": "read $HOME (root)",
      "category": "filesystem",
      "target": "/home/u",
      "decision": "allowed",
      "expected": "blocked",
      "reason": "not covered by any allow rule, so it is denied by default.",
      "fix": "grant access with --allow-read <PATH> (read) or --allow-write <PATH> (read+write), or add it under [allow] read/write in config."
    }
  ]
}`

// TestParseCpltCheckReport pins the contract doctor's enforcement check rests
// on: the real report shape decodes, and every way of not getting an answer is
// unknown rather than a verdict in either direction.
func TestParseCpltCheckReport(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantNil      bool
		wantEnforce  bool
		wantVerified int
	}{
		{name: "a real battery report", in: realCpltCheckBattery, wantVerified: 2},
		{name: "an enforcing battery",
			in:          `{"agent":"Copilot","platform":"macos (Seatbelt)","enforcing":true,"verified":7,"battery":true,"items":[]}`,
			wantEnforce: true, wantVerified: 7},
		// An older cplt has no `check` subcommand: clap writes usage to stderr
		// and stdout is empty. That is unknown, not "not enforcing".
		{name: "no check subcommand", in: "", wantNil: true},
		{name: "cplt missing entirely", in: "", wantNil: true},
		{name: "not JSON at all", in: "error: unrecognized subcommand 'check'\n", wantNil: true},
		{name: "truncated read", in: `{"enforcing":true,"battery":`, wantNil: true},
		// A targeted query is ungraded — `enforcing` is false there for a
		// reason that has nothing to do with the sandbox.
		{name: "a targeted, non-battery query",
			in:      `{"agent":"Copilot","platform":"macos (Seatbelt)","enforcing":false,"verified":0,"battery":false,"items":[]}`,
			wantNil: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCpltCheckReport([]byte(tc.in))
			if tc.wantNil {
				if got != nil {
					t.Fatalf("parseCpltCheckReport(%q) = %+v, want nil (unknown)", tc.in, got)
				}
				return
			}
			if got == nil {
				t.Fatal("parseCpltCheckReport returned nil, want a report")
			}
			if got.Enforcing != tc.wantEnforce {
				t.Errorf("Enforcing = %v, want %v", got.Enforcing, tc.wantEnforce)
			}
			if got.Verified != tc.wantVerified {
				t.Errorf("Verified = %d, want %d", got.Verified, tc.wantVerified)
			}
		})
	}
}
