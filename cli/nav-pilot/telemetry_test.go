package main

import (
	"errors"
	"testing"
)

func TestTelemetryResult(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "success", err: nil, want: "success"},
		{name: "updates available", err: errUpdatesAvailable, want: "updates_available"},
		{name: "wrapped updates available", err: errors.Join(errUpdatesAvailable, errors.New("other")), want: "updates_available"},
		{name: "error", err: errors.New("boom"), want: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := telemetryResult(tt.err); got != tt.want {
				t.Fatalf("telemetryResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTelemetryEnabled(t *testing.T) {
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "")
	if !telemetryEnabled() {
		t.Fatal("expected telemetryEnabled to return true by default")
	}

	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "1")
	if !telemetryEnabled() {
		t.Fatal("expected telemetryEnabled to return true for 1")
	}

	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "0")
	if telemetryEnabled() {
		t.Fatal("expected telemetryEnabled to return false for 0")
	}

	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "off")
	if telemetryEnabled() {
		t.Fatal("expected telemetryEnabled to return false for off")
	}
}

func TestNormalizeTelemetryDimension_AllowsStartupAndLaunch(t *testing.T) {
	if got := normalizeTelemetryDimension("startup", "unknown"); got != "startup" {
		t.Fatalf("normalizeTelemetryDimension(startup) = %q, want startup", got)
	}
	if got := normalizeTelemetryDimension("launch", "unknown"); got != "launch" {
		t.Fatalf("normalizeTelemetryDimension(launch) = %q, want launch", got)
	}
}

func TestDetectExecutionContext(t *testing.T) {
	keys := []string{
		"NAV_PILOT_EXECUTION_CONTEXT",
		"GITHUB_ACTIONS",
		"CI",
		"GITLAB_CI",
		"JENKINS_URL",
		"BUILDKITE",
		"CIRCLECI",
		"TF_BUILD",
		"BUILD_ID",
	}

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "override wins over github actions",
			env: map[string]string{
				"NAV_PILOT_EXECUTION_CONTEXT": "organic",
				"GITHUB_ACTIONS":              "true",
			},
			want: "organic",
		},
		{
			name: "github actions detected",
			env: map[string]string{
				"GITHUB_ACTIONS": "true",
			},
			want: "ci_github_actions",
		},
		{
			name: "generic ci detected",
			env: map[string]string{
				"CI": "true",
			},
			want: "ci_other",
		},
		{
			name: "generic ci env key detected",
			env: map[string]string{
				"GITLAB_CI": "1",
			},
			want: "ci_other",
		},
		{
			name: "unknown override allowed",
			env: map[string]string{
				"NAV_PILOT_EXECUTION_CONTEXT": "unknown",
			},
			want: "unknown",
		},
		{
			name: "invalid override falls back to github actions",
			env: map[string]string{
				"NAV_PILOT_EXECUTION_CONTEXT": "bogus",
				"GITHUB_ACTIONS":              "true",
			},
			want: "ci_github_actions",
		},
		{
			name: "organic by default",
			env:  map[string]string{},
			want: "organic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range keys {
				t.Setenv(key, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := detectExecutionContext(); got != tt.want {
				t.Fatalf("detectExecutionContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsGenericCIEnvKeys(t *testing.T) {
	keys := []string{"GITLAB_CI", "JENKINS_URL", "BUILDKITE", "CIRCLECI", "TF_BUILD", "BUILD_ID"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			t.Setenv("CI", "")
			t.Setenv("GITLAB_CI", "")
			t.Setenv("JENKINS_URL", "")
			t.Setenv("BUILDKITE", "")
			t.Setenv("CIRCLECI", "")
			t.Setenv("TF_BUILD", "")
			t.Setenv("BUILD_ID", "")
			t.Setenv(key, "1")
			if !isGenericCI() {
				t.Fatalf("expected isGenericCI() to return true when %s is set", key)
			}
		})
	}
}
