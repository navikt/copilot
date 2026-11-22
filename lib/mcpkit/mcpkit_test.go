package mcpkit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func texasWith(t *testing.T, status int, claims map[string]any) *Texas {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if r.URL.Path == "/exchange" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "obo"})
			return
		}
		_ = json.NewEncoder(w).Encode(claims)
	}))
	t.Cleanup(ts.Close)
	return &Texas{IntrospectionEndpoint: ts.URL + "/introspect", ExchangeEndpoint: ts.URL + "/exchange", GatewayClientID: "gw", HTTPClient: ts.Client()}
}

func TestIntrospect(t *testing.T) {
	user := func(mod func(map[string]any)) map[string]any {
		c := map[string]any{"active": true, "idtyp": "user", "NAVident": "Z1", "azp": "gw"}
		mod(c)
		return c
	}
	for _, tc := range []struct {
		name   string
		status int
		claims map[string]any
		want   error
	}{
		{"ok", 200, user(func(map[string]any) {}), nil},
		{"inactive", 200, map[string]any{"active": false}, ErrUnauthorized},
		{"app idtyp", 200, user(func(c map[string]any) { c["idtyp"] = "APP" }), ErrForbidden},
		{"app role", 200, user(func(c map[string]any) { c["roles"] = []string{"access_as_application"} }), ErrForbidden},
		{"no NAVident", 200, user(func(c map[string]any) { c["NAVident"] = " " }), ErrForbidden},
		{"wrong azp", 200, user(func(c map[string]any) { c["azp"] = "gw2" }), ErrForbidden},
		{"no azp", 200, user(func(c map[string]any) { delete(c, "azp") }), ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ident, err := texasWith(t, tc.status, tc.claims).Introspect(context.Background(), "tok")
			if !errors.Is(err, tc.want) || (tc.want == nil && ident != "Z1") {
				t.Fatalf("got %q, %v; want %v", ident, err, tc.want)
			}
		})
	}

	_, err := texasWith(t, 500, nil).Introspect(context.Background(), "tok")
	if err == nil || errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) {
		t.Fatalf("texas 500 must be a plain error (502), got %v", err)
	}
	for _, tx := range []*Texas{{GatewayClientID: "gw"}, {IntrospectionEndpoint: "http://x"}} {
		if _, err := tx.Introspect(context.Background(), "tok"); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("unconfigured: %v", err)
		}
	}
}

func TestExchange(t *testing.T) {
	tx := texasWith(t, 200, nil)
	if tok, err := tx.Exchange(context.Background(), "u", "api://x/.default"); err != nil || tok != "obo" {
		t.Fatalf("exchange: %q %v", tok, err)
	}
	var ee *ExchangeError
	if _, err := tx.Exchange(context.Background(), "u", ""); !errors.As(err, &ee) {
		t.Fatalf("empty target must fail closed: %v", err)
	}
	if _, err := texasWith(t, 403, nil).Exchange(context.Background(), "secret-user-token", "t"); !errors.As(err, &ee) || ee.StatusCode != 403 || strings.Contains(err.Error(), "secret") {
		t.Fatalf("403: %v", err)
	}
}

func TestBearerToken(t *testing.T) {
	for in, ok := range map[string]bool{
		"Bearer abc": true,
		"bearer abc": true,
		"Basic abc":  false,
		"Bearer ":    false,
		"Bearer a b": false,
		"Bearer " + strings.Repeat("a", MaxBearerLen): false,
	} {
		if _, got := BearerToken(in); got != ok {
			t.Errorf("%.20q: got %v", in, got)
		}
	}
}

func TestGatewayClientID(t *testing.T) {
	raw := `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"a"},{"name":"prod-gcp:copilot:fleet-gateway","clientId":"b"}]`
	if id, err := GatewayClientID(raw, "dev-gcp"); err != nil || id != "a" {
		t.Fatalf("got %q %v", id, err)
	}
	if id, _ := GatewayClientID(raw, ""); id != "" {
		t.Fatal("no cluster must give no client id")
	}
	if _, err := GatewayClientID("{", "dev-gcp"); err == nil {
		t.Fatal("malformed JSON must fail")
	}
}

func TestValidateHTTPURL(t *testing.T) {
	for in, ok := range map[string]bool{"https://a.b": true, "http://a:80/x": true, "ftp://a": false, "https://": false, "/rel": false} {
		if err := ValidateHTTPURL(in); (err == nil) != ok {
			t.Errorf("%q: %v", in, err)
		}
	}
}

type countingReader struct{ n int }

func (c *countingReader) Read(p []byte) (int, error) {
	c.n += len(p)
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

func TestReadRequestBodyStopsAtLimit(t *testing.T) {
	src := &countingReader{}
	r := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(src))
	_, tooLarge, err := ReadRequestBody(httptest.NewRecorder(), r, 1024)
	if !tooLarge || err == nil {
		t.Fatalf("tooLarge=%v err=%v", tooLarge, err)
	}
	if src.n > 64*1024 {
		t.Fatalf("read %d bytes of an endless body", src.n)
	}
	if _, err := ReadLimited(strings.NewReader("abcd"), 3); err == nil {
		t.Fatal("ReadLimited must reject 4 bytes over a 3-byte limit")
	}
}

func TestValidateModernRequest(t *testing.T) {
	env := Envelope{Method: "tools/call"}
	env.Params.Name = "list"
	env.Params.Meta = map[string]json.RawMessage{
		MetaProtocolVersion:    json.RawMessage(`"2026-07-28"`),
		MetaClientCapabilities: json.RawMessage(`{}`),
	}
	req := func(h map[string]string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		for k, v := range h {
			r.Header.Set(k, v)
		}
		return r
	}
	ok := map[string]string{"MCP-Protocol-Version": ModernVersion, "Mcp-Method": "tools/call", "Mcp-Name": "=?base64?bGlzdA==?="}
	if e := ValidateModernRequest(req(ok), env, time.Now()); e != nil {
		t.Fatalf("valid request: %+v", e)
	}
	for k, v := range map[string]string{"Mcp-Name": "other", "Mcp-Method": "tools/list", "MCP-Protocol-Version": "2099-01-01"} {
		h := map[string]string{}
		for a, b := range ok {
			h[a] = b
		}
		h[k] = v
		if e := ValidateModernRequest(req(h), env, time.Now()); e == nil || (e.Code != CodeHeaderMismatch && e.Code != CodeUnsupportedVersion) {
			t.Errorf("%s=%s: %+v", k, v, e)
		}
	}
}

func TestToolMetricsBoundedLabels(t *testing.T) {
	m := NewToolMetrics([]string{"a", "b"})
	m.Observe("a", false, 20*time.Millisecond)
	m.Observe("a", true, 2*time.Second)
	m.Observe("Bearer secret-token", false, time.Millisecond)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out := rec.Body.String()
	for _, want := range []string{
		`mcp_tool_requests_total{tool="a"} 2`,
		`mcp_tool_errors_total{tool="a"} 1`,
		`mcp_tool_requests_total{tool="unknown"} 1`,
		`mcp_tool_duration_seconds_bucket{tool="a",le="0.025"} 1`,
		`mcp_tool_duration_seconds_bucket{tool="a",le="+Inf"} 2`,
		`mcp_tool_duration_seconds_count{tool="b"} 0`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out, "secret") {
		t.Fatal("caller-supplied tool name leaked into a label")
	}
}
