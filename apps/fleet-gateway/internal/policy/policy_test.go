package policy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"strings"
	"sync/atomic"
	"testing"
)

// maxMCPRequestBodyBytes mirrors the gateway's request body cap, which bounds what these scans read.
const maxMCPRequestBodyBytes = 1 << 20

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

// peakLiveHeap runs f and returns the most heap any GC during f found live,
// above the level before f. GOGC=1 runs a GC almost every allocation step.
// It reads /gc/heap/live:bytes, which counts only what a GC marked, not
// /memory/classes/heap/objects:bytes, which also counts garbage not yet
// swept: in CI that made a transient 1 MiB decoder buffer read +8 MiB, and a
// sampler that had not yet been scheduled read +0.0 MiB (#2366). The sampler
// signals before f starts, and a GC plus a last read after f catches what f
// still holds when it returns.
//
// Scope: this measures retained live heap, not every transient allocation.
// /gc/heap/live:bytes moves only when a mark cycle completes, so one large
// buffer that f allocates and drops between cycles can read +0.0 MiB (seen
// for a filled 16 MiB slice at -cpu 1). The bound it guards is what the scan
// keeps reachable; the retained-buffer control below proves it sees that.
//
// Objects allocated while a GC is marking count as live for that cycle, so a
// busy machine can still add floating garbage to one run. That error only
// ever adds, while what f retains shows in every run, so the minimum of three
// runs is the estimate.
func peakLiveHeap(f func()) uint64 {
	m := peakLiveHeapOnce(f)
	for range 2 {
		m = min(m, peakLiveHeapOnce(f))
	}
	return m
}

func peakLiveHeapOnce(f func()) uint64 {
	defer debug.SetGCPercent(debug.SetGCPercent(1))
	sample := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	read := func() uint64 { metrics.Read(sample); return sample[0].Value.Uint64() }
	runtime.GC()
	base := read()
	var peak atomic.Uint64
	bump := func() {
		if v := read(); v > peak.Load() {
			peak.Store(v)
		}
	}
	started, done, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		close(started)
		for {
			bump()
			select {
			case <-done:
				return
			default:
				runtime.Gosched()
			}
		}
	}()
	<-started
	f()
	close(done)
	<-stopped
	runtime.GC()
	bump()
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
		peak := peakLiveHeap(func() { extractJSONRPCAudit(body) })
		t.Logf("%s: peak live heap +%.1f MiB", name, float64(peak)/(1<<20))
		if peak > 8<<20 {
			t.Errorf("%s: peak live heap +%d MiB for a %d-byte body, want < 8 MiB", name, peak>>20, len(body))
		}
	}
	// Control: decoding the whole body, as the scan did before #2309, holds
	// ~20 MiB live for {} members. If the helper cannot see that, it is blind.
	body := cases["empty objects"]
	if naive := peakLiveHeap(func() { var v []json.RawMessage; _ = json.Unmarshal(body, &v) }); naive <= 8<<20 {
		t.Errorf("control: naive Unmarshal peak live heap +%.1f MiB, want > 8 MiB; peakLiveHeap cannot detect the old behaviour", float64(naive)/(1<<20))
	}
	// Control: one 16 MiB buffer that is still reachable when f returns must
	// read over 8 MiB, so a single large retained allocation is not missed.
	// Each run keeps its own buffer: reusing one variable left the previous
	// run's buffer in the next run's baseline, so min-of-3 read +0.0 MiB.
	var kept [][]byte
	if big := peakLiveHeap(func() {
		buf := make([]byte, 16<<20)
		for i := range buf {
			buf[i] = 1
		}
		kept = append(kept, buf)
	}); big <= 8<<20 {
		t.Errorf("control: retained 16 MiB buffer read +%.1f MiB, want > 8 MiB", float64(big)/(1<<20))
	}
	runtime.KeepAlive(kept)
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

func TestAuditProtocolVersion(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "",
		"2025-06-18":            "2025-06-18",
		strings.Repeat("9", 40): strings.Repeat("9", 32),
		"2026-07-28 x":          `"2026-07-28 x"`,
	} {
		if got := ProtocolVersion(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestRPCMethodTool_MethodCaseIsAmbiguous(t *testing.T) {
	_, _, amb := rpcMethodTool(json.RawMessage(`"Tools/Call"`), json.RawMessage(`{"name":"x"}`))
	if !amb {
		t.Fatal("Tools/Call must be flagged ambiguous")
	}
}
