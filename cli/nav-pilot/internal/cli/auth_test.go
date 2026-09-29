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
		Scope:       "read:user",
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

// TestRequestDeviceCodeMalformed verifies that structurally valid JSON with
// missing or nonsensical device-flow fields is rejected with a clear error
// instead of propagating an unusable response into the polling loop.
func TestRequestDeviceCodeMalformed(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantSubstr string
	}{
		{
			name:       "missing device_code and user_code",
			body:       `{"verification_uri":"https://github.com/login/device","expires_in":900}`,
			wantSubstr: "missing device_code or user_code",
		},
		{
			name:       "missing verification_uri",
			body:       `{"device_code":"dc","user_code":"ABCD-1234","expires_in":900}`,
			wantSubstr: "missing verification_uri",
		},
		{
			name:       "missing expires_in",
			body:       `{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device"}`,
			wantSubstr: "expires_in must be positive",
		},
		{
			name:       "negative expires_in",
			body:       `{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":-1}`,
			wantSubstr: "expires_in must be positive",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			origDeviceURL, origTokenURL := deviceCodeURL, accessTokenURL
			setTestURLs(server.URL, server.URL)
			defer setTestURLs(origDeviceURL, origTokenURL)

			_, err := requestDeviceCode(context.Background(), "client-id", navPilotGitHubScopes)
			if err == nil {
				t.Fatal("expected error for malformed device code response")
			}
			if !strings.Contains(err.Error(), "malformed device code response") {
				t.Errorf("error %q does not mention malformed response", err.Error())
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSubstr)
			}
		})
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

func TestCmdAuthLoginSuccess(t *testing.T) {
	keyring.MockInit()
	t.Setenv("NAV_PILOT_GITHUB_CLIENT_ID", "Iv1.test-client")

	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user":
			_, _ = w.Write([]byte(`{"login":"starefossen","name":"Hans Kristian"}`))
		default:
			// The App has no permissions: nothing but /user may be asked.
			t.Errorf("unexpected GitHub call %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer githubServer.Close()

	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/device/code":
			_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":0}`))
		case "/login/oauth/access_token":
			_, _ = w.Write([]byte(`{"access_token":"gho_abc","token_type":"bearer","scope":"read:user"}`))
		}
	}))
	defer oauthServer.Close()

	origDeviceURL, origTokenURL := deviceCodeURL, accessTokenURL
	origGitHubAPI := githubAPIBaseURL
	setTestURLs(oauthServer.URL+"/login/device/code", oauthServer.URL+"/login/oauth/access_token")
	githubAPIBaseURL = githubServer.URL
	defer func() {
		setTestURLs(origDeviceURL, origTokenURL)
		githubAPIBaseURL = origGitHubAPI
	}()

	if err := cmdAuthLogin(); err != nil {
		t.Fatalf("cmdAuthLogin: %v", err)
	}

	tok, err := loadToken()
	if err != nil {
		t.Fatalf("loadToken after login: %v", err)
	}
	if tok.Login != "starefossen" || tok.AccessToken != "gho_abc" {
		t.Fatalf("unexpected stored token: %+v", tok)
	}
}

func TestCmdAuthLoginPlaceholderClientID(t *testing.T) {
	keyring.MockInit()
	// An override still set to the old placeholder does not name a GitHub App, so
	// login must fail fast before any network call.
	t.Setenv("NAV_PILOT_GITHUB_CLIENT_ID", navPilotGitHubClientIDPlaceholder)

	err := cmdAuthLogin()
	if err == nil {
		t.Fatal("expected error when client ID is the placeholder")
	}
	if !strings.Contains(err.Error(), "NAV_PILOT_GITHUB_CLIENT_ID") {
		t.Errorf("error should mention the env var to set, got: %v", err)
	}
}

func TestHasGitHubApp(t *testing.T) {
	for env, want := range map[string]bool{"": true, "  ": true, navPilotGitHubClientIDPlaceholder: false, " Iv1.other ": true} {
		t.Setenv("NAV_PILOT_GITHUB_CLIENT_ID", env)
		if got := hasGitHubApp(); got != want {
			t.Errorf("NAV_PILOT_GITHUB_CLIENT_ID=%q: hasGitHubApp() = %v, want %v", env, got, want)
		}
	}
	t.Setenv("NAV_PILOT_GITHUB_CLIENT_ID", " Iv1.other ")
	if got := navPilotGitHubClientID(); got != "Iv1.other" {
		t.Errorf("override not trimmed: %q", got)
	}
}

// TestCmdAuthLoginDefaultClientID proves the placeholder is gone: with no
// override, login runs the device flow with the bundled App's client ID.
func TestCmdAuthLoginDefaultClientID(t *testing.T) {
	keyring.MockInit()
	t.Setenv("NAV_PILOT_GITHUB_CLIENT_ID", "")

	if navPilotGitHubClientIDDefault == navPilotGitHubClientIDPlaceholder || !hasGitHubApp() {
		t.Fatalf("default client ID %q is still a placeholder", navPilotGitHubClientIDDefault)
	}

	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/user" {
			_, _ = w.Write([]byte(`{"login":"starefossen"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer githubServer.Close()

	var gotClientID string
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/device/code":
			gotClientID = r.FormValue("client_id")
			_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":0}`))
		case "/login/oauth/access_token":
			_, _ = w.Write([]byte(`{"access_token":"ghu_abc","token_type":"bearer"}`))
		}
	}))
	defer oauthServer.Close()

	origDeviceURL, origTokenURL := deviceCodeURL, accessTokenURL
	origGitHubAPI := githubAPIBaseURL
	setTestURLs(oauthServer.URL+"/login/device/code", oauthServer.URL+"/login/oauth/access_token")
	githubAPIBaseURL = githubServer.URL
	defer func() {
		setTestURLs(origDeviceURL, origTokenURL)
		githubAPIBaseURL = origGitHubAPI
	}()

	if err := cmdAuthLogin(); err != nil {
		t.Fatalf("cmdAuthLogin with the default client ID: %v", err)
	}
	if gotClientID != navPilotGitHubClientIDDefault {
		t.Errorf("device flow sent client_id %q, want %q", gotClientID, navPilotGitHubClientIDDefault)
	}
}

func TestRunDeviceFlowDefaultInterval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login/device/code":
			_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"CODE-0000","verification_uri":"https://github.com/login/device","expires_in":900,"interval":0}`))
		case "/login/oauth/access_token":
			_, _ = w.Write([]byte(`{"access_token":"gho_default","token_type":"bearer","scope":"read:user"}`))
		}
	}))
	defer server.Close()

	origDeviceURL, origTokenURL := deviceCodeURL, accessTokenURL
	setTestURLs(server.URL+"/login/device/code", server.URL+"/login/oauth/access_token")
	defer setTestURLs(origDeviceURL, origTokenURL)

	// runDeviceFlow (not the WithInterval variant) exercises the
	// default-interval branch directly; the server responds immediately
	// so this doesn't need to wait for the real 5s default.
	token, err := runDeviceFlow(t.Context(), "client", "scope", func(string, string) {})
	if err != nil {
		t.Fatalf("runDeviceFlow: %v", err)
	}
	if token.AccessToken != "gho_default" {
		t.Fatalf("unexpected token: %+v", token)
	}
}

// An App user token lasts 8 hours. Near expiry, currentToken trades the
// refresh token for a new pair and saves it, with no client secret (#1118).
func TestCurrentTokenRefreshes(t *testing.T) {
	keyring.MockInit()
	var form map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = map[string]string{}
		for k := range r.PostForm {
			form[k] = r.PostForm.Get(k)
		}
		_, _ = w.Write([]byte(`{"access_token":"ghu_new","token_type":"bearer","expires_in":28800,"refresh_token":"ghr_new","refresh_token_expires_in":15897600}`))
	}))
	defer server.Close()
	origDeviceURL, origTokenURL := deviceCodeURL, accessTokenURL
	setTestURLs(server.URL, server.URL)
	defer setTestURLs(origDeviceURL, origTokenURL)

	// Far from expiry: no request.
	fresh := storedToken{AccessToken: "ghu_old", Login: "kari", ExpiresAt: time.Now().Add(time.Hour), RefreshToken: "ghr_old"}
	if err := saveToken(fresh); err != nil {
		t.Fatal(err)
	}
	if got, _ := currentToken(context.Background()); got.AccessToken != "ghu_old" || form != nil {
		t.Fatalf("refreshed a token an hour from expiry: %+v, form %v", got, form)
	}

	fresh.ExpiresAt = time.Now().Add(time.Minute)
	if err := saveToken(fresh); err != nil {
		t.Fatal(err)
	}
	got, err := currentToken(context.Background())
	if err != nil || got.AccessToken != "ghu_new" || got.RefreshToken != "ghr_new" || got.Login != "kari" || got.expired() {
		t.Fatalf("currentToken = %+v, %v", got, err)
	}
	if form["grant_type"] != "refresh_token" || form["refresh_token"] != "ghr_old" || form["client_id"] == "" || form["client_secret"] != "" {
		t.Fatalf("refresh request form = %v", form)
	}
	if saved, _ := loadToken(); saved.AccessToken != "ghu_new" || time.Until(saved.RefreshExpiresAt) < 24*time.Hour {
		t.Fatalf("saved = %+v", saved)
	}

	// A refused refresh keeps the stored token as it was.
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form = map[string]string{"called": "yes"}
		_, _ = w.Write([]byte(`{"error":"bad_refresh_token"}`))
	})
	stale := storedToken{AccessToken: "ghu_stale", ExpiresAt: time.Now().Add(-time.Hour), RefreshToken: "ghr_stale"}
	if err := saveToken(stale); err != nil {
		t.Fatal(err)
	}
	if got, err := currentToken(context.Background()); err != nil || got.AccessToken != "ghu_stale" || !got.expired() || form["called"] != "yes" {
		t.Fatalf("after a refused refresh: %+v, %v, form %v", got, err, form)
	}
	if saved, _ := loadToken(); saved.RefreshToken != "ghr_stale" {
		t.Fatalf("a refused refresh changed the stored token: %+v", saved)
	}

	// Another process refreshed first: the refused one takes the saved pair.
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = saveToken(storedToken{AccessToken: "ghu_other", ExpiresAt: time.Now().Add(8 * time.Hour), RefreshToken: "ghr_other"})
		_, _ = w.Write([]byte(`{"error":"bad_refresh_token"}`))
	})
	if err := saveToken(stale); err != nil {
		t.Fatal(err)
	}
	if got, _ := currentToken(context.Background()); got.AccessToken != "ghu_other" {
		t.Fatalf("after losing a refresh race: %+v", got)
	}

	// An expired refresh token is not sent.
	form = nil
	stale.RefreshExpiresAt = time.Now().Add(-time.Minute)
	if err := saveToken(stale); err != nil {
		t.Fatal(err)
	}
	if _, _ = currentToken(context.Background()); form != nil {
		t.Fatalf("sent an expired refresh token: %v", form)
	}

	// A token without a refresh token (expiration off) is left alone.
	form = nil
	if err := saveToken(storedToken{AccessToken: "gho_x", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if got, _ := currentToken(context.Background()); got.AccessToken != "gho_x" || form != nil {
		t.Fatalf("got %+v, form %v", got, form)
	}
}
