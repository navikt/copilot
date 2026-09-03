package main

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"
)

// The client_id is the registration.
//
// Client registrations used to live in a map in this process, and this app runs
// a single replica, so every deploy forgot every client. Accepting an unknown
// client_id to paper over that is what became GHSA-7hwf-488h-59x8; rejecting it
// (the fix) strands every client until it re-registers. Neither is acceptable,
// and persisting the registrations to disk is worse still: the same store holds
// live employee GitHub tokens.
//
// So nothing is stored. /register signs the parts of the registration that
// /oauth/authorize has to trust — the redirect_uris — and hands them back as
// the client_id. Verifying the tag proves the server issued it, which is
// exactly what the map lookup used to prove, and it keeps proving it after a
// restart.
type clientIDInfo struct {
	RedirectURIs []string `json:"u"`
	// IssuedAt is what bounds the lifetime of a registration. Nothing is
	// stored, so there is no cleanup loop to expire one; verifyClientID
	// enforces the same 30-day window the deleted loop did.
	IssuedAt int64 `json:"t"`
	// Nonce keeps two registrations of the same redirect_uris distinct. Without
	// it the client_id would be a pure function of the registration, and two
	// editors on the same loopback port would share one client identity — which
	// is what binds an authorization code to the client that asked for it.
	Nonce string `json:"n"`
}

var errInvalidClientID = errors.New("invalid client_id")

// maxClientIDLen bounds what is worth base64-decoding. A real client_id with
// one loopback redirect_uri is around 110 characters. The registration handler
// checks the minted client_id against this same limit, so /register never hands
// out an id that verifyClientID would refuse to look at.
const maxClientIDLen = 4096

// clientIDTTL is how long a registration stays valid. /register is
// unauthenticated and accepts any https redirect_uri, so without this an
// attacker-minted client_id would be valid forever, revocable only by rotating
// GITHUB_CLIENT_SECRET — which strands every legitimate client too.
const clientIDTTL = 30 * 24 * time.Hour

// deriveClientIDKey derives the signing key from the GitHub OAuth client
// secret, which the app already receives through mcp-onboarding-secrets. That
// avoids a second secret to provision and rotate, and ties the two together:
// rotating the GitHub secret invalidates every outstanding client_id, and the
// clients then get the recoverable invalid_client answer and re-register.
//
// Without a secret (local development) the key is random per process, so
// client_ids die with the process — the same behaviour the in-memory map had.
func deriveClientIDKey(githubClientSecret string) []byte {
	if githubClientSecret == "" {
		key := make([]byte, 32)
		_, _ = rand.Read(key)
		slog.Warn("no GitHub client secret set: client_id signing key is random per process, clients must re-register after every restart")
		return key
	}
	key, err := hkdf.Key(sha256.New, []byte(githubClientSecret), nil, "mcp-onboarding client_id v1", 32)
	if err != nil {
		// hkdf.Key only fails on an absurd key length, which is a constant here.
		panic("deriving client_id signing key: " + err.Error())
	}
	return key
}

// mintClientID encodes the registration and appends its HMAC tag.
func mintClientID(key []byte, info clientIDInfo) string {
	payload, err := json.Marshal(info)
	if err != nil {
		panic("encoding client_id payload: " + err.Error()) // []string and int64 only
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(tagClientID(key, encoded))
}

func tagClientID(key []byte, encodedPayload string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(encodedPayload))
	return mac.Sum(nil)
}

// verifyClientID returns the registration carried by a client_id, or
// errInvalidClientID. The tag is compared in constant time and before anything
// in the payload is decoded, so an attacker never reaches the JSON parser;
// TestVerifyClientID_ChecksTagFirst pins both, since neither is observable from
// the outside — every path fails closed.
//
// The comparison is against the re-encoded tag rather than the decoded bytes:
// a 32-byte tag is 43 base64url characters with two slack bits, and Go's
// decoder accepts non-canonical trailing bits, so decoding first would make
// four spellings of one client_id verify. Comparing the encoding makes the
// client_id a canonical identifier.
func verifyClientID(key []byte, clientID string) (*clientIDInfo, error) {
	if clientID == "" || len(clientID) > maxClientIDLen {
		return nil, errInvalidClientID
	}
	encodedPayload, encodedTag, ok := strings.Cut(clientID, ".")
	if !ok {
		return nil, errInvalidClientID
	}
	want := base64.RawURLEncoding.EncodeToString(tagClientID(key, encodedPayload))
	if !hmac.Equal([]byte(encodedTag), []byte(want)) {
		return nil, errInvalidClientID
	}

	// Past this point the payload is one this server signed.
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, errInvalidClientID
	}
	var info clientIDInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		return nil, errInvalidClientID
	}
	if len(info.RedirectURIs) == 0 {
		return nil, errInvalidClientID
	}
	// The nonce is what keeps two registrations of the same redirect_uris
	// distinct, so an id without one has an identity that is not its own.
	// Enforced here rather than trusted from the mint side, because this is the
	// boundary that decides whether a client_id is usable.
	if info.Nonce == "" {
		return nil, errInvalidClientID
	}
	// An absent, zero or future IssuedAt is as unusable as an expired one: it
	// would mean a registration with no bounded lifetime.
	if age := time.Since(time.Unix(info.IssuedAt, 0)); info.IssuedAt <= 0 || age < 0 || age > clientIDTTL {
		return nil, errInvalidClientID
	}
	return &info, nil
}
