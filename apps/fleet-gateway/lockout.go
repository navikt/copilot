package main

import (
	"context"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/audit"
	"github.com/navikt/copilot/fleet-gateway/internal/lockout"
)

// Lockout logic lives in internal/lockout. These aliases and delegating
// methods keep the old names working in package main until the remaining
// callers move (#2499).

const (
	lockoutNoExpiry       = lockout.NoExpiry
	userRevokedMarkerKey  = lockout.MarkerKey
	lockoutAuditOutboxKey = lockout.AuditOutboxKey
)

type lockoutAuditEntry = audit.LockoutEntry

var (
	deriveMCPUserKey      = lockout.DeriveMCPUserKey
	lockoutKeyFingerprint = lockout.KeyFingerprint
	userKeyWith           = lockout.UserKey
	checkLockoutMarker    = lockout.CheckMarker
)

func (g *Gateway) lockouts() *lockout.Manager {
	return lockout.New(g.store, g.lockoutKey, g.mcpUserKey, g.now)
}

func (g *Gateway) userKey(navIdent string) string {
	return g.lockouts().UserKey(navIdent)
}

func (g *Gateway) mcpSessionUserKey(navIdent string) string {
	return g.lockouts().MCPSessionUserKey(navIdent)
}

func (g *Gateway) lockOutUser(ctx context.Context, navIdent, operator string) (time.Time, error) {
	return g.lockouts().LockOutUser(ctx, navIdent, operator)
}

func (g *Gateway) undoLockout(ctx context.Context, navIdent, operator string) error {
	return g.lockouts().UndoLockout(ctx, navIdent, operator)
}

func (g *Gateway) drainLockoutAudit(ctx context.Context) {
	if n := g.lockouts().DrainAudit(ctx); n >= 0 {
		lockoutAuditOutboxPending.Set(float64(n))
	}
}

func (g *Gateway) userRevoked(ctx context.Context, navIdent string, issuedAt time.Time) (bool, error) {
	return g.lockouts().UserRevoked(ctx, navIdent, issuedAt)
}
