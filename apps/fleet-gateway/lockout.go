package main

import (
	"context"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const userLockoutKeyInfo = "fleet-gateway user-lockout hmac v1"

func deriveUserKeys(rtKeys *keyRing) ([][]byte, error) {
	if rtKeys == nil || len(rtKeys.entries) == 0 {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate user lockout key: %w", err)
		}
		return [][]byte{secret}, nil
	}
	keys := make([][]byte, 0, len(rtKeys.entries))
	for _, e := range rtKeys.entries {
		k, err := hkdf.Key(sha256.New, e.key[:], nil, userLockoutKeyInfo, 32)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func userKeyWith(key []byte, navIdent string) string {
	mac := hmac.New(sha256.New, key)
	// Normalised so the CLI argument matches the Entra claim regardless of case.
	_, _ = mac.Write([]byte(strings.ToUpper(strings.TrimSpace(navIdent))))
	return hex.EncodeToString(mac.Sum(nil))
}

func (g *Gateway) userKey(navIdent string) string {
	if len(g.userKeys) == 0 {
		return ""
	}
	return userKeyWith(g.userKeys[0], navIdent)
}

func (g *Gateway) lockoutTTL() time.Duration {
	var ttl time.Duration
	if g.cfg != nil {
		ttl = max(g.cfg.SessionMaxAge, g.cfg.ConsentTTL)
	}
	if ttl <= 0 {
		ttl = max(defaultSessionMaxAge, defaultConsentTTL)
	}
	return ttl + 24*time.Hour
}

func (g *Gateway) lockOutUser(ctx context.Context, navIdent, operator string) (time.Time, error) {
	if navIdent == "" {
		return time.Time{}, errors.New("empty navIdent")
	}
	if len(g.userKeys) == 0 {
		return time.Time{}, errors.New("user lockout keys not initialized")
	}
	nowFn := g.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().UTC()
	key := g.userKey(navIdent)
	val := []byte(strconv.FormatInt(now.Unix(), 10))
	if err := g.store.Put(ctx, kindUserRevoked, key, val, g.lockoutTTL()); err != nil {
		return time.Time{}, err
	}
	slog.Info("audit lockout", "outcome", "ok", "user_key", key, "operator", sanitizeLogValue(operator))
	return now, nil
}

func (g *Gateway) revokedAt(ctx context.Context, navIdent string) (time.Time, bool, error) {
	if navIdent == "" || len(g.userKeys) == 0 {
		return time.Time{}, false, nil
	}
	var latest time.Time
	found := false
	for _, k := range g.userKeys {
		raw, ok, err := g.store.Get(ctx, kindUserRevoked, userKeyWith(k, navIdent))
		if err != nil {
			return time.Time{}, false, err
		}
		if !ok {
			continue
		}
		sec, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("parse lockout timestamp: %w", err)
		}
		ts := time.Unix(sec, 0).UTC()
		if !found || ts.After(latest) {
			latest = ts
			found = true
		}
	}
	return latest, found, nil
}

func (g *Gateway) userRevoked(ctx context.Context, navIdent string, issuedAt time.Time) (bool, error) {
	revoked, ok, err := g.revokedAt(ctx, navIdent)
	if err != nil || !ok {
		return false, err
	}
	return issuedAt.Unix() <= revoked.Unix(), nil
}
