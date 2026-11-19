package main

import (
	"context"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Store TTL 0 means expire now, so lockout records, the marker, and the audit outbox use a 100-year TTL.
const lockoutNoExpiry = 100 * 365 * 24 * time.Hour

const (
	userRevokedMarkerKey  = "any"
	lockoutAuditOutboxKey = "lockout"
	userLockoutKeyInfo    = "fleet-gateway user-lockout hmac v1"
)

// Records keyed with the old RT_KEYS scheme are not migrated (none exist in prod).

type lockoutRecord struct {
	RevokedBefore int64 `json:"revoked_before"`
	Blocked       bool  `json:"blocked"`
}

type lockoutAuditEntry struct {
	Action   string `json:"action"`
	UserKey  string `json:"user_key"`
	Operator string `json:"operator"`
	At       int64  `json:"at"`
}

func parseLockoutRecord(raw []byte) (lockoutRecord, error) {
	s := strings.TrimSpace(string(raw))
	if strings.HasPrefix(s, "{") {
		var rec lockoutRecord
		if err := json.Unmarshal([]byte(s), &rec); err != nil {
			return lockoutRecord{}, fmt.Errorf("parse lockout record: %w", err)
		}
		return rec, nil
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return lockoutRecord{}, fmt.Errorf("parse lockout timestamp: %w", err)
	}
	return lockoutRecord{RevokedBefore: sec, Blocked: true}, nil
}

func deriveMCPUserKey(lockoutKey []byte, rtKeys *keyRing) ([]byte, error) {
	if len(lockoutKey) > 0 {
		return lockoutKey, nil
	}
	if rtKeys != nil && len(rtKeys.entries) > 0 {
		return hkdf.Key(sha256.New, rtKeys.entries[0].key[:], nil, userLockoutKeyInfo, 32)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate mcp user key: %w", err)
	}
	return secret, nil
}

func userKeyWith(key []byte, navIdent string) string {
	mac := hmac.New(sha256.New, key)
	// Normalised so the CLI argument matches the Entra claim regardless of case.
	_, _ = mac.Write([]byte(strings.ToUpper(strings.TrimSpace(navIdent))))
	return hex.EncodeToString(mac.Sum(nil))
}

func (g *Gateway) userKey(navIdent string) string {
	if len(g.lockoutKey) == 0 {
		return ""
	}
	return userKeyWith(g.lockoutKey, navIdent)
}

func (g *Gateway) mcpSessionUserKey(navIdent string) string {
	if len(g.mcpUserKey) > 0 {
		return userKeyWith(g.mcpUserKey, navIdent)
	}
	return g.userKey(navIdent)
}

func (g *Gateway) nowTime() time.Time {
	if g.now == nil {
		return time.Now().UTC()
	}
	return g.now().UTC()
}

func (g *Gateway) lockOutUser(ctx context.Context, navIdent, operator string) (time.Time, error) {
	if strings.TrimSpace(navIdent) == "" {
		return time.Time{}, errors.New("empty navIdent")
	}
	if len(g.lockoutKey) == 0 {
		return time.Time{}, errors.New("lockout key not initialized")
	}
	now := g.nowTime()
	key := g.userKey(navIdent)
	val, err := json.Marshal(lockoutRecord{
		RevokedBefore: now.Unix(),
		Blocked:       true,
	})
	if err != nil {
		return time.Time{}, err
	}
	if err := g.store.Put(ctx, kindUserRevoked, key, val, lockoutNoExpiry); err != nil {
		return time.Time{}, err
	}
	if err := g.store.Put(ctx, kindUserRevokedMarker, userRevokedMarkerKey, []byte("1"), lockoutNoExpiry); err != nil {
		return time.Time{}, err
	}
	if err := g.appendLockoutAudit(ctx, lockoutAuditEntry{
		Action:   "lock",
		UserKey:  key,
		Operator: operator,
		At:       now.Unix(),
	}); err != nil {
		return time.Time{}, err
	}
	return now, nil
}

func (g *Gateway) undoLockout(ctx context.Context, navIdent, operator string) error {
	if strings.TrimSpace(navIdent) == "" {
		return errors.New("empty navIdent")
	}
	if len(g.lockoutKey) == 0 {
		return errors.New("lockout key not initialized")
	}
	rec, found, err := g.revokedAt(ctx, navIdent)
	if err != nil {
		return err
	}
	key := g.userKey(navIdent)
	if found {
		val, err := json.Marshal(lockoutRecord{
			RevokedBefore: rec.RevokedBefore,
			Blocked:       false,
		})
		if err != nil {
			return err
		}
		if err := g.store.Put(ctx, kindUserRevoked, key, val, lockoutNoExpiry); err != nil {
			return err
		}
	}
	return g.appendLockoutAudit(ctx, lockoutAuditEntry{
		Action:   "undo",
		UserKey:  key,
		Operator: operator,
		At:       g.nowTime().Unix(),
	})
}

// shortcut: Get+Put is not atomic; a drain racing an append can re-log entries (duplicates, never loss). Fine for a rare operator action.
func (g *Gateway) appendLockoutAudit(ctx context.Context, entry lockoutAuditEntry) error {
	raw, ok, err := g.store.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey)
	if err != nil {
		return err
	}
	var entries []lockoutAuditEntry
	if ok {
		if err = json.Unmarshal(raw, &entries); err != nil {
			return fmt.Errorf("parse lockout audit outbox: %w", err)
		}
	}
	entries = append(entries, entry)
	val, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return g.store.Put(ctx, kindAuditOutbox, lockoutAuditOutboxKey, val, lockoutNoExpiry)
}

func (g *Gateway) drainLockoutAudit(ctx context.Context) {
	raw, ok, err := g.store.Pop(ctx, kindAuditOutbox, lockoutAuditOutboxKey)
	if err != nil {
		slog.Warn("drain lockout audit outbox failed", "error", err)
		return
	}
	if !ok {
		return
	}
	var entries []lockoutAuditEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		slog.Warn("parse lockout audit outbox failed", "error", err)
		return
	}
	for _, e := range entries {
		slog.Info("audit lockout",
			"outcome", "ok",
			"action", e.Action,
			"user_key", e.UserKey,
			"operator", sanitizeLogValue(e.Operator),
			"at", e.At,
		)
	}
}

func (g *Gateway) revokedAt(ctx context.Context, navIdent string) (lockoutRecord, bool, error) {
	if strings.TrimSpace(navIdent) == "" || len(g.lockoutKey) == 0 {
		return lockoutRecord{}, false, nil
	}
	raw, ok, err := g.store.Get(ctx, kindUserRevoked, userKeyWith(g.lockoutKey, navIdent))
	if err != nil || !ok {
		return lockoutRecord{}, false, err
	}
	rec, err := parseLockoutRecord(raw)
	if err != nil {
		return lockoutRecord{}, false, err
	}
	return rec, true, nil
}

func (g *Gateway) userRevoked(ctx context.Context, navIdent string, issuedAt time.Time) (bool, error) {
	rec, ok, err := g.revokedAt(ctx, navIdent)
	if err != nil || !ok {
		return false, err
	}
	return rec.Blocked || issuedAt.Unix() <= rec.RevokedBefore, nil
}
