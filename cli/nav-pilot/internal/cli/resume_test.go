package cli

import (
	"slices"
	"strings"
	"testing"
)

func TestSessionClientArgs(t *testing.T) {
	const id = "0123abcd-0000-4000-8000-000000000000"
	tests := []struct {
		name    string
		client  string
		s       SessionFlag
		extra   []string
		want    []string
		wantErr string
	}{
		{"none", "copilot", SessionFlag{}, []string{"-p", "x"}, []string{"-p", "x"}, ""},
		{"copilot picker", "copilot", SessionFlag{Resume: true}, nil, []string{"--resume"}, ""},
		{"copilot id", "copilot", SessionFlag{Resume: true, ID: id}, nil, []string{"--resume=" + id}, ""},
		{"copilot continue", "copilot", SessionFlag{Continue: true}, []string{"-p", "x"}, []string{"-p", "x", "--continue"}, ""},
		{"opencode id", "opencode", SessionFlag{Resume: true, ID: "ses_1"}, nil, []string{"--session", "ses_1"}, ""},
		{"opencode continue", "opencode", SessionFlag{Continue: true}, []string{"run", "go"}, []string{"run", "go", "--continue"}, ""},
		{"before client --", "opencode", SessionFlag{Continue: true}, []string{"run", "--", "-x"}, []string{"run", "--continue", "--", "-x"}, ""},
		{"opencode no picker", "opencode", SessionFlag{Resume: true}, nil, nil, "no session picker"},
		{"pi refused", "pi", SessionFlag{Continue: true}, nil, nil, "not supported for client pi"},
		{"both", "copilot", SessionFlag{Resume: true, Continue: true}, nil, nil, "cannot be used together"},
		{"copilot conflict", "copilot", SessionFlag{Continue: true}, []string{"--resume=" + id}, nil, "conflicts"},
		{"copilot conflict -r", "copilot", SessionFlag{Resume: true}, []string{"-r"}, nil, "conflicts"},
		{"opencode conflict", "opencode", SessionFlag{Resume: true, ID: "a"}, []string{"-c"}, nil, "conflicts"},
		{"message after -- is no conflict", "opencode", SessionFlag{Continue: true}, []string{"run", "--", "--continue"}, []string{"run", "--continue", "--", "--continue"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sessionClientArgs(tt.client, tt.s, tt.extra)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || !slices.Equal(got, tt.want) {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

// The translated Copilot args reach session metrics as one word: a full id
// is that session.
func TestSessionClientArgsReachCopilotMetrics(t *testing.T) {
	const id = "0123abcd-0000-4000-8000-000000000000"
	args, err := sessionClientArgs("copilot", SessionFlag{Resume: true, ID: id}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(args, []string{"--resume=" + id}) {
		t.Fatalf("args = %q", args)
	}
	if got, add := copilotSessionArgs(args); got != id || add != nil {
		t.Fatalf("copilotSessionArgs = %q, %q", got, add)
	}
	if got, add := copilotSessionArgs([]string{"--continue"}); got != "" || add != nil {
		t.Fatalf("--continue: %q, %q", got, add)
	}
}

func TestSessionIDArg(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		wantID string
		wantOK bool
	}{
		{"id", []string{"--resume", "abc"}, "abc", true},
		{"--resume= after split", []string{"--resume", ""}, "", true},
		{"--resume=x after split", []string{"--resume", "x"}, "x", true},
		{"last", []string{"--resume"}, "", false},
		{"before --", []string{"--resume", "--", "-p", "x"}, "", false},
		{"before a command", []string{"--resume", "sync"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := sessionIDArg(tt.args, 0)
			if id != tt.wantID || ok != tt.wantOK {
				t.Fatalf("got %q, %v; want %q, %v", id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

// Refused in the pre-scan, before any config, sync or launch.
func TestRunRefusesSessionFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NAV_PILOT_CONFIG", home+"/config.toml")
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME"} {
		t.Setenv(k, home+"/"+k)
	}
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--resume", "--continue"}, "cannot be used together"},
		{[]string{"--continue", "--resume", "x"}, "cannot be used together"},
		{[]string{"--continue", "--continue"}, "--continue given twice"},
		{[]string{"--resume", "a", "--resume"}, "--resume given twice"},
		{[]string{"--resume="}, "needs an id"},
		{[]string{"--resume", "sync"}, "sync does not launch"},
		{[]string{"--continue", "doctor"}, "doctor does not launch"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			err := run(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
		})
	}
}
