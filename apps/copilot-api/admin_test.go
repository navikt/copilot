package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAdmin(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	cases := []struct {
		name string
		env  string
		user *User
		want int
	}{
		{"admin", group, &User{Groups: []string{"other", group}}, http.StatusOK},
		{"non-admin", group, &User{Groups: []string{"other"}}, http.StatusNotFound},
		{"no groups", group, &User{}, http.StatusNotFound},
		{"no user", group, nil, http.StatusNotFound},
		{"group unset", "", &User{Groups: []string{""}}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(adminGroupEnv, tc.env)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/x", nil)
			if tc.user != nil {
				req = req.WithContext(context.WithValue(req.Context(), userContextKey, tc.user))
			}
			rec := httptest.NewRecorder()
			requireAdmin(ok).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
