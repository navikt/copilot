package main

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func b64(fill byte, n int) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, n))
}

func testKeys() serverKeys {
	return serverKeys{
		confirm:  []signingKey{{kid: "k1", key: bytes.Repeat([]byte{1}, minKeyBytes)}},
		userHash: bytes.Repeat([]byte{2}, minKeyBytes),
	}
}

func setTestKeyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CONFIRM_KEYS", "k1:"+b64(1, 32))
	t.Setenv("USER_HASH_KEY", b64(2, 32))
	t.Setenv(generatedKeysEnv, "")
	t.Setenv("NAIS_CLUSTER_NAME", "")
}

func TestLoadServerKeys(t *testing.T) {
	setTestKeyEnv(t)
	t.Setenv("CONFIRM_KEYS", "new:"+b64(3, 32)+", old:"+b64(1, 32))
	k, err := loadServerKeys()
	if err != nil || len(k.confirm) != 2 || k.confirm[0].kid != "new" || k.generated {
		t.Fatalf("keys = %+v, err = %v", k, err)
	}

	bad := map[string][2]string{
		"missing confirm":   {"", b64(2, 32)},
		"missing user hash": {"k1:" + b64(1, 32), ""},
		"short confirm":     {"k1:" + b64(1, 31), b64(2, 32)},
		"short user hash":   {"k1:" + b64(1, 32), b64(2, 31)},
		"no kid":            {b64(1, 32), b64(2, 32)},
		"bad kid":           {"k.1:" + b64(1, 32), b64(2, 32)},
		"duplicate kid":     {"k1:" + b64(1, 32) + ",k1:" + b64(3, 32), b64(2, 32)},
		"not base64":        {"k1:!!!notbase64!!!", b64(2, 32)},
		"empty entry":       {"k1:" + b64(1, 32) + ",", b64(2, 32)},
	}
	for name, v := range bad {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CONFIRM_KEYS", v[0])
			t.Setenv("USER_HASH_KEY", v[1])
			_, err := loadServerKeys()
			if err == nil {
				t.Fatal("startup must fail")
			}
			if strings.HasPrefix(name, "missing") && !strings.Contains(err.Error(), "mcp-dolly-keys") {
				t.Fatalf("missing key should name the secret: %v", err)
			}
			// Errors must not echo key material.
			for _, secret := range []string{b64(1, 32), b64(2, 32), b64(1, 31), b64(2, 31)} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error leaks key: %v", err)
				}
			}
			if _, err := loadConfigFromEnv(); err == nil {
				t.Fatal("loadConfigFromEnv must fail")
			}
		})
	}
}

func TestGeneratedKeysFlag(t *testing.T) {
	setTestKeyEnv(t)
	t.Setenv("CONFIRM_KEYS", "")
	t.Setenv("USER_HASH_KEY", "")
	t.Setenv(generatedKeysEnv, "true")
	k, err := loadServerKeys()
	if err != nil || !k.generated || len(k.confirm[0].key) != minKeyBytes || len(k.userHash) != minKeyBytes {
		t.Fatalf("generated keys = %+v, err = %v", k, err)
	}
	t.Setenv("NAIS_CLUSTER_NAME", "dev-gcp")
	if _, err := loadServerKeys(); err != nil {
		t.Fatalf("dev with flag: %v", err)
	}
	t.Setenv("NAIS_CLUSTER_NAME", "prod-gcp")
	if _, err := loadServerKeys(); err == nil {
		t.Fatal("generated keys must be refused in prod")
	}
	t.Setenv("NAIS_CLUSTER_NAME", "dev-gcp")
	t.Setenv("USER_HASH_KEY", b64(2, 32))
	if _, err := loadServerKeys(); err == nil {
		t.Fatal("flag together with a configured key must fail")
	}
}

func TestConfirmKeyRotation(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	oldK := signingKey{"old", bytes.Repeat([]byte{1}, minKeyBytes)}
	newK := signingKey{"new", bytes.Repeat([]byte{3}, minKeyBytes)}

	before := newOrderConfirm([]signingKey{oldK})
	oldTok, _ := before.issue("u", "p", now)

	after := newOrderConfirm([]signingKey{newK, oldK})
	newTok, _ := after.issue("u", "p", now)
	if !strings.HasPrefix(newTok, "new.") {
		t.Fatalf("must sign with the first key: %s", newTok)
	}
	if err := after.consume(oldTok, "u", "p", now); err != nil {
		t.Fatalf("old key must still verify: %v", err)
	}
	if err := after.consume(newTok, "u", "p", now); err != nil {
		t.Fatalf("new token: %v", err)
	}

	retired := newOrderConfirm([]signingKey{newK})
	tok2, _ := before.issue("u", "p", now)
	if err := retired.consume(tok2, "u", "p", now); err == nil {
		t.Fatal("retired key must not verify")
	}
	// Relabelling the kid must not let a token verify under another key.
	relabel := "new" + tok2[len("old"):]
	if err := after.consume(relabel, "u", "p", now); err == nil {
		t.Fatal("kid swap must not verify")
	}
}

func TestTokenVerifiesAcrossPods(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	podA, podB := newServer(&config{Keys: testKeys()}), newServer(&config{Keys: testKeys()})
	if podA.quota.hashUser("Z123456") != podB.quota.hashUser("Z123456") {
		t.Fatal("user hash differs between pods with the same key")
	}
	tok, _ := podA.confirm.issue("u", "p", now)
	if err := podB.confirm.consume(tok, "u", "p", now); err != nil {
		t.Fatalf("token from pod A rejected on pod B: %v", err)
	}
	other := testKeys()
	other.confirm = []signingKey{{"k1", bytes.Repeat([]byte{7}, minKeyBytes)}}
	podC := newServer(&config{Keys: other})
	tok, _ = podA.confirm.issue("u", "p", now)
	if err := podC.confirm.consume(tok, "u", "p", now); err == nil {
		t.Fatal("pod with a different key must reject")
	}
}
