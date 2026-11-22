package main

// Golden test of mcp-kodeverk's HTTP behaviour. It was recorded before the
// shared code moved to lib/mcpkit, so a diff here is a behaviour change.
// Re-record only on purpose: go test -run TestHTTPGolden -update

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/http_golden.json")

const goldenFile = "testdata/http_golden.json"

// goldenTexas answers introspection by token, so one server covers every auth outcome.
func goldenTexas(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token     string `json:"token"`
		UserToken string `json:"user_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	claims := map[string]any{"active": true, "idtyp": "user", "NAVident": "Z123456", "azp": "gw-client-123"}
	switch r.URL.Path {
	case "/introspect":
		switch in.Token {
		case "inactive":
			claims = map[string]any{"active": false}
		case "app":
			claims["idtyp"] = "app"
		case "approle":
			claims["roles"] = []string{"access_as_application"}
		case "noident":
			delete(claims, "NAVident")
		case "wrongazp":
			claims["azp"] = "someone-else"
		case "texas500":
			w.WriteHeader(http.StatusInternalServerError)
			return
		case "garbage":
			_, _ = io.WriteString(w, "{not json")
			return
		}
		writeJSON(w, http.StatusOK, claims)
	case "/exchange":
		if in.UserToken == "obofail" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "obo-tok"})
	default:
		http.NotFound(w, r)
	}
}

type goldenCase struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Header  map[string]string `json:"header,omitempty"`
	Body    string            `json:"body,omitempty"`
	Status  int               `json:"status"`
	RespHdr map[string]string `json:"respHeader"`
	Resp    json.RawMessage   `json:"resp,omitempty"`
	RawResp string            `json:"rawResp,omitempty"`
}

func goldenCases() []goldenCase {
	auth := func(tok string) map[string]string { return map[string]string{"Authorization": "Bearer " + tok} }
	modern := func(tok, method, name string) map[string]string {
		h := auth(tok)
		h["MCP-Protocol-Version"] = mcpModernVersion
		h["Mcp-Method"] = method
		if name != "" {
			h["Mcp-Name"] = name
		}
		return h
	}
	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	mbody := func(method, params string) string {
		if params != "" {
			params += ","
		}
		return `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":{` + params + meta + `}}`
	}
	call := func(tool, args string) string {
		return mbody("tools/call", `"name":"`+tool+`","arguments":`+args)
	}
	return []goldenCase{
		{Name: "health", Method: "GET", Path: "/health"},
		{Name: "ready", Method: "GET", Path: "/ready"},
		{Name: "no auth", Method: "POST", Path: "/", Body: mbody("tools/list", "")},
		{Name: "basic auth", Method: "POST", Path: "/", Header: map[string]string{"Authorization": "Basic abc"}},
		{Name: "bearer with space", Method: "POST", Path: "/", Header: map[string]string{"Authorization": "Bearer a b"}},
		{Name: "bearer too long", Method: "POST", Path: "/", Header: auth(strings.Repeat("a", 16*1024))},
		{Name: "inactive token", Method: "POST", Path: "/", Header: auth("inactive")},
		{Name: "app token", Method: "POST", Path: "/", Header: auth("app")},
		{Name: "app role", Method: "POST", Path: "/", Header: auth("approle")},
		{Name: "no NAVident", Method: "POST", Path: "/", Header: auth("noident")},
		{Name: "wrong azp", Method: "POST", Path: "/", Header: auth("wrongazp")},
		{Name: "texas 500", Method: "POST", Path: "/", Header: auth("texas500")},
		{Name: "texas garbage", Method: "POST", Path: "/", Header: auth("garbage")},
		{Name: "GET mcp", Method: "GET", Path: "/", Header: auth("ok")},
		{Name: "DELETE mcp", Method: "DELETE", Path: "/", Header: auth("ok")},
		{Name: "other path", Method: "POST", Path: "/mcp", Header: modern("ok", "tools/list", ""), Body: mbody("tools/list", "")},
		{Name: "too large", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"x":"` + strings.Repeat("a", 1<<20) + `"}`},
		{Name: "empty body", Method: "POST", Path: "/", Header: auth("ok")},
		{Name: "array body", Method: "POST", Path: "/", Header: auth("ok"), Body: `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`},
		{Name: "null id", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","id":null,"method":"tools/list"}`},
		{Name: "missing id", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","method":"tools/list"}`},
		{Name: "no method", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","id":7}`},
		{Name: "bad params", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","id":7,"method":"tools/list","params":[1]}`},
		{Name: "modern discover", Method: "POST", Path: "/", Header: modern("ok", "server/discover", ""), Body: mbody("server/discover", "")},
		{Name: "modern tools/list", Method: "POST", Path: "/", Header: modern("ok", "tools/list", ""), Body: mbody("tools/list", "")},
		{Name: "modern tools/call", Method: "POST", Path: "/", Header: modern("ok", "tools/call", "list_kodeverk"), Body: call("list_kodeverk", "{}")},
		{Name: "modern base64 name", Method: "POST", Path: "/", Header: modern("ok", "tools/call", "=?base64?bGlzdF9rb2RldmVyaw==?="), Body: call("list_kodeverk", "{}")},
		{Name: "modern unknown tool", Method: "POST", Path: "/", Header: modern("ok", "tools/call", "nope"), Body: call("nope", "{}")},
		{Name: "modern bad args", Method: "POST", Path: "/", Header: modern("ok", "tools/call", "list_kodeverk"), Body: call("list_kodeverk", `{"x":1}`)},
		{Name: "modern obo fail", Method: "POST", Path: "/", Header: modern("obofail", "tools/call", "list_kodeverk"), Body: call("list_kodeverk", "{}")},
		{Name: "modern unknown method", Method: "POST", Path: "/", Header: modern("ok", "ping", ""), Body: mbody("ping", "")},
		{Name: "modern notification", Method: "POST", Path: "/", Header: modern("ok", "notifications/initialized", ""), Body: `{"jsonrpc":"2.0","method":"notifications/initialized","params":{` + meta + `}}`},
		{Name: "modern method mismatch", Method: "POST", Path: "/", Header: modern("ok", "tools/call", ""), Body: mbody("tools/list", "")},
		{Name: "modern name mismatch", Method: "POST", Path: "/", Header: modern("ok", "tools/call", "other"), Body: call("list_kodeverk", "{}")},
		{Name: "modern bad base64", Method: "POST", Path: "/", Header: modern("ok", "=?base64?!!?=", ""), Body: mbody("tools/list", "")},
		{Name: "modern missing header", Method: "POST", Path: "/", Header: auth("ok"), Body: mbody("tools/list", "")},
		{Name: "modern unsupported header", Method: "POST", Path: "/", Header: map[string]string{"Authorization": "Bearer ok", "MCP-Protocol-Version": "2099-01-01", "Mcp-Method": "tools/list"}, Body: mbody("tools/list", "")},
		{Name: "modern missing caps", Method: "POST", Path: "/", Header: modern("ok", "tools/list", ""), Body: `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`},
		{Name: "legacy initialize", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`},
		{Name: "legacy initialize unknown version", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","id":"a","method":"initialize","params":{"protocolVersion":"1999-01-01"}}`},
		{Name: "legacy ping", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","id":2,"method":"ping"}`},
		{Name: "legacy notification", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","method":"notifications/initialized"}`},
		{Name: "legacy tools/list", Method: "POST", Path: "/", Header: map[string]string{"Authorization": "Bearer ok", "MCP-Protocol-Version": "2025-06-18"}, Body: `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`},
		{Name: "legacy tools/call", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_kodeverk","arguments":{}}}`},
		{Name: "legacy unknown method", Method: "POST", Path: "/", Header: auth("ok"), Body: `{"jsonrpc":"2.0","id":5,"method":"resources/list"}`},
	}
}

func runGoldenCase(t *testing.T, h http.Handler, c goldenCase) goldenCase {
	t.Helper()
	req := httptest.NewRequest(c.Method, c.Path, strings.NewReader(c.Body))
	for k, v := range c.Header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	c.Status = rec.Code
	c.RespHdr = map[string]string{}
	for _, k := range []string{"Content-Type", "Cache-Control", "Allow", "WWW-Authenticate"} {
		if v := rec.Header().Get(k); v != "" {
			c.RespHdr[k] = v
		}
	}
	body := rec.Body.Bytes()
	if json.Valid(body) {
		c.Resp = json.RawMessage(strings.TrimSpace(string(body)))
	} else {
		c.RawResp = string(body)
	}
	if len(c.Body) > 4096 {
		c.Body = "<" + c.Name + ">" // keep the golden file small; the case list regenerates it
	}
	if v := c.Header["Authorization"]; len(v) > 4096 {
		c.Header = map[string]string{"Authorization": "<long>"}
	}
	return c
}

func TestHTTPGolden(t *testing.T) {
	t.Setenv(mcpLegacyUntilEnv, "2999-01-01")
	srv := newTestServer(t, goldenTexas, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/kodeverk" {
			writeJSON(w, http.StatusOK, map[string]any{"kodeverksnavn": []string{"Landkoder", "Postnummer"}})
			return
		}
		http.NotFound(w, r)
	})
	h := srv.routes()
	var got []goldenCase
	for _, c := range goldenCases() {
		got = append(got, runGoldenCase(t, h, c))
	}
	out, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, '\n')
	if *updateGolden {
		if err = os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(goldenFile, out, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("read golden file (record with -update): %v", err)
	}
	var wantCases []goldenCase
	if err := json.Unmarshal(want, &wantCases); err != nil {
		t.Fatal(err)
	}
	if len(wantCases) != len(got) {
		t.Fatalf("golden has %d cases, test has %d", len(wantCases), len(got))
	}
	for i := range got {
		g, _ := json.Marshal(got[i])
		w, _ := json.Marshal(wantCases[i])
		if string(g) != string(w) {
			t.Errorf("case %q changed:\n got: %s\nwant: %s", got[i].Name, g, w)
		}
	}
}
