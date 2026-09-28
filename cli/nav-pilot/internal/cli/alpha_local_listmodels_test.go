package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Ollama with nothing pulled answers "data": null: a server with no models.
// A list without data, or with data that is not an array, is not a server.
func TestListModelsData(t *testing.T) {
	for body, ok := range map[string]bool{
		`{"object":"list","data":null}`:         true,
		`{"object":"list","data":[{"id":"a"}]}`: true,
		`{"object":"list"}`:                     false,
		`{"object":"list","data":{"id":"a"}}`:   false,
		`{"data":[{"id":"a"}]}`:                 false,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		_, err := listModels(context.Background(), srv.URL, 2*time.Second)
		srv.Close()
		if (err == nil) != ok {
			t.Errorf("%s: err = %v, want ok = %v", body, err, ok)
		}
	}
}
