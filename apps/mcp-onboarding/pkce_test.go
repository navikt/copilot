package main

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestVerifyPKCE_ValidChallenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	if !VerifyPKCE(verifier, challenge) {
		t.Error("expected PKCE verification to pass for valid verifier/challenge pair")
	}
}

func TestVerifyPKCE_InvalidChallenge(t *testing.T) {
	verifier := "correct-verifier"
	challenge := "wrong-challenge"

	if VerifyPKCE(verifier, challenge) {
		t.Error("expected PKCE verification to fail for mismatched challenge")
	}
}

func TestVerifyPKCE_BothEmpty(t *testing.T) {
	if VerifyPKCE("", "") {
		t.Error("expected PKCE to fail when both verifier and challenge are empty")
	}
}

func TestVerifyPKCE_EmptyVerifierNonEmptyChallenge(t *testing.T) {
	if VerifyPKCE("", "some-challenge") {
		t.Error("expected PKCE to fail when verifier is empty but challenge is not")
	}
}

func TestVerifyPKCE_NonEmptyVerifierEmptyChallenge(t *testing.T) {
	if VerifyPKCE("some-verifier", "") {
		t.Error("expected PKCE to fail when challenge is empty but verifier is not")
	}
}
