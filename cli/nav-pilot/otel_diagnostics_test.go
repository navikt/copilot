package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
)

// captureStderr redirects os.Stderr to a pipe for the duration of fn, then
// returns everything written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}

// TestConfigureOTelDiagnostics_HandlerNonNil checks that configureOTelDiagnostics
// installs a non-nil global OTel error handler and that invoking it does not panic.
func TestConfigureOTelDiagnostics_HandlerNonNil(t *testing.T) {
	configureOTelDiagnostics()

	h := otel.GetErrorHandler()
	if h == nil {
		t.Fatal("expected non-nil OTel error handler after configureOTelDiagnostics()")
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("OTel error handler panicked: %v", r)
		}
	}()
	h.Handle(errors.New("no-panic-check"))
}

// TestConfigureOTelDiagnostics_SilentByDefault asserts that when DEBUG is unset,
// calling otel.Handle produces no output on stderr.
func TestConfigureOTelDiagnostics_SilentByDefault(t *testing.T) {
	configureOTelDiagnostics()
	t.Setenv("DEBUG", "")

	out := captureStderr(t, func() {
		otel.Handle(errors.New("silent-boom"))
	})

	if out != "" {
		t.Fatalf("expected no stderr output with DEBUG unset, got: %q", out)
	}
}

// TestConfigureOTelDiagnostics_VerboseUnderDebug asserts that when DEBUG is set,
// calling otel.Handle routes the error message through debugLog (stderr) and
// contains the error text.
func TestConfigureOTelDiagnostics_VerboseUnderDebug(t *testing.T) {
	configureOTelDiagnostics()
	t.Setenv("DEBUG", "1")

	out := captureStderr(t, func() {
		otel.Handle(errors.New("verbose-boom"))
	})

	if !strings.Contains(out, "verbose-boom") {
		t.Fatalf("expected stderr to contain 'verbose-boom' with DEBUG=1, got: %q", out)
	}
}
