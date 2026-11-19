package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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
		if r.URL.Path == "/api/v1/gruppe/42/page/0" {
			writeJSON(w, http.StatusOK, map[string]any{
				"id":          42,
				"opprettetAv": map[string]any{"brukerId": "b 1&x"},
			})
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

func TestToolsListAndAnnotations(t *testing.T) {
	var names []string
	for _, d := range toolDefinitions() {
		name := d["name"].(string)
		names = append(names, name)
		ann := d["annotations"].(map[string]bool)
		if name == "create_order" {
			wantAnn := map[string]bool{
				"readOnlyHint":    false,
				"destructiveHint": false,
				"idempotentHint":  false,
				"openWorldHint":   true,
			}
			for k, wantVal := range wantAnn {
				if gotVal, ok := ann[k]; !ok || gotVal != wantVal {
					t.Fatalf("create_order annotation %s = %v (present=%v), want %v", k, gotVal, ok, wantVal)
				}
			}
			desc, _ := d["description"].(string)
			if !strings.Contains(strings.ToLower(desc), "confirm with the user") {
				t.Fatalf("create_order description %q must instruct agent to confirm with the user first", desc)
			}
			continue
		}
		if ann["readOnlyHint"] != true || ann["destructiveHint"] != false || ann["idempotentHint"] != true || ann["openWorldHint"] != false {
			t.Fatalf("%s has unexpected annotations: %+v", name, ann)
		}
	}
	want := []string{
		"dolly_current_user",
		"dolly_list_groups",
		"dolly_get_group",
		"dolly_search",
		"dolly_get_order",
		"dolly_list_group_orders",
		"create_order",
	}
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
		{"create_order", map[string]any{}},
		{"create_order", map[string]any{"gruppeId": 0, "antall": 1, "environments": []string{"q1"}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 0, "environments": []string{"q1"}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": -1, "environments": []string{"q1"}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 6, "environments": []string{"q1"}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": "2", "environments": []string{"q1"}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"qx"}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"prod"}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"q1", "q1"}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": "q1"}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"q1"}, "minAlder": -1}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"q1"}, "maxAlder": 121}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"q1"}, "minAlder": 50, "maxAlder": 30}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"q1"}, "kjoenn": "OTHER"}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"q1"}, "pdldata": map[string]any{}}},
		{"create_order", map[string]any{"gruppeId": 42, "antall": 1, "environments": []string{"q1"}, "opprettFraIdenter": []string{"12345678901"}}},
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

func TestCreateOrderOwnershipAndRequestShape(t *testing.T) {
	fixedNow := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	t.Run("owned group sends verified POST body to bestilling endpoint", func(t *testing.T) {
		var (
			mu          sync.Mutex
			gotReqs     []recordedReq
			gotPostBody []byte
		)
		srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "fraidenter") {
				t.Fatalf("forbidden fraidenter endpoint called: %s", r.URL.Path)
			}
			var body []byte
			if r.Method == http.MethodPost {
				body, _ = io.ReadAll(r.Body)
			}
			mu.Lock()
			gotReqs = append(gotReqs, recordedReq{r.Method, r.URL.Path, r.URL.RawQuery})
			if r.Method == http.MethodPost {
				gotPostBody = body
			}
			mu.Unlock()

			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bruker/current":
				writeJSON(w, http.StatusOK, map[string]any{"brukerId": "owner-123"})
			case r.Method == http.MethodGet && r.URL.Path == "/api/v1/gruppe/42/page/0":
				writeJSON(w, http.StatusOK, map[string]any{
					"id":          42,
					"opprettetAv": map[string]any{"brukerId": "owner-123"},
				})
			case r.Method == http.MethodPost && r.URL.Path == "/api/v1/gruppe/42/bestilling":
				writeJSON(w, http.StatusCreated, map[string]any{"id": 99, "gruppeId": 42, "antallIdenter": 3})
			default:
				http.NotFound(w, r)
			}
		})
		srv.now = func() time.Time { return fixedNow }

		_, out := callToolModern(t, srv.routes(), "in", "create_order", map[string]any{
			"gruppeId":     42,
			"antall":       3,
			"environments": []string{"q1", "q2", "q4"},
			"minAlder":     25,
			"maxAlder":     40,
			"kjoenn":       "KVINNE",
		})
		txt, isErr := extractToolText(t, out)
		if isErr {
			t.Fatalf("unexpected tool error: %s", txt)
		}

		wantReqs := []recordedReq{
			{"GET", "/api/v1/bruker/current", ""},
			{"GET", "/api/v1/gruppe/42/page/0", "pageSize=20"},
			{"POST", "/api/v1/gruppe/42/bestilling", ""},
		}
		mu.Lock()
		defer mu.Unlock()
		if !slices.Equal(gotReqs, wantReqs) {
			t.Fatalf("upstream requests = %+v, want %+v", gotReqs, wantReqs)
		}

		var parsed dollyBestillingRequest
		if err := json.Unmarshal(gotPostBody, &parsed); err != nil {
			t.Fatalf("unmarshal post body: %v", err)
		}
		if parsed.Antall != 3 || !slices.Equal(parsed.Environments, []string{"q1", "q2", "q4"}) {
			t.Fatalf("unexpected order top-level fields: %+v", parsed)
		}
		if parsed.Pdldata.OpprettNyPerson.Identtype != "FNR" || !parsed.Pdldata.OpprettNyPerson.Syntetisk {
			t.Fatalf("unexpected opprettNyPerson: %+v", parsed.Pdldata.OpprettNyPerson)
		}
		if parsed.Pdldata.OpprettNyPerson.FoedtEtter != "1986-11-19T00:00:00" ||
			parsed.Pdldata.OpprettNyPerson.FoedtFoer != "2001-11-19T00:00:00" {
			t.Fatalf("unexpected age bounds: %+v", parsed.Pdldata.OpprettNyPerson)
		}
		if parsed.Pdldata.Person == nil || len(parsed.Pdldata.Person.Kjoenn) != 1 || parsed.Pdldata.Person.Kjoenn[0].Kjoenn != "KVINNE" {
			t.Fatalf("unexpected kjoenn: %+v", parsed.Pdldata.Person)
		}
	})

	t.Run("non-owner group is rejected before POST", func(t *testing.T) {
		var (
			mu         sync.Mutex
			postCalled bool
		)
		srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				mu.Lock()
				postCalled = true
				mu.Unlock()
			}
			switch r.URL.Path {
			case "/api/v1/bruker/current":
				writeJSON(w, http.StatusOK, map[string]any{"brukerId": "caller-123"})
			case "/api/v1/gruppe/42/page/0":
				writeJSON(w, http.StatusOK, map[string]any{
					"id":          42,
					"opprettetAv": map[string]any{"brukerId": "other-owner-999"},
				})
			default:
				writeJSON(w, http.StatusCreated, map[string]any{"id": 99})
			}
		})

		_, out := callToolModern(t, srv.routes(), "in", "create_order", map[string]any{
			"gruppeId":     42,
			"antall":       2,
			"environments": []string{"q1"},
		})
		txt, isErr := extractToolText(t, out)
		if !isErr || !strings.Contains(txt, "not owned") {
			t.Fatalf("expected ownership rejection, got isErr=%v txt=%q", isErr, txt)
		}
		mu.Lock()
		defer mu.Unlock()
		if postCalled {
			t.Fatal("POST /bestilling was called for a group owned by another user")
		}
	})
}

func TestCreateOrderPerUserRollingQuota(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	var (
		mu         sync.Mutex
		activeUser = "Z123456"
		postCount  int
	)

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/introspect":
			mu.Lock()
			u := activeUser
			mu.Unlock()
			writeJSON(w, http.StatusOK, map[string]any{
				"active":   true,
				"idtyp":    "user",
				"NAVident": u,
				"azp":      "gw-client-123",
			})
		case "/exchange":
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token": "obo-tok",
				"expires_in":   3600,
				"token_type":   "Bearer",
			})
		default:
			http.NotFound(w, r)
		}
	}, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bruker/current":
			writeJSON(w, http.StatusOK, map[string]any{"brukerId": "b1"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/gruppe/42/page/0":
			writeJSON(w, http.StatusOK, map[string]any{"id": 42, "opprettetAv": map[string]any{"brukerId": "b1"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/gruppe/42/bestilling":
			mu.Lock()
			postCount++
			mu.Unlock()
			writeJSON(w, http.StatusCreated, map[string]any{"id": 1})
		default:
			http.NotFound(w, r)
		}
	})
	srv.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}

	// 4 orders of 5 persons = 20 persons (fills the 24h quota for Z123456).
	for i := range 4 {
		_, out := callToolModern(t, srv.routes(), "in", "create_order", map[string]any{
			"gruppeId":     42,
			"antall":       5,
			"environments": []string{"q1"},
		})
		if txt, isErr := extractToolText(t, out); isErr {
			t.Fatalf("order %d failed unexpectedly: %s", i+1, txt)
		}
	}

	// Verify the quota map is keyed by hashed NAVident, never plaintext NAVident.
	srv.quota.mu.Lock()
	_, hasPlain := srv.quota.entries["Z123456"]
	_, hasHashed := srv.quota.entries[hashUser("Z123456")]
	srv.quota.mu.Unlock()
	if hasPlain || !hasHashed {
		t.Fatalf("quota keys: hasPlain=%v hasHashed=%v, want hashed NAVident key", hasPlain, hasHashed)
	}

	// 21st person within 24h (at +23h) must be rejected without calling POST.
	mu.Lock()
	now = now.Add(23 * time.Hour)
	mu.Unlock()
	_, out := callToolModern(t, srv.routes(), "in", "create_order", map[string]any{
		"gruppeId":     42,
		"antall":       1,
		"environments": []string{"q1"},
	})
	if txt, isErr := extractToolText(t, out); !isErr || !strings.Contains(txt, "quota") {
		t.Fatalf("expected 24h quota rejection on 21st person, got isErr=%v txt=%q", isErr, txt)
	}
	mu.Lock()
	if postCount != 4 {
		t.Fatalf("postCount = %d after quota rejection, want 4", postCount)
	}

	// Another user has their own independent 20-person quota.
	activeUser = "Z654321"
	mu.Unlock()
	_, out = callToolModern(t, srv.routes(), "in", "create_order", map[string]any{
		"gruppeId":     42,
		"antall":       5,
		"environments": []string{"q2"},
	})
	if txt, isErr := extractToolText(t, out); isErr {
		t.Fatalf("second user should have separate quota, got error: %s", txt)
	}

	// After 24h from the initial orders, Z123456's window rolls over and allows new orders.
	mu.Lock()
	activeUser = "Z123456"
	now = now.Add(1*time.Hour + time.Second)
	mu.Unlock()
	_, out = callToolModern(t, srv.routes(), "in", "create_order", map[string]any{
		"gruppeId":     42,
		"antall":       5,
		"environments": []string{"q4"},
	})
	if txt, isErr := extractToolText(t, out); isErr {
		t.Fatalf("order after 24h window rollover failed: %s", txt)
	}
}

func TestCreateOrderAuditLog(t *testing.T) {
	var logBuf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	srv, _ := recordingDolly(t)
	srv.now = func() time.Time { return time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC) }

	_, out := callToolModern(t, srv.routes(), "in", "create_order", map[string]any{
		"gruppeId":     42,
		"antall":       2,
		"environments": []string{"q1", "q4"},
		"minAlder":     28,
		"maxAlder":     35,
		"kjoenn":       "KVINNE",
	})
	if txt, isErr := extractToolText(t, out); isErr {
		t.Fatalf("unexpected tool error: %s", txt)
	}

	rawLogs := logBuf.String()
	for _, forbidden := range []string{"Z123456", "b 1&x", "KVINNE", "foedtEtter", "foedtFoer", "1991-", "1998-"} {
		if strings.Contains(rawLogs, forbidden) {
			t.Fatalf("audit log leaked %q: %s", forbidden, rawLogs)
		}
	}

	var auditLines []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(rawLogs), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("unmarshal log line: %v", err)
		}
		if entry["msg"] == "audit dolly create_order" {
			auditLines = append(auditLines, entry)
		}
	}
	if len(auditLines) != 1 {
		t.Fatalf("got %d audit lines, want 1; logs=%s", len(auditLines), rawLogs)
	}
	entry := auditLines[0]
	if entry["user"] != hashUser("Z123456") {
		t.Fatalf("audit user = %v, want %s", entry["user"], hashUser("Z123456"))
	}
	if entry["gruppeId"] != float64(42) || entry["antall"] != float64(2) {
		t.Fatalf("audit gruppeId/antall = %v/%v, want 42/2", entry["gruppeId"], entry["antall"])
	}
	envs, _ := entry["environments"].([]any)
	if len(envs) != 2 || envs[0] != "q1" || envs[1] != "q4" {
		t.Fatalf("audit environments = %v, want [q1 q4]", entry["environments"])
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
