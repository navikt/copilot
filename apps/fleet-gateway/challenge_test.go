package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// 401 challenge per RFC 6750 s3.1 and RFC 9728: error only when credentials were presented.
func TestMCP401Challenge(t *testing.T) {
	te := newTestEnv(t)
	path := "/mcp/team-etterlevelse/etterlevelse"
	for _, c := range []struct{ name, auth, wantErr string }{
		{"no credentials", "", ""},
		{"unknown token", "Bearer nope", `error="invalid_token", `},
		{"other scheme", "Basic abc", ""},
		{"lowercase scheme", "bearer nope", `error="invalid_token", `},
		{"bearer without token", "Bearer", `error="invalid_token", `},
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		if c.auth != "" {
			req.Header.Set("Authorization", c.auth)
		}
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: got %d", c.name, rec.Code)
		}
		got := rec.Header().Get("WWW-Authenticate")
		prm := te.gw.cfg.BaseURL + "/.well-known/oauth-protected-resource" + path
		if want := "Bearer " + c.wantErr + `resource_metadata="` + prm + `"`; got != want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, want)
		}
	}
}

func TestAuditRefreshLogsBoundedClientID(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	cid := "https://c.example/\nx" + strings.Repeat("a", 400)
	te.postToken(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"x"}, "client_id": {cid}})
	out := buf.String()
	want := `"client_id":"https://c.example/x` + strings.Repeat("a", 256-len("https://c.example/x")) + `"`
	if !strings.Contains(out, `"msg":"audit refresh"`) || !strings.Contains(out, want) {
		t.Fatalf("audit refresh line lacks bounded sanitised client_id: %s", out)
	}
}
