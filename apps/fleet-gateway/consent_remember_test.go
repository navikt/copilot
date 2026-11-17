package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const consentVerifier = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

// asksConsent logs in and reports whether the consent page was shown.
func (te *testEnv) asksConsent() bool {
	te.t.Helper()
	resp := te.callback(te.startLogin(consentVerifier))
	_ = resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true
	case http.StatusFound:
		return false
	default:
		te.t.Fatalf("unexpected callback status %d", resp.StatusCode)
		return false
	}
}

func TestConsent_RememberedPerUserClientRoute(t *testing.T) {
	ctx := context.Background()

	t.Run("same user, client and route skips the page", func(t *testing.T) {
		te := newTestEnv(t)
		if !te.asksConsent() {
			t.Fatal("first login must ask")
		}
		_ = te.obtainAuthCode(consentVerifier)
		if te.asksConsent() {
			t.Fatal("second login asked again")
		}
	})

	t.Run("consent for another client, route or user does not count", func(t *testing.T) {
		te := newTestEnv(t)
		for _, c := range [][3]string{
			{te.navIdent, "https://other.example/client.json", te.resource},
			{te.navIdent, te.clientID, te.resource + "-other"},
			{"X999999", te.clientID, te.resource},
		} {
			if err := te.gw.rememberConsent(ctx, c[0], c[1], c[2]); err != nil {
				t.Fatal(err)
			}
		}
		if !te.asksConsent() {
			t.Fatal("consent leaked across user, client or route")
		}
	})

	t.Run("expired consent asks again", func(t *testing.T) {
		te := newTestEnv(t)
		_ = te.obtainAuthCode(consentVerifier)
		te.advance(te.gw.cfg.ConsentTTL + time.Second)
		if !te.asksConsent() {
			t.Fatal("expired consent was still used")
		}
	})

	t.Run("prompt=consent always asks", func(t *testing.T) {
		te := newTestEnv(t)
		_ = te.obtainAuthCode(consentVerifier)
		req := httptestGet(te.authorizeURL(map[string]string{"prompt": "consent"}, consentVerifier))
		resp := te.callback(te.followToEntra(req))
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("prompt=consent skipped the page: %d", resp.StatusCode)
		}
	})

	t.Run("deny does not remember", func(t *testing.T) {
		te := newTestEnv(t)
		id, csrf, cookie, _ := te.startAuthorizeAndConsent(consentVerifier)
		_ = te.postConsent(id, csrf, cookie, "deny").Body.Close()
		if !te.asksConsent() {
			t.Fatal("denied consent was remembered")
		}
	})
}

func TestConsent_RevokeClearsConsent(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt, _ := te.obtainTokensForResource(te.resource)
	if te.asksConsent() {
		t.Fatal("expected consent to be remembered before revoke")
	}
	if code, _ := postRevoke(te.handler, rt, te.clientID); code != http.StatusOK {
		t.Fatalf("revoke: %d", code)
	}
	if !te.asksConsent() {
		t.Fatal("revoke did not clear remembered consent")
	}
}

func TestConsent_PageBannerAndFormAction(t *testing.T) {
	te := newTestEnv(t)
	page := func() (string, http.Header) {
		resp := te.callback(te.startLogin(consentVerifier))
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return string(body), resp.Header
	}

	body, h := page()
	if strings.Contains(body, "Unverified client") {
		t.Fatal("allowlisted client shown as unverified")
	}
	if csp := h.Get("Content-Security-Policy"); !strings.Contains(csp, "form-action 'self' http://127.0.0.1:") {
		t.Fatalf("form-action must allow the redirect back to the client: %q", csp)
	}

	// On main every client that reaches /authorize is allowlisted; drop it after
	// the redirect to Entra to stand in for a DCR client.
	st, sc := te.startLogin(consentVerifier)
	delete(te.gw.cfg.AllowedClientIDs, te.clientID)
	resp := te.callback(st, sc)
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(raw), "Unverified client") {
		t.Fatal("expected unverified banner for a client off the allowlist")
	}
}

func httptestGet(target string) *http.Request {
	return httptest.NewRequest(http.MethodGet, target, nil)
}
