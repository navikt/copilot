package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func (te *testEnv) refresh(rt string) (int, map[string]any) {
	return te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt},
		"client_id":     {te.clientID},
	})
}

func TestSeal_RejectsOversizedPlaintext(t *testing.T) {
	kr, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kr.Seal([]byte("aad"), make([]byte, sealMaxPlaintext+1)); err == nil {
		t.Fatal("expected oversized plaintext to be rejected")
	}
	if _, err := kr.Seal([]byte("aad"), make([]byte, 1024)); err != nil {
		t.Fatalf("normal plaintext must seal: %v", err)
	}
}

func TestRefreshToken_LegacyGroupsStillAcceptedAndNewTokenSmall(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	groups := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		groups = append(groups, fmt.Sprintf("%08d-aaaa-bbbb-cccc-dddddddddddd", i))
	}
	te.groups = append(te.groups, groups...)

	// Newly issued RTs ignore groups from Entra claims and stay under maxRefreshTokenLen.
	_, newRT, _ := te.obtainTokensForResource(te.resource)
	if len(newRT) > maxRefreshTokenLen {
		t.Fatalf("new RT should stay under maxRefreshTokenLen (%d), got %d", maxRefreshTokenLen, len(newRT))
	}

	// Build a legacy RT payload JSON by hand with a "groups" array exceeding maxRefreshTokenLen.
	now := te.gw.now()
	legacyJSON, err := json.Marshal(map[string]any{
		"v":         1,
		"jti":       "legacy-jti-1",
		"fid":       "legacy-fid-1",
		"iat":       now.Unix(),
		"exp":       now.Add(12 * time.Hour).Unix(),
		"client_id": te.clientID,
		"resource":  te.resource,
		"nav_ident": te.navIdent,
		"groups":    groups,
		"entra_rt":  "entra-legacy-rt",
	})
	if err != nil {
		t.Fatalf("marshal legacy RT payload: %v", err)
	}
	blob, err := te.gw.rtKeys.Seal([]byte(rtSealAAD), legacyJSON)
	if err != nil {
		t.Fatalf("seal legacy RT payload: %v", err)
	}
	legacyRT := base64.RawURLEncoding.EncodeToString(blob)
	if len(legacyRT) <= maxRefreshTokenLen {
		t.Fatalf("test premise: legacy RT with 100 groups must exceed maxRefreshTokenLen (%d), got %d", maxRefreshTokenLen, len(legacyRT))
	}
	if len(legacyRT) > maxLegacyRefreshTokenLen {
		t.Fatalf("test premise: legacy RT must stay within maxLegacyRefreshTokenLen (%d), got %d", maxLegacyRefreshTokenLen, len(legacyRT))
	}

	status, body := te.refresh(legacyRT)
	if status != http.StatusOK {
		t.Fatalf("refresh with legacy many-groups RT: %d (%v)", status, body)
	}
	rotatedRT, _ := body["refresh_token"].(string)
	if rotatedRT == "" || len(rotatedRT) > maxRefreshTokenLen {
		t.Fatalf("rotated RT must be non-empty and <= %d, got len=%d", maxRefreshTokenLen, len(rotatedRT))
	}
	rotatedBlob, err := base64.RawURLEncoding.DecodeString(rotatedRT)
	if err != nil {
		t.Fatalf("decode rotated RT: %v", err)
	}
	rotatedPlain, err := te.gw.rtKeys.Open([]byte(rtSealAAD), rotatedBlob)
	if err != nil {
		t.Fatalf("open rotated RT: %v", err)
	}
	var rotatedMap map[string]any
	if err = json.Unmarshal(rotatedPlain, &rotatedMap); err != nil {
		t.Fatalf("unmarshal rotated RT: %v", err)
	}
	if _, hasGroups := rotatedMap["groups"]; hasGroups {
		t.Fatalf("rotated RT must not contain groups key: %s", string(rotatedPlain))
	}

	// Legacy session JSON with a "groups" key must also decode from the session store.
	legacySessJSON, err := json.Marshal(map[string]any{
		"client_id":   te.clientID,
		"resource":    te.resource,
		"entra_token": te.entraToken,
		"nav_ident":   te.navIdent,
		"groups":      groups,
		"issued_at":   now,
		"expiry":      now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("marshal legacy session: %v", err)
	}
	sessHash := hashToken("legacy-access-token")
	if err = te.gw.store.Put(context.Background(), kindSession, sessHash, legacySessJSON, time.Hour); err != nil {
		t.Fatalf("put legacy session: %v", err)
	}
	gotSess, ok, err := te.gw.getSession(context.Background(), sessHash)
	if err != nil || !ok {
		t.Fatalf("get legacy session: ok=%v err=%v", ok, err)
	}
	if gotSess.NAVIdent != te.navIdent || gotSess.EntraToken != te.entraToken {
		t.Fatalf("unexpected decoded legacy session: %+v", gotSess)
	}
}

func TestRefreshToken_RealisticSizeWithoutGroups(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	te.mu.Lock()
	te.entraRefreshToken = strings.Repeat("r", 2048)
	te.mu.Unlock()

	_, rt, _ := te.obtainTokensForResource(te.resource)
	t.Logf("sealed RT length with 2 KiB Entra RT = %d (maxRefreshTokenLen = %d)", len(rt), maxRefreshTokenLen)
	if len(rt) > maxRefreshTokenLen {
		t.Fatalf("sealed RT with 2 KiB Entra RT (%d bytes) exceeds maxRefreshTokenLen (%d)", len(rt), maxRefreshTokenLen)
	}
}

func TestRefreshToken_ExpIsAbsoluteAcrossMaxAgeChange(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt, _ := te.obtainTokensForResource(te.resource)
	te.gw.cfg.SessionMaxAge = 48 * time.Hour // operator raises the limit later
	te.advance(13 * time.Hour)
	if status, body := te.refresh(rt); status != http.StatusBadRequest {
		t.Fatalf("exp must stay fixed at issue time, got %d (%v)", status, body)
	}
}

func TestRefreshToken_LoweredMaxAgeRejectsAndClampsRefresh(t *testing.T) {
	t.Run("rejects refresh past lowered SessionMaxAge", func(t *testing.T) {
		te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
		_, rt, _ := te.obtainTokensForResource(te.resource)
		te.gw.cfg.SessionAbsoluteMaxAge = time.Hour
		te.advance(2 * time.Hour)
		if status, body := te.refresh(rt); status != http.StatusBadRequest || body["error"] != "invalid_grant" {
			t.Fatalf("expected 400 invalid_grant after SessionMaxAge lowered past elapsed time, got %d (%v)", status, body)
		}
	})

	t.Run("clamps expires_in and issued session stays alive within lowered SessionMaxAge", func(t *testing.T) {
		te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
		_, rt, _ := te.obtainTokensForResource(te.resource)
		te.gw.cfg.SessionAbsoluteMaxAge = time.Hour
		te.advance(45 * time.Minute)
		status, body := te.refresh(rt)
		if status != http.StatusOK {
			t.Fatalf("expected 200 within lowered SessionMaxAge, got %d (%v)", status, body)
		}
		exp, _ := body["expires_in"].(float64)
		if exp <= 0 || exp > 900 {
			t.Fatalf("expected expires_in clamped to remaining <=900s, got %v", exp)
		}
		at, _ := body["access_token"].(string)
		if _, ok, err := te.gw.getSession(context.Background(), hashToken(at)); err != nil || !ok {
			t.Fatalf("issued session must be readable from store: ok=%v err=%v", ok, err)
		}
	})
}

func TestRefreshToken_ReplayRevokesFamily(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt1, _ := te.obtainTokensForResource(te.resource)
	status, body := te.refresh(rt1)
	if status != http.StatusOK {
		t.Fatalf("first refresh: %d (%v)", status, body)
	}
	rt2, _ := body["refresh_token"].(string)
	te.advance(te.gw.cfg.RefreshGraceWindow)
	if status, _ := te.refresh(rt1); status != http.StatusBadRequest {
		t.Fatalf("replay after the grace window must be rejected, got %d", status)
	}
	if status, _ := te.refresh(rt2); status != http.StatusBadRequest {
		t.Fatalf("newest RT in family must be revoked after replay, got %d", status)
	}
}

// Inside the grace window the previous RT gets the same pair again, without a
// second Entra call; at the window's end it is reuse and kills the family.
func TestRefreshToken_GraceWindowBoundary(t *testing.T) {
	for _, tc := range []struct {
		name  string
		after time.Duration
		grace bool
	}{
		{"just inside", 30*time.Second - time.Millisecond, true},
		{"at end", 30 * time.Second, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
			if te.gw.cfg.RefreshGraceWindow != 30*time.Second {
				t.Fatalf("default grace window = %s, want 30s", te.gw.cfg.RefreshGraceWindow)
			}
			_, rt1, _ := te.obtainTokensForResource(te.resource)
			status, first := te.refresh(rt1)
			if status != http.StatusOK {
				t.Fatalf("first refresh: %d (%v)", status, first)
			}
			te.advance(tc.after)
			status, again := te.refresh(rt1)
			if !tc.grace {
				if status != http.StatusBadRequest {
					t.Fatalf("replay at window end: want 400, got %d (%v)", status, again)
				}
				rt2, _ := first["refresh_token"].(string)
				if st, _ := te.refresh(rt2); st != http.StatusBadRequest {
					t.Fatalf("family must be revoked after replay outside the window, got %d", st)
				}
				return
			}
			if status != http.StatusOK || again["access_token"] != first["access_token"] || again["refresh_token"] != first["refresh_token"] {
				t.Fatalf("replay in window: want 200 with the same pair, got %d (%v)", status, again)
			}
			te.mu.Lock()
			n := te.entraRefreshCount
			te.mu.Unlock()
			if n != 1 {
				t.Fatalf("grace replay must not call Entra again, count=%d", n)
			}
			rt2, _ := first["refresh_token"].(string)
			if status, body := te.refresh(rt2); status != http.StatusOK {
				t.Fatalf("new RT must still work after a grace replay, got %d (%v)", status, body)
			}
		})
	}
}

// Only the immediately previous RT qualifies: once the new RT is rotated too,
// replaying the old one is reuse even inside the window.
func TestRefreshToken_GraceOnlyForImmediatelyPrevious(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt1, _ := te.obtainTokensForResource(te.resource)
	_, b := te.refresh(rt1)
	rt2, _ := b["refresh_token"].(string)
	status, b := te.refresh(rt2)
	if status != http.StatusOK {
		t.Fatalf("second rotation: %d (%v)", status, b)
	}
	rt3, _ := b["refresh_token"].(string)
	if status, _ := te.refresh(rt1); status != http.StatusBadRequest {
		t.Fatalf("replay of a token two rotations back must be rejected, got %d", status)
	}
	if status, _ := te.refresh(rt3); status != http.StatusBadRequest {
		t.Fatalf("family must be revoked, got %d", status)
	}
}

func TestRefreshToken_TransientUpstreamDoesNotBurnRT(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt, _ := te.obtainTokensForResource(te.resource)

	te.mu.Lock()
	te.entraRefreshStatus = http.StatusServiceUnavailable
	te.mu.Unlock()
	if status, _ := te.refresh(rt); status != http.StatusServiceUnavailable {
		t.Fatalf("transient upstream error must be 503, got %d", status)
	}

	te.mu.Lock()
	te.entraRefreshStatus = http.StatusOK
	te.mu.Unlock()
	te.advance(refreshPendingTTL + time.Second)
	if status, body := te.refresh(rt); status != http.StatusOK {
		t.Fatalf("RT must survive a transient failure, got %d (%v)", status, body)
	}
}

func TestRefreshToken_RevokeSurvivesLoweredThenRaisedHardCap(t *testing.T) {
	buf := captureAudit(t)
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt1, _ := te.obtainTokensForResource(te.resource)

	// The cap is lowered while rt1 is rotated and replayed; the family revocation
	// must outlive the lowered cap, or raising it again would revive rt2.
	te.gw.cfg.SessionAbsoluteMaxAge = time.Hour
	status, body := te.refresh(rt1)
	if status != http.StatusOK {
		t.Fatalf("first refresh: %d (%v)", status, body)
	}
	rt2, _ := body["refresh_token"].(string)

	te.advance(30 * time.Minute) // past the grace window, inside the lowered cap
	if status, body := te.refresh(rt1); status != http.StatusBadRequest {
		t.Fatalf("replay of used RT: got %d (%v), want 400", status, body)
	}
	if got := lastRefreshOutcome(buf); !strings.Contains(got, `"outcome":"replayed_family_revoked"`) {
		t.Fatalf("want replayed_family_revoked, got %s", got)
	}

	te.advance(2 * time.Hour) // past the lowered cap
	te.gw.cfg.SessionAbsoluteMaxAge = 7 * 24 * time.Hour
	if status, body := te.refresh(rt2); status != http.StatusBadRequest {
		t.Fatalf("newest RT must stay revoked after the cap is raised again: got %d (%v)", status, body)
	}
	if got := lastRefreshOutcome(buf); !strings.Contains(got, `"outcome":"family_revoked"`) {
		t.Fatalf("want family_revoked (denylist path), got %s", got)
	}
}
