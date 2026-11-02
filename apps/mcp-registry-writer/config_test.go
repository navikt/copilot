package main

import (
	"log/slog"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOGGED_ENDPOINTS", "")
	t.Setenv("DB_URL", "")
	t.Setenv("ADMIN_USERS", "")
	t.Setenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "")

	cfg := loadConfig()
	if cfg.Port != "8080" {
		t.Errorf("expected Port 8080, got %s", cfg.Port)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("expected LogLevel INFO, got %v", cfg.LogLevel)
	}
	if len(cfg.AdminUsers) != 0 {
		t.Errorf("expected 0 admin users, got %d", len(cfg.AdminUsers))
	}
	if !cfg.LoggedEndpoints["/api/v1/entries"] {
		t.Errorf("expected /api/v1/entries to be logged by default")
	}
}

func TestLoadConfig_Custom(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("LOGGED_ENDPOINTS", "/health,/api/v1/entries")
	t.Setenv("DB_URL", "postgres://localhost/test")
	t.Setenv("ADMIN_USERS", "Alice@example.com, bob@example.com ")
	t.Setenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://localhost:8088/introspect")

	cfg := loadConfig()
	if cfg.Port != "9090" {
		t.Errorf("expected Port 9090, got %s", cfg.Port)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("expected LogLevel DEBUG, got %v", cfg.LogLevel)
	}
	if !cfg.IsAdmin("alice@example.com") {
		t.Errorf("expected alice@example.com to be admin")
	}
	if !cfg.IsAdmin("BOB@EXAMPLE.COM") {
		t.Errorf("expected BOB@EXAMPLE.COM to be admin (case-insensitive)")
	}
	if cfg.IsAdmin("charlie@example.com") {
		t.Errorf("charlie should not be admin")
	}
	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("expected DatabaseURL postgres://localhost/test, got %s", cfg.DatabaseURL)
	}
	if cfg.TexasEndpoint != "http://localhost:8088/introspect" {
		t.Errorf("expected TexasEndpoint http://localhost:8088/introspect, got %s", cfg.TexasEndpoint)
	}
}

func TestConfig_IsAdmin_CaseInsensitiveDirectMap(t *testing.T) {
	cfg := &Config{
		AdminUsers: map[string]bool{
			"SuperAdmin@NAV.NO": true,
		},
	}
	if !cfg.IsAdmin("superadmin@nav.no") {
		t.Errorf("expected superadmin@nav.no to match SuperAdmin@NAV.NO")
	}
	if !cfg.IsAdmin("SUPERADMIN@NAV.NO") {
		t.Errorf("expected SUPERADMIN@NAV.NO to match SuperAdmin@NAV.NO")
	}
	if cfg.IsAdmin("other@nav.no") {
		t.Errorf("expected other@nav.no not to be admin")
	}
}

func TestGetDatabaseURL_FromHost(t *testing.T) {
	t.Setenv("DB_URL", "")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USERNAME", "dbuser")
	t.Setenv("DB_PASSWORD", "dbpass")
	t.Setenv("DB_DATABASE", "mydb")

	got := getDatabaseURL()
	want := "postgres://dbuser:dbpass@127.0.0.1:5433/mydb?sslmode=disable"
	if got != want {
		t.Errorf("getDatabaseURL() = %q, want %q", got, want)
	}
}

func TestGetDatabaseURL_SpecialCharactersInPassword(t *testing.T) {
	t.Setenv("DB_URL", "")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USERNAME", "dbuser")
	t.Setenv("DB_PASSWORD", "p@ss:word/!#")
	t.Setenv("DB_DATABASE", "mydb")

	got := getDatabaseURL()
	want := "postgres://dbuser:p%40ss%3Aword%2F%21%23@127.0.0.1:5432/mydb?sslmode=disable"
	if got != want {
		t.Errorf("getDatabaseURL() = %q, want %q", got, want)
	}
}
