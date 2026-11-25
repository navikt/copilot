package github

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockTransport serves HTTP requests in-process without TCP connections.
// This avoids macOS sandbox restrictions that block outbound TCP.
type mockTransport struct {
	handler http.Handler
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	t.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

// mockServer wraps a handler as a mock that replaces httptest.Server.
// It satisfies the same usage pattern (Close is a no-op).
type mockServer struct {
	handler http.Handler
}

func (m *mockServer) Close() {} // no-op, no TCP listener to close

func newGitHubMock(_ *testing.T, handlers map[string]http.HandlerFunc) *mockServer {
	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}
	return &mockServer{handler: mux}
}

func newTestGitHubClient(mock *mockServer) *Client {
	client := &Client{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		APIBaseURL:   "http://test",
	}
	if mock != nil {
		client.HTTPClient = &http.Client{Transport: &mockTransport{handler: mock.handler}}
	} else {
		client.HTTPClient = http.DefaultClient
	}
	return client
}
