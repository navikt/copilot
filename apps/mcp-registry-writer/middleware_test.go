package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestObfuscateHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer sensitive-secret-token")
	headers.Set("Cookie", "session=12345")
	headers.Set("User-Agent", "nav-pilot/1.0")

	obfuscated := obfuscateHeaders(headers)

	if obfuscated["Authorization"] != "[REDACTED]" {
		t.Errorf("expected Authorization to be [REDACTED], got %q", obfuscated["Authorization"])
	}
	if obfuscated["Cookie"] != "[REDACTED]" {
		t.Errorf("expected Cookie to be [REDACTED], got %q", obfuscated["Cookie"])
	}
	if obfuscated["User-Agent"] != "nav-pilot/1.0" {
		t.Errorf("expected User-Agent to be preserved, got %q", obfuscated["User-Agent"])
	}
}

func TestLoggingMiddleware(t *testing.T) {
	cfg := loadConfig()
	cfg.LoggedEndpoints["/test"] = true

	handler := loggingMiddleware(cfg, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}
