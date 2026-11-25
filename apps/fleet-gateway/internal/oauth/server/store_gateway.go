package server

import (
	"context"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/oauth"
	"github.com/navikt/copilot/fleet-gateway/internal/session"
)

// These bind the oauth records to the gateway's current store, clock and
// config, which tests swap after construction.

// PutPending stores a pending authorization.
func (g *Server) PutPending(ctx context.Context, kind, key string, p *oauth.PendingAuth) error {
	return oauth.PutPending(ctx, g.store, kind, key, p)
}

// PopPending reads and deletes a pending authorization.
func (g *Server) PopPending(ctx context.Context, kind, key string) (*oauth.PendingAuth, bool, error) {
	return oauth.PopPending(ctx, g.store, kind, key)
}

// PutCode stores an issued authorization code.
func (g *Server) PutCode(ctx context.Context, code string, c *oauth.IssuedCode) error {
	return oauth.PutCode(ctx, g.store, code, c)
}

// PopCode reads and deletes an authorization code, so it is single-use.
func (g *Server) PopCode(ctx context.Context, code string) (*oauth.IssuedCode, bool, error) {
	return oauth.PopCode(ctx, g.store, code)
}

// PutSession stores a session.
func (g *Server) PutSession(ctx context.Context, hash string, s *oauth.Session, ttl time.Duration) error {
	return oauth.PutSession(ctx, g.store, g.now, g.cfg.SessionAbsoluteMaxAge, hash, s, ttl)
}

// GetSession returns a session unless it, its user or its family is revoked.
func (g *Server) GetSession(ctx context.Context, hash string) (*oauth.Session, bool, error) {
	sess, ok, err := oauth.GetSession(ctx, g.store, g.now, g.cfg.SessionAbsoluteMaxAge, hash)
	if err != nil || !ok {
		return sess, ok, err
	}
	revoked, err := g.userRevoked(ctx, sess.NAVIdent, sess.IssuedAt)
	if err != nil {
		return nil, false, err
	}
	if revoked {
		_ = g.store.Delete(ctx, session.KindSession, hash)
		return nil, false, nil
	}
	if sess.FID != "" {
		famRevoked, err := g.store.IsRevoked(ctx, "fam:"+sess.FID)
		if err != nil {
			return nil, false, err
		}
		if famRevoked {
			_ = g.store.Delete(ctx, session.KindSession, hash)
			return nil, false, nil
		}
	}
	return sess, true, nil
}

// HasConsent reports whether the user's remembered consent is still valid.
func (g *Server) HasConsent(ctx context.Context, navIdent, clientID, resource string) (bool, error) {
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
		_ = g.store.Delete(ctx, session.KindConsent, oauth.ConsentKey(navIdent, clientID, resource))
		return false, nil
	}
	return true, nil
}

// RememberConsent stores the user's consent for ConsentTTL.
func (g *Server) RememberConsent(ctx context.Context, navIdent, clientID, resource string) error {
	return oauth.RememberConsent(ctx, g.store, g.now(), g.cfg.ConsentTTL, navIdent, clientID, resource)
}

func (g *Server) forgetConsent(ctx context.Context, navIdent, clientID, resource string) error {
	return oauth.ForgetConsent(ctx, g.store, navIdent, clientID, resource)
}
