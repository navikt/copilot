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
	if status, _ := te.refresh(rt1); status != http.StatusBadRequest {
		t.Fatalf("replay must be rejected, got %d", status)
	}
	if status, _ := te.refresh(rt2); status != http.StatusBadRequest {
		t.Fatalf("newest RT in family must be revoked after replay, got %d", status)
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
