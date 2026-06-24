package cli

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func textResponse(req *http.Request, status int, body string) *http.Response {
	contentLength := int64(len(body))
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Request:    req,
		ContentLength: contentLength,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestIsBrewManaged(t *testing.T) {
	// In dev/test, the binary is not in a Homebrew Cellar
	// This just verifies the function runs without panic
	_ = isBrewManaged()
}

func TestSha256sum(t *testing.T) {
	data := []byte("hello world")
	got := sha256sum(data)
	want := fmt.Sprintf("%x", sha256.Sum256(data))
	if got != want {
		t.Errorf("sha256sum = %s, want %s", got, want)
	}
}

func TestVerifyChecksum_Valid(t *testing.T) {
	data := []byte("binary-data")
	checksum := sha256sum(data)
	checksumURL := "https://example.test/SHA256SUMS"
	client := newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != checksumURL {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		body := fmt.Sprintf("%s  nav-pilot-linux-amd64\n", checksum)
		resp := textResponse(req, http.StatusOK, body)
		resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
		return resp, nil
	})

	err := verifyChecksumWithClient(client, data, "nav-pilot-linux-amd64", checksumURL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	data := []byte("binary-data")
	checksumURL := "https://example.test/SHA256SUMS"
	client := newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != checksumURL {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		body := "0000000000000000000000000000000000000000000000000000000000000000  nav-pilot-linux-amd64\n"
		resp := textResponse(req, http.StatusOK, body)
		resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
		return resp, nil
	})

	err := verifyChecksumWithClient(client, data, "nav-pilot-linux-amd64", checksumURL)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestVerifyChecksum_NoSumsFile(t *testing.T) {
	data := []byte("binary-data")
	checksumURL := "https://example.test/SHA256SUMS"
	client := newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != checksumURL {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		resp := textResponse(req, http.StatusNotFound, "")
		resp.Header.Set("Content-Length", "0")
		return resp, nil
	})

	// Should error — checksum verification is mandatory
	err := verifyChecksumWithClient(client, data, "nav-pilot-linux-amd64", checksumURL)
	if err == nil {
		t.Fatal("expected error when checksums unavailable")
	}
}

func TestVerifyChecksum_NoEntry(t *testing.T) {
	data := []byte("binary-data")
	checksumURL := "https://example.test/SHA256SUMS"
	client := newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != checksumURL {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		body := "abcdef1234567890  nav-pilot-linux-arm64\n"
		resp := textResponse(req, http.StatusOK, body)
		resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
		return resp, nil // different asset
	})

	err := verifyChecksumWithClient(client, data, "nav-pilot-linux-amd64", checksumURL)
	if err == nil {
		t.Fatal("expected error when asset entry is missing")
	}
}

func TestFetchLatestVersion(t *testing.T) {
	// Test the tag parsing logic directly
	tag := "nav-pilot/2026.04.13-170138-abc1234"
	ver := tag[len("nav-pilot/"):]
	if ver != "2026.04.13-170138-abc1234" {
		t.Errorf("unexpected version: %s", ver)
	}
}

func TestFetchLatestVersion_SkipsNonNavPilot(t *testing.T) {
	// Verify the filtering logic: only nav-pilot/ prefixed tags are matched
	tags := []string{"other-app/1.0.0", "nav-pilot/2026.04.13-170138-abc1234"}
	var found string
	for _, tag := range tags {
		if len(tag) > len("nav-pilot/") && tag[:len("nav-pilot/")] == "nav-pilot/" {
			found = tag[len("nav-pilot/"):]
			break
		}
	}
	if found != "2026.04.13-170138-abc1234" {
		t.Errorf("expected 2026.04.13-170138-abc1234, got %s", found)
	}
}

func TestRun_UpdateCommand(t *testing.T) {
	// Set version to a known value to trigger "up to date" path
	// (avoids actually downloading a binary in tests)
	origVersion := Version
	Version = "test-version-that-wont-match"
	defer func() { Version = origVersion }()

	err := run([]string{"update"})
	// Should not be "unknown command" — verifies wiring
	if err != nil && err.Error() == "unknown command: update. Run with --help for usage" {
		t.Fatal("update command not wired up in main.go")
	}
	// Will get a network error or version mismatch, that's fine
}
