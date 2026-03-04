package main

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"x-auth-token":        true,
	"proxy-authorization": true,
}

func obfuscateHeaders(headers http.Header) map[string]string {
	obfuscated := make(map[string]string)
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		if sensitiveHeaders[lowerKey] {
			obfuscated[key] = "[REDACTED]"
		} else {
			obfuscated[key] = strings.Join(values, ", ")
		}
	}
	return obfuscated
}

func loggingMiddleware(config *Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loggingEnabled := config.LoggedEndpoints[r.URL.Path]
		start := time.Now()

		if loggingEnabled {
			slog.Debug("Request received",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"headers", obfuscateHeaders(r.Header),
			)
		}

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next(wrapped, r)

		duration := time.Since(start)

		if r.URL.Path != "/health" && r.URL.Path != "/ready" {
			recordHTTPMetrics(r.Method, r.URL.Path, wrapped.statusCode, duration)
		}

		if loggingEnabled {
			slog.Info("Request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		}
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
