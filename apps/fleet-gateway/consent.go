package main

import (
	"crypto/subtle"
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

const csrfCookieName = "__Host-fleet_csrf"

//go:embed web/pages.html
var pagesHTML string

//go:embed web/consent.css
var consentCSS []byte

var pageTemplates = template.Must(template.New("pages").Parse(pagesHTML))

// consentCSSHandler serves the stylesheet for the consent and error pages.
func consentCSSHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(consentCSS)
}

// renderErrorPage shows a user-facing error in the same look as the consent page.
func renderErrorPage(w http.ResponseWriter, status int, title, message, action string) {
	setPageSecurityHeaders(w, "'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTemplates.ExecuteTemplate(w, "error", map[string]string{"Title": title, "Message": message, "Action": action})
}

const startAgain = "Gå tilbake til verktøyet ditt og logg inn på nytt."

// setConsentSecurityHeaders allows the form to post to the gateway and to
// follow the redirect back to the client (Chromium applies form-action to it).
func setConsentSecurityHeaders(w http.ResponseWriter, redirectURI string) {
	formAction := "'self'"
	if u, err := url.Parse(redirectURI); err == nil && u.Scheme != "" && u.Host != "" {
		formAction += " " + u.Scheme + "://" + u.Host
	}
	setPageSecurityHeaders(w, formAction)
}

func setPageSecurityHeaders(w http.ResponseWriter, formAction string) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; form-action "+formAction+"; frame-ancestors 'none'")
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
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(defaultPendingTTL.Seconds()),
	})
	setConsentSecurityHeaders(w, p.RedirectURI)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	route := strings.TrimPrefix(p.Resource, g.cfg.BaseURL)
	_ = pageTemplates.ExecuteTemplate(w, "consent", map[string]any{
		"Route":        route,
		"Team":         g.cfg.RoutesByPath[route].Team,
		"Unverified":   p.Unverified || !g.cfg.AllowedClientIDs[p.ClientID],
		"RememberDays": int(g.cfg.ConsentTTL.Hours() / 24),
		"ID":           consentID,
		"CSRF":         p.CSRF,
		"ClientName":   p.ClientName,
		"ClientID":     p.ClientID,
		"RedirectURI":  p.RedirectURI,
		"Resource":     p.Resource,
	})
}

func (g *Gateway) consentPostHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
	if err := r.ParseForm(); err != nil {
		clearCSRFCookie(w)
		renderErrorPage(w, http.StatusBadRequest, "Ugyldig skjema", "Svaret ditt kom ikke fram i riktig format.", startAgain)
		return
	}

	consentID := r.PostFormValue("id")
	formCSRF := r.PostFormValue("csrf")
	cookie, cookieErr := r.Cookie(csrfCookieName)

	if consentID == "" || cookieErr != nil || formCSRF == "" ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(formCSRF)) != 1 {
		renderErrorPage(w, http.StatusForbidden, "Forespørselen er utløpt",
			"Vi kunne ikke bekrefte svaret ditt. Siden kan ha vært åpen for lenge, du kan allerede ha svart, eller svaret kom fra en annen fane.", startAgain)
		return
	}

	clearCSRFCookie(w)

	p, ok, err := g.popPending(r.Context(), kindPendingConsent, consentID)
	if err != nil {
		slog.Warn("pop pending consent failed", "error", err)
		renderErrorPage(w, http.StatusServiceUnavailable, "Tjenesten er ikke tilgjengelig", "Gatewayen får ikke kontakt med lageret sitt akkurat nå.", "Vent litt, og logg så inn på nytt fra verktøyet ditt.")
		return
	}
	if !ok || subtle.ConstantTimeCompare([]byte(formCSRF), []byte(p.CSRF)) != 1 {
		renderErrorPage(w, http.StatusForbidden, "Forespørselen er utløpt",
			"Vi kunne ikke bekrefte svaret ditt. Siden kan ha vært åpen for lenge, du kan allerede ha svart, eller svaret kom fra en annen fane.", startAgain)
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

	if p.Issued == nil {
		// Pending consent from a pod that still asked before the Entra login.
		g.redirectToEntra(w, r, p)
		return
	}
	if err = g.rememberConsent(r.Context(), p.Issued.Session.NAVIdent, p.ClientID, p.Resource); err != nil {
		slog.Warn("remember consent failed", "error", err)
	}
	gwCode, err := randomToken()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	g.issueCode(w, r, p, gwCode, p.Issued)
}

func clearCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
