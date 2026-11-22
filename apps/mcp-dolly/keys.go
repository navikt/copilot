package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	minKeyBytes      = 32
	generatedKeysEnv = "MCP_DOLLY_GENERATED_KEYS"
)

var kidPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// signingKey is one HMAC key for confirmation tokens; kid names it in the token.
type signingKey struct {
	kid string
	key []byte
}

// serverKeys come from the Nais secret mcp-dolly-keys so every pod signs and
// verifies with the same material and restarts keep tokens and quota hashes valid.
type serverKeys struct {
	confirm   []signingKey // newest first: sign with [0], verify with any
	userHash  []byte
	generated bool
}

// loadServerKeys reads CONFIRM_KEYS ("kid:base64,kid:base64", newest first) and
// USER_HASH_KEY (base64). Errors never include key material.
func loadServerKeys() (serverKeys, error) {
	rawConfirm := strings.TrimSpace(os.Getenv("CONFIRM_KEYS"))
	rawHash := strings.TrimSpace(os.Getenv("USER_HASH_KEY"))
	generate, err := getEnvBool(generatedKeysEnv)
	if err != nil {
		return serverKeys{}, err
	}
	if generate {
		if rawConfirm != "" || rawHash != "" {
			return serverKeys{}, fmt.Errorf("%s is set together with CONFIRM_KEYS or USER_HASH_KEY; use one", generatedKeysEnv)
		}
		if strings.HasPrefix(os.Getenv("NAIS_CLUSTER_NAME"), "prod") {
			return serverKeys{}, fmt.Errorf("%s is not allowed in prod", generatedKeysEnv)
		}
		k := serverKeys{confirm: []signingKey{{kid: "gen", key: make([]byte, minKeyBytes)}}, userHash: make([]byte, minKeyBytes), generated: true}
		_, _ = rand.Read(k.confirm[0].key)
		_, _ = rand.Read(k.userHash)
		return k, nil
	}
	if rawConfirm == "" || rawHash == "" {
		return serverKeys{}, errors.New("CONFIRM_KEYS and USER_HASH_KEY are required (Nais secret mcp-dolly-keys)")
	}
	var k serverKeys
	seen := map[string]bool{}
	for i, entry := range strings.Split(rawConfirm, ",") {
		kid, b64, ok := strings.Cut(strings.TrimSpace(entry), ":")
		if !ok || !kidPattern.MatchString(kid) || seen[kid] {
			return serverKeys{}, fmt.Errorf("CONFIRM_KEYS entry %d: want unique kid:base64 with kid [A-Za-z0-9_-]{1,32}", i+1)
		}
		key, err := decodeKey(b64)
		if err != nil {
			return serverKeys{}, fmt.Errorf("CONFIRM_KEYS entry %d (kid %s): %w", i+1, kid, err)
		}
		seen[kid] = true
		k.confirm = append(k.confirm, signingKey{kid, key})
	}
	if k.userHash, err = decodeKey(rawHash); err != nil {
		return serverKeys{}, fmt.Errorf("USER_HASH_KEY: %w", err)
	}
	return k, nil
}

func decodeKey(b64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, errors.New("not valid base64")
	}
	if len(key) < minKeyBytes {
		return nil, fmt.Errorf("decodes to %d bytes, need at least %d", len(key), minKeyBytes)
	}
	return key, nil
}
