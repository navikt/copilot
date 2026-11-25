package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/oauth/entra"
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
	cookie := a.oauth().SignEntraState("state-123")
	if !b.oauth().VerifyEntraStateCookie(cookie, "state-123") {
		t.Fatal("state cookie issued on instance A rejected by instance B")
	}
	if b.oauth().VerifyEntraStateCookie(cookie, "other-state") {
		t.Fatal("cookie accepted for a different state")
	}

	t.Setenv("RT_KEYS", "")
	if c := newGW(); c.oauth().VerifyEntraStateCookie(cookie, "state-123") {
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

// During an RT_KEYS rotation rollout, pods on the new ring (new primary first)
// must accept cookies signed by pods still on the old primary.
func TestEntraStateCookie_ValidatesAcrossKeyRotation(t *testing.T) {
	oldRing, _ := testRTKeys(t)
	newPrimary, _ := testRTKeys(t)
	newRing := strings.Replace(newPrimary, "k1:", "k2:", 1) + "," + oldRing

	oldKeys, err := loadCookieSecret("", oldRing, true)
	if err != nil {
		t.Fatal(err)
	}
	newKeys, err := loadCookieSecret("", newRing, true)
	if err != nil {
		t.Fatal(err)
	}
	gw := &Gateway{cfg: &Config{CookieSecret: newKeys[0], CookieVerifyKeys: newKeys[1:]}}
	if !gw.oauth().VerifyEntraStateCookie(entra.SignState(oldKeys[0], "s"), "s") {
		t.Fatal("cookie signed with the old primary rejected after rotation")
	}
	if gw.oauth().VerifyEntraStateCookie(entra.SignState(make([]byte, 32), "s"), "s") {
		t.Fatal("cookie signed with an unknown key accepted")
	}
}
