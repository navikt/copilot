package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"
)

func (g *Gateway) putPending(ctx context.Context, kind, key string, p *pendingAuth) error {
	if p == nil {
		return errors.New("nil pendingAuth")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return g.store.Put(ctx, kind, hashToken(key), raw, defaultPendingTTL)
}

func (g *Gateway) popPending(ctx context.Context, kind, key string) (*pendingAuth, bool, error) {
	raw, ok, err := g.popHashed(ctx, kind, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	var p pendingAuth
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, false, err
	}
	return &p, true, nil
}

func (g *Gateway) putCode(ctx context.Context, code string, c *issuedCode) error {
	if c == nil {
		return errors.New("nil issuedCode")
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return g.store.Put(ctx, kindCode, hashToken(code), raw, defaultCodeTTL)
}

func (g *Gateway) popCode(ctx context.Context, code string) (*issuedCode, bool, error) {
	raw, ok, err := g.popHashed(ctx, kindCode, code)
	if err != nil || !ok {
		return nil, ok, err
	}
	var c issuedCode
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, false, err
	}
	return &c, true, nil
}

func (g *Gateway) popHashed(ctx context.Context, kind, key string) ([]byte, bool, error) {
	raw, ok, err := g.store.Pop(ctx, kind, hashToken(key))
	if err == nil && !ok {
		// shortcut: plaintext-key fallback for rows written before this change; remove after one pending/code TTL (defaultPendingTTL/defaultCodeTTL) has passed since deploy
		raw, ok, err = g.store.Pop(ctx, kind, key)
	}
	return raw, ok, err
}

func (g *Gateway) putSession(ctx context.Context, hash string, s *gatewaySession, ttl time.Duration) error {
	return putSessionInStore(ctx, g.store, g.now, g.cfg.SessionMaxAge, hash, s, ttl)
}

func (g *Gateway) getSession(ctx context.Context, hash string) (*gatewaySession, bool, error) {
	sess, ok, err := getSessionFromStore(ctx, g.store, g.now, g.cfg.SessionMaxAge, hash)
	if err != nil || !ok {
		return sess, ok, err
	}
	revoked, err := g.userRevoked(ctx, sess.NAVIdent, sess.IssuedAt)
	if err != nil {
		return nil, false, err
	}
	if revoked {
		_ = g.store.Delete(ctx, kindSession, hash)
		return nil, false, nil
	}
	return sess, true, nil
}

func putSessionInStore(ctx context.Context, store SessionStore, nowFn func() time.Time, maxAge time.Duration, hash string, s *gatewaySession, ttl time.Duration) error {
	if s == nil {
		return errors.New("nil gatewaySession")
	}
	now := nowFn()
	sessCopy := *s
	if maxAge > 0 {
		if sessCopy.IssuedAt.IsZero() {
			sessCopy.IssuedAt = now
		}
		absExpiry := sessCopy.IssuedAt.Add(maxAge)
		if sessCopy.Expiry.IsZero() || sessCopy.Expiry.After(absExpiry) {
			sessCopy.Expiry = absExpiry
		}
		if maxTTL := absExpiry.Sub(now); ttl > maxTTL {
			ttl = maxTTL
		}
	}
	raw, err := json.Marshal(&sessCopy)
	if err != nil {
		return err
	}
	return store.Put(ctx, kindSession, hash, raw, ttl)
}

func getSessionFromStore(ctx context.Context, store SessionStore, nowFn func() time.Time, maxAge time.Duration, hash string) (*gatewaySession, bool, error) {
	raw, ok, err := store.Get(ctx, kindSession, hash)
	if err != nil {
		if errors.Is(err, errSealUnknownKID) {
			// Keep the row: during a key rollout another replica may hold the new kid.
			slog.Warn("session row sealed with unknown key id, treating as missing")
			return nil, false, nil
		}
		if errors.Is(err, errSealDecrypt) || errors.Is(err, errSealMalformed) {
			dropUnreadableSession(ctx, store, hash, err)
			return nil, false, nil
		}
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	var sess gatewaySession
	if err := json.Unmarshal(raw, &sess); err != nil {
		dropUnreadableSession(ctx, store, hash, err)
		return nil, false, nil
	}
	now := nowFn()
	if !now.Before(sess.Expiry) {
		return nil, false, nil
	}
	if maxAge > 0 && !sess.IssuedAt.IsZero() {
		absExpiry := sess.IssuedAt.Add(maxAge)
		if !now.Before(absExpiry) {
			return nil, false, nil
		}
		if sess.Expiry.After(absExpiry) {
			sess.Expiry = absExpiry
		}
	}
	return &sess, true, nil
}

func dropUnreadableSession(ctx context.Context, store SessionStore, hash string, err error) {
	slog.Warn("unreadable session row, treating as missing", "reason", err.Error())
	_ = store.Delete(ctx, kindSession, hash)
}

// consentKey hashes the (user, client, route) triple so client IDs of any
// length fit the store key and NAVidents are not stored as plain keys.
func consentKey(navIdent, clientID, resource string) string {
	return hashToken(navIdent + "\x00" + clientID + "\x00" + resource)
}

func (g *Gateway) hasConsent(ctx context.Context, navIdent, clientID, resource string) (bool, error) {
	key := consentKey(navIdent, clientID, resource)
	raw, ok, err := g.store.Get(ctx, kindConsent, key)
	if err != nil || !ok {
		return ok, err
	}
	created := time.Unix(0, 0).UTC()
	if s := string(raw); s != "1" {
		if sec, parseErr := strconv.ParseInt(s, 10, 64); parseErr == nil {
			created = time.Unix(sec, 0).UTC()
		}
	}
	revoked, err := g.userRevoked(ctx, navIdent, created)
	if err != nil {
		return false, err
	}
	if revoked {
		// Delete lazily at read time: consent keys are hashed per (user, client, resource) and cannot be enumerated per user.
		_ = g.store.Delete(ctx, kindConsent, key)
		return false, nil
	}
	return true, nil
}

func (g *Gateway) rememberConsent(ctx context.Context, navIdent, clientID, resource string) error {
	val := []byte(strconv.FormatInt(g.now().Unix(), 10))
	return g.store.Put(ctx, kindConsent, consentKey(navIdent, clientID, resource), val, g.cfg.ConsentTTL)
}

func (g *Gateway) forgetConsent(ctx context.Context, navIdent, clientID, resource string) error {
	return g.store.Delete(ctx, kindConsent, consentKey(navIdent, clientID, resource))
}
