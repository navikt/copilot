package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

const devRegistry = "https://mcp-registry.ekstern.dev.nav.no"

// fallbackEnv answers the policy with policy and each registry by host:
// answer(host) gives the status, 0 for a connect timeout.
func fallbackEnv(t *testing.T, policy string, answer func(host string) int) (asked *[]string, stderr func() string) {
	t.Helper()
	mcpEnv(t, "", testRegistry(), errMCPRegistryNotRead) // isolated HOME and state; no read yet
	offline(t, func(r *http.Request) (*http.Response, error) {
		*asked = append(*asked, r.URL.Host)
		code := answer(r.URL.Host)
		if code == 0 {
			return nil, context.DeadlineExceeded
		}
		body := `{"servers":[{"server":{"name":"io.github.navikt/x","remotes":[{"type":"streamable-http","url":"https://x.nav.no/mcp"}]}}],"metadata":{}}`
		return &http.Response{StatusCode: code, Status: http.StatusText(code), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	asked = new([]string)
	fetchMCPPolicy = func() (string, error) { return policy, nil }

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = prev })
	return asked, func() string {
		w.Close()
		os.Stderr = prev
		var b bytes.Buffer
		_, _ = io.Copy(&b, r)
		return b.String()
	}
}

func TestMCPRegistryFallsBackWhenPolicyRegistryIsUnreachable(t *testing.T) {
	for _, code := range []int{0, 503} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			_, stderr := fallbackEnv(t, devRegistry, func(host string) int {
				if strings.Contains(host, "ekstern.dev") {
					return code
				}
				return 200
			})
			if err := refreshMCPRegistry(false); err != nil {
				t.Fatal(err)
			}
			if out := stderr(); !strings.Contains(out, "The policy registry "+devRegistry+" did not answer, so nav-pilot used "+agentpakke.MCPRegistryURL) {
				t.Fatalf("notice = %q", out)
			}
			c, ok := readMCPRegistryCache()
			if !ok || !c.Fallback || c.Registry.URL != agentpakke.MCPRegistryURL || c.Policy != devRegistry {
				t.Fatalf("cache = %+v", c)
			}
			if !MCPRegistryFallback() {
				t.Fatal("MCPRegistryFallback() = false")
			}
			// A fallback answer stands an hour, not a day.
			c.At = time.Now().Add(-2 * time.Hour)
			if c.fresh() {
				t.Fatal("a 2 h old fallback answer is fresh")
			}
			c.Fallback = false
			if !c.fresh() {
				t.Fatal("a 2 h old policy answer is stale")
			}
		})
	}
}

func TestMCPRegistryFallbackNoticeOncePerPolicy(t *testing.T) {
	_, stderr := fallbackEnv(t, devRegistry, func(host string) int {
		if strings.Contains(host, "ekstern.dev") {
			return 0
		}
		return 200
	})
	for range 2 {
		fetchMCPRegistry = memoMCPRegistry(askMCPRegistry)
		if err := refreshMCPRegistry(false); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(stderr(), "did not answer"); n != 1 {
		t.Fatalf("notice said %d times, want 1", n)
	}
}

func TestMCPRegistryNoFallback(t *testing.T) {
	for name, tc := range map[string]struct {
		policy string
		code   int
	}{
		"404":            {devRegistry, 404},
		"not Nav's":      {"https://registry.example", 0},
		"reachable":      {devRegistry, 200},
		"invalid answer": {devRegistry, -1},
	} {
		t.Run(name, func(t *testing.T) {
			asked, stderr := fallbackEnv(t, tc.policy, func(host string) int {
				if host == "mcp-registry.nav.no" {
					return 200
				}
				return tc.code
			})
			if tc.code == -1 {
				prev := mcpHTTPClient.Transport
				mcpHTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
					*asked = append(*asked, r.URL.Host)
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("<html>"))}, nil
				})
				t.Cleanup(func() { mcpHTTPClient.Transport = prev })
			}
			err := refreshMCPRegistry(false)
			if out := stderr(); strings.Contains(out, "did not answer, so") {
				t.Fatalf("fell back: %q", out)
			}
			for _, h := range *asked {
				if h == "mcp-registry.nav.no" {
					t.Fatalf("asked the prod registry: %v", *asked)
				}
			}
			if tc.code == 200 {
				c, ok := readMCPRegistryCache()
				if err != nil || !ok || c.Fallback || c.Registry.URL != tc.policy {
					t.Fatalf("err = %v, cache = %+v", err, c)
				}
			} else if err == nil {
				t.Fatal("no error")
			}
		})
	}
}

func TestMCPHTTPClientBoundsDialAndHandshake(t *testing.T) {
	tr := newMCPHTTPClient().Transport.(*http.Transport)
	if tr.TLSHandshakeTimeout < MCPPolicyTimeout || tr.DialContext == nil {
		t.Fatalf("handshake %v, dial set %v", tr.TLSHandshakeTimeout, tr.DialContext != nil)
	}
}
