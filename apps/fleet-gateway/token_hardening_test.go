package main

import (
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

func TestValidatePostgresURL_ErrorDoesNotLeakDSN(t *testing.T) {
	err := validatePostgresURL("postgres://user:s3cretpw@host:notaport/db")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "s3cretpw") {
		t.Fatalf("error leaks DSN: %v", err)
	}
}

func TestRefreshToken_ManyGroupsRoundTrip(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	for i := 0; i < 100; i++ {
		te.groups = append(te.groups, fmt.Sprintf("%08d-aaaa-bbbb-cccc-dddddddddddd", i))
	}
	_, rt, _ := te.obtainTokensForResource(te.resource)
	if len(rt) <= 4096 {
		t.Fatalf("test premise: RT must exceed the old 4096 limit, got %d", len(rt))
	}
	if status, body := te.refresh(rt); status != http.StatusOK {
		t.Fatalf("refresh with 100 groups: %d (%v)", status, body)
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
