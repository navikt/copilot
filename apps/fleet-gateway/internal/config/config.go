// Package config holds fleet-gateway's environment parsing and URL validation
// helpers. The env var names, defaults and error texts are part of the startup
// contract: change them only on purpose.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ValidatePostgresURL checks a postgres DSN without echoing it in errors.
func ValidatePostgresURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return errors.New("empty URL")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		// url.Error embeds the raw URL, i.e. the DSN with its password.
		return errors.New("unparseable URL")
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" && u.Query().Get("host") == "" {
		return errors.New("missing host")
	}
	return nil
}

// ValidateHTTPURL accepts an http(s) URL with a host and no userinfo or fragment.
func ValidateHTTPURL(raw string, allowQuery bool) error {
	if raw == "" {
		return errors.New("empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("missing host")
	}
	if u.User != nil {
		return errors.New("userinfo not allowed in URL")
	}
	if u.Fragment != "" {
		return errors.New("fragment not allowed in URL")
	}
	if !allowQuery && u.RawQuery != "" {
		return errors.New("query string not allowed in URL")
	}
	return nil
}

// NewHardenedHTTPClient returns a client that refuses redirects.
func NewHardenedHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("redirects refused")
		},
	}
}

// CSVSet splits a comma-separated list into a set, dropping blanks.
func CSVSet(raw string) map[string]bool {
	out := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		item := strings.TrimSpace(part)
		if item != "" {
			out[item] = true
		}
	}
	return out
}

// LogLevel maps a level name to slog, defaulting to INFO.
func LogLevel(raw string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Get returns the trimmed env var, or fallback when empty.
func Get(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// Bool parses a boolean env var; empty means false.
func Bool(key string) (bool, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return b, nil
}

// Int parses a positive int env var, or returns fallback.
func Int(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// Int64 parses a positive int64 env var, or returns fallback.
func Int64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// Duration parses a positive duration env var, or returns fallback.
func Duration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
