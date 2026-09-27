package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Opaque test tokens that only the fake Texas below knows, built at run time.
var (
	cliToken     = "tok-" + "cli"
	webToken     = "tok-" + "web"
	cliUserToken = "tok-" + "cli-user"
	otherApp     = "tok-" + "other-app"
	webAppToken  = "tok-" + "web-app"
	roleOnly     = "tok-" + "role-only"
)

const preAuthorized = `[{"name":"dev-gcp:copilot:copilot-cli","clientId":"cli-id"},{"name":"dev-gcp:copilot:my-copilot","clientId":"web-id"}]`

// testAuthenticator answers introspection for the tokens above.
func testAuthenticator(t *testing.T) *authenticator {
	t.Helper()
	claims := map[string]map[string]any{
		cliToken:     {"active": true, "azp": "cli-id", "idtyp": "app", "roles": []string{"access_as_application"}},
		roleOnly:     {"active": true, "azp": "cli-id", "roles": []string{"access_as_application"}},
		webToken:     {"active": true, "azp": "web-id", "NAVident": "Z999999", "preferred_username": "Hans.Test@nav.no"},
		cliUserToken: {"active": true, "azp": "cli-id", "NAVident": "Z999999", "preferred_username": "Hans.Test@nav.no"},
		otherApp:     {"active": true, "azp": "other-id", "idtyp": "app"},
		webAppToken:  {"active": true, "azp": "web-id", "idtyp": "app"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		c, ok := claims[body.Token]
		if !ok {
			c = map[string]any{"active": false}
		}
		_ = json.NewEncoder(w).Encode(c)
	}))
	t.Cleanup(srv.Close)
	return newAuthenticator(srv.URL, preAuthorized)
}

func TestAuthBranches(t *testing.T) {
	a := testAuthenticator(t)
	for _, tc := range []struct {
		name, token, header string
		want                caller
		err                 error
	}{
		{"copilot-cli app token", cliToken, "hans", caller{login: "hans"}, nil},
		{"copilot-cli, role without idtyp", roleOnly, "hans", caller{login: "hans"}, nil},
		{"my-copilot user token", webToken, "", caller{email: "Hans.Test@nav.no"}, nil},
		{"copilot-cli without the header", cliToken, "", caller{}, errBadHeader},
		{"copilot-cli with a malformed login", cliToken, "inv@lid", caller{}, errBadHeader},
		{"user token with the header", webToken, "someone-else", caller{}, errBadHeader},
		{"user token from copilot-cli's azp", cliUserToken, "", caller{}, errForbidden},
		{"app token from another app", otherApp, "hans", caller{}, errForbidden},
		{"app token from my-copilot", webAppToken, "hans", caller{}, errForbidden},
		{"inactive", "nope", "", caller{}, errUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.resolve(t.Context(), tc.token, tc.header)
			if !errors.Is(err, tc.err) || (err == nil && *got != tc.want) {
				t.Fatalf("got %+v, %v; want %+v, %v", got, err, tc.want, tc.err)
			}
		})
	}
}

func TestAuthOffWithoutPreAuthorizedApps(t *testing.T) {
	a := testAuthenticator(t)
	a.cliID, a.webID = "", ""
	for _, tok := range []string{cliToken, webToken} {
		if _, err := a.resolve(t.Context(), tok, map[string]string{cliToken: "hans"}[tok]); !errors.Is(err, errForbidden) {
			t.Fatalf("%s: %v, want errForbidden", tok, err)
		}
	}
}

func TestClientIDForApp(t *testing.T) {
	if id := clientIDForApp(preAuthorized, "copilot-cli"); id != "cli-id" {
		t.Fatalf("copilot-cli = %q", id)
	}
	for name, raw := range map[string]string{
		"missing":   `[{"name":"dev-gcp:copilot:x","clientId":"x"}]`,
		"ambiguous": `[{"name":"a:b:copilot-cli","clientId":"1"},{"name":"c:d:copilot-cli","clientId":"2"}]`,
		"malformed": `nope`,
		"empty":     ``,
	} {
		if id := clientIDForApp(raw, "copilot-cli"); id != "" {
			t.Errorf("%s: %q, want empty", name, id)
		}
	}
}

func TestNameIDClient(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"m2m","expires_in":3600}`))
			return
		}
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		switch {
		case r.Method != http.MethodPost || r.URL.Path != "/internal/v1/saml/name-id":
			w.WriteHeader(http.StatusTeapot)
		case strings.Contains(gotBody, `"nosso"`):
			w.WriteHeader(http.StatusNotFound)
		case strings.Contains(gotBody, `"broken"`):
			w.WriteHeader(http.StatusBadGateway)
		default:
			_, _ = w.Write([]byte(`{"name_id":"hans.test@nav.no"}`))
		}
	}))
	defer srv.Close()
	c := newNameIDClient(srv.URL+"/", newTexasClient(srv.URL+"/token", "api://x"))

	if email, err := c.emailFor(t.Context(), &caller{login: "hans"}); err != nil || email != "hans.test@nav.no" {
		t.Fatalf("hans: %q, %v", email, err)
	}
	if gotAuth != "Bearer m2m" || gotBody != `{"login":"hans"}` {
		t.Fatalf("request: %q %q", gotAuth, gotBody)
	}
	if _, err := c.emailFor(t.Context(), &caller{login: "nosso"}); !errors.Is(err, errNoNavIdentity) {
		t.Fatalf("nosso: %v", err)
	}
	if _, err := c.emailFor(t.Context(), &caller{login: "broken"}); err == nil || strings.Contains(err.Error(), "broken") {
		t.Fatalf("broken: %v", err)
	}
	if email, err := c.emailFor(context.Background(), &caller{email: "web@nav.no"}); err != nil || email != "web@nav.no" {
		t.Fatalf("web: %q, %v", email, err)
	}
}
