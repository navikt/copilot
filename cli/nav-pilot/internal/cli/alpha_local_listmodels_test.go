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
		_, _, err := listModels(context.Background(), srv.URL, 2*time.Second)
		srv.Close()
		if (err == nil) != ok {
			t.Errorf("%s: err = %v, want ok = %v", body, err, ok)
		}
	}
}

// mlx_lm.server is built on Python's http.server, and its Server header says
// so. Seen as llama-server before (#1102).
func TestListModelsTellsMLXLM(t *testing.T) {
	for server, want := range map[string]bool{"BaseHTTP/0.6 Python/3.12.13": true, "llama.cpp": false, "": false} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if server != "" {
				w.Header().Set("Server", server)
			}
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"a"}]}`))
		}))
		_, mlxLM, err := listModels(context.Background(), srv.URL, 2*time.Second)
		srv.Close()
		if err != nil || mlxLM != want {
			t.Errorf("Server %q: mlxLM = %v, %v; want %v", server, mlxLM, err, want)
		}
	}
}
