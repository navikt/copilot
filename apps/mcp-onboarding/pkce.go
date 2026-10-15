package main

import (
	"crypto/sha256"
	"encoding/base64"
)

func VerifyPKCE(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}

	h := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])

	return computed == challenge
}
