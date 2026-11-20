package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

func captureAudit(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func lastRefreshOutcome(buf *bytes.Buffer) string {
	lines := strings.Split(buf.String(), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], `"audit refresh"`) {
			return lines[i]
		}
	}
	return ""
}

func TestSliding_RefreshExtendsPastOldCapUpToHardCap(t *testing.T) {
	buf := captureAudit(t)
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	login := te.gw.now()
	_, rt, _ := te.obtainTokensForResource(te.resource)

	// Active every 11 h: lives well past the old 12 h cap.
	for i := 0; i < 15; i++ {
		te.advance(11 * time.Hour)
		status, body := te.refresh(rt)
		if status != http.StatusOK {
			t.Fatalf("round %d (%v after login): got %d %v", i, te.gw.now().Sub(login), status, body)
		}
		rt, _ = body["refresh_token"].(string)
	}
	// 15 x 11 h = 165 h = 6.9 d: still inside the hard cap. The next one crosses 7 d even though active.
	te.advance(11 * time.Hour)
	status, body := te.refresh(rt)
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("refresh past 7 d from login: got %d %v, want 400 invalid_grant", status, body)
	}
	if got := lastRefreshOutcome(buf); !strings.Contains(got, `"outcome":"session_max_age_expired"`) {
		t.Fatalf("want session_max_age_expired, got %s", got)
	}
}

func TestSliding_RefreshedSessionExpiryCappedAtHardCap(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt, _ := te.obtainTokensForResource(te.resource)
	for i := 0; i < 15; i++ { // 165 h of activity
		te.advance(11 * time.Hour)
		_, body := te.refresh(rt)
		rt, _ = body["refresh_token"].(string)
	}
	te.advance(2*time.Hour + 50*time.Minute) // 10 min before 168 h
	status, body := te.refresh(rt)
	if status != http.StatusOK {
		t.Fatalf("refresh 10 min before hard cap: %d %v", status, body)
	}
	if exp, _ := body["expires_in"].(float64); exp <= 0 || exp > 600 {
		t.Fatalf("expires_in must be clamped to the hard cap (<=600 s), got %v", exp)
	}
	// The new refresh token must not outlive the hard cap either.
	np, err := te.gw.openRefreshToken(body["refresh_token"].(string))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if want := np.IAT + int64((7 * 24 * time.Hour).Seconds()); np.EXP != want || np.ABS != want {
		t.Fatalf("new RT exp/abs = %d/%d, want both capped at %d", np.EXP, np.ABS, want)
	}
}

func TestSliding_IdleTimeout(t *testing.T) {
	buf := captureAudit(t)
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt, _ := te.obtainTokensForResource(te.resource)
	te.advance(12*time.Hour + time.Second)
	status, body := te.refresh(rt)
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("idle > 12 h: got %d %v, want 400 invalid_grant", status, body)
	}
	if got := lastRefreshOutcome(buf); !strings.Contains(got, `"outcome":"session_idle_expired"`) {
		t.Fatalf("want session_idle_expired, got %s", got)
	}
}

func TestSliding_LockoutStillKillsAfterRefreshes(t *testing.T) {
	te := withLockoutKey(t, newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false)))
	_, rt, _ := te.obtainTokensForResource(te.resource)
	te.advance(6 * time.Hour)
	status, body := te.refresh(rt)
	if status != http.StatusOK {
		t.Fatalf("refresh: %d %v", status, body)
	}
	rt, _ = body["refresh_token"].(string)
	te.advance(time.Minute)
	if _, err := te.gw.lockOutUser(t.Context(), te.navIdent, "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}
	te.advance(time.Minute)
	if status, body := te.refresh(rt); status != http.StatusBadRequest || body["error_description"] != "session revoked" {
		t.Fatalf("refresh after lockout: %d %v, want 400 session revoked", status, body)
	}
}

// A refresh token minted before this change has EXP = IAT + 12 h and no other login time: IAT is the login time.
func TestSliding_LegacyTokenTreatsIATAsLogin(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt, _ := te.obtainTokensForResource(te.resource)
	p, err := te.gw.openRefreshToken(rt)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	p.EXP = p.IAT + int64((12 * time.Hour).Seconds())
	legacy, err := te.gw.sealRefreshToken(*p)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	te.advance(6 * time.Hour)
	status, body := te.refresh(legacy)
	if status != http.StatusOK {
		t.Fatalf("legacy RT inside its 12 h: %d %v", status, body)
	}
	np, err := te.gw.openRefreshToken(body["refresh_token"].(string))
	if err != nil {
		t.Fatalf("open new: %v", err)
	}
	if want := p.IAT + int64((7 * 24 * time.Hour).Seconds()); np.ABS != want {
		t.Fatalf("legacy family gets its hard cap at the first rotation: abs %d want %d", np.ABS, want)
	}
	if np.IAT != p.IAT {
		t.Fatalf("login time must carry over: got %d want %d", np.IAT, p.IAT)
	}
	if want := te.gw.now().Add(12 * time.Hour).Unix(); np.EXP != want {
		t.Fatalf("new idle deadline: got %d want %d", np.EXP, want)
	}
	// And a legacy RT past its own EXP is rejected as idle.
	te.advance(7 * time.Hour)
	if status, _ := te.refresh(legacy); status != http.StatusBadRequest {
		t.Fatalf("legacy RT past EXP: got %d, want 400", status)
	}
}

func TestSliding_ConfigDefaultsAndValidation(t *testing.T) {
	te := newTestEnv(t)
	if te.gw.cfg.SessionAbsoluteMaxAge != 7*24*time.Hour || te.gw.cfg.SessionIdleTimeout != 12*time.Hour {
		t.Fatalf("defaults: %v %v", te.gw.cfg.SessionAbsoluteMaxAge, te.gw.cfg.SessionIdleTimeout)
	}
	c := *te.gw.cfg
	c.SessionAbsoluteMaxAge = time.Hour
	if err := validateConfig(&c); err == nil {
		t.Fatal("absolute cap below SESSION_MAX_AGE must be rejected")
	}
}

func TestSliding_ReplayAfterIdleDeadlineStillRevokesFamily(t *testing.T) {
	buf := captureAudit(t)
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt1, _ := te.obtainTokensForResource(te.resource)
	te.advance(time.Minute)
	status, body := te.refresh(rt1)
	if status != http.StatusOK {
		t.Fatalf("refresh: %d %v", status, body)
	}
	rt2, _ := body["refresh_token"].(string)

	te.advance(12 * time.Hour) // rt1 is now past its idle deadline, rt2 is not
	if status, _ := te.refresh(rt1); status != http.StatusBadRequest {
		t.Fatalf("replay: got %d, want 400", status)
	}
	if got := lastRefreshOutcome(buf); !strings.Contains(got, `"outcome":"replayed_family_revoked"`) {
		t.Fatalf("replay past idle deadline must revoke the family, got %s", got)
	}
	if status, _ := te.refresh(rt2); status != http.StatusBadRequest {
		t.Fatalf("rt2 must be dead after the replay, got %d", status)
	}
}

func TestSliding_GraceRetryAcrossIdleDeadline(t *testing.T) {
	buf := captureAudit(t)
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt1, _ := te.obtainTokensForResource(te.resource)
	te.advance(12*time.Hour - time.Second)
	status, body := te.refresh(rt1)
	if status != http.StatusOK {
		t.Fatalf("refresh just inside the deadline: %d %v", status, body)
	}
	te.advance(5 * time.Second) // rt1 is past its deadline, still inside the 30 s grace window
	if status, body := te.refresh(rt1); status != http.StatusOK {
		t.Fatalf("grace retry across the deadline: %d %v", status, body)
	}
	if got := lastRefreshOutcome(buf); !strings.Contains(got, `"outcome":"grace_replay"`) {
		t.Fatalf("want grace_replay, got %s", got)
	}
}
