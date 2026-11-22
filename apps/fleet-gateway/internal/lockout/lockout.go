// Package lockout implements per-user lockout records, marker verification,
// and the lockout audit outbox for fleet-gateway.
package lockout

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
	"github.com/navikt/copilot/fleet-gateway/internal/session"
)

// NoExpiry is the TTL used for lockout records, the marker, and the audit
// outbox because store TTL 0 means expire now.
const NoExpiry = 100 * 365 * 24 * time.Hour

const (
	// MarkerKey is the store key for the lockout presence/fingerprint marker.
	MarkerKey = "any"
	// AuditOutboxKey is the store key for the queued lockout audit entries.
	AuditOutboxKey = "lockout"

	auditCASAttempts    = 10
	userKeyInfo         = "fleet-gateway user-lockout hmac v1"
	keyFingerprintLabel = "fleet-gateway/lockout-key-fingerprint/v1"
)

// Records keyed with the old RT_KEYS scheme are not migrated (none exist in prod).

// Record represents a per-user lockout entry stored under session.KindUserRevoked.
type Record struct {
	RevokedBefore int64 `json:"revoked_before"`
	Blocked       bool  `json:"blocked"`
}

// ParseRecord decodes a JSON lockout record or a legacy Unix timestamp record.
func ParseRecord(raw []byte) (Record, error) {
	s := strings.TrimSpace(string(raw))
	if strings.HasPrefix(s, "{") {
		var rec Record
		if err := json.Unmarshal([]byte(s), &rec); err != nil {
			return Record{}, fmt.Errorf("parse lockout record: %w", err)
		}
		return rec, nil
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Record{}, fmt.Errorf("parse lockout timestamp: %w", err)
	}
	return Record{RevokedBefore: sec, Blocked: true}, nil
}

// DeriveMCPUserKey returns the HMAC key for per-user MCP session counters:
// lockoutKey when configured, otherwise HKDF from rtKeys' primary key, or an
// ephemeral random key.
func DeriveMCPUserKey(lockoutKey []byte, rtKeys *session.KeyRing) ([]byte, error) {
	if len(lockoutKey) > 0 {
		return lockoutKey, nil
	}
	if rtKeys != nil {
		if keys := rtKeys.Keys(); len(keys) > 0 {
			return hkdf.Key(sha256.New, keys[0], nil, userKeyInfo, 32)
		}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate mcp user key: %w", err)
	}
	return secret, nil
}

// KeyFingerprint returns a hex-encoded HMAC fingerprint of the lockout key.
func KeyFingerprint(key []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(keyFingerprintLabel))
	return hex.EncodeToString(mac.Sum(nil))
}

// UserKey returns the hex-encoded HMAC of a normalised NAVident.
func UserKey(key []byte, navIdent string) string {
	mac := hmac.New(sha256.New, key)
	// Normalised so the CLI argument matches the Entra claim regardless of case.
	_, _ = mac.Write([]byte(strings.ToUpper(strings.TrimSpace(navIdent))))
	return hex.EncodeToString(mac.Sum(nil))
}

// CheckMarker verifies that lockoutKey matches any existing lockout marker in
// store, upgrading a legacy "1" marker to a key fingerprint on success.
func CheckMarker(store session.Store, lockoutKey []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	marker, exists, err := store.Get(ctx, session.KindUserRevokedMarker, MarkerKey)
	if err != nil {
		return fmt.Errorf("check lockout marker: %w", err)
	}
	if !exists {
		return nil
	}
	if len(lockoutKey) == 0 {
		return errors.New("LOCKOUT_KEY is missing but lockout records exist; set the LOCKOUT_KEY secret")
	}
	want := []byte(KeyFingerprint(lockoutKey))
	if hmac.Equal(marker, want) {
		return nil
	}
	if hmac.Equal(marker, []byte("1")) {
		if err := store.Put(ctx, session.KindUserRevokedMarker, MarkerKey, want, NoExpiry); err != nil {
			return fmt.Errorf("update lockout marker: %w", err)
		}
		slog.Warn("legacy lockout marker upgraded to key fingerprint")
		return nil
	}
	return errors.New("LOCKOUT_KEY does not match the key the existing lockout records were written with (changing it would lift all lockouts; restore the old key)")
}

// Manager reads and writes per-user lockout records and drains the lockout
// audit outbox against a session.Store.
type Manager struct {
	store      session.Store
	lockoutKey []byte
	mcpUserKey []byte
	now        func() time.Time
}

// New creates a Manager backed by store. A nil now function defaults to
// time.Now.
func New(store session.Store, lockoutKey, mcpUserKey []byte, now func() time.Time) *Manager {
	return &Manager{
		store:      store,
		lockoutKey: lockoutKey,
		mcpUserKey: mcpUserKey,
		now:        now,
	}
}

// UserKey returns the HMAC key for navIdent under lockoutKey, or "" if
// lockoutKey is empty.
func (m *Manager) UserKey(navIdent string) string {
	if len(m.lockoutKey) == 0 {
		return ""
	}
	return UserKey(m.lockoutKey, navIdent)
}

// MCPSessionUserKey returns the HMAC key for navIdent used for MCP session
// user counters.
func (m *Manager) MCPSessionUserKey(navIdent string) string {
	if len(m.mcpUserKey) > 0 {
		return UserKey(m.mcpUserKey, navIdent)
	}
	return m.UserKey(navIdent)
}

func (m *Manager) nowTime() time.Time {
	if m.now == nil {
		return time.Now().UTC()
	}
	return m.now().UTC()
}

// LockOutUser records an active lockout for navIdent, writes the lockout
// marker, and queues an audit entry.
func (m *Manager) LockOutUser(ctx context.Context, navIdent, operator string) (time.Time, error) {
	if strings.TrimSpace(navIdent) == "" {
		return time.Time{}, errors.New("empty navIdent")
	}
	if len(m.lockoutKey) == 0 {
		return time.Time{}, errors.New("lockout key not initialized")
	}
	now := m.nowTime()
	key := m.UserKey(navIdent)
	val, err := json.Marshal(Record{
		RevokedBefore: now.Unix(),
		Blocked:       true,
	})
	if err != nil {
		return time.Time{}, err
	}
	if err := m.store.Put(ctx, session.KindUserRevoked, key, val, NoExpiry); err != nil {
		return time.Time{}, err
	}
	if err := m.store.Put(ctx, session.KindUserRevokedMarker, MarkerKey, []byte(KeyFingerprint(m.lockoutKey)), NoExpiry); err != nil {
		return time.Time{}, err
	}
	if err := m.appendAudit(ctx, audit.LockoutEntry{
		Action:   "lock",
		UserKey:  key,
		Operator: operator,
		At:       now.Unix(),
	}); err != nil {
		return time.Time{}, err
	}
	return now, nil
}

// UndoLockout lifts the active block for new logins while keeping RevokedBefore
// intact, and queues an audit entry.
func (m *Manager) UndoLockout(ctx context.Context, navIdent, operator string) error {
	if strings.TrimSpace(navIdent) == "" {
		return errors.New("empty navIdent")
	}
	if len(m.lockoutKey) == 0 {
		return errors.New("lockout key not initialized")
	}
	rec, found, err := m.RevokedAt(ctx, navIdent)
	if err != nil {
		return err
	}
	key := m.UserKey(navIdent)
	if found {
		val, err := json.Marshal(Record{
			RevokedBefore: rec.RevokedBefore,
			Blocked:       false,
		})
		if err != nil {
			return err
		}
		if err := m.store.Put(ctx, session.KindUserRevoked, key, val, NoExpiry); err != nil {
			return err
		}
	}
	return m.appendAudit(ctx, audit.LockoutEntry{
		Action:   "undo",
		UserKey:  key,
		Operator: operator,
		At:       m.nowTime().Unix(),
	})
}

// shortcut: drain logs before CAS-removing logged IDs, so a crash or failed write between log and remove produces duplicates rather than losing entries. Fine for a rare operator action.
func (m *Manager) appendAudit(ctx context.Context, entry audit.LockoutEntry) error {
	if entry.ID == "" {
		var idBuf [16]byte
		if _, err := rand.Read(idBuf[:]); err != nil {
			return fmt.Errorf("generate lockout audit id: %w", err)
		}
		entry.ID = hex.EncodeToString(idBuf[:])
	}
	for range auditCASAttempts {
		raw, ok, err := m.store.Get(ctx, session.KindAuditOutbox, AuditOutboxKey)
		if err != nil {
			return err
		}
		var entries []audit.LockoutEntry
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
		swapped, err := m.store.CompareAndSwap(ctx, session.KindAuditOutbox, AuditOutboxKey, oldVal, val, NoExpiry)
		if err != nil {
			return err
		}
		if swapped {
			return nil
		}
	}
	return errors.New("append lockout audit outbox: conflict after retries")
}

// DrainAudit logs all pending lockout audit entries and CAS-removes them from
// the outbox. It returns how many entries the outbox still holds afterwards, or
// -1 when the outbox could not be read. The outbox lives in the session store
// (Postgres in dev and prod), so entries left behind are drained on a later
// sweep, also after a restart.
func (m *Manager) DrainAudit(ctx context.Context) int {
	raw, ok, err := m.store.Get(ctx, session.KindAuditOutbox, AuditOutboxKey)
	if err != nil {
		slog.Warn("drain lockout audit outbox failed", "error", err)
		return -1
	}
	if !ok {
		return 0
	}
	var entries []audit.LockoutEntry
	if err = json.Unmarshal(raw, &entries); err != nil {
		slog.Warn("parse lockout audit outbox failed", "error", err)
		return -1
	}
	loggedIDs := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		audit.LogLockout(e)
		loggedIDs[e.ID] = struct{}{}
	}

	currentRaw := raw
	current := entries
	for attempt := range auditCASAttempts {
		if attempt > 0 {
			currentRaw, ok, err = m.store.Get(ctx, session.KindAuditOutbox, AuditOutboxKey)
			if err != nil {
				slog.Warn("re-read lockout audit outbox failed", "error", err)
				return -1
			}
			if !ok {
				return 0
			}
			current = nil
			if err = json.Unmarshal(currentRaw, &current); err != nil {
				slog.Warn("parse lockout audit outbox failed", "error", err)
				return -1
			}
		}
		remaining := make([]audit.LockoutEntry, 0, len(current))
		for _, e := range current {
			if _, logged := loggedIDs[e.ID]; !logged {
				remaining = append(remaining, e)
			}
		}
		if len(remaining) == len(current) {
			return len(current)
		}
		var newVal []byte
		if len(remaining) > 0 {
			newVal, err = json.Marshal(remaining)
			if err != nil {
				slog.Warn("marshal lockout audit outbox failed", "error", err)
				return len(current)
			}
		}
		swapped, casErr := m.store.CompareAndSwap(ctx, session.KindAuditOutbox, AuditOutboxKey, currentRaw, newVal, NoExpiry)
		if casErr != nil {
			if newVal == nil {
				slog.Warn("delete lockout audit outbox failed", "error", casErr)
			} else {
				slog.Warn("update lockout audit outbox failed", "error", casErr)
			}
			return len(current)
		}
		if swapped {
			return len(remaining)
		}
	}
	slog.Warn("update lockout audit outbox failed", "error", "conflict after retries")
	return len(current)
}

// RevokedAt returns the lockout record for navIdent, if any.
func (m *Manager) RevokedAt(ctx context.Context, navIdent string) (Record, bool, error) {
	if strings.TrimSpace(navIdent) == "" || len(m.lockoutKey) == 0 {
		return Record{}, false, nil
	}
	raw, ok, err := m.store.Get(ctx, session.KindUserRevoked, UserKey(m.lockoutKey, navIdent))
	if err != nil || !ok {
		return Record{}, false, err
	}
	rec, err := ParseRecord(raw)
	if err != nil {
		return Record{}, false, err
	}
	return rec, true, nil
}

// UserRevoked reports whether navIdent is blocked or was locked out at or after
// issuedAt.
func (m *Manager) UserRevoked(ctx context.Context, navIdent string, issuedAt time.Time) (bool, error) {
	rec, ok, err := m.RevokedAt(ctx, navIdent)
	if err != nil || !ok {
		return false, err
	}
	return rec.Blocked || issuedAt.Unix() <= rec.RevokedBefore, nil
}
