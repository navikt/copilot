package main

import (
	"log/slog"
	"strings"
	"testing"
)

func TestLoadConfig_DefaultValues(t *testing.T) {
	// Clear all env vars that loadConfig reads, so defaults kick in.
	for _, key := range []string{
		"PORT", "LOG_LEVEL",
		"GITHUB_ENTERPRISE_SLUG", "GITHUB_ORG",
		"GITHUB_APP_ID", "GITHUB_APP_PRIVATE_KEY", "GITHUB_APP_INSTALLATION_ID",
		"GCP_TEAM_PROJECT_ID", "BIGQUERY_DATASET", "BIGQUERY_TABLE",
		"BIGQUERY_USER_TEAMS_TABLE", "BIGQUERY_USER_METRICS_TABLE",
		"BIGQUERY_REPO_METRICS_TABLE",
		"SLACK_WEBHOOK_URL",
	} {
		t.Setenv(key, "")
	}

	cfg := loadConfig()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Port", cfg.Port, "8080"},
		{"EnterpriseSlug", cfg.EnterpriseSlug, "nav"},
		{"OrganizationSlug", cfg.OrganizationSlug, "navikt"},
		{"BigQueryDataset", cfg.BigQueryDataset, "copilot_metrics"},
		{"BigQueryTable", cfg.BigQueryTable, "usage_metrics"},
		{"BigQueryUserTeamsTable", cfg.BigQueryUserTeamsTable, "user_teams"},
		{"BigQueryUserMetricsTable", cfg.BigQueryUserMetricsTable, "user_metrics"},
		{"BigQueryRepoMetricsTable", cfg.BigQueryRepoMetricsTable, "repository_metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}

	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelInfo)
	}
	if cfg.GitHubAppID != 0 {
		t.Errorf("GitHubAppID = %d, want 0", cfg.GitHubAppID)
	}
	if cfg.GitHubAppInstallationID != 0 {
		t.Errorf("GitHubAppInstallationID = %d, want 0", cfg.GitHubAppInstallationID)
	}
}

func TestLoadConfig_CustomEnvValues(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("GITHUB_ENTERPRISE_SLUG", "my-enterprise")
	t.Setenv("GITHUB_ORG", "my-org")
	t.Setenv("GITHUB_APP_ID", "42")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "secret-key")
	t.Setenv("GITHUB_APP_INSTALLATION_ID", "99")
	t.Setenv("GCP_TEAM_PROJECT_ID", "my-project")
	t.Setenv("BIGQUERY_DATASET", "custom_dataset")
	t.Setenv("BIGQUERY_TABLE", "custom_table")
	t.Setenv("BIGQUERY_REPO_METRICS_TABLE", "custom_repos")
	t.Setenv("SLACK_WEBHOOK_URL", "https://hooks.slack.com/test")

	cfg := loadConfig()

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
	if cfg.EnterpriseSlug != "my-enterprise" {
		t.Errorf("EnterpriseSlug = %q, want %q", cfg.EnterpriseSlug, "my-enterprise")
	}
	if cfg.OrganizationSlug != "my-org" {
		t.Errorf("OrganizationSlug = %q, want %q", cfg.OrganizationSlug, "my-org")
	}
	if cfg.GitHubAppID != 42 {
		t.Errorf("GitHubAppID = %d, want 42", cfg.GitHubAppID)
	}
	if cfg.GitHubAppPrivateKey != "secret-key" {
		t.Errorf("GitHubAppPrivateKey = %q, want %q", cfg.GitHubAppPrivateKey, "secret-key")
	}
	if cfg.GitHubAppInstallationID != 99 {
		t.Errorf("GitHubAppInstallationID = %d, want 99", cfg.GitHubAppInstallationID)
	}
	if cfg.BigQueryProjectID != "my-project" {
		t.Errorf("BigQueryProjectID = %q, want %q", cfg.BigQueryProjectID, "my-project")
	}
	if cfg.BigQueryDataset != "custom_dataset" {
		t.Errorf("BigQueryDataset = %q, want %q", cfg.BigQueryDataset, "custom_dataset")
	}
	if cfg.BigQueryTable != "custom_table" {
		t.Errorf("BigQueryTable = %q, want %q", cfg.BigQueryTable, "custom_table")
	}
	if cfg.BigQueryRepoMetricsTable != "custom_repos" {
		t.Errorf("BigQueryRepoMetricsTable = %q, want %q", cfg.BigQueryRepoMetricsTable, "custom_repos")
	}
	if cfg.SlackWebhookURL != "https://hooks.slack.com/test" {
		t.Errorf("SlackWebhookURL = %q, want %q", cfg.SlackWebhookURL, "https://hooks.slack.com/test")
	}
}

func TestGetEnvInt64_InvalidValue(t *testing.T) {
	t.Setenv("TEST_INT64_VAR", "not-a-number")

	result := getEnvInt64("TEST_INT64_VAR", 42)
	if result != 42 {
		t.Errorf("getEnvInt64 with invalid value = %d, want fallback 42", result)
	}
}

func TestGetEnvInt64_ValidValue(t *testing.T) {
	t.Setenv("TEST_INT64_VAR", "12345")

	result := getEnvInt64("TEST_INT64_VAR", 0)
	if result != 12345 {
		t.Errorf("getEnvInt64 = %d, want 12345", result)
	}
}

func TestGetEnvInt64_EmptyValue(t *testing.T) {
	t.Setenv("TEST_INT64_VAR", "")

	result := getEnvInt64("TEST_INT64_VAR", 99)
	if result != 99 {
		t.Errorf("getEnvInt64 with empty = %d, want fallback 99", result)
	}
}

func TestGetEnv_UsesEnvValue(t *testing.T) {
	t.Setenv("TEST_STRING_VAR", "custom-value")

	result := getEnv("TEST_STRING_VAR", "default")
	if result != "custom-value" {
		t.Errorf("getEnv = %q, want %q", result, "custom-value")
	}
}

func TestGetEnv_UsesFallback(t *testing.T) {
	t.Setenv("TEST_STRING_VAR", "")

	result := getEnv("TEST_STRING_VAR", "fallback")
	if result != "fallback" {
		t.Errorf("getEnv = %q, want %q", result, "fallback")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"debug", slog.LevelDebug},
		{"Debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"warn", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestConfigValidate_SingleMissingVar(t *testing.T) {
	cfg := &Config{
		GitHubAppID:             0, // Missing
		GitHubAppPrivateKey:     "key",
		GitHubAppInstallationID: 123,
		BigQueryProjectID:       "project",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	configErr, ok := err.(*ConfigError)
	if !ok {
		t.Fatalf("expected *ConfigError, got %T", err)
	}

	if len(configErr.MissingVars) != 1 {
		t.Errorf("expected 1 missing var, got %d: %v", len(configErr.MissingVars), configErr.MissingVars)
	}
	if configErr.MissingVars[0] != "GITHUB_APP_ID" {
		t.Errorf("expected GITHUB_APP_ID, got %s", configErr.MissingVars[0])
	}
}

func TestConfigValidate_MultipleMissing(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		missing []string
	}{
		{
			name: "missing private key and project",
			cfg: Config{
				GitHubAppID:             1,
				GitHubAppPrivateKey:     "",
				GitHubAppInstallationID: 1,
				BigQueryProjectID:       "",
			},
			missing: []string{"GITHUB_APP_PRIVATE_KEY", "GCP_TEAM_PROJECT_ID"},
		},
		{
			name: "missing installation ID only",
			cfg: Config{
				GitHubAppID:             1,
				GitHubAppPrivateKey:     "key",
				GitHubAppInstallationID: 0,
				BigQueryProjectID:       "proj",
			},
			missing: []string{"GITHUB_APP_INSTALLATION_ID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			configErr := err.(*ConfigError)
			if len(configErr.MissingVars) != len(tt.missing) {
				t.Fatalf("expected %d missing vars, got %d: %v", len(tt.missing), len(configErr.MissingVars), configErr.MissingVars)
			}
			for i, want := range tt.missing {
				if configErr.MissingVars[i] != want {
					t.Errorf("MissingVars[%d] = %q, want %q", i, configErr.MissingVars[i], want)
				}
			}
		})
	}
}

func TestConfigError_ErrorMessage(t *testing.T) {
	err := &ConfigError{MissingVars: []string{"VAR_A", "VAR_B"}}
	msg := err.Error()

	if !strings.Contains(msg, "VAR_A") || !strings.Contains(msg, "VAR_B") {
		t.Errorf("error message should list missing vars, got: %s", msg)
	}
	if !strings.Contains(msg, "missing required environment variables") {
		t.Errorf("error message should have prefix, got: %s", msg)
	}
}

func TestConfigValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		GitHubAppID:             1,
		GitHubAppPrivateKey:     "key",
		GitHubAppInstallationID: 1,
		BigQueryProjectID:       "project",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
