package agentpakke

// Moved with the grammar from internal/provider's runtime gate (#504 U2).

import (
	"testing"
)

func TestParseVersionRange(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		want  []Comparator
		isErr bool
	}{
		{name: "contract example", in: ">=1.18.20,<2", want: []Comparator{
			{Op: ">=", V: Semver3{1, 18, 20}},
			{Op: "<", V: Semver3{2, 0, 0}},
		}},
		{name: "spaces are tolerated", in: " >= 1.0.79 , < 2 ", want: []Comparator{
			{Op: ">=", V: Semver3{1, 0, 79}},
			{Op: "<", V: Semver3{2, 0, 0}},
		}},
		{name: "exact", in: "=1.2.3", want: []Comparator{{Op: "=", V: Semver3{1, 2, 3}}}},
		{name: "two-part operand zero-fills", in: "<=1.18", want: []Comparator{{Op: "<=", V: Semver3{1, 18, 0}}}},
		{name: "greater than", in: ">1.0.0", want: []Comparator{{Op: ">", V: Semver3{1, 0, 0}}}},

		{name: "empty", in: "", isErr: true},
		{name: "no operator", in: "1.2.3", isErr: true},
		{name: "operator without operand", in: ">=", isErr: true},
		{name: "trailing comma", in: ">=1.0.0,", isErr: true},
		{name: "leading comma", in: ",<2", isErr: true},
		{name: "non-numeric part", in: ">=1.x.3", isErr: true},
		{name: "four parts", in: ">=1.2.3.4", isErr: true},
		{name: "prerelease operand", in: ">=1.2.3-beta", isErr: true},
		{name: "leading zero", in: ">=01.2.3", isErr: true},
		{name: "caret is not an operator", in: "^1.2.3", isErr: true},
		{name: "double equals", in: "==1.2.3", isErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseVersionRange(tc.in)
			if tc.isErr {
				if err == nil {
					t.Fatalf("ParseVersionRange(%q) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersionRange(%q) errored: %v", tc.in, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ParseVersionRange(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("ParseVersionRange(%q)[%d] = %v, want %v", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestVersionRangeContains(t *testing.T) {
	tests := []struct {
		rng     string
		version Semver3
		want    bool
	}{
		{">=1.18.20,<2", Semver3{1, 18, 20}, true},  // lower bound is inclusive
		{">=1.18.20,<2", Semver3{1, 18, 19}, false}, // one patch below
		{">=1.18.20,<2", Semver3{1, 19, 0}, true},
		{">=1.18.20,<2", Semver3{2, 0, 0}, false}, // upper bound is exclusive
		{">=1.18.20,<2", Semver3{1, 99, 99}, true},
		{">=1.18.20,<2", Semver3{0, 99, 99}, false},
		{">1.0.0", Semver3{1, 0, 0}, false},
		{">1.0.0", Semver3{1, 0, 1}, true},
		{"<=1.0.0", Semver3{1, 0, 0}, true},
		{"=1.2.3", Semver3{1, 2, 3}, true},
		{"=1.2.3", Semver3{1, 2, 4}, false},
		{"=1.2", Semver3{1, 2, 0}, true},
	}
	for _, tc := range tests {
		rng, err := ParseVersionRange(tc.rng)
		if err != nil {
			t.Fatalf("ParseVersionRange(%q): %v", tc.rng, err)
		}
		if got := rng.Contains(tc.version); got != tc.want {
			t.Errorf("%q contains %v = %v, want %v", tc.rng, tc.version, got, tc.want)
		}
	}
}
