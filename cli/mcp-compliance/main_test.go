package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

const (
	userToken = "user-token-secret" // #nosec G101 -- test fixture
	appToken  = "app-token-secret"
)

// fakeServer follows the request order in apps/mcp-kodeverk/main.go mcpHandler.
// Each field switches one rule off so a single check fails.
type fakeServer struct {
	acceptMissingToken, acceptInvalidToken, acceptAppToken bool
	noDiscover, noVersionCheck, noMethodCheck, noNameCheck bool
	noEnvelopeCheck, allowGET, noSizeLimit                 bool
}

func (f fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch auth := r.Header.Get("Authorization"); {
	case auth == "" && !f.acceptMissingToken, auth == "Bearer "+invalidToken && !f.acceptInvalidToken:
		w.WriteHeader(401)
		return
	case auth == "Bearer "+appToken && !f.acceptAppToken:
		w.WriteHeader(403)
		return
	}
	if r.Method != http.MethodPost && (!f.allowGET || r.Method != http.MethodGet) {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(405)
		return
	}
	limit := int64(1 << 20)
	if f.noSizeLimit {
		limit = 1 << 30
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		w.WriteHeader(413)
		return
	}
	var env struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			Name string         `json:"name"`
			Meta map[string]any `json:"_meta"`
		} `json:"params"`
	}
	_ = json.Unmarshal(body, &env)
	rpcErr := func(code int) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "error": map[string]any{"code": code}})
	}
	if !f.noEnvelopeCheck && (env.Method == "" || len(env.ID) == 0 || bytes.Equal(env.ID, []byte("null"))) {
		rpcErr(codeInvalidRequest)
		return
	}
	if v := r.Header.Get("MCP-Protocol-Version"); !f.noVersionCheck {
		switch {
		case v == "" || env.Params.Meta[metaProtocolVersion] != v && v == modernVersion:
			rpcErr(codeHeaderMismatch)
			return
		case v != modernVersion:
			rpcErr(codeUnsupportedVer)
			return
		}
	}
	if !f.noMethodCheck && r.Header.Get("Mcp-Method") != env.Method {
		rpcErr(codeHeaderMismatch)
		return
	}
	if !f.noNameCheck && env.Method == "tools/call" && r.Header.Get("Mcp-Name") != env.Params.Name {
		rpcErr(codeHeaderMismatch)
		return
	}
	result := map[string]any{"resultType": "complete", "_meta": map[string]any{metaServerInfo: map[string]any{"name": "fake"}}}
	switch env.Method {
	case "server/discover":
		if !f.noDiscover {
			result["supportedVersions"] = []any{modernVersion}
		}
		result["capabilities"], result["ttlMs"], result["cacheScope"] = map[string]any{}, 0, "public"
	case "tools/list":
		result["tools"] = []any{map[string]any{"name": "ping_tool"}}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
}

func runAgainst(t *testing.T, f fakeServer) (int, string) {
	t.Helper()
	srv := httptest.NewServer(f)
	defer srv.Close()
	env := map[string]string{envToken: userToken, envAppToken: appToken}
	var out bytes.Buffer
	code := run([]string{"-url", srv.URL}, &out, func(k string) string { return env[k] })
	if s := out.String(); strings.Contains(s, userToken) || strings.Contains(s, appToken) {
		t.Fatalf("output contains a token:\n%s", s)
	}
	return code, out.String()
}

func TestCompliantServerPasses(t *testing.T) {
	code, out := runAgainst(t, fakeServer{})
	if code != 0 || strings.Contains(out, "FAIL") {
		t.Fatalf("exit %d, want 0:\n%s", code, out)
	}
}

func TestEachBrokenRuleFails(t *testing.T) {
	cases := []struct {
		srv   fakeServer
		check string
	}{
		{fakeServer{acceptMissingToken: true}, "missing token is 401"},
		{fakeServer{acceptInvalidToken: true}, "invalid token is 401"},
		{fakeServer{acceptAppToken: true}, "app token is 403"},
		{fakeServer{noDiscover: true}, "server/discover lists"},
		{fakeServer{noVersionCheck: true}, "missing MCP-Protocol-Version is -32020"},
		{fakeServer{noVersionCheck: true}, "unsupported MCP-Protocol-Version is -32022"},
		{fakeServer{noMethodCheck: true}, "Mcp-Method mismatch is -32020"},
		{fakeServer{noNameCheck: true}, "Mcp-Name mismatch is -32020"},
		{fakeServer{noEnvelopeCheck: true}, "missing method is -32600"},
		{fakeServer{noEnvelopeCheck: true}, "null id is -32600"},
		{fakeServer{allowGET: true}, "GET is 405"},
		{fakeServer{noSizeLimit: true}, "is 413"},
	}
	for _, tc := range cases {
		t.Run(tc.check, func(t *testing.T) {
			code, out := runAgainst(t, tc.srv)
			if code != 1 {
				t.Fatalf("exit %d, want 1:\n%s", code, out)
			}
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, tc.check) {
					if !strings.HasPrefix(line, "FAIL") {
						t.Fatalf("check %q not FAIL:\n%s", tc.check, out)
					}
					return
				}
			}
			t.Fatalf("check %q missing:\n%s", tc.check, out)
		})
	}
}

func TestAppTokenUnsetSkips(t *testing.T) {
	srv := httptest.NewServer(fakeServer{})
	defer srv.Close()
	var out bytes.Buffer
	if code := run([]string{"-url", srv.URL}, &out, func(k string) string {
		if k == envToken {
			return userToken
		}
		return ""
	}); code != 0 || !regexp.MustCompile(`SKIP +app token is 403`).MatchString(out.String()) {
		t.Fatalf("exit %d:\n%s", code, out.String())
	}
}

func TestMissingURLIsUsageError(t *testing.T) {
	if code := run(nil, io.Discard, func(string) string { return "" }); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}
