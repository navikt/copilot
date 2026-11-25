// Package oauth holds the gateway's typed OAuth records over session.Store:
// pending authorizations, authorization codes, sessions and consent.
package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/session"
)

const (
	// PendingTTL bounds a pending authorization and its state cookies.
	PendingTTL = 10 * time.Minute
	// codeTTL bounds a single-use authorization code.
	codeTTL = 60 * time.Second
)

// PendingAuth is an /authorize request waiting for the Entra login or consent.
type PendingAuth struct {
	ClientID      string    `json:"client_id"`
	ClientName    string    `json:"client_name"`
	Unverified    bool      `json:"unverified,omitempty"`
	RedirectURI   string    `json:"redirect_uri"`
	State         string    `json:"state"`
	Challenge     string    `json:"challenge"`
	Resource      string    `json:"resource"`
	Prompt        string    `json:"prompt,omitempty"`
	Claims        string    `json:"claims,omitempty"`
	EntraVerifier string    `json:"entra_verifier,omitempty"`
	EntraNonce    string    `json:"entra_nonce,omitempty"`
	CSRF          string    `json:"csrf"`
	Created       time.Time `json:"created"`
	// Issued is set when consent is asked after the Entra login: the code is
	// held here until the user approves.
	Issued *IssuedCode `json:"issued,omitempty"`
}

// Session is the gateway session behind an access token.
type Session struct {
	ClientID     string    `json:"client_id"`
	Resource     string    `json:"resource"`
	EntraToken   string    `json:"entra_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"` //nolint:gosec // serialized only into SessionStore (sealed with RT_KEYS in Postgres)
	NAVIdent     string    `json:"nav_ident"`
	FID          string    `json:"fid,omitempty"`
	IssuedAt     time.Time `json:"issued_at"`
	Expiry       time.Time `json:"expiry"`
}

// IssuedCode is an authorization code waiting for /token.
type IssuedCode struct {
	ClientID    string    `json:"client_id"`
	RedirectURI string    `json:"redirect_uri"`
	Challenge   string    `json:"challenge"`
	Resource    string    `json:"resource"`
	Session     Session   `json:"session"`
	EntraRT     string    `json:"entra_rt,omitempty"` //nolint:gosec // transient on 60s single-use code until /token seals it
	Created     time.Time `json:"created"`
}

// HashToken is the hex SHA-256 of token, used as the store key for secrets.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// PutPending stores p under the hash of key.
func PutPending(ctx context.Context, store session.Store, kind, key string, p *PendingAuth) error {
	if p == nil {
		return errors.New("nil pendingAuth")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return store.Put(ctx, kind, HashToken(key), raw, PendingTTL)
}

// PopPending reads and deletes the pending authorization for key.
func PopPending(ctx context.Context, store session.Store, kind, key string) (*PendingAuth, bool, error) {
	raw, ok, err := popHashed(ctx, store, kind, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	var p PendingAuth
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, false, err
	}
	return &p, true, nil
}

// PutCode stores c under the hash of code.
func PutCode(ctx context.Context, store session.Store, code string, c *IssuedCode) error {
	if c == nil {
		return errors.New("nil issuedCode")
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return store.Put(ctx, session.KindCode, HashToken(code), raw, codeTTL)
}

// PopCode reads and deletes the code, so it is single use.
func PopCode(ctx context.Context, store session.Store, code string) (*IssuedCode, bool, error) {
	raw, ok, err := popHashed(ctx, store, session.KindCode, code)
	if err != nil || !ok {
		return nil, ok, err
	}
	var c IssuedCode
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, false, err
	}
	return &c, true, nil
}

func popHashed(ctx context.Context, store session.Store, kind, key string) ([]byte, bool, error) {
	raw, ok, err := store.Pop(ctx, kind, HashToken(key))
	if err == nil && !ok {
		// shortcut: plaintext-key fallback for rows written before this change; remove after one pending/code TTL (PendingTTL/codeTTL) has passed since deploy
		raw, ok, err = store.Pop(ctx, kind, key)
	}
	return raw, ok, err
}

// PutSession stores s, capping its expiry and ttl at IssuedAt+maxAge.
func PutSession(ctx context.Context, store session.Store, nowFn func() time.Time, maxAge time.Duration, hash string, s *Session, ttl time.Duration) error {
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
	return store.Put(ctx, session.KindSession, hash, raw, ttl)
}

// GetSession reads a live session. It does not check user or family revocation.
func GetSession(ctx context.Context, store session.Store, nowFn func() time.Time, maxAge time.Duration, hash string) (*Session, bool, error) {
	raw, ok, err := store.Get(ctx, session.KindSession, hash)
	if err != nil {
		if errors.Is(err, session.ErrSealUnknownKID) {
			// Keep the row: during a key rollout another replica may hold the new kid.
			slog.Warn("session row sealed with unknown key id, treating as missing")
			return nil, false, nil
		}
		if errors.Is(err, session.ErrSealDecrypt) || errors.Is(err, session.ErrSealMalformed) {
			dropUnreadableSession(ctx, store, hash, err)
			return nil, false, nil
		}
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	var sess Session
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

func dropUnreadableSession(ctx context.Context, store session.Store, hash string, err error) {
	slog.Warn("unreadable session row, treating as missing", "reason", err.Error())
	_ = store.Delete(ctx, session.KindSession, hash)
}

// ConsentKey hashes the (user, client, route) triple so client IDs of any
// length fit the store key and NAVidents are not stored as plain keys.
func ConsentKey(navIdent, clientID, resource string) string {
	return HashToken(navIdent + "\x00" + clientID + "\x00" + resource)
}

// GetConsent reports whether consent is stored and when it was given. Rows
// with the old value "1" report the Unix epoch.
func GetConsent(ctx context.Context, store session.Store, navIdent, clientID, resource string) (time.Time, bool, error) {
	raw, ok, err := store.Get(ctx, session.KindConsent, ConsentKey(navIdent, clientID, resource))
	if err != nil || !ok {
		return time.Time{}, ok, err
	}
	created := time.Unix(0, 0).UTC()
	if s := string(raw); s != "1" {
		if sec, parseErr := strconv.ParseInt(s, 10, 64); parseErr == nil {
			created = time.Unix(sec, 0).UTC()
		}
	}
	return created, true, nil
}

// RememberConsent stores consent given at now for ttl.
func RememberConsent(ctx context.Context, store session.Store, now time.Time, ttl time.Duration, navIdent, clientID, resource string) error {
	val := []byte(strconv.FormatInt(now.Unix(), 10))
	_, err := store.PutEvict(ctx, session.KindConsent, ConsentKey(navIdent, clientID, resource), val, ttl)
	return err
}

// ForgetConsent deletes stored consent.
func ForgetConsent(ctx context.Context, store session.Store, navIdent, clientID, resource string) error {
	return store.Delete(ctx, session.KindConsent, ConsentKey(navIdent, clientID, resource))
}
