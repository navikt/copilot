package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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
	return g.store.Put(ctx, kind, key, raw, defaultPendingTTL)
}

func (g *Gateway) popPending(ctx context.Context, kind, key string) (*pendingAuth, bool, error) {
	raw, ok, err := g.store.Pop(ctx, kind, key)
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
	return g.store.Put(ctx, kindCode, code, raw, defaultCodeTTL)
}

func (g *Gateway) popCode(ctx context.Context, code string) (*issuedCode, bool, error) {
	raw, ok, err := g.store.Pop(ctx, kindCode, code)
	if err != nil || !ok {
		return nil, ok, err
	}
	var c issuedCode
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, false, err
	}
	return &c, true, nil
}

func (g *Gateway) putSession(ctx context.Context, hash string, s *gatewaySession, ttl time.Duration) error {
	return putSessionInStore(ctx, g.store, g.now, g.cfg.SessionMaxAge, hash, s, ttl)
}

func (g *Gateway) getSession(ctx context.Context, hash string) (*gatewaySession, bool, error) {
	return getSessionFromStore(ctx, g.store, g.now, g.cfg.SessionMaxAge, hash)
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
		if errors.Is(err, errSealUnknownKID) || errors.Is(err, errSealDecrypt) || errors.Is(err, errSealMalformed) {
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
