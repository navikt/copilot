package local

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateEndpoint(t *testing.T) {
	for raw, want := range map[string]string{
		"http://127.0.0.1:11434/v1":      "http://127.0.0.1:11434",
		"http://127.0.0.1:11434/v1/":     "http://127.0.0.1:11434",
		"http://localhost:8080":          "http://localhost:8080",
		"http://[::1]:8080/v1":           "http://[::1]:8080",
		"https://10.1.2.3/llm/v1":        "https://10.1.2.3/llm",
		"http://192.168.1.20:1234/v1":    "http://192.168.1.20:1234",
		"http://172.16.0.9:8000/v1":      "http://172.16.0.9:8000",
		" http://127.0.0.1:11434/v1 ":    "http://127.0.0.1:11434",
		"http://[fd00::1]:11434/v1":      "http://[fd00::1]:11434",
		"http://127.0.0.1:11434/foo/v1/": "http://127.0.0.1:11434/foo",
	} {
		got, err := ValidateEndpoint(raw)
		if err != nil || got != want {
			t.Errorf("ValidateEndpoint(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{
		"http://8.8.8.8:11434/v1",
		"https://api.example.com/v1",
		"http://gpu-box.lan:8080/v1", // a hostname can resolve anywhere later
		"http://169.254.169.254/v1",  // cloud metadata: link-local, not private
		"http://100.64.0.1/v1",       // CGNAT, not RFC 1918
		"http://0.0.0.0:11434/v1",
		"ftp://127.0.0.1/v1",
		"127.0.0.1:11434",
		"http://user:pw@127.0.0.1:11434/v1",
		"http://127.0.0.1:11434/v1?x=1",
		"",
	} {
		if got, err := ValidateEndpoint(raw); err == nil {
			t.Errorf("ValidateEndpoint(%q) = %q, want an error", raw, got)
		}
	}
}

// A redirect is not followed, and the dial-time check refuses a public
// address whatever the config said.
func TestServerClientRefusesPublicAddresses(t *testing.T) {
	srv := httptest.NewServer(http.RedirectHandler("http://8.8.8.8/v1/models", http.StatusFound))
	defer srv.Close()
	resp, err := ServerClient.Get(srv.URL)
	if err != nil || resp.StatusCode != http.StatusFound {
		t.Fatalf("redirect: %v, %v; want the 302 itself, not followed", resp, err)
	}
	resp.Body.Close()
	if _, err := ServerClient.Get("http://8.8.8.8:1/"); err == nil || !strings.Contains(err.Error(), "refused to connect to 8.8.8.8:1") {
		t.Fatalf("public address: err = %v, want a refusal", err)
	}
	if err := checkDial("tcp", "127.0.0.1:1", nil); err != nil {
		t.Errorf("loopback refused: %v", err)
	}
	if err := checkDial("tcp", "[2001:4860:4860::8888]:443", nil); err == nil {
		t.Error("public IPv6 allowed")
	}
}

// ServerClient must not go through a proxy: HTTPS_PROXY would otherwise carry
// a prompt for a private address off the network.
func TestServerClientIgnoresProxy(t *testing.T) {
	if serverTransport.Proxy != nil {
		t.Fatal("serverTransport has a Proxy")
	}
}

func withEndpoint(t *testing.T, base, model string) {
	t.Helper()
	SetEndpoint(base, model)
	t.Cleanup(func() { SetEndpoint("", "") })
}

// In endpoint mode nothing is recorded and nothing is owned: the proof is
// that the server answers, Acquire hands back the endpoint, and no state file
// is read.
func TestEndpointModeSkipsOwnership(t *testing.T) {
	dir := t.TempDir()
	orig := dataDir
	dataDir = func() string { return dir }
	t.Cleanup(func() { dataDir = orig })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen3.6:35b"}]}`))
	}))
	withEndpoint(t, srv.URL, "qwen3.6:35b")

	if err := EnsureOwnServer(); err != nil {
		t.Fatalf("EnsureOwnServer: %v", err)
	}
	if err := EnsureServerRunning(context.Background(), func(string) { t.Error("announced a start") }, nil); err != nil {
		t.Fatalf("EnsureServerRunning: %v", err)
	}
	url, model, release, err := Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
	if url != srv.URL || model != "qwen3.6:35b" || ServerURL() != srv.URL {
		t.Errorf("Acquire = %s %s, ServerURL = %s; want the endpoint", url, model, ServerURL())
	}
	if m, ok, _ := ServedModel(); !ok || m != "qwen3.6:35b" {
		t.Errorf("ServedModel = %q, %v", m, ok)
	}

	srv.Close()
	if err := EnsureOwnServer(); err == nil || !strings.Contains(err.Error(), "nav-pilot alpha local doctor") {
		t.Errorf("a stopped endpoint: err = %v, want the way to check it", err)
	}
}

func TestEndpointManifestIsUnmeasured(t *testing.T) {
	m := EndpointManifest("qwen3.6:35b")
	e, ok := Chosen(m)
	if !ok || e.Model != "qwen3.6:35b" || e.Capabilities != nil || e.Backend != BackendEndpoint {
		t.Fatalf("Chosen = %+v, %v", e, ok)
	}
	withEndpoint(t, "http://127.0.0.1:1", "qwen3.6:35b")
	if TelemetryModel("qwen3.6:35b") != "custom" || Backend() != "endpoint" {
		t.Error("an endpoint's model reached telemetry as typed")
	}
}

// The guard never hands a redirect to the client, whose own HTTP stack would
// follow it past the address checks.
func TestGuardRefusesRedirects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.RedirectHandler("http://8.8.8.8/v1/chat/completions", http.StatusTemporaryRedirect))
	defer srv.Close()
	g, err := StartGuard(srv.URL, Model{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	orig := ownershipCheck
	ownershipCheck = func() error { return nil }
	t.Cleanup(func() { ownershipCheck = orig })
	resp, err := http.Get(g.URL() + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway || resp.Header.Get("Location") != "" {
		t.Errorf("guard answered %s with Location %q, want 502 and none", resp.Status, resp.Header.Get("Location"))
	}
}
