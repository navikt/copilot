package fetchers

import (
	"net/http"
	"net/http/httptest"
)

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
