package main

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
)

type recordedReq struct{ method, path, query string }

func recordingDolly(t *testing.T) (*server, func() []recordedReq) {
	t.Helper()
	var mu sync.Mutex
	var got []recordedReq
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, recordedReq{r.Method, r.URL.Path, r.URL.RawQuery})
		mu.Unlock()
		if r.URL.Path == "/api/v1/bruker/current" {
			writeJSON(w, http.StatusOK, map[string]any{"brukerId": "b 1&x"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	return srv, func() []recordedReq {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

func TestToolsHitOnlyExpectedGETPaths(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
		want []recordedReq
	}{
		{"dolly_current_user", nil, []recordedReq{{"GET", "/api/v1/bruker/current", ""}}},
		{"dolly_list_groups", map[string]any{"pageNo": 2, "pageSize": 5}, []recordedReq{
			{"GET", "/api/v1/bruker/current", ""},
			{"GET", "/api/v1/gruppe", "brukerId=b+1%26x&pageNo=2&pageSize=5"},
		}},
		{"dolly_get_group", map[string]any{"gruppeId": 42}, []recordedReq{{"GET", "/api/v1/gruppe/42/page/0", "pageSize=20"}}},
		{"dolly_search", map[string]any{"fragment": " a/b?c ", "kind": "gruppe"}, []recordedReq{{"GET", "/api/v1/gruppe/soekGruppe", "fragment=a%2Fb%3Fc"}}},
		{"dolly_search", map[string]any{"fragment": "7", "kind": "bestilling"}, []recordedReq{{"GET", "/api/v1/bestilling/soekBestilling", "fragment=7"}}},
		{"dolly_get_order", map[string]any{"bestillingId": 9007199254740991}, []recordedReq{{"GET", "/api/v1/bestilling/9007199254740991", ""}}},
		{"dolly_list_group_orders", map[string]any{"gruppeId": 3, "pageNo": 1, "pageSize": 50}, []recordedReq{{"GET", "/api/v1/bestilling/gruppe/3", "page=1&pageSize=50"}}},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			srv, got := recordingDolly(t)
			_, out := callToolModern(t, srv.routes(), "in", tc.tool, tc.args)
			if txt, isErr := extractToolText(t, out); isErr {
				t.Fatalf("tool error: %s", txt)
			}
			if g := got(); !slices.Equal(g, tc.want) {
				t.Fatalf("upstream requests = %+v, want %+v", g, tc.want)
			}
		})
	}
}

func TestToolsListIsExactlyReadOnlySet(t *testing.T) {
	var names []string
	for _, d := range toolDefinitions() {
		names = append(names, d["name"].(string))
		if d["annotations"].(map[string]bool)["readOnlyHint"] != true {
			t.Fatalf("%s is not marked read-only", d["name"])
		}
	}
	want := []string{"dolly_current_user", "dolly_list_groups", "dolly_get_group", "dolly_search", "dolly_get_order", "dolly_list_group_orders"}
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestInvalidArgumentsNeverReachUpstream(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
	}{
		{"dolly_get_group", map[string]any{}},
		{"dolly_get_group", map[string]any{"gruppeId": 0}},
		{"dolly_get_group", map[string]any{"gruppeId": -1}},
		{"dolly_get_group", map[string]any{"gruppeId": "1"}},
		{"dolly_get_group", map[string]any{"gruppeId": 1.5}},
		{"dolly_get_group", map[string]any{"gruppeId": "1/../../x"}},
		{"dolly_get_group", map[string]any{"gruppeId": 1, "pageSize": 51}},
		{"dolly_get_group", map[string]any{"gruppeId": 1, "pageSize": 0}},
		{"dolly_get_group", map[string]any{"gruppeId": 1, "pageNo": -1}},
		{"dolly_get_order", map[string]any{"bestillingId": json.Number("99999999999999999999")}},
		{"dolly_get_order", map[string]any{"bestillingId": 1, "extra": true}},
		{"dolly_list_groups", map[string]any{"brukerId": "someone-else"}},
		{"dolly_search", map[string]any{"fragment": "x"}},
		{"dolly_search", map[string]any{"fragment": "x", "kind": "person"}},
		{"dolly_search", map[string]any{"fragment": "   ", "kind": "gruppe"}},
		{"dolly_search", map[string]any{"fragment": strings.Repeat("x", 101), "kind": "gruppe"}},
		{"dolly_search", map[string]any{"fragment": "a\nb", "kind": "gruppe"}},
		{"dolly_create_group", map[string]any{}},
	}
	for _, tc := range cases {
		srv, got := recordingDolly(t)
		_, out := callToolModern(t, srv.routes(), "in", tc.tool, tc.args)
		if _, ok := out["error"]; !ok {
			t.Errorf("%s %v: expected JSON-RPC error, got %+v", tc.tool, tc.args, out)
		}
		if g := got(); len(g) != 0 {
			t.Errorf("%s %v: reached upstream: %+v", tc.tool, tc.args, g)
		}
	}
}

func TestResultCapAndStatusMapping(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/bestilling/1" {
			writeJSON(w, http.StatusOK, map[string]any{"x": strings.Repeat("a", maxToolResultBytes)})
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})
	_, out := callToolModern(t, srv.routes(), "in", "dolly_get_order", map[string]any{"bestillingId": 1})
	if txt, isErr := extractToolText(t, out); !isErr || !strings.Contains(txt, "exceeds") {
		t.Fatalf("expected size cap error, got %q", txt)
	}
	_, out = callToolModern(t, srv.routes(), "in", "dolly_get_order", map[string]any{"bestillingId": 2})
	if txt, isErr := extractToolText(t, out); !isErr || !strings.Contains(txt, "denied access") {
		t.Fatalf("expected access error, got %q", txt)
	}
}
