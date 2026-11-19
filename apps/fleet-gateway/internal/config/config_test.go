package config

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHardenedHTTPClient_RefusesRedirectsAndTimesOut(t *testing.T) {
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/other", http.StatusFound)
	}))
	defer redirectServer.Close()

	client := NewHardenedHTTPClient(100 * time.Millisecond)
	resp1, err := client.Get(redirectServer.URL)
	if resp1 != nil {
		_ = resp1.Body.Close()
	}
	if err == nil {
		t.Fatal("expected redirect to be refused")
	}

	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	resp2, err := client.Get(slowServer.URL)
	if resp2 != nil {
		_ = resp2.Body.Close()
	}
	if err == nil {
		t.Fatal("expected slow request to time out")
	}
}

func TestValidatePostgresURL_ErrorDoesNotLeakDSN(t *testing.T) {
	err := ValidatePostgresURL("postgres://user:s3cretpw@host:notaport/db")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "s3cretpw") {
		t.Fatalf("error leaks DSN: %v", err)
	}
}
