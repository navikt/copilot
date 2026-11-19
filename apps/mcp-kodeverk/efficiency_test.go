package main

import (
	"encoding/json"
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
	txt := content[0].(map[string]any)["text"].(string)
	if len(txt) > maxToolResultBytes {
		t.Fatalf("text length %d exceeds %d", len(txt), maxToolResultBytes)
	}
	var textDecoded map[string]any
	if err := json.Unmarshal([]byte(txt), &textDecoded); err != nil {
		t.Fatalf("unmarshal text block: %v", err)
	}
	if textDecoded["nextOffset"] != float64(60) {
		t.Fatalf("text block nextOffset = %v, want 60", textDecoded["nextOffset"])
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
	lastTxt, _ := extractToolText(t, out)
	if len(res["content"].([]any)) != 1 || res["structuredContent"].(map[string]any)["nextOffset"] != nil || strings.Contains(lastTxt, "nextOffset") {
		t.Fatalf("last page should carry no hint or nextOffset: %+v", res)
	}
}

func TestSingleItemExceedingCapDoesNotLoopNextOffset(t *testing.T) {
	huge := strings.Repeat("X", maxToolResultBytes+1024)
	betydninger := map[string][]betydning{
		"HUGE": {{
			GyldigFra: "2000-01-01",
			GyldigTil: "9999-12-31",
			Beskrivelser: map[string]beskrivelse{
				"nb": {Term: "Huge", Tekst: huge},
			},
		}},
	}
	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"betydninger": betydninger})
	})
	_, out := callToolModern(t, srv.routes(), "in", "get_kodeverk_betydninger", map[string]any{
		"kodeverksnavn": "Stor",
		"offset":        0,
	})
	res := toolResultMap(t, out)
	sc := res["structuredContent"].(map[string]any)
	if sc["returned"] != float64(0) || sc["truncated"] != true {
		t.Fatalf("want returned=0 truncated=true, got %+v", sc)
	}
	if _, hasNext := sc["nextOffset"]; hasNext {
		t.Fatalf("must not advertise non-advancing nextOffset when returned==0: %+v", sc)
	}
	content := res["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("want data and truncation hint blocks, got %d", len(content))
	}
	txt := content[0].(map[string]any)["text"].(string)
	if strings.Contains(txt, "nextOffset") {
		t.Fatalf("text block must not contain nextOffset when returned==0: %s", txt)
	}
	hint := content[1].(map[string]any)["text"].(string)
	if strings.Contains(hint, "offset=0") || !strings.Contains(hint, "32 KiB") {
		t.Fatalf("unexpected hint for oversized single item: %q", hint)
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
	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/kodeverk":
			writeJSON(w, http.StatusOK, map[string]any{"kodeverksnavn": []string{"Landkoder", "Postnummer"}})
		case "/api/v1/kodeverk/Postnummer/koder":
			writeJSON(w, http.StatusOK, map[string]any{"koder": []string{"0150", "0151"}})
		case "/api/v1/kodeverk/Postnummer/koder/betydninger":
			writeJSON(w, http.StatusOK, map[string]any{
				"betydninger": map[string]any{
					"0150": []map[string]any{
						{"gyldigFra": "2000-01-01", "gyldigTil": "9999-12-31", "beskrivelser": map[string]any{"nb": map[string]any{"term": "OSLO", "tekst": "OSLO"}}},
					},
				},
			})
		case "/api/v1/hierarki":
			writeJSON(w, http.StatusOK, map[string]any{"hierarkinavn": []string{"Geografi"}})
		case "/api/v1/hierarki/Geografi/noder":
			writeJSON(w, http.StatusOK, map[string]any{
				"hierarkinivaaer": []string{"Land"},
				"noder":           map[string]any{"NOR": map[string]any{"kode": "NOR", "termer": map[string]any{"nb": "Norge"}}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	schemas := map[string]map[string]any{}
	for _, d := range toolDefinitions() {
		schemas[d["name"].(string)] = d["outputSchema"].(map[string]any)
	}

	calls := []struct {
		name string
		tool string
		args map[string]any
	}{
		{"list_kodeverk", "list_kodeverk", map[string]any{"limit": 1}},
		{"get_kodeverk_koder", "get_kodeverk_koder", map[string]any{"kodeverksnavn": "Postnummer", "limit": 1}},
		{"get_kodeverk_betydninger", "get_kodeverk_betydninger", map[string]any{"kodeverksnavn": "Postnummer"}},
		{"list_hierarkier", "list_hierarkier", map[string]any{}},
		{"get_hierarki_noder", "get_hierarki_noder", map[string]any{"hierarkinavn": "Geografi"}},
		{"search_kodeverk names", "search_kodeverk", map[string]any{"query": "Post"}},
		{"search_kodeverk codes", "search_kodeverk", map[string]any{"query": "OSLO", "kodeverksnavn": "Postnummer"}},
	}

	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			_, out := callToolModern(t, srv.routes(), "in", tc.tool, tc.args)
			res := toolResultMap(t, out)
			sc, ok := res["structuredContent"].(map[string]any)
			if !ok {
				t.Fatalf("no structuredContent: %+v", res)
			}
			schema := schemas[tc.tool]
			props := schema["properties"].(map[string]any)
			for _, reqKey := range schema["required"].([]string) {
				if _, ok := sc[reqKey]; !ok {
					t.Errorf("%s: structuredContent lacks required %q", tc.name, reqKey)
				}
			}
			for k := range sc {
				if _, ok := props[k]; !ok {
					t.Errorf("%s: structuredContent field %q missing from outputSchema properties (%v)", tc.name, k, props)
				}
			}
			txt, _ := extractToolText(t, out)
			var fromText map[string]any
			if err := json.Unmarshal([]byte(txt), &fromText); err != nil {
				t.Fatalf("%s: text block invalid JSON: %v", tc.name, err)
			}
			for k, v := range sc {
				if fmt.Sprintf("%v", fromText[k]) != fmt.Sprintf("%v", v) {
					t.Errorf("%s: text block key %q = %v, structuredContent = %v", tc.name, k, fromText[k], v)
				}
			}
		})
	}
}

func TestTruncationHintsOnlySuggestSupportedArguments(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/kodeverk":
			writeJSON(w, http.StatusOK, map[string]any{"kodeverksnavn": []string{"Landkoder", "LandkoderISO2", "Landkoderalle"}})
		case "/api/v1/hierarki/Geografi/noder":
			writeJSON(w, http.StatusOK, map[string]any{
				"hierarkinivaaer": []string{"Verdensdel", "Land"},
				"noder": map[string]any{
					"EUR": map[string]any{
						"kode":       "EUR",
						"undernoder": map[string]any{"NOR": map[string]any{"kode": "NOR"}},
					},
					"ASI": map[string]any{
						"kode": "ASI",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	t.Run("get_hierarki_noder depth truncation suggests increasing maxDepth", func(t *testing.T) {
		_, out := callToolModern(t, srv.routes(), "in", "get_hierarki_noder", map[string]any{
			"hierarkinavn": "Geografi",
			"maxDepth":     1,
			"limit":        10,
		})
		content := toolResultMap(t, out)["content"].([]any)
		if len(content) != 2 {
			t.Fatalf("expected hint block, got %d blocks", len(content))
		}
		hint := content[1].(map[string]any)["text"].(string)
		if !strings.Contains(hint, "Increase maxDepth") {
			t.Fatalf("depth-truncated hint should suggest increasing maxDepth, got %q", hint)
		}
		for _, forbidden := range []string{"query", "kodeverksnavn", "lower maxDepth", "Lower maxDepth", "filter", "kode"} {
			if strings.Contains(hint, forbidden) {
				t.Fatalf("hint %q mentions unsupported or counterproductive %q", hint, forbidden)
			}
		}
	})

	t.Run("get_hierarki_noder root truncation suggests supported args only", func(t *testing.T) {
		_, out := callToolModern(t, srv.routes(), "in", "get_hierarki_noder", map[string]any{
			"hierarkinavn": "Geografi",
			"maxDepth":     2,
			"limit":        1,
		})
		content := toolResultMap(t, out)["content"].([]any)
		if len(content) != 2 {
			t.Fatalf("expected hint block, got %d blocks", len(content))
		}
		hint := content[1].(map[string]any)["text"].(string)
		if !strings.Contains(hint, "Lower maxDepth") && !strings.Contains(hint, "increase limit") {
			t.Fatalf("root-truncated hint = %q", hint)
		}
		for _, forbidden := range []string{"query", "kodeverksnavn", "filter", "kode"} {
			if strings.Contains(hint, forbidden) {
				t.Fatalf("hint %q mentions unsupported argument %q", hint, forbidden)
			}
		}
	})

	t.Run("search_kodeverk truncation suggests query or kodeverksnavn, not maxDepth", func(t *testing.T) {
		_, out := callToolModern(t, srv.routes(), "in", "search_kodeverk", map[string]any{
			"query": "Land",
			"limit": 1,
		})
		content := toolResultMap(t, out)["content"].([]any)
		if len(content) != 2 {
			t.Fatalf("expected hint block, got %d blocks", len(content))
		}
		hint := content[1].(map[string]any)["text"].(string)
		if !strings.Contains(hint, "query") || !strings.Contains(hint, "kodeverksnavn") {
			t.Fatalf("search_kodeverk hint = %q", hint)
		}
		for _, forbidden := range []string{"maxDepth", "filter", "spraak", "offset"} {
			if strings.Contains(hint, forbidden) {
				t.Fatalf("hint %q mentions unsupported argument %q", hint, forbidden)
			}
		}
	})
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
	if c.size != len("a")+len("x") {
		t.Fatalf("size = %d, want %d (len(key)+len(body))", c.size, len("a")+len("x"))
	}
	if _, ok := c.get("a"); !ok {
		t.Fatal("fresh entry missing")
	}
	now = now.Add(cacheTTL)
	if _, ok := c.get("a"); ok {
		t.Fatal("expired entry served")
	}
	if _, exists := c.entries["a"]; exists || len(c.order) != 0 || c.size != 0 {
		t.Fatalf("expired entry not deleted on get: entries=%d order=%d size=%d", len(c.entries), len(c.order), c.size)
	}

	// Verify key length counts toward cacheMaxBytes eviction.
	halfBody := make([]byte, cacheMaxBytes/2)
	longKey1 := strings.Repeat("k", 16)
	longKey2 := strings.Repeat("m", 16)
	c.put(longKey1, halfBody)
	c.put(longKey2, halfBody)
	if _, ok := c.get(longKey1); ok {
		t.Fatal("oldest entry not evicted when len(key)+len(body) exceeded cacheMaxBytes")
	}
	if c.size != len(longKey2)+len(halfBody) || c.size > cacheMaxBytes {
		t.Fatalf("size = %d, want %d (<= %d)", c.size, len(longKey2)+len(halfBody), cacheMaxBytes)
	}
}
