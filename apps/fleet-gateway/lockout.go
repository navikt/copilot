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

	"github.com/navikt/copilot/fleet-gateway/internal/audit"
)

// Store TTL 0 means expire now, so lockout records, the marker, and the audit outbox use a 100-year TTL.
const lockoutNoExpiry = 100 * 365 * 24 * time.Hour

const (
	userRevokedMarkerKey       = "any"
	lockoutAuditOutboxKey      = "lockout"
	lockoutAuditCASAttempts    = 10
	userLockoutKeyInfo         = "fleet-gateway user-lockout hmac v1"
	lockoutKeyFingerprintLabel = "fleet-gateway/lockout-key-fingerprint/v1"
)

// Records keyed with the old RT_KEYS scheme are not migrated (none exist in prod).

type lockoutRecord struct {
	RevokedBefore int64 `json:"revoked_before"`
	Blocked       bool  `json:"blocked"`
}

type lockoutAuditEntry = audit.LockoutEntry

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
	if rtKeys != nil {
		if keys := rtKeys.Keys(); len(keys) > 0 {
			return hkdf.Key(sha256.New, keys[0], nil, userLockoutKeyInfo, 32)
		}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate mcp user key: %w", err)
	}
	return secret, nil
}

func lockoutKeyFingerprint(key []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(lockoutKeyFingerprintLabel))
	return hex.EncodeToString(mac.Sum(nil))
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
	if err := g.store.Put(ctx, kindUserRevokedMarker, userRevokedMarkerKey, []byte(lockoutKeyFingerprint(g.lockoutKey)), lockoutNoExpiry); err != nil {
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

// shortcut: drain logs before CAS-removing logged IDs, so a crash or failed write between log and remove produces duplicates rather than losing entries. Fine for a rare operator action.
func (g *Gateway) appendLockoutAudit(ctx context.Context, entry lockoutAuditEntry) error {
	if entry.ID == "" {
		var idBuf [16]byte
		if _, err := rand.Read(idBuf[:]); err != nil {
			return fmt.Errorf("generate lockout audit id: %w", err)
		}
		entry.ID = hex.EncodeToString(idBuf[:])
	}
	for range lockoutAuditCASAttempts {
		raw, ok, err := g.store.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey)
		if err != nil {
			return err
		}
		var entries []lockoutAuditEntry
		var oldVal []byte
		if ok {
			oldVal = raw
			if err = json.Unmarshal(raw, &entries); err != nil {
				return fmt.Errorf("parse lockout audit outbox: %w", err)
			}
		}
		entries = append(entries, entry)
		val, err := json.Marshal(entries)
		if err != nil {
			return err
		}
		swapped, err := g.store.CompareAndSwap(ctx, kindAuditOutbox, lockoutAuditOutboxKey, oldVal, val, lockoutNoExpiry)
		if err != nil {
			return err
		}
		if swapped {
			return nil
		}
	}
	return errors.New("append lockout audit outbox: conflict after retries")
}

func (g *Gateway) drainLockoutAudit(ctx context.Context) {
	raw, ok, err := g.store.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey)
	if err != nil {
		slog.Warn("drain lockout audit outbox failed", "error", err)
		return
	}
	if !ok {
		return
	}
	var entries []lockoutAuditEntry
	if err = json.Unmarshal(raw, &entries); err != nil {
		slog.Warn("parse lockout audit outbox failed", "error", err)
		return
	}
	loggedIDs := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		audit.LogLockout(e)
		loggedIDs[e.ID] = struct{}{}
	}

	currentRaw := raw
	current := entries
	for attempt := range lockoutAuditCASAttempts {
		if attempt > 0 {
			currentRaw, ok, err = g.store.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey)
			if err != nil {
				slog.Warn("re-read lockout audit outbox failed", "error", err)
				return
			}
			if !ok {
				return
			}
			current = nil
			if err = json.Unmarshal(currentRaw, &current); err != nil {
				slog.Warn("parse lockout audit outbox failed", "error", err)
				return
			}
		}
		remaining := make([]lockoutAuditEntry, 0, len(current))
		for _, e := range current {
			if _, logged := loggedIDs[e.ID]; !logged {
				remaining = append(remaining, e)
			}
		}
		if len(remaining) == len(current) {
			return
		}
		var newVal []byte
		if len(remaining) > 0 {
			newVal, err = json.Marshal(remaining)
			if err != nil {
				slog.Warn("marshal lockout audit outbox failed", "error", err)
				return
			}
		}
		swapped, casErr := g.store.CompareAndSwap(ctx, kindAuditOutbox, lockoutAuditOutboxKey, currentRaw, newVal, lockoutNoExpiry)
		if casErr != nil {
			if newVal == nil {
				slog.Warn("delete lockout audit outbox failed", "error", casErr)
			} else {
				slog.Warn("update lockout audit outbox failed", "error", casErr)
			}
			return
		}
		if swapped {
			return
		}
	}
	slog.Warn("update lockout audit outbox failed", "error", "conflict after retries")
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
