package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// navPilotGitHubClientIDDefault is the public client ID of the GitHub App
// "Nav Copilot (nav-pilot)" (App ID 5105952), used for device flow
// authentication. It is not a secret: device flow needs no client secret, and
// client IDs are safe to embed in distributed binaries (see GitHub's device
// flow docs). The App has "Device Flow" enabled in its settings.
//
// Overridable via NAV_PILOT_GITHUB_CLIENT_ID, to point nav-pilot at another
// App without rebuilding it.
const navPilotGitHubClientIDDefault = "Iv23lipLVaf9muvHhIXC"

// navPilotGitHubClientIDPlaceholder is the client ID nav-pilot shipped with
// before the App existed. An override still set to it does not name an App.
const navPilotGitHubClientIDPlaceholder = "Iv1.nav-pilot-devflow"

func navPilotGitHubClientID() string {
	if v := strings.TrimSpace(os.Getenv("NAV_PILOT_GITHUB_CLIENT_ID")); v != "" {
		return v
	}
	return navPilotGitHubClientIDDefault
}

// hasGitHubApp reports whether there is a GitHub App to log in with: an
// override still set to the old placeholder does not name one.
func hasGitHubApp() bool {
	return navPilotGitHubClientID() != navPilotGitHubClientIDPlaceholder
}

// navPilotGitHubScopes is sent with the device code request, but GitHub
// ignores it for a GitHub App: the user token gets the permissions set on the
// App itself. It only matters for an OAuth App named by
// NAV_PILOT_GITHUB_CLIENT_ID, where read:user and read:org are the minimum to
// check identity and navikt org membership.
const navPilotGitHubScopes = "read:user read:org"

// deviceCodeURL and accessTokenURL are GitHub's device flow endpoints,
// overridable only by tests via setTestURLs to point at an httptest server.
// These are package-level mutable vars for test convenience — none of the
// tests in this package use t.Parallel(), and these vars must stay that way
// (sequential only) for as long as the override mechanism works this way.
var (
	deviceCodeURL  = "https://github.com/login/device/code"
	accessTokenURL = "https://github.com/login/oauth/access_token"
)

// setTestURLs overrides the GitHub device flow endpoints for testing against
// an httptest server. Test-only; not safe for concurrent/parallel use.
func setTestURLs(deviceURL, tokenURL string) {
	deviceCodeURL = deviceURL
	accessTokenURL = tokenURL
}

var errAuthorizationPending = errors.New("authorization_pending")
var errSlowDown = errors.New("slow_down")
var errAccessDenied = errors.New("access_denied")
var errDeviceCodeExpired = errors.New("expired_token")

// deviceCodeResponse is GitHub's response to POST /login/device/code.
type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// accessTokenResponse is GitHub's response to POST /login/oauth/access_token
// while polling the device flow, or the final success response. ExpiresIn is
// only populated when the GitHub App has "token expiration" enabled; classic
// OAuth Apps and Apps without expiration enabled omit it, meaning the token
// does not expire (ExpiresIn stays 0).
//
// With expiration on (GitHub's default for Apps), the token lasts 8 hours and
// comes with a refresh_token that lasts 6 months (#1118).
type accessTokenResponse struct {
	AccessToken           string `json:"access_token"`
	TokenType             string `json:"token_type"`
	Scope                 string `json:"scope"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Error                 string `json:"error"`
}

// requestDeviceCode starts the GitHub device flow, returning the code the
// user must enter at the verification URL.
func requestDeviceCode(ctx context.Context, clientID, scope string) (*deviceCodeResponse, error) {
	form := url.Values{
		"client_id": {clientID},
		"scope":     {scope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading device code response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result deviceCodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding device code response: %w", err)
	}
	if result.DeviceCode == "" || result.UserCode == "" {
		return nil, fmt.Errorf("malformed device code response: missing device_code or user_code")
	}
	if result.VerificationURI == "" {
		return nil, fmt.Errorf("malformed device code response: missing verification_uri")
	}
	if result.ExpiresIn <= 0 {
		return nil, fmt.Errorf("malformed device code response: expires_in must be positive, got %d", result.ExpiresIn)
	}
	return &result, nil
}

// pollAccessToken performs a single poll of the device flow token endpoint.
// Returns errAuthorizationPending, errSlowDown, errAccessDenied, or
// errDeviceCodeExpired as sentinel errors the caller should distinguish from
// hard failures; any other error indicates a transport or protocol failure.
func pollAccessToken(ctx context.Context, clientID, deviceCode string) (*accessTokenResponse, error) {
	result, err := postTokenForm(ctx, url.Values{
		"client_id":   {clientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	})
	if err != nil {
		return nil, err
	}
	switch result.Error {
	case "":
		if result.AccessToken == "" {
			return nil, fmt.Errorf("token poll response missing access_token")
		}
		return result, nil
	case "authorization_pending":
		return nil, errAuthorizationPending
	case "slow_down":
		return nil, errSlowDown
	case "access_denied":
		return nil, errAccessDenied
	case "expired_token":
		return nil, errDeviceCodeExpired
	default:
		return nil, fmt.Errorf("device flow error: %s", result.Error)
	}
}

// refreshAccessToken trades a refresh token for a new access token and a new
// refresh token. A token from the device flow needs no client secret.
func refreshAccessToken(ctx context.Context, clientID, refreshToken string) (*accessTokenResponse, error) {
	result, err := postTokenForm(ctx, url.Values{
		"client_id":     {clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("token refresh error: %s", result.Error)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("token refresh response missing access_token")
	}
	return result, nil
}

// postTokenForm posts form to GitHub's token endpoint and decodes the answer,
// error field and all.
func postTokenForm(ctx context.Context, form url.Values) (*accessTokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, accessTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result accessTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	return &result, nil
}

// runDeviceFlow drives the full GitHub device authorization flow: request a
// device code, display it to the user, then poll until the user approves (or
// the code expires / is denied). display is called once with instructions to
// show the user; it is separated out so tests can capture it without writing
// to stdout.
func runDeviceFlow(ctx context.Context, clientID, scope string, display func(userCode, verificationURI string)) (*accessTokenResponse, error) {
	return runDeviceFlowWithInterval(ctx, clientID, scope, 0, display)
}

// runDeviceFlowWithInterval is runDeviceFlow with an overridable minimum poll
// interval, used by tests to avoid waiting on GitHub's real (multi-second)
// polling interval.
func runDeviceFlowWithInterval(ctx context.Context, clientID, scope string, minInterval time.Duration, display func(userCode, verificationURI string)) (*accessTokenResponse, error) {
	dc, err := requestDeviceCode(ctx, clientID, scope)
	if err != nil {
		return nil, err
	}

	display(dc.UserCode, dc.VerificationURI)

	interval := time.Duration(dc.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if minInterval > 0 {
		interval = minInterval
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("device code expired before approval — run 'nav-pilot auth login' again")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		token, err := pollAccessToken(ctx, clientID, dc.DeviceCode)
		switch {
		case err == nil:
			return token, nil
		case errors.Is(err, errAuthorizationPending):
			continue
		case errors.Is(err, errSlowDown):
			interval += 5 * time.Second
			continue
		case errors.Is(err, errAccessDenied):
			return nil, fmt.Errorf("authorization was denied")
		case errors.Is(err, errDeviceCodeExpired):
			return nil, fmt.Errorf("device code expired before approval — run 'nav-pilot auth login' again")
		default:
			return nil, err
		}
	}
}

// formatSecondsRemaining renders a duration until expiry as a short human
// string like "6h 23m", or "expired" if not in the future.
func formatSecondsRemaining(until time.Time) string {
	if until.IsZero() {
		return "does not expire"
	}
	remaining := time.Until(until)
	if remaining <= 0 {
		return "expired"
	}
	hours := int(remaining.Hours())
	minutes := int(remaining.Minutes()) % 60
	if hours > 0 {
		return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
	}
	return strconv.Itoa(minutes) + "m"
}
