package provider

import (
	"bytes"
	"context"
	"crypto/x509"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
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
	for name, code := range map[string]int{"connect timeout": 0, "503": 503} {
		t.Run(name, func(t *testing.T) {
			_, stderr := fallbackEnv(t, devRegistry+"/", func(host string) int {
				if strings.Contains(host, "ekstern.dev") {
					return code
				}
				return 200
			})
			if err := refreshMCPRegistry(false); err != nil {
				t.Fatal(err)
			}
			if out := stderr(); !strings.Contains(out, "The policy registry "+devRegistry+"/ did not answer, so nav-pilot used "+agentpakke.MCPRegistryURL) {
				t.Fatalf("notice = %q", out)
			}
			c, ok := readMCPRegistryCache()
			if !ok || !c.Fallback || c.Registry.URL != agentpakke.MCPRegistryURL || c.Policy != devRegistry+"/" {
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

// Policy is prod and prod is down: no second ask, the error stands.
func TestMCPRegistryNoFallbackFromProdToProd(t *testing.T) {
	asked, stderr := fallbackEnv(t, agentpakke.MCPRegistryURL+"/", func(string) int { return 503 })
	if err := refreshMCPRegistry(false); err == nil {
		t.Fatal("no error")
	}
	if out := stderr(); strings.Contains(out, "did not answer, so") || len(*asked) != 1 {
		t.Fatalf("asked %v, said %q", *asked, out)
	}
}

// A certificate that does not verify may be a MITM: it fails, never falls back.
func TestMCPRegistryBadCertificateDoesNotFallBack(t *testing.T) {
	asked, stderr := fallbackEnv(t, devRegistry, func(string) int { return 200 })
	mcpHTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*asked = append(*asked, r.URL.Host)
		return nil, &url.Error{Op: "Get", URL: r.URL.String(), Err: x509.UnknownAuthorityError{}}
	})
	if err := refreshMCPRegistry(false); err == nil {
		t.Fatal("no error")
	}
	if out := stderr(); strings.Contains(out, "did not answer, so") || len(*asked) != 1 {
		t.Fatalf("asked %v, said %q", *asked, out)
	}
}

// A 5xx on a later page is as unreachable as one on the first.
func TestMCPRegistryFallsBackOn5xxOnPage2(t *testing.T) {
	_, stderr := fallbackEnv(t, devRegistry, nil)
	mcpHTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		page := `{"servers":[{"server":{"name":"io.github.navikt/x","remotes":[{"type":"streamable-http","url":"https://x.nav.no/mcp"}]}}],"metadata":{"nextCursor":"2"}}`
		if r.URL.Host == "mcp-registry.nav.no" {
			page = strings.Replace(page, `"nextCursor":"2"`, "", 1)
		} else if r.URL.Query().Get("cursor") == "2" {
			return &http.Response{StatusCode: 502, Status: "502", Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(page))}, nil
	})
	if err := refreshMCPRegistry(false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr(), "did not answer, so") || !MCPRegistryFallback() {
		t.Fatal("no fallback")
	}
}

// The background refresh at launch never prints: it would write over the session.
func TestMCPRegistryBackgroundRefreshIsQuiet(t *testing.T) {
	_, stderr := fallbackEnv(t, devRegistry, func(host string) int {
		if strings.Contains(host, "ekstern.dev") {
			return 0
		}
		return 200
	})
	startMCPRegistryRefresh()
	mcpRefreshing.Wait()
	if out := stderr(); out != "" {
		t.Fatalf("background refresh said %q", out)
	}
	if !MCPRegistryFallback() {
		t.Fatal("background refresh did not cache the fallback")
	}
}

// An OpenCode launch checks its servers against the fallback, not "not checked".
func TestOpenCodeLaunchUsesTheFallback(t *testing.T) {
	_, stderr := fallbackEnv(t, devRegistry, func(host string) int {
		if strings.Contains(host, "ekstern.dev") {
			return 0
		}
		return 200
	})
	proj := t.TempDir()
	_ = os.Mkdir(filepath.Join(proj, ".git"), 0o700)
	_ = os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{"mcp": {"x": {"type": "remote", "url": "https://x.nav.no/mcp"}, "bad": {"type": "remote", "url": "https://bad/mcp"}}}`), 0o600)
	env := strings.Join(applyOpenCodeMCPPolicy(nil, proj), "\n")
	if out := stderr(); strings.Contains(out, "were not checked") {
		t.Fatalf("said %q", out)
	}
	if !strings.Contains(env, `"blocked":["bad"]`) {
		t.Fatalf("env = %s", env)
	}
}

// Prod fills {{domain_internal}} with intern.nav.no, dev with
// intern.dev.nav.no: the fallback approves dev's host, as the policy does.
func TestMCPRegistryFallbackKeepsDevHosts(t *testing.T) {
	_, stderr := fallbackEnv(t, devRegistry, nil)
	mcpHTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "mcp-registry.nav.no" {
			return nil, context.DeadlineExceeded
		}
		body := `{"servers":[{"server":{"name":"io.github.navikt/mcp-onboarding","remotes":[{"type":"streamable-http","url":"https://mcp-onboarding.intern.nav.no/mcp"}]}},
			{"server":{"name":"io.github.navikt/aksel","remotes":[{"type":"streamable-http","url":"https://aksel-mcp.nav.no/mcp"}]}}],"metadata":{}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	if err := refreshMCPRegistry(false); err != nil {
		t.Fatal(err)
	}
	stderr()
	c, _ := readMCPRegistryCache()
	if got := c.Registry.Servers["io.github.navikt/mcp-onboarding"]; len(got) != 1 || got[0] != "https://mcp-onboarding.intern.dev.nav.no/mcp" {
		t.Fatalf("servers = %v", c.Registry.Servers)
	}
	if !c.Registry.Remotes["https://mcp-onboarding.intern.dev.nav.no/mcp"] || c.Registry.Remotes["https://mcp-onboarding.intern.nav.no/mcp"] || !c.Registry.Remotes["https://aksel-mcp.nav.no/mcp"] {
		t.Fatalf("remotes = %v", c.Registry.Remotes)
	}
	if u := c.Registry.Entries[0].Remotes[0].URL; u != "https://mcp-onboarding.intern.dev.nav.no/mcp" {
		t.Fatalf("entry remote = %s", u)
	}
}

// asDevRegistry maps {{domain_internal}} only. A literal intern.nav.no URL
// or a {{domain_external}} template would make dev and prod differ in a way
// it cannot undo: extend it first.
func TestAllowlistTemplatesAreMapped(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "apps", "mcp-registry", "allowlist.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "{{domain_external}}") {
		t.Fatal("allowlist.json uses {{domain_external}}, which asDevRegistry does not map")
	}
	if m := prodInternalHost.Find(data); m != nil {
		t.Fatalf("allowlist.json has a literal intern.nav.no URL (%s), which asDevRegistry would rewrite", m)
	}
}

// After a quiet background read cached a fallback, the next foreground read
// of the fresh cache says it.
func TestMCPRegistryFallbackSaidFromCache(t *testing.T) {
	_, stderr := fallbackEnv(t, devRegistry, func(host string) int {
		if strings.Contains(host, "ekstern.dev") {
			return 0
		}
		return 200
	})
	if err := refreshMCPRegistry(true); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := MCPRegistryServers(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr(), "did not answer, so") {
		t.Fatal("no notice from the cached fallback")
	}
}

// A 4xx whose body is cut off is still an answer: no fallback.
func TestMCPRegistry4xxWithBrokenBodyDoesNotFallBack(t *testing.T) {
	asked, stderr := fallbackEnv(t, devRegistry, nil)
	mcpHTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*asked = append(*asked, r.URL.Host)
		return &http.Response{StatusCode: 401, Status: "401", Body: io.NopCloser(iotest.ErrReader(io.ErrUnexpectedEOF))}, nil
	})
	if err := refreshMCPRegistry(false); err == nil {
		t.Fatal("no error")
	}
	if out := stderr(); strings.Contains(out, "did not answer, so") || len(*asked) != 1 {
		t.Fatalf("asked %v, said %q", *asked, out)
	}
}
