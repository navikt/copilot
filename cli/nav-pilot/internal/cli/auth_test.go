package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestSaveLoadDeleteToken(t *testing.T) {
	keyring.MockInit()

	if _, err := loadToken(); err == nil {
		t.Fatal("expected error loading token before any is saved")
	}

	tok := storedToken{
		AccessToken: "gho_test123",
		TokenType:   "bearer",
		Scope:       "read:user read:org",
		Login:       "starefossen",
		ObtainedAt:  time.Now().Truncate(time.Second),
	}
	if err := saveToken(tok); err != nil {
		t.Fatalf("saveToken: %v", err)
	}

	got, err := loadToken()
	if err != nil {
		t.Fatalf("loadToken: %v", err)
	}
	if got.AccessToken != tok.AccessToken || got.Login != tok.Login {
		t.Fatalf("loadToken mismatch: got %+v, want %+v", got, tok)
	}

	if err := deleteToken(); err != nil {
		t.Fatalf("deleteToken: %v", err)
	}
	if _, err := loadToken(); err == nil {
		t.Fatal("expected error loading token after deletion")
	}

	// Deleting again should be idempotent.
	if err := deleteToken(); err != nil {
		t.Fatalf("second deleteToken should not error: %v", err)
	}
}

func TestRunDeviceFlowSuccess(t *testing.T) {
	pollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/device/code":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"device_code":"dc123","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":0}`))
		case "/login/oauth/access_token":
			pollCount++
			w.Header().Set("Content-Type", "application/json")
			if pollCount < 2 {
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"gho_abc","token_type":"bearer","scope":"read:user"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	origDeviceURL, origTokenURL := deviceCodeURL, accessTokenURL
	setTestURLs(server.URL+"/login/device/code", server.URL+"/login/oauth/access_token")
	defer setTestURLs(origDeviceURL, origTokenURL)

	var displayedCode, displayedURI string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	token, err := runDeviceFlowWithInterval(ctx, "test-client", "read:user", 10*time.Millisecond, func(userCode, verificationURI string) {
		displayedCode = userCode
		displayedURI = verificationURI
	})
	if err != nil {
		t.Fatalf("runDeviceFlow: %v", err)
	}
	if token.AccessToken != "gho_abc" {
		t.Fatalf("unexpected access token: %s", token.AccessToken)
	}
	if displayedCode != "ABCD-1234" || !strings.Contains(displayedURI, "github.com") {
		t.Fatalf("display callback got unexpected values: %q %q", displayedCode, displayedURI)
	}
}

func TestRunDeviceFlowAccessDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/device/code":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"device_code":"dc123","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":0}`))
		case "/login/oauth/access_token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error":"access_denied"}`))
		}
	}))
	defer server.Close()

	origDeviceURL, origTokenURL := deviceCodeURL, accessTokenURL
	setTestURLs(server.URL+"/login/device/code", server.URL+"/login/oauth/access_token")
	defer setTestURLs(origDeviceURL, origTokenURL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := runDeviceFlowWithInterval(ctx, "test-client", "read:user", 10*time.Millisecond, func(string, string) {})
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected access denied error, got: %v", err)
	}
}

func TestFormatSecondsRemaining(t *testing.T) {
	if got := formatSecondsRemaining(time.Time{}); got != "does not expire" {
		t.Fatalf("zero time: got %q", got)
	}
	if got := formatSecondsRemaining(time.Now().Add(-time.Hour)); got != "expired" {
		t.Fatalf("past time: got %q", got)
	}
	if got := formatSecondsRemaining(time.Now().Add(90 * time.Minute)); !strings.Contains(got, "h") {
		t.Fatalf("future time should include hours: got %q", got)
	}
}
