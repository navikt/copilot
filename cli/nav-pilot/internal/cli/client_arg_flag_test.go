package cli

import (
	"errors"
	"slices"
	"testing"
)

var errStopAfterParse = errors.New("stop after parse")

// parseLaunchFlags runs run() up to the end of flag parsing and returns what it parsed.
func parseLaunchFlags(t *testing.T, args ...string) (CLIOverrides, error) {
	t.Helper()
	var got CLIOverrides
	orig := parsedOverrides
	t.Cleanup(func() { parsedOverrides = orig })
	parsedOverrides = func(o CLIOverrides) error { got = o; return errStopAfterParse }
	err := run(args)
	if errors.Is(err, errStopAfterParse) {
		err = nil
	}
	return got, err
}

func TestClientArgParsing(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantClient []string
		wantExtra  []string
	}{
		{"separate word", []string{"--client-arg", "--allow-all-paths"}, []string{"--allow-all-paths"}, nil},
		{"with =", []string{"--client-arg=--allow-all-paths"}, []string{"--allow-all-paths"}, nil},
		{"= in the value", []string{"--client-arg=--model=gpt-5"}, []string{"--model=gpt-5"}, nil},
		{"nav-pilot flag as the value is not split", []string{"--client-arg", "--model=gpt-5"}, []string{"--model=gpt-5"}, nil},
		{"repeat keeps order and duplicates", []string{"--client-arg", "--model", "--client-arg=gpt-5", "--client-arg", "-v", "--client-arg", "-v"}, []string{"--model", "gpt-5", "-v", "-v"}, nil},
		{"with --", []string{"--client-arg", "--foo", "--", "-p", "hi"}, []string{"--foo"}, []string{"-p", "hi"}},
		{"after -- it is the client's", []string{"--", "--client-arg", "x"}, nil, []string{"--client-arg", "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := parseLaunchFlags(t, tt.args...)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(o.ClientArgs, tt.wantClient) || !slices.Equal(o.ExtraArgs, tt.wantExtra) {
				t.Errorf("ClientArgs = %q, ExtraArgs = %q; want %q, %q", o.ClientArgs, o.ExtraArgs, tt.wantClient, tt.wantExtra)
			}
		})
	}
}

func TestClientArgRefused(t *testing.T) {
	for _, args := range [][]string{{"--client-arg"}, {"--client-arg", "--"}} {
		if _, err := parseLaunchFlags(t, args...); err == nil {
			t.Errorf("%q accepted", args)
		}
	}
}

// --client-arg values go first, then the args after --; only -- makes the
// launch explicit (headless without a terminal, no menu).
func TestResolveClientArgs(t *testing.T) {
	t.Parallel()
	r := resolve(&Config{}, CLIOverrides{ClientArgs: []string{"--model", "x"}, ExtraArgs: []string{"-p", "hi"}})
	if want := []string{"--model", "x", "-p", "hi"}; !slices.Equal(r.ExtraArgs, want) || !r.DashDash {
		t.Errorf("ExtraArgs = %q DashDash = %v, want %q true", r.ExtraArgs, r.DashDash, want)
	}
	r = resolve(&Config{}, CLIOverrides{ClientArgs: []string{"--foo"}})
	if !slices.Equal(r.ExtraArgs, []string{"--foo"}) || r.DashDash {
		t.Errorf("ExtraArgs = %q DashDash = %v, want [--foo] false", r.ExtraArgs, r.DashDash)
	}
}
