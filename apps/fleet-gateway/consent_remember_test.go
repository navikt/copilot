package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/oauth"
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
	if strings.Contains(body, "uverifisert verktøy") {
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
	if !strings.Contains(string(raw), "uverifisert verktøy") {
		t.Fatal("expected unverified banner for a client off the allowlist")
	}
}

func httptestGet(target string) *http.Request {
	return httptest.NewRequest(http.MethodGet, target, nil)
}

// A rolling deploy runs old pods (consent before Entra, pending_entra) next to
// new ones. Neither may complete the other's login without the consent page.
func TestConsent_RollingDeployNeverSkipsConsent(t *testing.T) {
	ctx := context.Background()

	t.Run("new pod's pending state is invisible to old pods", func(t *testing.T) {
		te := newTestEnv(t)
		st, _ := te.startLogin(consentVerifier)
		if _, ok, _ := te.gw.store.Get(ctx, "pending_entra", st); ok {
			t.Fatal("new flow wrote the kind old pods pop")
		}
		if _, ok, _ := te.gw.store.Get(ctx, kindPendingEntra, oauth.HashToken(st)); !ok {
			t.Fatal("new flow did not store its pending state")
		}
	})

	t.Run("old pod's pending state always gets the consent page", func(t *testing.T) {
		te := newTestEnv(t)
		_ = te.obtainAuthCode(consentVerifier) // consent now remembered
		st, sc := te.startLogin(consentVerifier)
		raw, ok, err := te.gw.store.Pop(ctx, kindPendingEntra, oauth.HashToken(st))
		if err != nil || !ok {
			t.Fatalf("pop: %v %v", ok, err)
		}
		if err := te.gw.store.Put(ctx, kindPendingEntraLegacy, st, raw, time.Minute); err != nil {
			t.Fatal(err)
		}
		resp := te.callback(st, sc)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("legacy pending state skipped consent: %d", resp.StatusCode)
		}
	})
}

type failConsentDeleteStore struct{ SessionStore }

func (s failConsentDeleteStore) Delete(ctx context.Context, kind, key string) error {
	if kind == kindConsent {
		return io.ErrUnexpectedEOF
	}
	return s.SessionStore.Delete(ctx, kind, key)
}

// A failed consent delete must leave the session, so a retried revoke can
// still find it and clear consent instead of returning 200 with consent kept.
func TestConsent_RevokeKeepsSessionWhenConsentDeleteFails(t *testing.T) {
	te := newTestEnv(t)
	at, _, _ := te.obtainTokensForResource(te.resource)
	if _, ok, _ := te.gw.store.Get(context.Background(), kindSession, oauth.HashToken(at)); !ok {
		t.Fatal("precondition: expected a stored session")
	}
	inner := te.gw.store
	te.gw.store = failConsentDeleteStore{inner}
	if code, _ := postRevoke(te.handler, at, te.clientID); code != http.StatusServiceUnavailable {
		t.Fatalf("revoke with failing consent delete: %d", code)
	}
	if _, ok, _ := inner.Get(context.Background(), kindSession, oauth.HashToken(at)); !ok {
		t.Fatal("session deleted although consent delete failed")
	}
}

func TestConsent_EvictsOldestAtCap(t *testing.T) {
	ctx := context.Background()
	te := newTestEnv(t)
	te.gw.store.Close()
	te.gw.store = newMemoryStore(te.gw.now, map[string]int{kindConsent: 2})

	if err := te.gw.rememberConsent(ctx, "A100001", te.clientID, te.resource); err != nil {
		t.Fatalf("first rememberConsent: %v", err)
	}
	te.advance(time.Hour)
	if err := te.gw.rememberConsent(ctx, "A100002", te.clientID, te.resource); err != nil {
		t.Fatalf("second rememberConsent: %v", err)
	}
	te.advance(time.Hour)
	// Cap is 2: third consent must succeed by evicting the earliest-expiring one (A100001).
	if err := te.gw.rememberConsent(ctx, te.navIdent, te.clientID, te.resource); err != nil {
		t.Fatalf("third rememberConsent at cap: %v", err)
	}

	okNew, err := te.gw.hasConsent(ctx, te.navIdent, te.clientID, te.resource)
	if err != nil || !okNew {
		t.Fatalf("expected newest consent to be remembered: ok=%v err=%v", okNew, err)
	}
	okKept, err := te.gw.hasConsent(ctx, "A100002", te.clientID, te.resource)
	if err != nil || !okKept {
		t.Fatalf("expected second consent to remain: ok=%v err=%v", okKept, err)
	}
	okEvicted, err := te.gw.hasConsent(ctx, "A100001", te.clientID, te.resource)
	if err != nil || okEvicted {
		t.Fatalf("expected oldest consent to be evicted: ok=%v err=%v", okEvicted, err)
	}
}
