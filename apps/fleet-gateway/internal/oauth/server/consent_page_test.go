package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConsentCSSHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	ConsentCSS(rec, httptest.NewRequest(http.MethodGet, "/static/consent.css", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "text/css; charset=utf-8" || rec.Body.Len() == 0 {
		t.Fatalf("content-type %q, %d bytes", ct, rec.Body.Len())
	}
}
