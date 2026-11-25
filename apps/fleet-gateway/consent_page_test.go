package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/fleet-gateway/internal/oauth/server"
)

func TestConsentPage_ContentAndCSP(t *testing.T) {
	te := newTestEnv(t)
	resp := te.callback(te.startLogin(consentVerifier))
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	body := string(raw)

	csp := resp.Header.Get("Content-Security-Policy")
	formAction := strings.TrimSuffix(strings.SplitN(csp, "form-action ", 2)[1], "; frame-ancestors 'none'")
	want := "default-src 'none'; style-src 'self'; form-action " + formAction + "; frame-ancestors 'none'"
	if csp != want || !strings.HasPrefix(formAction, "'self' http://127.0.0.1:") {
		t.Fatalf("CSP = %q", csp)
	}
	for _, s := range []string{
		`<html lang="nb">`,
		`<link rel="stylesheet" href="/static/consent.css">`,
		"<h1>Vil du gi ",
		"på dine vegne, med Nav-identiteten din",
		"Gir du tilgang, huskes valget i 30 dager",
		"logger ut og trekker tilbake tilgangen",
		"<details>", "<summary>Vis tekniske detaljer</summary>",
		"<code>" + te.clientID + "</code>",
		`value="approve">Gi tilgang</button>`,
		`value="deny">Avslå</button>`,
	} {
		if !strings.Contains(body, s) {
			t.Errorf("consent page missing %q", s)
		}
	}
	if strings.Contains(body, "style=") || strings.Contains(body, "<style") {
		t.Error("inline styles are blocked by the CSP")
	}
}

func TestConsentPage_CSRFFailureIsFriendlyPage(t *testing.T) {
	te := newTestEnv(t)
	id, _, cookie, _ := te.startAuthorizeAndConsent(consentVerifier)
	resp := te.postConsent(id, "wrong", cookie, "approve")
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'self'; form-action 'none'; frame-ancestors 'none'" {
		t.Fatalf("CSP = %q", got)
	}
	if !strings.Contains(string(raw), `role="alert"`) || !strings.Contains(string(raw), "Forespørselen er utløpt") {
		t.Fatalf("expected friendly error page, got %s", raw)
	}
}

func TestConsentPage_CSRFFailurePreservesPendingConsent(t *testing.T) {
	te := newTestEnv(t)
	id, csrf, cookie, _ := te.startAuthorizeAndConsent(consentVerifier)

	// Missing cookie and mismatched form CSRF must be rejected without popping the pending row.
	missingCookieResp := te.postConsent(id, csrf, nil, "approve")
	_ = missingCookieResp.Body.Close()
	if missingCookieResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 on missing CSRF cookie, got %d", missingCookieResp.StatusCode)
	}

	wrongCSRFResp := te.postConsent(id, "wrong", cookie, "approve")
	_ = wrongCSRFResp.Body.Close()
	if wrongCSRFResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 on wrong CSRF token, got %d", wrongCSRFResp.StatusCode)
	}

	// The user's legitimate submission still finds the pending consent row and succeeds.
	resp := te.postConsent(id, csrf, cookie, "approve")
	defer func() { _ = resp.Body.Close() }()
	if code := te.clientCode(resp); code == "" {
		t.Fatal("expected valid consent submission to succeed after CSRF failure")
	}
}

func TestConsentCSSHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	server.ConsentCSS(rec, httptest.NewRequest(http.MethodGet, "/static/consent.css", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "text/css; charset=utf-8" || rec.Body.Len() == 0 {
		t.Fatalf("content-type %q, %d bytes", ct, rec.Body.Len())
	}
}
