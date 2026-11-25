package jobs

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"
)

// Shared test helpers used across *_test.go files.

var errTest = errors.New("test error")

func fixedDay() time.Time {
	return time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)
}

func singleRecord() []json.RawMessage {
	return []json.RawMessage{json.RawMessage(`{"test":"data"}`)}
}

// mockTransport serves HTTP requests in-process without TCP connections.
// This avoids macOS sandbox restrictions that block localhost TCP.
type mockTransport struct {
	handler http.Handler
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	t.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

// mockClient creates an *http.Client that serves requests via handler in-process.
func mockClient(handler http.Handler) *http.Client {
	return &http.Client{Transport: &mockTransport{handler: handler}}
}
