package provider

import "testing"

// Real outputs: the first 22 releases (2026-04-09 .. 2026-04-13 17:37) print
// "cplt 0.1.0" (clap default, Cargo.toml 0.1.0) or "cplt 2026.04.11-48aedfc"
// (date-only stamp, navikt/cplt ed1c219f..90b9ae7). A plain cargo build today
// prints "cplt 0.0.0"; a mise build on a dirty tree appends -dirty.
func TestCheckCpltFlagFloorOldFormats(t *testing.T) {
	args := []string{"--agent", "copilot", "--allow-socket", "/tmp/m.sock", "--"}
	for _, tc := range []struct {
		version   string
		wantBlock bool
	}{
		{"cplt 0.1.0", true},
		{"cplt 2026.04.11-48aedfc", true},
		{"cplt 0.0.0", false},
		{"cplt 2026.10.30-120000-abcdef0-dirty", false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			t.Cleanup(StubCpltVersion(tc.version + "\n"))
			err := checkCpltFlagFloor(args)
			if (err != nil) != tc.wantBlock {
				t.Fatalf("%q: block=%v, want %v (err %v)", tc.version, err != nil, tc.wantBlock, err)
			}
		})
	}
}
