package main

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

const (
	sealMaxKIDLen = 32
	// sealMaxPlaintext bounds what Seal accepts (sessions and RTs are a few KiB).
	sealMaxPlaintext = 1 << 20
	sealKeyBytes     = 32
	sealNonceSize    = 12
)

var (
	errSealUnknownKID = errors.New("sealed store: unknown key id")
	errSealDecrypt    = errors.New("sealed store: decryption failed")
	errSealMalformed  = errors.New("sealed store: malformed ciphertext")
)

type keyEntry struct {
	kid string
	key [sealKeyBytes]byte
}

type keyRing struct {
	entries []keyEntry
}

func parseKeyRing(raw string) (*keyRing, error) {
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

	return &keyRing{entries: entries}, nil
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
	if len(decoded) != sealKeyBytes {
		return nil, fmt.Errorf("key must decode to %d bytes, got %d", sealKeyBytes, len(decoded))
	}
	return decoded, nil
}

func (kr *keyRing) Seal(aad, plaintext []byte) ([]byte, error) {
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
	if len(plaintext) > sealMaxPlaintext {
		return nil, errors.New("sealed store: plaintext too large")
	}
	out := append([]byte{byte(len(kidBytes))}, kidBytes...) //nolint:gosec // bounded by sealMaxKIDLen (32)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plaintext, aad)
	return out, nil
}

func (kr *keyRing) Open(aad, blob []byte) ([]byte, error) {
	if kr == nil || len(kr.entries) == 0 {
		return nil, errSealDecrypt
	}
	if len(blob) < 1 {
		return nil, errSealMalformed
	}
	kidLen := int(blob[0])
	if kidLen == 0 || kidLen > sealMaxKIDLen || len(blob) < 1+kidLen+sealNonceSize {
		return nil, errSealMalformed
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
		return nil, errSealUnknownKID
	}

	block, err := aes.NewCipher(matched.key[:])
	if err != nil {
		return nil, errSealDecrypt
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errSealDecrypt
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errSealDecrypt
	}
	return plaintext, nil
}

type sealedStore struct {
	inner SessionStore
	keys  *keyRing
}

func newSealedStore(inner SessionStore, keys *keyRing) *sealedStore {
	return &sealedStore{
		inner: inner,
		keys:  keys,
	}
}

// Lockout records, the marker and the audit outbox hold only HMAC keys, timestamps and operator idents, and must stay readable after RT_KEYS is replaced (global logout must not lift lockouts or lose audit lines).
func unsealedKind(kind string) bool {
	return kind == kindUserRevoked || kind == kindUserRevokedMarker || kind == kindAuditOutbox
}

func sealAAD(kind, key string) []byte {
	return []byte(kind + "\x00" + key)
}

func (s *sealedStore) Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error {
	if unsealedKind(kind) {
		return s.inner.Put(ctx, kind, key, val, ttl)
	}
	blob, err := s.keys.Seal(sealAAD(kind, key), val)
	if err != nil {
		return err
	}
	return s.inner.Put(ctx, kind, key, blob, ttl)
}

func (s *sealedStore) PutEvict(ctx context.Context, kind, key string, val []byte, ttl time.Duration) (bool, error) {
	if unsealedKind(kind) {
		return s.inner.PutEvict(ctx, kind, key, val, ttl)
	}
	blob, err := s.keys.Seal(sealAAD(kind, key), val)
	if err != nil {
		return false, err
	}
	return s.inner.PutEvict(ctx, kind, key, blob, ttl)
}

// Legacy audit outbox rows written before kindAuditOutbox became unsealed start with the key-id header rather than '['.
func (s *sealedStore) decodeUnsealed(kind, key string, raw []byte) ([]byte, error) {
	if kind != kindAuditOutbox || (len(raw) > 0 && raw[0] == '[') {
		return raw, nil
	}
	return s.keys.Open(sealAAD(kind, key), raw)
}

func (s *sealedStore) Get(ctx context.Context, kind, key string) ([]byte, bool, error) {
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
	plaintext, err := s.keys.Open(sealAAD(kind, key), blob)
	if err != nil {
		return nil, false, err
	}
	return plaintext, true, nil
}

func (s *sealedStore) Pop(ctx context.Context, kind, key string) ([]byte, bool, error) {
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
	plaintext, err := s.keys.Open(sealAAD(kind, key), blob)
	if err != nil {
		return nil, false, err
	}
	return plaintext, true, nil
}

func (s *sealedStore) Delete(ctx context.Context, kind, key string) error {
	return s.inner.Delete(ctx, kind, key)
}

func (s *sealedStore) CompareAndSwap(ctx context.Context, kind, key string, oldVal, newVal []byte, ttl time.Duration) (bool, error) {
	if !unsealedKind(kind) {
		return false, fmt.Errorf("sealed store: compare-and-swap unsupported for sealed kind %q", kind)
	}
	if kind != kindAuditOutbox || oldVal == nil {
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
	plaintext, err := s.keys.Open(sealAAD(kind, key), raw)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(plaintext, oldVal) {
		return false, nil
	}
	return s.inner.CompareAndSwap(ctx, kind, key, raw, newVal, ttl)
}

func (s *sealedStore) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	return s.inner.Revoke(ctx, jti, expiresAt)
}

func (s *sealedStore) RevokeOnce(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
	return s.inner.RevokeOnce(ctx, jti, expiresAt)
}

func (s *sealedStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return s.inner.IsRevoked(ctx, jti)
}

func (s *sealedStore) Incr(ctx context.Context, key string, window time.Duration) (int, error) {
	return s.inner.Incr(ctx, key, window)
}

// Keys are never sealed, so a prefix count runs on the inner store.
func (s *sealedStore) CountPrefix(ctx context.Context, kind, prefix string) (int, error) {
	return s.inner.CountPrefix(ctx, kind, prefix)
}

func (s *sealedStore) Sweep(ctx context.Context) (int, error) {
	return s.inner.Sweep(ctx)
}

func (s *sealedStore) Ping(ctx context.Context) error {
	return s.inner.Ping(ctx)
}

func (s *sealedStore) Close() {
	s.inner.Close()
}
