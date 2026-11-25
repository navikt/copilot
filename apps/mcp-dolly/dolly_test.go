package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/navikt/copilot/apps/mcp-dolly/internal/dolly"
	"io"
	"log/slog"
	"net"
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
	checkTool := func(t *testing.T, d map[string]any) string {
		t.Helper()
		name := d["name"].(string)
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
			return name
		}
		if ann["readOnlyHint"] != true || ann["destructiveHint"] != false || ann["idempotentHint"] != true || ann["openWorldHint"] != false {
			t.Fatalf("%s has unexpected annotations: %+v", name, ann)
		}
		return name
	}

	readOnlyWant := []string{
		"dolly_current_user",
		"dolly_list_groups",
		"dolly_get_group",
		"dolly_search",
		"dolly_get_order",
		"dolly_list_group_orders",
	}
	var disabledNames []string
	for _, d := range toolDefinitions(false, false) {
		disabledNames = append(disabledNames, checkTool(t, d))
	}
	if !slices.Equal(disabledNames, readOnlyWant) {
		t.Fatalf("tools (disabled) = %v, want %v", disabledNames, readOnlyWant)
	}

	var enabledNames []string
	for _, d := range toolDefinitions(true, false) {
		enabledNames = append(enabledNames, checkTool(t, d))
	}
	enabledWant := append(slices.Clone(readOnlyWant), "create_order")
	if !slices.Equal(enabledNames, enabledWant) {
		t.Fatalf("tools (enabled) = %v, want %v", enabledNames, enabledWant)
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
		srv.cfg.EnableCreateOrder = true
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
		srv.cfg.EnableCreateOrder = true
		srv.now = func() time.Time { return fixedNow }

		_, out := callOrderConfirmed(t, srv, map[string]any{
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
			// preview checks ownership, confirm re-checks it
			{"GET", "/api/v1/bruker/current", ""},
			{"GET", "/api/v1/gruppe/42/page/0", "pageSize=20"},
			{"GET", "/api/v1/bruker/current", ""},
			{"GET", "/api/v1/gruppe/42/page/0", "pageSize=20"},
			{"POST", "/api/v1/gruppe/42/bestilling", ""},
		}
		mu.Lock()
		defer mu.Unlock()
		if !slices.Equal(gotReqs, wantReqs) {
			t.Fatalf("upstream requests = %+v, want %+v", gotReqs, wantReqs)
		}

		var parsed dolly.BestillingRequest
		if err := json.Unmarshal(gotPostBody, &parsed); err != nil {
			t.Fatalf("unmarshal post body: %v", err)
		}
		if parsed.Antall != 3 || !slices.Equal(parsed.Environments, []string{"q1", "q2", "q4"}) {
			t.Fatalf("unexpected order top-level fields: %+v", parsed)
		}
		if parsed.Pdldata.OpprettNyPerson.Identtype != "FNR" || !parsed.Pdldata.OpprettNyPerson.Syntetisk {
			t.Fatalf("unexpected opprettNyPerson: %+v", parsed.Pdldata.OpprettNyPerson)
		}
		if parsed.Pdldata.OpprettNyPerson.FoedtEtter != "1985-11-20T00:00:00" ||
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
		srv.cfg.EnableCreateOrder = true

		_, out := callOrderConfirmed(t, srv, map[string]any{
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

	rejectCases := []struct {
		name      string
		callerID  string
		groupResp func(w http.ResponseWriter, r *http.Request)
	}{
		{"group without opprettetAv.brukerId is rejected before POST", "caller-123", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"id": 42})
		}},
		{"missing brukerId on both endpoints is rejected before POST", "", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"id": 42})
		}},
		{"whitespace brukerId on both endpoints is rejected before POST", " ", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"id": 42, "opprettetAv": map[string]any{"brukerId": " "}})
		}},
		{"404 on group lookup is rejected before POST", "caller-123", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}},
	}
	for _, tc := range rejectCases {
		t.Run(tc.name, func(t *testing.T) {
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
					writeJSON(w, http.StatusOK, map[string]any{"brukerId": tc.callerID})
				case "/api/v1/gruppe/42/page/0":
					tc.groupResp(w, r)
				default:
					writeJSON(w, http.StatusCreated, map[string]any{"id": 99})
				}
			})
			srv.cfg.EnableCreateOrder = true

			_, out := callOrderConfirmed(t, srv, map[string]any{
				"gruppeId":     42,
				"antall":       2,
				"environments": []string{"q1"},
			})
			if txt, isErr := extractToolText(t, out); !isErr {
				t.Fatalf("expected rejection, got success: %q", txt)
			}
			mu.Lock()
			defer mu.Unlock()
			if postCalled {
				t.Fatal("POST /bestilling was called although group ownership could not be verified")
			}
		})
	}
}

func TestCreateOrderPerUserRollingQuota(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	var (
		mu         sync.Mutex
		activeUser = "Z123456"
		postCount  int
		failPost   bool
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
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/gruppe/43/page/0":
			writeJSON(w, http.StatusOK, map[string]any{"id": 43, "opprettetAv": map[string]any{"brukerId": "someone-else"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/gruppe/42/bestilling":
			mu.Lock()
			fail := failPost
			if !fail {
				postCount++
			}
			mu.Unlock()
			if fail {
				http.Error(w, "bad order", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"id": 1})
		default:
			http.NotFound(w, r)
		}
	})
	srv.cfg.EnableCreateOrder = true
	srv.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}

	// Failed orders that provably did not reach Dolly or returned 4xx must refund
	// their reservation: an unowned group and a 400 must not count against the 20-person quota.
	_, out := callOrderConfirmed(t, srv, map[string]any{
		"gruppeId":     43,
		"antall":       5,
		"environments": []string{"q1"},
	})
	if txt, isErr := extractToolText(t, out); !isErr {
		t.Fatalf("order on unowned group should fail, got: %s", txt)
	}
	mu.Lock()
	failPost = true
	mu.Unlock()
	_, out = callOrderConfirmed(t, srv, map[string]any{
		"gruppeId":     42,
		"antall":       5,
		"environments": []string{"q1"},
	})
	if txt, isErr := extractToolText(t, out); !isErr {
		t.Fatalf("order with upstream 400 should fail, got: %s", txt)
	}
	mu.Lock()
	failPost = false
	mu.Unlock()

	// 4 orders of 5 persons = 20 persons (fills the 24h quota for Z123456).
	for i := range 4 {
		_, orderOut := callOrderConfirmed(t, srv, map[string]any{
			"gruppeId":     42,
			"antall":       5,
			"environments": []string{"q1"},
		})
		if txt, isErr := extractToolText(t, orderOut); isErr {
			t.Fatalf("order %d failed unexpectedly: %s", i+1, txt)
		}
	}

	// Verify the quota map is keyed by HMAC-SHA256 of NAVident (never plaintext or plain SHA-256).
	plainSHA := sha256.Sum256([]byte("Z123456"))
	plainSHAHex := hex.EncodeToString(plainSHA[:])
	hmacKey := srv.quota.hashUser("Z123456")
	otherQuota := newUserQuota(bytes.Repeat([]byte{9}, minKeyBytes))
	if otherQuota.hashUser("Z123456") == hmacKey {
		t.Fatal("expected the hash to depend on USER_HASH_KEY")
	}
	srv.quota.mu.Lock()
	_, hasPlain := srv.quota.entries["Z123456"]
	_, hasPlainSHA := srv.quota.entries[plainSHAHex]
	_, hasHashed := srv.quota.entries[hmacKey]
	srv.quota.mu.Unlock()
	if hasPlain || hasPlainSHA || !hasHashed {
		t.Fatalf("quota keys: hasPlain=%v hasPlainSHA=%v hasHashed=%v, want HMAC key only", hasPlain, hasPlainSHA, hasHashed)
	}

	// 21st person within 24h (at +23h) must be rejected without calling POST.
	mu.Lock()
	now = now.Add(23 * time.Hour)
	mu.Unlock()
	_, out = callOrderConfirmed(t, srv, map[string]any{
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
	_, out = callOrderConfirmed(t, srv, map[string]any{
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
	_, out = callOrderConfirmed(t, srv, map[string]any{
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
	srv.cfg.EnableCreateOrder = true
	srv.now = func() time.Time { return time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC) }

	_, out := callOrderConfirmed(t, srv, map[string]any{
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
	plainSHA := sha256.Sum256([]byte("Z123456"))
	for _, forbidden := range []string{
		"Z123456",
		srv.quota.hashUser("Z123456"),
		hex.EncodeToString(plainSHA[:]),
		"b 1&x",
		"KVINNE",
		"foedtEtter",
		"foedtFoer",
		"1990-",
		"1998-",
	} {
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
	if _, hasUser := entry["user"]; hasUser {
		t.Fatalf("audit log must not contain user field: %+v", entry)
	}
	if entry["gruppeId"] != float64(42) || entry["antall"] != float64(2) {
		t.Fatalf("audit gruppeId/antall = %v/%v, want 42/2", entry["gruppeId"], entry["antall"])
	}
	envs, _ := entry["environments"].([]any)
	if len(envs) != 2 || envs[0] != "q1" || envs[1] != "q4" {
		t.Fatalf("audit environments = %v, want [q1 q4]", entry["environments"])
	}
	for k := range entry {
		switch k {
		case "time", "level", "msg", "gruppeId", "antall", "environments":
		default:
			t.Fatalf("unexpected field %q in audit log: %+v", k, entry)
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCreateOrderQuotaRefundRules(t *testing.T) {
	var (
		mu       sync.Mutex
		postMode string
		failDial bool
	)

	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bruker/current":
			writeJSON(w, http.StatusOK, map[string]any{"brukerId": "b1"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/gruppe/42/page/0":
			writeJSON(w, http.StatusOK, map[string]any{"id": 42, "opprettetAv": map[string]any{"brukerId": "b1"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/gruppe/42/bestilling":
			mu.Lock()
			mode := postMode
			mu.Unlock()
			switch mode {
			case "4xx":
				http.Error(w, "invalid", http.StatusUnprocessableEntity)
			case "5xx":
				http.Error(w, "upstream failure", http.StatusInternalServerError)
			case "timeout":
				time.Sleep(150 * time.Millisecond)
				writeJSON(w, http.StatusCreated, map[string]any{"id": 1})
			case "bad201json":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":`))
			case "truncated201":
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":1}`))
				if fl, ok := w.(http.Flusher); ok {
					fl.Flush()
				}
				if hj, ok := w.(http.Hijacker); ok {
					if conn, _, err := hj.Hijack(); err == nil {
						_ = conn.Close()
					}
				}
			default:
				writeJSON(w, http.StatusCreated, map[string]any{"id": 1})
			}
		default:
			http.NotFound(w, r)
		}
	})
	srv.cfg.EnableCreateOrder = true
	baseTransport := srv.cfg.HTTPClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	srv.dolly.HTTPClient = &http.Client{
		Timeout: 60 * time.Millisecond,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			mu.Lock()
			dialErr := failDial && r.Method == http.MethodPost
			mu.Unlock()
			if dialErr {
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
			}
			return baseTransport.RoundTrip(r)
		}),
	}

	callOrder := func(antall int) (string, bool) {
		_, out := callOrderConfirmed(t, srv, map[string]any{
			"gruppeId":     42,
			"antall":       antall,
			"environments": []string{"q1"},
		})
		return extractToolText(t, out)
	}

	usedQuota := func() int {
		srv.quota.mu.Lock()
		defer srv.quota.mu.Unlock()
		total := 0
		for _, e := range srv.quota.entries[srv.quota.hashUser("Z123456")] {
			total += e.antall
		}
		return total
	}

	// 1. Dial error before POST leaves client -> provably did not reach Dolly -> MUST refund.
	mu.Lock()
	failDial = true
	mu.Unlock()
	if txt, isErr := callOrder(5); !isErr {
		t.Fatalf("expected error on dial failure, got %q", txt)
	}
	if got := usedQuota(); got != 0 {
		t.Fatalf("usedQuota after pre-send dial error = %d, want 0 (refunded)", got)
	}
	mu.Lock()
	failDial = false
	mu.Unlock()

	// 2. Dolly 4xx -> MUST refund.
	mu.Lock()
	postMode = "4xx"
	mu.Unlock()
	if txt, isErr := callOrder(5); !isErr {
		t.Fatalf("expected error on 4xx, got %q", txt)
	}
	if got := usedQuota(); got != 0 {
		t.Fatalf("usedQuota after 4xx = %d, want 0 (refunded)", got)
	}

	// 3. POST timeout -> POST reached Dolly -> MUST NOT refund.
	mu.Lock()
	postMode = "timeout"
	mu.Unlock()
	if txt, isErr := callOrder(5); !isErr {
		t.Fatalf("expected error on timeout, got %q", txt)
	}
	if got := usedQuota(); got != 5 {
		t.Fatalf("usedQuota after timeout = %d, want 5 (not refunded)", got)
	}

	// 4. Unparseable 201 JSON body -> order may exist -> MUST NOT refund.
	mu.Lock()
	postMode = "bad201json"
	mu.Unlock()
	if txt, isErr := callOrder(5); !isErr {
		t.Fatalf("expected error on invalid 201 JSON, got %q", txt)
	}
	if got := usedQuota(); got != 10 {
		t.Fatalf("usedQuota after invalid 201 JSON = %d, want 10 (not refunded)", got)
	}

	// 5. Truncated 201 body -> order may exist -> MUST NOT refund.
	mu.Lock()
	postMode = "truncated201"
	mu.Unlock()
	if txt, isErr := callOrder(5); !isErr {
		t.Fatalf("expected error on truncated 201 body, got %q", txt)
	}
	if got := usedQuota(); got != 15 {
		t.Fatalf("usedQuota after truncated 201 body = %d, want 15 (not refunded)", got)
	}

	// 6. Upstream 5xx -> order may exist -> MUST NOT refund.
	mu.Lock()
	postMode = "5xx"
	mu.Unlock()
	if txt, isErr := callOrder(5); !isErr {
		t.Fatalf("expected error on 5xx, got %q", txt)
	}
	if got := usedQuota(); got != 20 {
		t.Fatalf("usedQuota after 5xx = %d, want 20 (not refunded)", got)
	}

	// Quota is now full (20/20); next order must be rejected by quota.
	mu.Lock()
	postMode = "ok"
	mu.Unlock()
	if txt, isErr := callOrder(1); !isErr || !strings.Contains(txt, "quota") {
		t.Fatalf("expected quota rejection after non-refunded failures, got isErr=%v txt=%q", isErr, txt)
	}
}

func TestCreateOrderFeatureGate(t *testing.T) {
	listToolNames := func(t *testing.T, handler http.Handler) []string {
		t.Helper()
		rec, out := mcpCall(t, mcpComplianceTarget{
			handler: handler,
			header:  map[string]string{"Authorization": "Bearer in"},
		}, http.MethodPost, modernBody("tools/list", nil), modernHeaders("tools/list", ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("tools/list status = %d, body = %s", rec.Code, rec.Body.String())
		}
		res, _ := out["result"].(map[string]any)
		tools, _ := res["tools"].([]any)
		names := make([]string, 0, len(tools))
		for _, item := range tools {
			m, _ := item.(map[string]any)
			names = append(names, m["name"].(string))
		}
		return names
	}

	t.Run("disabled by default", func(t *testing.T) {
		srv, got := recordingDolly(t)
		if srv.cfg.EnableCreateOrder {
			t.Fatal("expected EnableCreateOrder default false")
		}
		names := listToolNames(t, srv.routes())
		if slices.Contains(names, "create_order") {
			t.Fatalf("create_order must not be listed when disabled: %v", names)
		}

		_, out := callOrderConfirmed(t, srv, map[string]any{
			"gruppeId":     42,
			"antall":       1,
			"environments": []string{"q1"},
		})
		if errCode(out) != mcpCodeInvalidParams {
			t.Fatalf("expected JSON-RPC error %d when disabled, got %+v", mcpCodeInvalidParams, out)
		}
		if reqs := got(); len(reqs) != 0 {
			t.Fatalf("disabled create_order reached upstream: %+v", reqs)
		}
	})

	t.Run("enabled when configured", func(t *testing.T) {
		srv, got := recordingDolly(t)
		srv.cfg.EnableCreateOrder = true
		names := listToolNames(t, srv.routes())
		if !slices.Contains(names, "create_order") {
			t.Fatalf("create_order must be listed when enabled: %v", names)
		}

		_, out := callOrderConfirmed(t, srv, map[string]any{
			"gruppeId":     42,
			"antall":       1,
			"environments": []string{"q1"},
		})
		if txt, isErr := extractToolText(t, out); isErr {
			t.Fatalf("enabled create_order failed: %s", txt)
		}
		if reqs := got(); len(reqs) != 5 || reqs[4].method != http.MethodPost {
			t.Fatalf("expected POST /bestilling when enabled, got %+v", reqs)
		}
	})
}

func TestCreateOrderAgeRangeBounds(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		args      map[string]any
		wantEtter string
		wantFoer  string
	}{
		{
			name:      "minAlder equals maxAlder covers full year",
			args:      map[string]any{"gruppeId": 1, "antall": 1, "environments": []string{"q1"}, "minAlder": 25, "maxAlder": 25},
			wantEtter: "2000-11-20T00:00:00",
			wantFoer:  "2001-11-19T00:00:00",
		},
		{
			name:      "age 0 covers full year",
			args:      map[string]any{"gruppeId": 1, "antall": 1, "environments": []string{"q1"}, "minAlder": 0, "maxAlder": 0},
			wantEtter: "2025-11-20T00:00:00",
			wantFoer:  "2026-11-19T00:00:00",
		},
		{
			name:      "only minAlder set",
			args:      map[string]any{"gruppeId": 1, "antall": 1, "environments": []string{"q1"}, "minAlder": 18},
			wantEtter: "",
			wantFoer:  "2008-11-19T00:00:00",
		},
		{
			name:      "only maxAlder set",
			args:      map[string]any{"gruppeId": 1, "antall": 1, "environments": []string{"q1"}, "maxAlder": 67},
			wantEtter: "1958-11-20T00:00:00",
			wantFoer:  "",
		},
		{
			name:      "range 25 to 40",
			args:      map[string]any{"gruppeId": 1, "antall": 1, "environments": []string{"q1"}, "minAlder": 25, "maxAlder": 40},
			wantEtter: "1985-11-20T00:00:00",
			wantFoer:  "2001-11-19T00:00:00",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawArgs, err := json.Marshal(tc.args)
			if err != nil {
				t.Fatalf("marshal args: %v", err)
			}
			fields, rpcErr := decodeArgMap(rawArgs, map[string]bool{
				"gruppeId": true, "antall": true, "environments": true, "minAlder": true, "maxAlder": true,
			})
			if rpcErr != nil {
				t.Fatalf("decodeArgMap: %+v", rpcErr)
			}
			_, req, rpcErr := parseCreateOrder(fields, now)
			if rpcErr != nil {
				t.Fatalf("parseCreateOrder: %+v", rpcErr)
			}
			got := req.Pdldata.OpprettNyPerson
			if got.FoedtEtter != tc.wantEtter || got.FoedtFoer != tc.wantFoer {
				t.Fatalf("FoedtEtter=%q FoedtFoer=%q, want %q / %q", got.FoedtEtter, got.FoedtFoer, tc.wantEtter, tc.wantFoer)
			}
		})
	}
}
