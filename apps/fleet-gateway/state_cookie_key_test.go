package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func testRTKeys(t *testing.T) (string, []byte) {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return "k1:" + base64.StdEncoding.EncodeToString(raw), raw
}

// Two replicas that share config must accept each other's state cookie,
// or the Entra callback fails whenever it lands on the other pod.
func TestEntraStateCookie_ValidatesAcrossInstances(t *testing.T) {
	setBaseValidEnv(t)
	rtKeys, aesKey := testRTKeys(t)
	t.Setenv("RT_KEYS", rtKeys)

	newGW := func() *Gateway {
		cfg, err := loadConfigFromEnv()
		if err != nil {
			t.Fatalf("loadConfigFromEnv: %v", err)
		}
		gw, err := newGateway(cfg, time.Now)
		if err != nil {
			t.Fatalf("newGateway: %v", err)
		}
		return gw
	}
	a, b := newGW(), newGW()

	if bytes.Equal(a.cfg.CookieSecret, aesKey) {
		t.Fatal("state cookie key must be an HKDF subkey, not the RT_KEYS AES key")
	}
	cookie := a.signEntraState("state-123")
	if !b.verifyEntraStateCookie(cookie, "state-123") {
		t.Fatal("state cookie issued on instance A rejected by instance B")
	}
	if b.verifyEntraStateCookie(cookie, "other-state") {
		t.Fatal("cookie accepted for a different state")
	}

	t.Setenv("RT_KEYS", "")
	if c := newGW(); c.verifyEntraStateCookie(cookie, "state-123") {
		t.Fatal("cookie accepted by an instance with a different key")
	}
}

func TestLoadCookieSecret_FailsClosedOnNais(t *testing.T) {
	if _, err := loadCookieSecret("", "", true); err == nil {
		t.Fatal("expected error on Nais without COOKIE_SIGNING_SECRET or RT_KEYS")
	}
	rtKeys, _ := testRTKeys(t)
	if _, err := loadCookieSecret("", rtKeys, true); err != nil {
		t.Fatalf("RT_KEYS on Nais: %v", err)
	}
	if _, err := loadCookieSecret("", "", false); err != nil {
		t.Fatalf("local random key: %v", err)
	}
}
