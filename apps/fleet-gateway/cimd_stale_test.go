package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func wrapCIMDTransport(te *testEnv, fn func(*http.Request) (*http.Response, error)) {
	base := te.gw.cfg.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	te.gw.cfg.HTTPClient = &http.Client{
		Timeout:       te.gw.cfg.HTTPClient.Timeout,
		CheckRedirect: te.gw.cfg.HTTPClient.CheckRedirect,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == te.clientID {
				if resp, err := fn(r); resp != nil || err != nil {
					return resp, err
				}
			}
			return base.RoundTrip(r)
		}),
	}
}

func TestCIMDStaleFallback(t *testing.T) {
	const verifier = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	t.Run("serves stale document on transport, status, and decode failures within limit", func(t *testing.T) {
		var logBuf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
		t.Cleanup(func() { slog.SetDefault(prev) })

		var mu sync.Mutex
		mode := "ok"
		var status int
		var body string

		te := newTestEnv(t)
		wrapCIMDTransport(te, func(*http.Request) (*http.Response, error) {
			mu.Lock()
			m, st, b := mode, status, body
			mu.Unlock()
			switch m {
			case "transport":
				return nil, errors.New("dial tcp: connection refused")
			case "status":
				return &http.Response{
					StatusCode: st,
					Body:       http.NoBody,
					Header:     make(http.Header),
				}, nil
			case "raw":
				return &http.Response{
					StatusCode: st,
					Body:       io.NopCloser(strings.NewReader(b)),
					Header:     make(http.Header),
				}, nil
			default:
				return nil, nil
			}
		})

		// Initial valid fetch populates cache.
		doc, err := te.gw.fetchCIMD(context.Background(), te.clientID)
		if err != nil {
			t.Fatalf("initial fetchCIMD failed: %v", err)
		}
		if doc.ClientID != te.clientID {
			t.Fatalf("unexpected client_id %q", doc.ClientID)
		}

		cases := []struct {
			name   string
			mode   string
			status int
			body   string
		}{
			{name: "transport error", mode: "transport"},
			{name: "non-200 status", mode: "status", status: http.StatusServiceUnavailable},
			{name: "decode failure", mode: "raw", status: http.StatusOK, body: "{not-json"},
		}
		for _, tc := range cases {
			te.advance(defaultCIMDTTL + time.Minute)
			mu.Lock()
			mode, status, body = tc.mode, tc.status, tc.body
			mu.Unlock()

			got, err := te.gw.fetchCIMD(context.Background(), te.clientID)
			if err != nil {
				t.Fatalf("%s: expected stale fallback, got error: %v", tc.name, err)
			}
			if got.ClientID != te.clientID || len(got.RedirectURIs) != 1 || got.RedirectURIs[0] != te.redirectURI {
				t.Fatalf("%s: unexpected stale doc: %+v", tc.name, got)
			}

			// Verify /authorize succeeds using the stale document.
			req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusFound {
				t.Fatalf("%s: expected /authorize 302 with stale fallback, got %d (%s)", tc.name, rec.Code, rec.Body.String())
			}
		}

		if !strings.Contains(logBuf.String(), "CIMD fetch failed, serving stale document") {
			t.Fatalf("expected slog.Warn on stale fallback, got logs:\n%s", logBuf.String())
		}
	})

	t.Run("fails after stale max age and sweep evicts expired entry", func(t *testing.T) {
		var mu sync.Mutex
		failFetch := false

		te := newTestEnv(t)
		wrapCIMDTransport(te, func(*http.Request) (*http.Response, error) {
			mu.Lock()
			shouldFail := failFetch
			mu.Unlock()
			if shouldFail {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Body:       http.NoBody,
					Header:     make(http.Header),
				}, nil
			}
			return nil, nil
		})

		if _, err := te.gw.fetchCIMD(context.Background(), te.clientID); err != nil {
			t.Fatalf("initial fetchCIMD failed: %v", err)
		}

		mu.Lock()
		failFetch = true
		mu.Unlock()

		// Still within 24h after Sweep: entry is retained and usable.
		te.advance(cimdStaleMax - time.Minute)
		if removed := te.gw.cimdCache.Sweep(); removed != 0 {
			t.Fatalf("expected 0 swept before cimdStaleMax, got %d", removed)
		}
		if _, err := te.gw.fetchCIMD(context.Background(), te.clientID); err != nil {
			t.Fatalf("expected stale fallback before limit, got: %v", err)
		}

		// Reach 24h limit: /authorize and fetchCIMD must fail, and Sweep cleans up.
		te.advance(time.Minute)
		if _, err := te.gw.fetchCIMD(context.Background(), te.clientID); err == nil {
			t.Fatal("expected error after cimdStaleMax, got nil")
		}
		req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 after cimdStaleMax, got %d", rec.Code)
		}
		if te.gw.cimdCache.Len() != 0 {
			t.Fatalf("expected expired cache entry evicted, got Len=%d", te.gw.cimdCache.Len())
		}
	})

	t.Run("invalid 200 document does not fall back to stale", func(t *testing.T) {
		cases := []struct {
			name string
			body string
		}{
			{
				name: "client_id mismatch",
				body: `{"client_id":"https://evil.example/other.json","redirect_uris":["http://127.0.0.1:38249/callback"],"token_endpoint_auth_method":"none"}`,
			},
			{
				name: "bad token_endpoint_auth_method",
				body: `{"client_id":%q,"redirect_uris":["http://127.0.0.1:38249/callback"],"token_endpoint_auth_method":"client_secret_post"}`,
			},
			{
				name: "empty redirect_uris",
				body: `{"client_id":%q,"redirect_uris":[],"token_endpoint_auth_method":"none"}`,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var mu sync.Mutex
				serveInvalid := false

				te := newTestEnv(t)
				payload := strings.ReplaceAll(tc.body, "%q", `"`+te.clientID+`"`)
				wrapCIMDTransport(te, func(*http.Request) (*http.Response, error) {
					mu.Lock()
					inv := serveInvalid
					mu.Unlock()
					if inv {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(payload)),
							Header:     make(http.Header),
						}, nil
					}
					return nil, nil
				})

				if _, err := te.gw.fetchCIMD(context.Background(), te.clientID); err != nil {
					t.Fatalf("initial fetchCIMD failed: %v", err)
				}

				te.advance(defaultCIMDTTL + time.Minute)
				mu.Lock()
				serveInvalid = true
				mu.Unlock()

				if _, err := te.gw.fetchCIMD(context.Background(), te.clientID); err == nil {
					t.Fatalf("%s: expected validation error, got nil", tc.name)
				}
				req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
				rec := httptest.NewRecorder()
				te.handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("%s: expected 400 on invalid 200 document, got %d", tc.name, rec.Code)
				}
			})
		}
	})
}
