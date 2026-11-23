package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	gatewayID  = "11111111-1111-1111-1111-111111111111"
	registryID = "22222222-2222-2222-2222-222222222222"
)

// fakeTexas treats the token as the azp of an active app token, with special
// tokens for the other outcomes.
func fakeTexas(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Token string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		switch in.Token {
		case "texas-500":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"active":true,"idtyp":"app","azp":"` + gatewayID + `"}`))
		case "texas-garbage":
			_, _ = w.Write([]byte("{not json"))
		case "texas-trailing":
			_, _ = w.Write([]byte(`{"active":true,"idtyp":"app","azp":"` + gatewayID + `"}not-json`))
		case "texas-oversize":
			_, _ = w.Write([]byte(`{"active":true,"idtyp":"app","azp":"` + gatewayID + `"}` + strings.Repeat(" ", 1<<20)))
		case "inactive":
			_, _ = w.Write([]byte(`{"active":false,"idtyp":"app","azp":"` + gatewayID + `"}`))
		case "app-with-ident":
			_, _ = w.Write([]byte(`{"active":true,"idtyp":"app","azp":"44444444-4444-4444-4444-444444444444","NAVident":"A111111"}`))
		case "user-token":
			_, _ = w.Write([]byte(`{"active":true,"azp":"` + gatewayID + `","NAVident":"Z999999"}`))
		default:
			// "claims:<json>" returns exactly those claims, marked active.
			if raw, ok := strings.CutPrefix(in.Token, "claims:"); ok {
				claims := map[string]any{}
				_ = json.Unmarshal([]byte(raw), &claims)
				claims["active"] = true
				_ = json.NewEncoder(w).Encode(claims)
				return
			}
			// "user:<azp>:<NAVident>[:<preferred_username>]" is a person's token
			// obtained by <azp>. Without a fourth part preferred_username is the
			// NAVident, so test team lists can key on it; an empty fourth part
			// leaves the claim out.
			if parts := strings.Split(in.Token, ":"); (len(parts) == 3 || len(parts) == 4) && parts[0] == "user" {
				claims := map[string]any{"active": true, "azp": parts[1], "NAVident": parts[2], "preferred_username": parts[2], "tid": testTenant}
				if len(parts) == 4 {
					claims["preferred_username"] = parts[3]
					if parts[3] == "" {
						delete(claims, "preferred_username")
					}
				}
				_ = json.NewEncoder(w).Encode(claims)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "idtyp": "app", "azp": in.Token})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testAuthz(t *testing.T) *authz {
	t.Helper()
	a, err := newAuthz(fakeTexas(t).URL, gatewayID, registryID)
	if err != nil {
		t.Fatal(err)
	}
	a.homeTenant = testTenant
	return a
}

// TestAuthzMatrix runs every endpoint against every caller. The handlers
// behind the guard return 503 (no database), so anything not 401/403 got
// through.
func TestAuthzMatrix(t *testing.T) {
	h := newServer().routes(testAuthz(t), "", nil, nil)
	const through = http.StatusServiceUnavailable
	callers := map[string]string{"gateway": gatewayID, "registry": registryID, "other": "33333333-3333-3333-3333-333333333333", "prefix": gatewayID[:8]}
	want := map[string]map[string]int{
		"/internal/v1/servers":            {"gateway": 403, "registry": through, "other": 403, "prefix": 403},
		"/internal/v1/routes":             {"gateway": through, "registry": 403, "other": 403, "prefix": 403},
		"/internal/v1/snapshots/routes":   {"gateway": through, "registry": 403, "other": 403, "prefix": 403},
		"/internal/v1/snapshots/registry": {"gateway": 403, "registry": through, "other": 403, "prefix": 403},
	}
	for path, row := range want {
		for name, code := range row {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+callers[name])
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != code {
				t.Errorf("%s as %s: got %d, want %d", path, name, rec.Code, code)
			}
		}
		for _, hdr := range []string{"", "Basic " + gatewayID, "Bearer ", "Bearer a b", "Bearer inactive", "Bearer user-token", "bearer" + strings.Repeat("x", 17000)} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if hdr != "" {
				req.Header.Set("Authorization", hdr)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Header().Get("WWW-Authenticate"), "invalid_token") {
				t.Errorf("%s with %.20q: got %d, want 401", path, hdr, rec.Code)
			}
		}
	}
}

func TestAuthzTexasFailures(t *testing.T) {
	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	deadAuthz, err := newAuthz(down.URL, gatewayID, registryID)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		a   *authz
		tok string
	}{
		"texas 500":      {testAuthz(t), "texas-500"},
		"texas garbage":  {testAuthz(t), "texas-garbage"},
		"texas trailing": {testAuthz(t), "texas-trailing"},
		"texas oversize": {testAuthz(t), "texas-oversize"},
		"texas down":     {deadAuthz, gatewayID},
	}
	for name, c := range cases {
		reached := false
		h := c.a.require(func(http.ResponseWriter, *http.Request) { reached = true }, callerGateway)
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/routes", nil)
		req.Header.Set("Authorization", "Bearer "+c.tok)
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != http.StatusServiceUnavailable || reached {
			t.Errorf("%s: got %d reached=%v, want 503 and no data", name, rec.Code, reached)
		}
	}
}

func TestNewAuthzFailsClosed(t *testing.T) {
	for name, args := range map[string][3]string{
		"no endpoint":     {"", gatewayID, registryID},
		"bad endpoint":    {"ftp://x", gatewayID, registryID},
		"no gateway id":   {"http://texas", "", registryID},
		"no registry id":  {"http://texas", gatewayID, ""},
		"same client ids": {"http://texas", gatewayID, gatewayID},
	} {
		if _, err := newAuthz(args[0], args[1], args[2]); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

const testTenant = "62366534-1ec3-4962-8869-9b5535279d0b"

// Person routes take member accounts of the home tenant only. A claim that is
// missing or unexpected is refused like a guest.
func TestUserRefusesGuests(t *testing.T) {
	a := testAuthz(t)
	tok := func(extra string) string {
		return `claims:{"azp":"` + adminID + `","NAVident":"A111111","preferred_username":"kari.nordmann@example.com"` + extra + `}`
	}
	cases := map[string]struct {
		tok string
		ok  bool
	}{
		"member, acct 0":        {tok(`,"acct":0,"tid":"` + testTenant + `"`), true},
		"member, no acct":       {tok(`,"tid":"` + testTenant + `"`), true},
		"member, idtyp user":    {tok(`,"idtyp":"user","tid":"` + testTenant + `"`), true},
		"guest, acct 1":         {tok(`,"acct":1,"tid":"` + testTenant + `"`), false},
		"acct unexpected":       {tok(`,"acct":2,"tid":"` + testTenant + `"`), false},
		"no acct, no tid":       {tok(``), false},
		"no acct, other tenant": {tok(`,"tid":"00000000-0000-4000-8000-000000000001"`), false},
		"idtyp unexpected":      {tok(`,"idtyp":"device","tid":"` + testTenant + `"`), false},
		"no acct, #EXT# upn":    {`claims:{"azp":"` + adminID + `","NAVident":"A111111","tid":"` + testTenant + `","preferred_username":"kari_example.com#ext#@nav.onmicrosoft.com"}`, false},
	}
	for name, c := range cases {
		_, _, err := a.user(context.Background(), "Bearer "+c.tok, adminID)
		if (err == nil) != c.ok {
			t.Errorf("%s: err %v, want ok=%v", name, err, c.ok)
		}
	}
	a.homeTenant = ""
	if _, _, err := a.user(context.Background(), "Bearer "+tok(`,"tid":"`+testTenant+`"`), adminID); err == nil {
		t.Error("no home tenant configured: want refusal")
	}
}
