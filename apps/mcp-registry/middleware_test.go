package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestObfuscateHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret-token")
	headers.Set("Cookie", "session=abc123")
	headers.Set("X-Api-Key", "api-key-123")
	headers.Set("X-Auth-Token", "auth-token-456")
	headers.Set("Content-Type", "application/json")
	headers.Set("User-Agent", "Test/1.0")

	obfuscated := obfuscateHeaders(headers)

	sensitiveHeaders := []string{"Authorization", "Cookie", "X-Api-Key", "X-Auth-Token"}
	for _, header := range sensitiveHeaders {
		if obfuscated[header] != "[REDACTED]" {
			t.Errorf("expected header %s to be [REDACTED], got %s", header, obfuscated[header])
		}
	}

	if obfuscated["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type to be preserved, got %s", obfuscated["Content-Type"])
	}

	if obfuscated["User-Agent"] != "Test/1.0" {
		t.Errorf("expected User-Agent to be preserved, got %s", obfuscated["User-Agent"])
	}
}

func TestObfuscateHeaders_CaseInsensitive(t *testing.T) {
	headers := http.Header{}
	headers.Set("authorization", "Bearer secret-token")
	headers.Set("AUTHORIZATION", "Bearer another-token")
	headers.Set("Authorization", "Bearer third-token")

	obfuscated := obfuscateHeaders(headers)

	for key, value := range obfuscated {
		if value != "[REDACTED]" {
			t.Errorf("expected header %s to be [REDACTED], got %s", key, value)
		}
	}
}

func TestLoggingMiddleware_EnabledEndpoint(t *testing.T) {
	config := &Config{
		Port:     "8080",
		LogLevel: 0,
		LoggedEndpoints: map[string]bool{
			"/test": true,
		},
	}

	called := false
	handler := LoggingMiddleware(config, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("expected handler to be called")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestLoggingMiddleware_DisabledEndpoint(t *testing.T) {
	config := &Config{
		Port:     "8080",
		LogLevel: 0,
		LoggedEndpoints: map[string]bool{
			"/other": true,
		},
	}

	called := false
	handler := LoggingMiddleware(config, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("expected handler to be called even when logging is disabled")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
