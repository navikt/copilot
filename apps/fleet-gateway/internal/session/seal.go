package session

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Sealing parameters for AES-256-GCM at-rest encryption.
const (
	sealMaxKIDLen = 32
	// SealMaxPlaintext bounds what Seal accepts (sessions and RTs are a few KiB).
	SealMaxPlaintext = 1 << 20
	SealKeyBytes     = 32
	sealNonceSize    = 12
)

// Sealing and unsealing errors returned by KeyRing.Open and SealedStore.
var (
	ErrSealUnknownKID = errors.New("sealed store: unknown key id")
	ErrSealDecrypt    = errors.New("sealed store: decryption failed")
	ErrSealMalformed  = errors.New("sealed store: malformed ciphertext")
)

type keyEntry struct {
	kid string
	key [SealKeyBytes]byte
}

// KeyRing holds AES-256 keys indexed by key ID; the first key seals new values.
type KeyRing struct {
	entries []keyEntry
}

// Keys returns a copy of each key in ring order; the first one seals.
func (kr *KeyRing) Keys() [][]byte {
	keys := make([][]byte, 0, len(kr.entries))
	for _, e := range kr.entries {
		keys = append(keys, append([]byte(nil), e.key[:]...))
	}
	return keys
}

// ParseKeyRing parses a comma-separated list of kid:base64key entries.
func ParseKeyRing(raw string) (*KeyRing, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("key ring must not be empty")
	}

	parts := strings.Split(trimmed, ",")
	entries := make([]keyEntry, 0, len(parts))
	seen := make(map[string]bool, len(parts))

	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			return nil, errors.New("key ring contains empty entry")
		}
		kid, b64, ok := strings.Cut(item, ":")
		if !ok {
			return nil, errors.New("key ring entry must be formatted as kid:base64key")
		}
		if !isValidKID(kid) {
			return nil, fmt.Errorf("invalid key id %q", kid)
		}
		if seen[kid] {
			return nil, fmt.Errorf("duplicate key id %q", kid)
		}
		seen[kid] = true

		decoded, err := decodeKeyMaterial(b64)
		if err != nil {
			return nil, fmt.Errorf("invalid key material for %q: %w", kid, err)
		}

		var entry keyEntry
		entry.kid = kid
		copy(entry.key[:], decoded)
		entries = append(entries, entry)
	}

	return &KeyRing{entries: entries}, nil
}

func isValidKID(kid string) bool {
	if len(kid) == 0 || len(kid) > sealMaxKIDLen {
		return false
	}
	for i := 0; i < len(kid); i++ {
		c := kid[i]
		isUpper := c >= 'A' && c <= 'Z'
		isLower := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		if !isUpper && !isLower && !isDigit && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

func decodeKeyMaterial(raw string) ([]byte, error) {
	if raw == "" {
		return nil, errors.New("empty key")
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return nil, errors.New("invalid base64 key")
		}
	}
	if len(decoded) != SealKeyBytes {
		return nil, fmt.Errorf("key must decode to %d bytes, got %d", SealKeyBytes, len(decoded))
	}
	return decoded, nil
}

// Seal encrypts plaintext with AES-GCM under the primary key, authenticated with aad.
func (kr *KeyRing) Seal(aad, plaintext []byte) ([]byte, error) {
	if kr == nil || len(kr.entries) == 0 {
		return nil, errors.New("sealed store: empty key ring")
	}
	primary := kr.entries[0]
	block, err := aes.NewCipher(primary.key[:])
	if err != nil {
		return nil, errors.New("sealed store: cipher initialization failed")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("sealed store: GCM initialization failed")
	}

	nonce := make([]byte, sealNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.New("sealed store: nonce generation failed")
	}

	kidBytes := []byte(primary.kid)
	if len(kidBytes) == 0 || len(kidBytes) > sealMaxKIDLen {
		return nil, errors.New("sealed store: invalid key id length")
	}
	if len(plaintext) > SealMaxPlaintext {
		return nil, errors.New("sealed store: plaintext too large")
	}
	out := append([]byte{byte(len(kidBytes))}, kidBytes...) //nolint:gosec // bounded by sealMaxKIDLen (32)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plaintext, aad)
	return out, nil
}

// Open decrypts and authenticates a blob produced by Seal using the key matching its header kid.
func (kr *KeyRing) Open(aad, blob []byte) ([]byte, error) {
	if kr == nil || len(kr.entries) == 0 {
		return nil, ErrSealDecrypt
	}
	if len(blob) < 1 {
		return nil, ErrSealMalformed
	}
	kidLen := int(blob[0])
	if kidLen == 0 || kidLen > sealMaxKIDLen || len(blob) < 1+kidLen+sealNonceSize {
		return nil, ErrSealMalformed
	}

	kid := string(blob[1 : 1+kidLen])
	nonce := blob[1+kidLen : 1+kidLen+sealNonceSize]
	ciphertext := blob[1+kidLen+sealNonceSize:]

	var matched *keyEntry
	for i := range kr.entries {
		if kr.entries[i].kid == kid {
			matched = &kr.entries[i]
			break
		}
	}
	if matched == nil {
		return nil, ErrSealUnknownKID
	}

	block, err := aes.NewCipher(matched.key[:])
	if err != nil {
		return nil, ErrSealDecrypt
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrSealDecrypt
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrSealDecrypt
	}
	return plaintext, nil
}

// SealedStore wraps a Store, sealing values at rest with a KeyRing.
type SealedStore struct {
	inner Store
	keys  *KeyRing
}

// NewSealedStore returns a SealedStore backed by inner and keys.
func NewSealedStore(inner Store, keys *KeyRing) *SealedStore {
	return &SealedStore{
		inner: inner,
		keys:  keys,
	}
}

// WithKeys returns a store over the same inner store that seals and opens with keys.
func (s *SealedStore) WithKeys(keys *KeyRing) *SealedStore {
	return NewSealedStore(s.inner, keys)
}

// Inner returns the underlying unsealed store.
func (s *SealedStore) Inner() Store {
	return s.inner
}

// KeyRing returns the key ring used to seal and open values.
func (s *SealedStore) KeyRing() *KeyRing {
	return s.keys
}

// Lockout records, the marker and the audit outbox hold only HMAC keys, timestamps and operator idents, and must stay readable after RT_KEYS is replaced (global logout must not lift lockouts or lose audit lines).
func unsealedKind(kind string) bool {
	return kind == KindUserRevoked || kind == KindUserRevokedMarker || kind == KindAuditOutbox
}

// SealAAD builds the associated data binding a sealed blob to its (kind, key).
func SealAAD(kind, key string) []byte {
	return []byte(kind + "\x00" + key)
}

// Put seals val (except unsealed kinds) and stores it in the inner store.
func (s *SealedStore) Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error {
	if unsealedKind(kind) {
		return s.inner.Put(ctx, kind, key, val, ttl)
	}
	blob, err := s.keys.Seal(SealAAD(kind, key), val)
	if err != nil {
		return err
	}
	return s.inner.Put(ctx, kind, key, blob, ttl)
}

// PutEvict seals val (except unsealed kinds) and stores it via PutEvict on the inner store.
func (s *SealedStore) PutEvict(ctx context.Context, kind, key string, val []byte, ttl time.Duration) (bool, error) {
	if unsealedKind(kind) {
		return s.inner.PutEvict(ctx, kind, key, val, ttl)
	}
	blob, err := s.keys.Seal(SealAAD(kind, key), val)
	if err != nil {
		return false, err
	}
	return s.inner.PutEvict(ctx, kind, key, blob, ttl)
}

// Legacy audit outbox rows written before KindAuditOutbox became unsealed start with the key-id header rather than '['.
func (s *SealedStore) decodeUnsealed(kind, key string, raw []byte) ([]byte, error) {
	if kind != KindAuditOutbox || (len(raw) > 0 && raw[0] == '[') {
		return raw, nil
	}
	return s.keys.Open(SealAAD(kind, key), raw)
}

// Get fetches and unseals the live value for (kind, key).
func (s *SealedStore) Get(ctx context.Context, kind, key string) ([]byte, bool, error) {
	if unsealedKind(kind) {
		raw, ok, err := s.inner.Get(ctx, kind, key)
		if err != nil || !ok {
			return nil, ok, err
		}
		decoded, err := s.decodeUnsealed(kind, key, raw)
		if err != nil {
			return nil, false, err
		}
		return decoded, true, nil
	}
	blob, ok, err := s.inner.Get(ctx, kind, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	plaintext, err := s.keys.Open(SealAAD(kind, key), blob)
	if err != nil {
		return nil, false, err
	}
	return plaintext, true, nil
}

// Pop removes, fetches and unseals the live value for (kind, key).
func (s *SealedStore) Pop(ctx context.Context, kind, key string) ([]byte, bool, error) {
	if unsealedKind(kind) {
		raw, ok, err := s.inner.Pop(ctx, kind, key)
		if err != nil || !ok {
			return nil, ok, err
		}
		decoded, err := s.decodeUnsealed(kind, key, raw)
		if err != nil {
			return nil, false, err
		}
		return decoded, true, nil
	}
	blob, ok, err := s.inner.Pop(ctx, kind, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	plaintext, err := s.keys.Open(SealAAD(kind, key), blob)
	if err != nil {
		return nil, false, err
	}
	return plaintext, true, nil
}

// Delete removes (kind, key) from the inner store.
func (s *SealedStore) Delete(ctx context.Context, kind, key string) error {
	return s.inner.Delete(ctx, kind, key)
}

// CompareAndSwap performs a compare-and-swap on unsealed kinds only.
func (s *SealedStore) CompareAndSwap(ctx context.Context, kind, key string, oldVal, newVal []byte, ttl time.Duration) (bool, error) {
	if !unsealedKind(kind) {
		return false, fmt.Errorf("sealed store: compare-and-swap unsupported for sealed kind %q", kind)
	}
	if kind != KindAuditOutbox || oldVal == nil {
		return s.inner.CompareAndSwap(ctx, kind, key, oldVal, newVal, ttl)
	}
	raw, ok, err := s.inner.Get(ctx, kind, key)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if len(raw) > 0 && raw[0] == '[' {
		return s.inner.CompareAndSwap(ctx, kind, key, oldVal, newVal, ttl)
	}
	plaintext, err := s.keys.Open(SealAAD(kind, key), raw)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(plaintext, oldVal) {
		return false, nil
	}
	return s.inner.CompareAndSwap(ctx, kind, key, raw, newVal, ttl)
}

// Revoke delegates to the inner store.
func (s *SealedStore) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	return s.inner.Revoke(ctx, jti, expiresAt)
}

// RevokeOnce delegates to the inner store.
func (s *SealedStore) RevokeOnce(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
	return s.inner.RevokeOnce(ctx, jti, expiresAt)
}

// IsRevoked delegates to the inner store.
func (s *SealedStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return s.inner.IsRevoked(ctx, jti)
}

// Incr delegates to the inner store.
func (s *SealedStore) Incr(ctx context.Context, key string, window time.Duration) (int, error) {
	return s.inner.Incr(ctx, key, window)
}

// CountPrefix delegates to the inner store because keys are never sealed.
func (s *SealedStore) CountPrefix(ctx context.Context, kind, prefix string) (int, error) {
	return s.inner.CountPrefix(ctx, kind, prefix)
}

// Sweep delegates to the inner store.
func (s *SealedStore) Sweep(ctx context.Context) (int, error) {
	return s.inner.Sweep(ctx)
}

// Ping delegates to the inner store.
func (s *SealedStore) Ping(ctx context.Context) error {
	return s.inner.Ping(ctx)
}

// Close closes the inner store.
func (s *SealedStore) Close() {
	s.inner.Close()
}
