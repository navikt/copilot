package main

// MCP 2026-07-28 compliance table, shared verbatim with
// apps/mcp-onboarding/mcp2026_compliance_test.go. Each row is a MUST/SHOULD from
// https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http#request-metadata,
// .../basic/versioning and .../server/discover.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mcpComplianceTarget struct {
	handler  http.Handler
	tool     string            // a tool callable with empty arguments
	header   map[string]string // extra headers, e.g. Authorization
	checkGET bool              // whether GET must answer 405 outside the legacy window
}

func mcpCall(t *testing.T, tgt mcpComplianceTarget, httpMethod string, body map[string]any, hdr map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(httpMethod, "/mcp", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range tgt.header {
		req.Header.Set(k, v)
	}
	for k, v := range hdr {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	rec := httptest.NewRecorder()
	tgt.handler.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func modernBody(method string, params map[string]any) map[string]any {
	if params == nil {
		params = map[string]any{}
	}
	params["_meta"] = map[string]any{
		mcpMetaProtocolVersion:                       mcpModernVersion,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "compliance-test", "version": "0"},
	}
	return map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
}

func modernHeaders(method, name string) map[string]string {
	h := map[string]string{"MCP-Protocol-Version": mcpModernVersion, "Mcp-Method": method}
	if name != "" {
		h["Mcp-Name"] = name
	}
	return h
}

func with(h map[string]string, k, v string) map[string]string {
	out := map[string]string{}
	for kk, vv := range h {
		out[kk] = vv
	}
	out[k] = v
	return out
}

func errCode(out map[string]any) int {
	e, _ := out["error"].(map[string]any)
	c, _ := e["code"].(float64)
	return int(c)
}

func runMCP2026Compliance(t *testing.T, tgt mcpComplianceTarget) {
	callParams := func() map[string]any { return map[string]any{"name": tgt.tool, "arguments": map[string]any{}} }
	cases := []struct {
		name     string
		legacy   string // MCP_LEGACY_PROTOCOL_UNTIL
		body     map[string]any
		hdr      map[string]string
		status   int
		code     int
		check    func(t *testing.T, result map[string]any)
		httpVerb string
	}{
		{name: "server/discover answers with supportedVersions", body: modernBody("server/discover", nil), hdr: modernHeaders("server/discover", ""), status: 200,
			check: func(t *testing.T, r map[string]any) {
				v, _ := r["supportedVersions"].([]any)
				if len(v) != 1 || v[0] != mcpModernVersion {
					t.Errorf("supportedVersions = %v, want only %s", v, mcpModernVersion)
				}
				if r["capabilities"] == nil || r["cacheScope"] == nil || r["ttlMs"] == nil {
					t.Errorf("discover result lacks capabilities/ttlMs/cacheScope: %v", r)
				}
			}},
		{name: "legacy window lists legacy versions in discover", legacy: "2999-01-01", body: modernBody("server/discover", nil), hdr: modernHeaders("server/discover", ""), status: 200,
			check: func(t *testing.T, r map[string]any) {
				if v, _ := r["supportedVersions"].([]any); len(v) < 2 {
					t.Errorf("supportedVersions = %v, want legacy versions too", v)
				}
			}},
		{name: "tools/list carries ttlMs and cacheScope", body: modernBody("tools/list", nil), hdr: modernHeaders("tools/list", ""), status: 200,
			check: func(t *testing.T, r map[string]any) {
				tools, _ := r["tools"].([]any)
				if len(tools) == 0 || r["ttlMs"] == nil || r["cacheScope"] == nil {
					t.Errorf("tools/list result incomplete: %v", r)
				}
				for _, tl := range tools {
					if m, _ := tl.(map[string]any); m["annotations"] == nil {
						t.Errorf("tool %v has no annotations", m["name"])
					}
				}
			}},
		{name: "tools/call with matching Mcp-Name", body: modernBody("tools/call", callParams()), hdr: modernHeaders("tools/call", tgt.tool), status: 200},
		{name: "Mcp-Session-Id is ignored", body: modernBody("server/discover", nil), hdr: with(modernHeaders("server/discover", ""), "Mcp-Session-Id", "stale"), status: 200},
		{name: "missing MCP-Protocol-Version", body: modernBody("tools/list", nil), hdr: with(modernHeaders("tools/list", ""), "MCP-Protocol-Version", ""), status: 400, code: mcpCodeHeaderMismatch},
		{name: "unsupported MCP-Protocol-Version", body: modernBody("tools/list", nil), hdr: with(modernHeaders("tools/list", ""), "MCP-Protocol-Version", "2099-01-01"), status: 400, code: mcpCodeUnsupportedVersion},
		{name: "header version without _meta version", body: map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, hdr: modernHeaders("tools/list", ""), status: 400, code: mcpCodeHeaderMismatch},
		{name: "Mcp-Method mismatch", body: modernBody("tools/list", nil), hdr: modernHeaders("server/discover", ""), status: 400, code: mcpCodeHeaderMismatch},
		{name: "Mcp-Method missing", body: modernBody("tools/list", nil), hdr: with(modernHeaders("tools/list", ""), "Mcp-Method", ""), status: 400, code: mcpCodeHeaderMismatch},
		{name: "Mcp-Name mismatch", body: modernBody("tools/call", callParams()), hdr: modernHeaders("tools/call", "other_tool"), status: 400, code: mcpCodeHeaderMismatch},
		{name: "Mcp-Name missing", body: modernBody("tools/call", callParams()), hdr: modernHeaders("tools/call", ""), status: 400, code: mcpCodeHeaderMismatch},
		{name: "unknown method is 404", body: modernBody("nope/nope", nil), hdr: modernHeaders("nope/nope", ""), status: 404, code: mcpCodeMethodNotFound},
		{name: "ping is removed", body: modernBody("ping", nil), hdr: modernHeaders("ping", ""), status: 404, code: mcpCodeMethodNotFound},
		{name: "initialize refused outside legacy window", body: map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-06-18"}}, status: 400, code: mcpCodeUnsupportedVersion},
		{name: "initialize refused after legacy window closed", legacy: "2000-01-01", body: map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"}, status: 400, code: mcpCodeUnsupportedVersion},
		{name: "initialize served inside legacy window", legacy: "2999-01-01", body: map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-06-18"}}, status: 200},
	}
	if tgt.checkGET {
		cases = append(cases, struct {
			name     string
			legacy   string
			body     map[string]any
			hdr      map[string]string
			status   int
			code     int
			check    func(t *testing.T, result map[string]any)
			httpVerb string
		}{name: "GET stream removed", httpVerb: http.MethodGet, status: 405})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(mcpLegacyUntilEnv, tc.legacy)
			verb := tc.httpVerb
			if verb == "" {
				verb = http.MethodPost
			}
			rec, out := mcpCall(t, tgt, verb, tc.body, tc.hdr)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.code != 0 && errCode(out) != tc.code {
				t.Fatalf("error code = %d, want %d; body %s", errCode(out), tc.code, rec.Body.String())
			}
			if tc.code == mcpCodeUnsupportedVersion {
				e, _ := out["error"].(map[string]any)
				d, _ := e["data"].(map[string]any)
				if d["supported"] == nil {
					t.Errorf("UnsupportedProtocolVersion lacks data.supported: %s", rec.Body.String())
				}
			}
			if tc.status == 200 && tc.legacy == "" {
				r, _ := out["result"].(map[string]any)
				if r["resultType"] != "complete" {
					t.Errorf("resultType = %v, want complete", r["resultType"])
				}
				meta, _ := r["_meta"].(map[string]any)
				if meta[mcpMetaServerInfo] == nil {
					t.Errorf("result _meta lacks %s", mcpMetaServerInfo)
				}
				if tc.check != nil {
					tc.check(t, r)
				}
			} else if tc.check != nil && tc.status == 200 {
				r, _ := out["result"].(map[string]any)
				tc.check(t, r)
			}
		})
	}
}
