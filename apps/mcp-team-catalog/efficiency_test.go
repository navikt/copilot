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
	teams := make([]map[string]any, 60)
	for i := range teams {
		teams[i] = map[string]any{
			"id":     fmt.Sprintf("00000000-0000-0000-0000-%012d", i),
			"name":   fmt.Sprintf("Team %03d", i),
			"status": "ACTIVE",
		}
	}
	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"content": teams})
	})

	_, out := callToolModern(t, srv.routes(), "in", "list_teams", map[string]any{"limit": 20, "offset": 10})
	res := toolResultMap(t, out)
	content := res["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("want data and hint blocks, got %d", len(content))
	}
	hint := content[1].(map[string]any)["text"].(string)
	if !strings.Contains(hint, "30 more items") || !strings.Contains(hint, "offset=30") {
		t.Fatalf("hint = %q", hint)
	}
	sc := res["structuredContent"].(map[string]any)
	if sc["nextOffset"].(float64) != 30 {
		t.Fatalf("nextOffset = %v", sc["nextOffset"])
	}

	// Last page: no hint, no nextOffset.
	_, out = callToolModern(t, srv.routes(), "in", "list_teams", map[string]any{"limit": 20, "offset": 40})
	res = toolResultMap(t, out)
	if len(res["content"].([]any)) != 1 || res["structuredContent"].(map[string]any)["nextOffset"] != nil {
		t.Fatalf("last page should carry no hint or nextOffset: %+v", res)
	}
}

func TestSizeCap32KiB(t *testing.T) {
	bigDesc := strings.Repeat("A", 3000)
	teams := make([]map[string]any, 50)
	for i := range teams {
		teams[i] = map[string]any{
			"id":          fmt.Sprintf("00000000-0000-0000-0000-%012d", i),
			"name":        fmt.Sprintf("Team %03d", i),
			"description": bigDesc,
			"status":      "ACTIVE",
		}
	}
	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"content": teams})
	})

	_, out := callToolModern(t, srv.routes(), "in", "list_teams", map[string]any{"limit": 50})
	txt, isErr := extractToolText(t, out)
	if isErr {
		t.Fatalf("unexpected tool error: %s", txt)
	}
	toolRes := toolResultMap(t, out)
	serializedToolRes, err := json.Marshal(map[string]any{
		"content":           toolRes["content"],
		"structuredContent": toolRes["structuredContent"],
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(serializedToolRes) > maxToolResultBytes {
		t.Fatalf("serialized tool result length %d exceeds 32 KiB (%d)", len(serializedToolRes), maxToolResultBytes)
	}
	var res struct {
		Total     int           `json:"total"`
		Returned  int           `json:"returned"`
		Truncated bool          `json:"truncated"`
		Teams     []teamSummary `json:"teams"`
	}
	if err := json.Unmarshal([]byte(txt), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || res.Total != 50 || res.Returned == 0 || res.Returned >= 50 || len(res.Teams) != res.Returned {
		t.Fatalf("unexpected capped metadata: %+v", res)
	}
}

func TestSizeCapNearLimitIncludesEscapingAndDuplicate(t *testing.T) {
	t.Run("fitCappedJSON caps serialized toolResult including escaping and structuredContent", func(t *testing.T) {
		// Each team is ~1.8 KiB raw JSON; 10 teams = ~18 KiB raw text (< 32 KiB),
		// but duplicated in content[0].text (with quote escaping) + structuredContent exceeds 32 KiB.
		quotedDesc := strings.Repeat(`a"b`, 500)
		teams := make([]map[string]any, 10)
		for i := range teams {
			teams[i] = map[string]any{
				"id":          fmt.Sprintf("00000000-0000-0000-0000-%012d", i),
				"name":        fmt.Sprintf("Team %02d", i),
				"description": quotedDesc,
				"status":      "ACTIVE",
			}
		}
		srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"content": teams})
		})

		_, out := callToolModern(t, srv.routes(), "in", "list_teams", map[string]any{"limit": 10})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected tool error: %s", txt)
		}
		toolRes := toolResultMap(t, out)
		serialized, err := json.Marshal(map[string]any{
			"content":           toolRes["content"],
			"structuredContent": toolRes["structuredContent"],
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(serialized) > maxToolResultBytes {
			t.Fatalf("serialized toolResult length %d exceeds %d", len(serialized), maxToolResultBytes)
		}
		var decoded struct {
			Returned  int  `json:"returned"`
			Truncated bool `json:"truncated"`
		}
		if err := json.Unmarshal([]byte(txt), &decoded); err != nil {
			t.Fatal(err)
		}
		if !decoded.Truncated || decoded.Returned == 0 || decoded.Returned >= 10 {
			t.Fatalf("expected truncation below 10 items, got %+v", decoded)
		}
	})

	t.Run("buildGetNaisTeamResult rejects near-limit payload whose serialized toolResult exceeds 32 KiB", func(t *testing.T) {
		// ~18 KiB raw JSON (< 32 KiB), but > 36 KiB once serialized in content + structuredContent.
		nearLimitPurpose := strings.Repeat(`x"y`, 6000)
		srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"slug":         "copilot",
				"purpose":      nearLimitPurpose,
				"slackChannel": "#copilot",
			})
		})

		_, out := callToolModern(t, srv.routes(), "in", "get_nais_team", map[string]any{"teamId": "copilot"})
		txt, isErr := extractToolText(t, out)
		if !isErr || !strings.Contains(txt, "32 KiB") {
			t.Fatalf("expected 32 KiB cap error, got isErr=%v txt=%q", isErr, txt)
		}
	})
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
	sampleTeam := map[string]any{"id": testTeamID, "name": "Team Copilot", "status": "ACTIVE", "members": []any{}}
	sampleArea := map[string]any{"id": testAreaID, "name": "Plattform", "status": "ACTIVE", "members": []any{}}
	sampleCluster := map[string]any{"id": testClusterID, "name": "KI", "status": "ACTIVE", "members": []any{}}
	sampleNaisTeam := map[string]any{"slug": "copilot", "purpose": "Copilot platform", "slackChannel": "#copilot"}

	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/team", "/team/search/copilot", "/team/naisteam/copilot":
			writeJSON(w, http.StatusOK, map[string]any{"content": []any{sampleTeam}})
		case "/team/" + testTeamID:
			writeJSON(w, http.StatusOK, sampleTeam)
		case "/productarea":
			writeJSON(w, http.StatusOK, map[string]any{"content": []any{sampleArea}})
		case "/productarea/" + testAreaID:
			writeJSON(w, http.StatusOK, sampleArea)
		case "/cluster":
			writeJSON(w, http.StatusOK, map[string]any{"content": []any{sampleCluster}})
		case "/cluster/" + testClusterID:
			writeJSON(w, http.StatusOK, sampleCluster)
		case "/member/membership/Z123456":
			writeJSON(w, http.StatusOK, map[string]any{"teams": []any{sampleTeam}, "productAreas": []any{sampleArea}, "clusters": []any{sampleCluster}})
		case "/naisteam/copilot":
			writeJSON(w, http.StatusOK, sampleNaisTeam)
		case "/naisteam/search/copilot":
			writeJSON(w, http.StatusOK, map[string]any{"content": []any{sampleNaisTeam}})
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
		{"list_teams", "list_teams", map[string]any{}},
		{"search_teams", "search_teams", map[string]any{"query": "copilot"}},
		{"get_team", "get_team", map[string]any{"id": testTeamID}},
		{"get_teams_by_nais_team", "get_teams_by_nais_team", map[string]any{"naisTeam": "copilot"}},
		{"list_product_areas", "list_product_areas", map[string]any{}},
		{"get_product_area", "get_product_area", map[string]any{"id": testAreaID}},
		{"list_clusters", "list_clusters", map[string]any{}},
		{"get_cluster", "get_cluster", map[string]any{"id": testClusterID}},
		{"get_member_memberships", "get_member_memberships", map[string]any{"navIdent": "Z123456"}},
		{"get_nais_team by id", "get_nais_team", map[string]any{"teamId": "copilot"}},
		{"get_nais_team by query", "get_nais_team", map[string]any{"query": "copilot"}},
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
			if reqKeys, ok := schema["required"].([]string); ok {
				for _, reqKey := range reqKeys {
					if _, ok := sc[reqKey]; !ok {
						t.Errorf("%s: structuredContent lacks required %q", tc.name, reqKey)
					}
				}
			}
			for k := range sc {
				if _, ok := props[k]; !ok {
					t.Errorf("%s: structuredContent field %q missing from outputSchema properties", tc.name, k)
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

func TestCachingBehavior(t *testing.T) {
	var catalogHits atomic.Int32
	var memberHits atomic.Int32

	srv := newTestServer(t, defaultTexasHandler("obo"), func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/team":
			catalogHits.Add(1)
			writeJSON(w, http.StatusOK, map[string]any{"content": []any{}})
		case strings.HasPrefix(r.URL.Path, "/member/membership/"):
			memberHits.Add(1)
			writeJSON(w, http.StatusOK, map[string]any{"teams": []any{}, "productAreas": []any{}, "clusters": []any{}})
		default:
			http.NotFound(w, r)
		}
	})

	// Shared catalog endpoints are cached.
	callToolModern(t, srv.routes(), "in", "list_teams", map[string]any{})
	callToolModern(t, srv.routes(), "in", "list_teams", map[string]any{"filter": "copilot"})
	if n := catalogHits.Load(); n != 1 {
		t.Fatalf("catalogHits = %d, want 1", n)
	}

	// Member lookups are never cached.
	callToolModern(t, srv.routes(), "in", "get_member_memberships", map[string]any{"navIdent": "Z123456"})
	callToolModern(t, srv.routes(), "in", "get_member_memberships", map[string]any{"navIdent": "Z123456"})
	if n := memberHits.Load(); n != 2 {
		t.Fatalf("memberHits = %d, want 2 (member lookups must not be cached)", n)
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
}
