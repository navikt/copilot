package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestNewGitHubClient_AcceptsLiteralNewlineEscapesInPrivateKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	privateKeyEscaped := strings.ReplaceAll(string(privateKeyPEM), "\n", `\n`)

	config := &Config{
		GitHubOrg:            "navikt",
		GitHubAppID:          "12345",
		GitHubAppPrivateKey:  privateKeyEscaped,
		GitHubInstallationID: "67890",
	}

	client, err := newGitHubClient(config)
	if err != nil {
		t.Fatalf("newGitHubClient() error = %v", err)
	}
	if client == nil || client.privateKey == nil {
		t.Fatal("expected initialized github client with parsed private key")
	}
}
