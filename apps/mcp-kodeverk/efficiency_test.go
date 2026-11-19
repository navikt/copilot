package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func toolResultMap(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	res, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %+v", out)
	}
	return res
}

func TestMaxToolResultIs32KiB(t *testing.T) {
	if maxToolResultBytes != 32*1024 {
		t.Fatalf("maxToolResultBytes = %d, want 32 KiB", maxToolResultBytes)
	}
}

func TestPaginationHintNamesRemainingAndNextOffset(t *testing.T) {
	names := make([]string, 120)
	for i := range names {
		names[i] = fmt.Sprintf("Kv%03d", i)
	}
	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"kodeverksnavn": names})
	})
	_, out := callToolModern(t, srv.routes(), "in", "list_kodeverk", map[string]any{"limit": 50, "offset": 10})
	res := toolResultMap(t, out)
	content := res["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("want data and hint blocks, got %d", len(content))
	}
	hint := content[1].(map[string]any)["text"].(string)
	if !strings.Contains(hint, "60 more items") || !strings.Contains(hint, "offset=60") {
		t.Fatalf("hint = %q", hint)
	}
	sc := res["structuredContent"].(map[string]any)
	if sc["nextOffset"].(float64) != 60 {
		t.Fatalf("nextOffset = %v", sc["nextOffset"])
	}
	// Following the hint gives the next items.
	_, out = callToolModern(t, srv.routes(), "in", "list_kodeverk", map[string]any{"limit": 50, "offset": 60})
	sc = toolResultMap(t, out)["structuredContent"].(map[string]any)
	if first := sc["kodeverksnavn"].([]any)[0]; first != "Kv060" {
		t.Fatalf("next page starts at %v", first)
	}
	// Last page: no hint, no nextOffset.
	_, out = callToolModern(t, srv.routes(), "in", "list_kodeverk", map[string]any{"limit": 50, "offset": 100})
	res = toolResultMap(t, out)
	if len(res["content"].([]any)) != 1 || res["structuredContent"].(map[string]any)["nextOffset"] != nil {
		t.Fatalf("last page should carry no hint: %+v", res)
	}
}

func TestEveryToolHasOutputSchemaAndExample(t *testing.T) {
	for _, d := range toolDefinitions() {
		name := d["name"].(string)
		os, ok := d["outputSchema"].(map[string]any)
		if !ok || os["type"] != "object" {
			t.Errorf("%s: missing object outputSchema", name)
		}
		if !strings.Contains(d["description"].(string), "Example: {") {
			t.Errorf("%s: description has no example", name)
		}
	}
}

func TestStructuredContentMatchesTextAndSchema(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"koder": []string{"0150", "0151"}})
	})
	_, out := callToolModern(t, srv.routes(), "in", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "Postnummer"})
	res := toolResultMap(t, out)
	sc, ok := res["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("no structuredContent: %+v", res)
	}
	var schema map[string]any
	for _, d := range toolDefinitions() {
		if d["name"] == "get_kodeverk_koder" {
			schema = d["outputSchema"].(map[string]any)
		}
	}
	for _, k := range schema["required"].([]string) {
		if _, ok := sc[k]; !ok {
			t.Errorf("structuredContent lacks required %q", k)
		}
	}
	if txt, _ := extractToolText(t, out); !strings.Contains(txt, `"0150"`) {
		t.Fatalf("legacy text block missing data: %s", txt)
	}
}

func TestUpstreamCacheSharedAndKeyedByQuery(t *testing.T) {
	var hits atomic.Int32
	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		writeJSON(w, http.StatusOK, map[string]any{"betydninger": map[string]any{}})
	})
	call := func(args map[string]any) {
		_, out := callToolModern(t, srv.routes(), "in", "get_kodeverk_betydninger", args)
		if _, isErr := extractToolText(t, out); isErr {
			t.Fatalf("tool error: %+v", out)
		}
	}
	call(map[string]any{"kodeverksnavn": "Postnummer"})
	call(map[string]any{"kodeverksnavn": "Postnummer", "kode": "0150"}) // same upstream request
	if n := hits.Load(); n != 1 {
		t.Fatalf("upstream hits = %d, want 1", n)
	}
	call(map[string]any{"kodeverksnavn": "Postnummer", "spraak": []string{"en"}})
	call(map[string]any{"kodeverksnavn": "Landkoder"})
	if n := hits.Load(); n != 3 {
		t.Fatalf("upstream hits = %d, want 3 after different query and kodeverk", n)
	}
}

func TestUpstreamCacheSkipsErrors(t *testing.T) {
	var hits atomic.Int32
	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	})
	for range 2 {
		callToolModern(t, srv.routes(), "in", "list_kodeverk", nil)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("errors were cached: hits = %d", n)
	}
}

func TestResponseCacheExpiresAndBounds(t *testing.T) {
	c := newResponseCache()
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	c.put("a", []byte("x"))
	if _, ok := c.get("a"); !ok {
		t.Fatal("fresh entry missing")
	}
	now = now.Add(cacheTTL)
	if _, ok := c.get("a"); ok {
		t.Fatal("expired entry served")
	}
	big := make([]byte, cacheMaxBytes/2+1)
	c.put("b", big)
	c.put("c", big)
	if _, ok := c.get("b"); ok {
		t.Fatal("oldest entry not evicted over byte bound")
	}
	if c.size > cacheMaxBytes {
		t.Fatalf("size %d over bound", c.size)
	}
}
