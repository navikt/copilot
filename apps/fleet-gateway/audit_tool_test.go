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

// auditPost sends body through the gateway and returns the audit entry and the
// body upstream received.
func auditPost(t *testing.T, body string) (map[string]any, []byte, int) {
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
	gwToken := te.obtainAccessTokenForResource(te.resource)

	req := httptest.NewRequest(http.MethodPost, "/mcp/etterlevelse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gwToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)

	var audit map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		var e map[string]any
		if json.Unmarshal([]byte(line), &e) == nil && e["msg"] == "audit mcp" {
			audit = e
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
	if !strings.HasPrefix(tool, "evil") || len(tool) > maxAuditMethodLen {
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

func TestExtractJSONRPCAudit_ExactCaseKeys(t *testing.T) {
	for _, body := range []string{
		`{"method":"tools/call","params":{"name":"evil","Name":"benign"}}`,
		`{"method":"tools/call","params":{"Name":"benign","name":"evil"}}`,
	} {
		if _, tool := extractJSONRPCAudit([]byte(body)); tool != "evil" {
			t.Fatalf("%s: got tool %q, want evil", body, tool)
		}
	}
	if m, _ := extractJSONRPCAudit([]byte(`{"Method":"tools/call"}`)); m != "" {
		t.Fatalf("Method must not match method, got %q", m)
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
	if err := json.Unmarshal([]byte(tool), &names); err != nil || len(names) != 1 || names[0] != `a","b` {
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
