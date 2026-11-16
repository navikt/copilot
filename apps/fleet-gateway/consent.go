package main

import (
	"crypto/subtle"
	"html/template"
	"net/http"
	"net/url"
)

const csrfCookieName = "fleet_csrf"

var consentTemplate = template.Must(template.New("consent").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>fleet-gateway consent</title>
</head>
<body style="font-family:sans-serif;max-width:40em;margin:2em auto;padding:0 1em">
<h1>Allow access?</h1>
<p><strong>{{.ClientName}}</strong> (<code>{{.ClientID}}</code>) requests access to <strong>{{.Resource}}</strong> on your behalf.</p>
<p>Redirect destination: <code>{{.RedirectURI}}</code></p>
<form method="post" action="/consent">
<input type="hidden" name="id" value="{{.ID}}">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<button type="submit" name="decision" value="approve">Allow</button>
<button type="submit" name="decision" value="deny">Deny</button>
</form>
</body>
</html>`))

func setConsentSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (g *Gateway) renderConsentPage(w http.ResponseWriter, consentID string, p *pendingAuth) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    p.CSRF,
		Path:     "/consent",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(defaultPendingTTL.Seconds()),
	})
	setConsentSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = consentTemplate.Execute(w, map[string]string{
		"ID":          consentID,
		"CSRF":        p.CSRF,
		"ClientName":  p.ClientName,
		"ClientID":    p.ClientID,
		"RedirectURI": p.RedirectURI,
		"Resource":    p.Resource,
	})
}

func (g *Gateway) consentPostHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
	if err := r.ParseForm(); err != nil {
		clearCSRFCookie(w)
		http.Error(w, "invalid consent form submission", http.StatusBadRequest)
		return
	}

	consentID := r.PostFormValue("id")
	formCSRF := r.PostFormValue("csrf")
	cookie, err := r.Cookie(csrfCookieName)

	clearCSRFCookie(w)

	p, ok := g.pendingConsent.Pop(consentID)
	if !ok || err != nil || formCSRF == "" ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(formCSRF)) != 1 ||
		subtle.ConstantTimeCompare([]byte(formCSRF), []byte(p.CSRF)) != 1 {
		http.Error(w, "consent request expired or CSRF check failed; start the login again", http.StatusForbidden)
		return
	}

	if r.PostFormValue("decision") != "approve" {
		target, parseErr := url.Parse(p.RedirectURI)
		if parseErr != nil {
			http.Error(w, "invalid stored redirect_uri", http.StatusInternalServerError)
			return
		}
		redirectWithValues(w, r, target, url.Values{
			"error": {"access_denied"},
			"state": {p.State},
			"iss":   {g.cfg.BaseURL},
		})
		return
	}

	g.redirectToEntra(w, r, p)
}

func clearCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/consent",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
