package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// flatAudit maps the AuthZEN-shaped audit record onto flat keys so tests can
// assert single fields; TestAuditMCP_AuthZENShape pins the real shape.
func flatAudit(e map[string]any) map[string]any {
	get := func(path ...string) any {
		var v any = e
		for _, k := range path {
			m, _ := v.(map[string]any)
			v = m[k]
		}
		return v
	}
	decision := "deny"
	if get("decision", "decision") == true {
		decision = "allow"
	}
	return map[string]any{
		"navident":        get("subject", "id"),
		"client_id":       get("subject", "properties", "client_id"),
		"method":          get("action", "name"),
		"tool":            get("action", "properties", "tool"),
		"route":           "/mcp/" + fmt.Sprint(get("resource", "id")),
		"decision":        decision,
		"decision_reason": get("decision", "context", "reason"),
		"header_mismatch": get("context", "header_mismatch"),
		"outcome":         get("context", "outcome"),
		"duration_ms":     get("context", "duration_ms"),
		"raw":             e,
	}
}

// auditPost sends body through the gateway and returns the audit entry and the
// body upstream received.
func auditPost(t *testing.T, body string) (map[string]any, []byte, int) {
	t.Helper()
	return auditPostTools(t, nil, body)
}

// auditPostTools is auditPost on a route whose tool allowlist is tools.
func auditPostTools(t *testing.T, tools map[string]bool, body string) (map[string]any, []byte, int) {
	t.Helper()
	return auditTools(t, http.MethodPost, tools, body)
}

func auditTools(t *testing.T, method string, tools map[string]bool, body string) (map[string]any, []byte, int) {
	t.Helper()
	return auditToolsHeaders(t, method, tools, body, nil)
}

func auditToolsHeaders(t *testing.T, method string, tools map[string]bool, body string, hdrs http.Header) (map[string]any, []byte, int) {
	t.Helper()
	var logBuf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(old) })

	te := newTestEnv(t)
	var (
		mu  sync.Mutex
		got []byte
	)
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = b
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	})
	te.gw.cfg.Routes[0].allowedTools = tools
	te.handler = te.gw.routes()
	gwToken := te.obtainAccessTokenForResource(te.resource)

	req := httptest.NewRequest(method, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gwToken)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdrs {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)

	var audit map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		var e map[string]any
		if json.Unmarshal([]byte(line), &e) == nil && e["msg"] == "audit mcp" {
			audit = flatAudit(e)
		}
	}
	if audit == nil {
		t.Fatalf("no audit entry in logs:\n%s", logBuf.String())
	}
	if strings.Contains(logBuf.String(), "top-secret-arg") {
		t.Fatalf("tool arguments leaked into logs:\n%s", logBuf.String())
	}
	mu.Lock()
	defer mu.Unlock()
	return audit, got, rec.Code
}

func TestAuditMCP_LogsToolName_ForwardsBodyUnchanged(t *testing.T) {
	// Odd whitespace and key order must survive: upstream gets the exact bytes.
	body := "{ \"jsonrpc\":\"2.0\",\"id\":7,\n \"params\":{\"arguments\":{\"q\":\"top-secret-arg\"},\"name\":\"search_docs\"},\"method\":\"tools/call\"}  "
	audit, got, code := auditPost(t, body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if audit["method"] != "tools/call" || audit["tool"] != "search_docs" {
		t.Fatalf("expected method=tools/call tool=search_docs, got %v / %v", audit["method"], audit["tool"])
	}
	if string(got) != body {
		t.Fatalf("upstream body changed:\n got %q\nwant %q", got, body)
	}
}

func TestAuditMCP_ToolName_LogInjection(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"evil\n{\"msg\":\"audit mcp\",\"navident\":\"X999999\"}\r","arguments":{"a":"top-secret-arg"}}}`
	audit, got, _ := auditPost(t, body)
	tool, _ := audit["tool"].(string)
	if strings.ContainsAny(tool, "\n\r") {
		t.Fatalf("tool contains line breaks: %q", tool)
	}
	if !strings.HasPrefix(tool, `"evil\n{`) {
		t.Fatalf("unexpected sanitised tool %q", tool)
	}
	if audit["navident"] != "Z123456" {
		t.Fatalf("navident overwritten: %v", audit["navident"])
	}
	if string(got) != body {
		t.Fatal("upstream body changed")
	}
}

func TestAuditMCP_OversizedBody_NoToolLogged(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"big","arguments":{"x":"` +
		strings.Repeat("a", maxMCPRequestBodyBytes) + `"}}}`
	audit, got, code := auditPost(t, body)
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", code)
	}
	if audit["tool"] != "" || got != nil {
		t.Fatalf("oversized body must not be parsed or forwarded: tool=%v forwarded=%d bytes", audit["tool"], len(got))
	}
}

func TestAuditMCP_Batch(t *testing.T) {
	var parts []string
	for i := 0; i < maxAuditBatchTools+2; i++ {
		parts = append(parts, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t","arguments":{"a":"top-secret-arg"}}}`)
	}
	parts = append(parts, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	body := "[" + strings.Join(parts, ",") + "]"
	audit, got, _ := auditPost(t, body)
	if audit["method"] != "batch" {
		t.Fatalf("expected method=batch, got %v", audit["method"])
	}
	want := `[` + strings.Repeat(`"t",`, maxAuditBatchTools) + `"..."]`
	if audit["tool"] != want {
		t.Fatalf("expected tool=%q, got %v", want, audit["tool"])
	}
	if string(got) != body {
		t.Fatal("upstream body changed")
	}
}

func TestExtractJSONRPCAudit_NonToolCallHasNoTool(t *testing.T) {
	m, tool := extractJSONRPCAudit([]byte(`{"method":"tools/list","params":{"name":"x"}}`))
	if m != "tools/list" || tool != "" {
		t.Fatalf("got %q %q", m, tool)
	}
}

func TestExtractJSONRPCAudit_BadBatchMemberSkipped(t *testing.T) {
	m, tool := extractJSONRPCAudit([]byte(`[42,{"method":"tools/call","params":"x"},{"method":"tools/call","params":{"name":"a"}},"s",{"method":"tools/call","params":{"name":"b"}}]`))
	if m != "batch" || tool != `["","a","b"]` {
		t.Fatalf("got %q %q", m, tool)
	}
}

func TestExtractJSONRPCAudit_BadParamsKeepMethod(t *testing.T) {
	for _, body := range []string{
		`{"method":"tools/call","params":[1]}`,
		`{"method":"tools/call","params":{"name":7}}`,
	} {
		if m, tool := extractJSONRPCAudit([]byte(body)); m != "tools/call" || tool != "" {
			t.Fatalf("%s: got %q %q", body, m, tool)
		}
	}
}

func TestExtractJSONRPCAudit_AmbiguousKeysIncomplete(t *testing.T) {
	inc := `["` + auditIncomplete + `"]`
	for _, tc := range []struct{ body, method, tool string }{
		{`{"method":"tools/call","params":{"name":"evil","Name":"benign"}}`, "tools/call", auditIncomplete},
		{`{"method":"tools/call","params":{"Name":"benign","name":"evil"}}`, "tools/call", auditIncomplete},
		{`{"method":"tools/call","params":{"name":"a","name":"b"}}`, "tools/call", auditIncomplete},
		{`{"Method":"tools/call","Params":{"Name":"x"}}`, "", auditIncomplete},
		{`{"method":"ping","METHOD":"tools/call","params":{"name":"x"}}`, "", auditIncomplete},
		{`{"method":"ping","method":"tools/call","params":{"name":"x"}}`, "", auditIncomplete},
		{`{"method":"tools/call","params":{"name":"x"},"PARAMS":{"name":"y"}}`, "", auditIncomplete},
		{`{"method":"tools/call","params":{"name":"x"},"params":{"name":"y"}}`, "", auditIncomplete},
		{`{"method":"ping"} {"method":"tools/call","params":{"name":"x"}}`, "", auditIncomplete},
		{`{"method":"ping"}x`, "", auditIncomplete},
		{`[{"Method":"tools/call","Params":{"Name":"x"}}]`, "batch", inc},
		{`[{"method":"tools/call","params":{"name":"a","NAME":"b"}}]`, "batch", inc},
		{`[{"method":"ping","method":"tools/call"}]`, "batch", inc},
		{`[{"method":"ping"}] [{"method":"tools/call","params":{"name":"x"}}]`, "batch", inc},
		{"{\"method\":\"tools/call\",\"params\":{\"name\":\"ok\"}}\n", "tools/call", "ok"},
	} {
		if m, tool := extractJSONRPCAudit([]byte(tc.body)); m != tc.method || tool != tc.tool {
			t.Errorf("%s: got %q %q, want %q %q", tc.body, m, tool, tc.method, tc.tool)
		}
	}
}

func TestExtractJSONRPCAudit_BatchEscapedMethod(t *testing.T) {
	for _, body := range []string{
		`[{"method":"tools\/call","params":{"name":"hidden"}}]`,
		`[{"method":"tools/call","params":{"n\u0061me":"hidden"}}]`,
		`[{"method":"tools/call","params":{"name":"hidden"}}]`,
	} {
		if m, tool := extractJSONRPCAudit([]byte(body)); m != "batch" || tool != `["hidden"]` {
			t.Fatalf("%s: got %q %q", body, m, tool)
		}
	}
}

func TestExtractJSONRPCAudit_BatchNameCannotForgeEntries(t *testing.T) {
	_, tool := extractJSONRPCAudit([]byte(`[{"method":"tools/call","params":{"name":"a\",\"b"}}]`))
	var names []string
	if err := json.Unmarshal([]byte(tool), &names); err != nil || len(names) != 1 || names[0] != `"a\",\"b"` {
		t.Fatalf("got %q", tool)
	}
}

// peakHeapDuring runs f and returns the peak heap above the level before f,
// sampled concurrently. GOGC=1 collects garbage almost at once, so the peak is
// close to what f holds live rather than to GC pacing.
func peakHeapDuring(f func()) uint64 {
	defer debug.SetGCPercent(debug.SetGCPercent(1))
	sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	read := func() uint64 { metrics.Read(sample); return sample[0].Value.Uint64() }
	runtime.GC()
	base := read()
	var peak atomic.Uint64
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			if v := read(); v > peak.Load() {
				peak.Store(v)
			}
			select {
			case <-done:
				return
			default:
			}
		}
	}()
	f()
	close(done)
	<-stopped
	if p := peak.Load(); p > base {
		return p - base
	}
	return 0
}

func TestExtractJSONRPCAudit_BoundedHeap(t *testing.T) {
	fill := func(open, member, sep, end string) []byte {
		n := (maxMCPRequestBodyBytes - len(open) - len(end)) / (len(member) + len(sep))
		return []byte(open + strings.Repeat(member+sep, n-1) + member + end)
	}
	keys := make([]string, 50000)
	for i := range keys {
		keys[i] = fmt.Sprintf(`"k%d":0`, i)
	}
	cases := map[string][]byte{
		"empty objects":  fill("[", "{}", ",", "]"),
		"m tools/call":   fill("[", `{"m":"tools/call"}`, ",", "]"),
		"tools/call":     fill("[", `{"method":"tools/call","params":{"name":"t"}}`, ",", "]"),
		"50k keys":       []byte(`{"method":"tools/call",` + strings.Join(keys, ",") + `}`),
		"50k param keys": []byte(`{"method":"tools/call","params":{` + strings.Join(keys, ",") + `}}`),
		"huge string":    []byte(`{"method":"tools/call","params":{"name":"` + strings.Repeat("a", maxMCPRequestBodyBytes-60) + `"}}`),
		"deep nesting":   fill("[", "[", "", "]"),
		"deep in object": []byte(`{"method":"x","a":` + strings.Repeat("[", maxMCPRequestBodyBytes-40) + `}`),
	}
	for name, body := range cases {
		if len(body) > maxMCPRequestBodyBytes {
			t.Fatalf("%s: body %d bytes over cap", name, len(body))
		}
		peak := peakHeapDuring(func() { extractJSONRPCAudit(body) })
		t.Logf("%s: peak heap +%.1f MiB", name, float64(peak)/(1<<20))
		if peak > 8<<20 {
			t.Errorf("%s: peak heap +%d MiB for a %d-byte body, want < 8 MiB", name, peak>>20, len(body))
		}
	}
}

func TestExtractJSONRPCAudit_IncompleteScanIsMarked(t *testing.T) {
	call := `{"method":"tools/call","params":{"name":"hidden"}}`
	for name, body := range map[string]string{
		"skip depth cap": `[` + strings.Repeat("[", 65) + strings.Repeat("]", 65) + `,` + call + `]`,
		"decoder depth":  `[{"a":` + strings.Repeat("[", 10001) + strings.Repeat("]", 10001) + `},` + call + `]`,
		"syntax error":   `[{"method":"tools/call","params":{"name":"a"}} x]`,
	} {
		if m, tool := extractJSONRPCAudit([]byte(body)); m != "batch" || !strings.Contains(tool, `"`+auditIncomplete+`"`) {
			t.Errorf("%s: got %q %q", name, m, tool)
		}
	}
	deep := `{"a":` + strings.Repeat("[", 10001) + strings.Repeat("]", 10001) + `,"method":"tools/call","params":{"name":"hidden"}}`
	if _, tool := extractJSONRPCAudit([]byte(deep)); tool != auditIncomplete {
		t.Errorf("single object: got %q", tool)
	}
	for name, body := range map[string]string{"BOM object": "\xef\xbb\xbf" + call, "BOM batch": "\xef\xbb\xbf[" + call + `]`} {
		if m, tool := extractJSONRPCAudit([]byte(body)); m != "" || tool != auditIncomplete {
			t.Errorf("%s: got %q %q", name, m, tool)
		}
	}
	if m, tool := extractJSONRPCAudit(nil); m != "" || tool != "" {
		t.Errorf("empty body: got %q %q", m, tool)
	}
	if _, tool := extractJSONRPCAudit([]byte(`[` + call + `,[[1]]]`)); tool != `["hidden"]` {
		t.Errorf("complete batch: got %q", tool)
	}
}

func TestAuditMCP_MarksUnusualMethodAndTool(t *testing.T) {
	for _, tc := range []struct{ body, method, tool string }{
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whoami "}}`, "tools/call", `"whoami "`},
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whoami\u0000"}}`, "tools/call", `"whoami\x00"`},
		{`{"jsonrpc":"2.0","id":1,"method":"tools/\u200bcall"}`, `"tools/\u200bcall"`, ""},
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call\ufeff"}`, `"tools/call\ufeff"`, ""},
	} {
		audit, _, _ := auditPostTools(t, map[string]bool{"whoami": true}, tc.body)
		if audit["method"] != tc.method || audit["tool"] != tc.tool {
			t.Errorf("%s: got method %v tool %v, want %s / %s", tc.body, audit["method"], audit["tool"], tc.method, tc.tool)
		}
	}
}

func TestAuditMCP_HeaderMismatch(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search"}}`
	for _, tc := range []struct {
		hdrs http.Header
		want bool
	}{
		{http.Header{}, false},
		{http.Header{"Mcp-Method": {"tools/call"}, "Mcp-Name": {"search"}}, false},
		{http.Header{"Mcp-Name": {"=?base64?c2VhcmNo?="}}, false},
		{http.Header{"Mcp-Method": {"tools/list"}}, true},
		{http.Header{"Mcp-Name": {"delete"}}, true},
	} {
		audit, _, code := auditToolsHeaders(t, http.MethodPost, nil, body, tc.hdrs)
		if code != http.StatusOK || audit["header_mismatch"] != tc.want {
			t.Errorf("%v: got code %d header_mismatch %v, want %v", tc.hdrs, code, audit["header_mismatch"], tc.want)
		}
	}
}

func TestAuditMCP_AuthZENShape(t *testing.T) {
	audit, _, _ := auditToolsHeaders(t, http.MethodPost, map[string]bool{"search": true},
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete","arguments":{"a":"top-secret-arg"}}}`,
		http.Header{"Mcp-Name": {"search"}})
	raw := audit["raw"].(map[string]any)
	ctx := raw["context"].(map[string]any)
	if id, _ := ctx["request_id"].(string); len(id) != 26 {
		t.Errorf("request_id %q, want 26 base32 characters", id)
	}
	ctx["request_id"], ctx["duration_ms"] = "ID", 0
	delete(raw, "time")
	got, _ := json.Marshal(raw)
	want := `{"action":{"name":"tools/call","properties":{"tool":"delete"}},` +
		`"context":{"duration_ms":0,"header_mismatch":true,"http_method":"POST","outcome":"tool_denied","request_id":"ID","scan_incomplete":false},` +
		`"decision":{"context":{"reason":"not_listed"},"decision":false},"level":"INFO","msg":"audit mcp",` +
		`"resource":{"id":"team-etterlevelse/etterlevelse","properties":{"backend":"dev-gcp:team-etterlevelse:nav-etterlevelse-mcp"},"type":"mcp_route"},` +
		`"subject":{"id":"Z123456","properties":{"client_id":"` + fmt.Sprint(audit["client_id"]) + `"},"type":"user"}}`
	if audit["client_id"] == "" || string(got) != want {
		t.Errorf("audit record\n got %s\nwant %s", got, want)
	}
}

func TestMCPHeadersMismatch_ComparesRawValues(t *testing.T) {
	long := strings.Repeat("a", 70)
	call := func(name string) string {
		return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":` + name + `}}`
	}
	for _, tc := range []struct {
		body, name string
		want       bool
	}{
		{call(`"søk"`), "=?base64?c8O4aw==?=", false},
		{call(`"` + long + `"`), long, false},
		{call(`"` + long + `"`), long + "b", true},
		{call(`"whoami\u0000"`), `"whoami\x00"`, true},
		{call(`"whoami "`), `"whoami "`, true},
		{call(`"...incomplete"`), "other", true},
		{call(`"...incomplete"`), "...incomplete", false},
	} {
		a := scanJSONRPCAudit([]byte(tc.body))
		h := http.Header{"Mcp-Method": {"tools/call"}, "Mcp-Name": {tc.name}}
		if got := mcpHeadersMismatch(h, a); got != tc.want {
			t.Errorf("%s vs %q: mismatch %v, want %v", tc.body, tc.name, got, tc.want)
		}
	}
}

func TestScanJSONRPCAudit_IncompleteIsNotAToolName(t *testing.T) {
	if a := scanJSONRPCAudit([]byte(`{"method":"tools/call","params":{"name":"...incomplete"}}`)); a.incomplete {
		t.Fatalf("tool named ...incomplete set incomplete: %+v", a)
	}
	if a := scanJSONRPCAudit([]byte(`[{"method":"tools/call","params":{"name":"...incomplete"}}]`)); a.incomplete {
		t.Fatalf("batch tool named ...incomplete set incomplete: %+v", a)
	}
	for _, body := range []string{`{"method":"ping"}x`, `[{"method":"tools/call","params":{"name":"a","name":"b"}}]`, "\ufeff{}"} {
		if a := scanJSONRPCAudit([]byte(body)); !a.incomplete {
			t.Errorf("%q: want incomplete, got %+v", body, a)
		}
	}
}

func TestAuditField_QuotesQuoteAndBackslash(t *testing.T) {
	literal, nul := auditField(`"whoami\x00"`), auditField("whoami\x00")
	if literal == nul {
		t.Fatalf("literal and NUL names both log as %s", nul)
	}
	if got := auditField(`a\b`); got != `"a\\b"` {
		t.Errorf("backslash: got %s", got)
	}
}

func TestAuditMCP_UnauthenticatedHasNoSubject(t *testing.T) {
	audit, _, code := auditToolsHeaders(t, http.MethodPost, nil, `{"method":"ping"}`,
		http.Header{"Authorization": {"Bearer not-a-token"}})
	raw := audit["raw"].(map[string]any)
	if code != http.StatusUnauthorized {
		t.Fatalf("code %d, want 401", code)
	}
	if _, ok := raw["subject"]; ok {
		t.Errorf("unauthenticated record has subject %v; AuthZEN requires subject.id", raw["subject"])
	}
}

func TestAuditMCP_ScanIncompleteFromScan(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{`{"method":"tools/call","params":{"name":"...incomplete"}}`, false},
		{`{"method":"ping"}x`, true},
	} {
		audit, _, _ := auditPost(t, tc.body)
		ctx := audit["raw"].(map[string]any)["context"].(map[string]any)
		if ctx["scan_incomplete"] != tc.want {
			t.Errorf("%s: scan_incomplete %v, want %v", tc.body, ctx["scan_incomplete"], tc.want)
		}
	}
}
