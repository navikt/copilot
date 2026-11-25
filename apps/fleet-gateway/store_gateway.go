package main

import (
	"context"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/oauth"
)

// These bind the oauth records to the gateway's current store, clock and
// config, which tests swap after construction.

func (g *Gateway) putPending(ctx context.Context, kind, key string, p *oauth.PendingAuth) error {
	return oauth.PutPending(ctx, g.store, kind, key, p)
}

func (g *Gateway) popPending(ctx context.Context, kind, key string) (*oauth.PendingAuth, bool, error) {
	return oauth.PopPending(ctx, g.store, kind, key)
}

func (g *Gateway) putCode(ctx context.Context, code string, c *oauth.IssuedCode) error {
	return oauth.PutCode(ctx, g.store, code, c)
}

func (g *Gateway) popCode(ctx context.Context, code string) (*oauth.IssuedCode, bool, error) {
	return oauth.PopCode(ctx, g.store, code)
}

func (g *Gateway) putSession(ctx context.Context, hash string, s *oauth.Session, ttl time.Duration) error {
	return oauth.PutSession(ctx, g.store, g.now, g.cfg.SessionAbsoluteMaxAge, hash, s, ttl)
}

func (g *Gateway) getSession(ctx context.Context, hash string) (*oauth.Session, bool, error) {
	sess, ok, err := oauth.GetSession(ctx, g.store, g.now, g.cfg.SessionAbsoluteMaxAge, hash)
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
	if sess.FID != "" {
		famRevoked, err := g.store.IsRevoked(ctx, "fam:"+sess.FID)
		if err != nil {
			return nil, false, err
		}
		if famRevoked {
			_ = g.store.Delete(ctx, kindSession, hash)
			return nil, false, nil
		}
	}
	return sess, true, nil
}

func (g *Gateway) hasConsent(ctx context.Context, navIdent, clientID, resource string) (bool, error) {
	created, ok, err := oauth.GetConsent(ctx, g.store, navIdent, clientID, resource)
	if err != nil || !ok {
		return ok, err
	}
	revoked, err := g.userRevoked(ctx, navIdent, created)
	if err != nil {
		return false, err
	}
	if revoked {
		// Delete lazily at read time: consent keys are hashed per (user, client, resource) and cannot be enumerated per user.
		_ = g.store.Delete(ctx, kindConsent, oauth.ConsentKey(navIdent, clientID, resource))
		return false, nil
	}
	return true, nil
}

func (g *Gateway) rememberConsent(ctx context.Context, navIdent, clientID, resource string) error {
	return oauth.RememberConsent(ctx, g.store, g.now(), g.cfg.ConsentTTL, navIdent, clientID, resource)
}

func (g *Gateway) forgetConsent(ctx context.Context, navIdent, clientID, resource string) error {
	return oauth.ForgetConsent(ctx, g.store, navIdent, clientID, resource)
}
