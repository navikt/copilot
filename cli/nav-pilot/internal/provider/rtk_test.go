package provider

import (
	"errors"
	"testing"
)

func TestParseRTKSetting(t *testing.T) {
	tests := []struct {
		name string
		value string
		want rtkSetting
	}{
		{name: "enabled", value: "1", want: rtkSettingEnabled},
		{name: "unset", value: "", want: rtkSettingUnset},
		{name: "non-empty enables", value: "enabled", want: rtkSettingEnabled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(string) string { return tt.value }
			if got := parseRTKSetting(false, getenv); got != tt.want {
				t.Fatalf("parseRTKSetting() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRTKWrappedCommand(t *testing.T) {
	deps := rtkDeps{
		getenv: func(string) string { return "1" },
		lookPath: func(file string) (string, error) {
			if file == "rtk" {
				return "/usr/local/bin/rtk", nil
			}
			return "", errors.New("not found")
		},
		isInteractive: func() bool { return true },
	}

	path, args, result := rtkWrappedCommandWithDeps("/usr/bin/cplt", []string{"--agent", "copilot"}, false, deps)
	if result != rtkResultApplied {
		t.Fatalf("result = %q, want %q", result, rtkResultApplied)
	}
	if path != "/usr/local/bin/rtk" {
		t.Fatalf("path = %q, want rtk path", path)
	}
	if len(args) != 3 || args[0] != "/usr/bin/cplt" {
		t.Fatalf("args = %v, want wrapped cplt argv", args)
	}
}

func TestRTKWrappedCommand_FallbackWhenMissing(t *testing.T) {
	deps := rtkDeps{
		getenv:        func(string) string { return "1" },
		lookPath:      func(string) (string, error) { return "", errors.New("missing") },
		isInteractive: func() bool { return true },
	}

	path, args, result := rtkWrappedCommandWithDeps("/usr/bin/cplt", []string{"--agent", "copilot"}, false, deps)
	if result != rtkResultMissing {
		t.Fatalf("result = %q, want %q", result, rtkResultMissing)
	}
	if path != "/usr/bin/cplt" {
		t.Fatalf("path = %q, want original path", path)
	}
	if len(args) != 2 || args[0] != "--agent" {
		t.Fatalf("args = %v, want original args", args)
	}
}

func TestRTKWrappedCommand_NotEnabled(t *testing.T) {
	deps := rtkDeps{
		getenv:        func(string) string { return "" },
		lookPath:      func(string) (string, error) { return "/usr/local/bin/rtk", nil },
		isInteractive: func() bool { return true },
	}

	path, args, result := rtkWrappedCommandWithDeps("/usr/bin/cplt", []string{"--agent", "copilot"}, false, deps)
	if result != rtkResultNotEnabled {
		t.Fatalf("result = %q, want %q", result, rtkResultNotEnabled)
	}
	if path != "/usr/bin/cplt" || len(args) != 2 {
		t.Fatalf("unexpected fallback path/args: path=%q args=%v", path, args)
	}
}

func TestRTKWrappedCommand_NonInteractive(t *testing.T) {
	deps := rtkDeps{
		getenv:        func(string) string { return "1" },
		lookPath:      func(string) (string, error) { return "/usr/local/bin/rtk", nil },
		isInteractive: func() bool { return false },
	}

	path, args, result := rtkWrappedCommandWithDeps("/usr/bin/cplt", []string{"--agent", "copilot"}, false, deps)
	if result != rtkResultNonInteractive {
		t.Fatalf("result = %q, want %q", result, rtkResultNonInteractive)
	}
	if path != "/usr/bin/cplt" || len(args) != 2 {
		t.Fatalf("unexpected fallback path/args: path=%q args=%v", path, args)
	}
}

func TestRTKWrappedCommand_CLIFlagForcesEnable(t *testing.T) {
	deps := rtkDeps{
		getenv:        func(string) string { return "" },
		lookPath:      func(string) (string, error) { return "/usr/local/bin/rtk", nil },
		isInteractive: func() bool { return true },
	}

	path, _, result := rtkWrappedCommandWithDeps("/usr/bin/cplt", []string{"--agent", "copilot"}, true, deps)
	if result != rtkResultApplied {
		t.Fatalf("result = %q, want %q", result, rtkResultApplied)
	}
	if path != "/usr/local/bin/rtk" {
		t.Fatalf("path = %q, want rtk path", path)
	}
}
