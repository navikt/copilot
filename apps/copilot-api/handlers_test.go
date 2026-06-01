package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %q", rec.Body.String())
	}
}

func TestReadyHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	readyHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %q", rec.Body.String())
	}
}

func TestHealthHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
		t.Errorf("Expected Allow header %q, got %q", http.MethodGet, allow)
	}
}

func TestReadyHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	rec := httptest.NewRecorder()

	readyHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
		t.Errorf("Expected Allow header %q, got %q", http.MethodGet, allow)
	}
}

func TestNotImplementedHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec := httptest.NewRecorder()

	notImplementedHandler(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("Expected Content-Type 'application/problem+json', got %q", contentType)
	}
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		name       string
		errorType  string
		detail     string
		status     int
		wantStatus int
	}{
		{
			name:       "unauthorized",
			errorType:  "unauthorized",
			detail:     "Missing token",
			status:     http.StatusUnauthorized,
			wantStatus: 401,
		},
		{
			name:       "not found",
			errorType:  "not_found",
			detail:     "Resource not found",
			status:     http.StatusNotFound,
			wantStatus: 404,
		},
		{
			name:       "internal error",
			errorType:  "internal_error",
			detail:     "Something went wrong",
			status:     http.StatusInternalServerError,
			wantStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			respondError(rec, tt.errorType, tt.detail, tt.status)

			if rec.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rec.Code)
			}

			contentType := rec.Header().Get("Content-Type")
			if contentType != "application/problem+json" {
				t.Errorf("Expected Content-Type 'application/problem+json', got %q", contentType)
			}

			// Verify response body contains expected fields
			body := rec.Body.String()
			if !strings.Contains(body, tt.detail) {
				t.Errorf("Response body should contain detail %q, got %q", tt.detail, body)
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	config := &Config{
		LoggedEndpoints: map[string]bool{
			"/api/v1/": true,
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := loggingMiddleware(config, handler)

	tests := []struct {
		name       string
		path       string
		shouldLog  bool
		wantStatus int
	}{
		{
			name:       "logged endpoint",
			path:       "/api/v1/test",
			shouldLog:  true,
			wantStatus: 200,
		},
		{
			name:       "not logged endpoint",
			path:       "/health",
			shouldLog:  false,
			wantStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			middleware.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}
