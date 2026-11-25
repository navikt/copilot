package entra

import (
	"strings"
	"testing"
	"time"
)

const testIssuer = "https://login.test/tenant/v2.0"

func TestValidateIDTokenClaims(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	n := float64(now.Unix())
	valid := func() map[string]any {
		return map[string]any{"iss": testIssuer, "aud": "azure-client-id", "iat": n, "exp": n + 3600}
	}
	cases := []struct {
		name    string
		mutate  func(map[string]any)
		issuer  string
		wantErr string
	}{
		{name: "valid", mutate: func(map[string]any) {}},
		{name: "valid multi-aud with azp", mutate: func(c map[string]any) { c["aud"] = []any{"other", "azure-client-id"}; c["azp"] = "azure-client-id" }},
		{name: "valid within skew", mutate: func(c map[string]any) { c["exp"] = n - 60; c["iat"] = n + 60; c["nbf"] = n + 60 }},
		{name: "issuer not configured", mutate: func(map[string]any) {}, issuer: "-", wantErr: "not configured"},
		{name: "iss missing", mutate: func(c map[string]any) { delete(c, "iss") }, wantErr: "iss"},
		{name: "iss other tenant", mutate: func(c map[string]any) { c["iss"] = "https://login.test/evil/v2.0" }, wantErr: "iss"},
		{name: "aud missing", mutate: func(c map[string]any) { delete(c, "aud") }, wantErr: "aud mismatch"},
		{name: "aud other client", mutate: func(c map[string]any) { c["aud"] = "other" }, wantErr: "aud mismatch"},
		{name: "aud malformed", mutate: func(c map[string]any) { c["aud"] = []any{"azure-client-id", 1.0} }, wantErr: "aud malformed"},
		{name: "multi-aud without azp", mutate: func(c map[string]any) { c["aud"] = []any{"other", "azure-client-id"} }, wantErr: "azp"},
		{name: "azp other client", mutate: func(c map[string]any) { c["azp"] = "other" }, wantErr: "azp"},
		{name: "exp missing", mutate: func(c map[string]any) { delete(c, "exp") }, wantErr: "exp"},
		{name: "exp past skew", mutate: func(c map[string]any) { c["exp"] = n - 300 }, wantErr: "exp"},
		{name: "iat missing", mutate: func(c map[string]any) { delete(c, "iat") }, wantErr: "iat"},
		{name: "iat future", mutate: func(c map[string]any) { c["iat"] = n + 300 }, wantErr: "iat"},
		{name: "nbf future", mutate: func(c map[string]any) { c["nbf"] = n + 300 }, wantErr: "not yet valid"},
		{name: "iat overflow", mutate: func(c map[string]any) { c["iat"] = 9223372000000000000.0 }, wantErr: "iat"},
		{name: "nbf overflow", mutate: func(c map[string]any) { c["nbf"] = 9223372000000000000.0 }, wantErr: "not yet valid"},
		{name: "exp overflow", mutate: func(c map[string]any) { c["exp"] = 9223372000000000000.0 }, wantErr: "exp"},
		{name: "nbf malformed", mutate: func(c map[string]any) { c["nbf"] = "soon" }, wantErr: "not yet valid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mutate(c)
			issuer := testIssuer
			if tc.issuer == "-" {
				issuer = ""
			}
			err := ValidateIDTokenClaims(c, issuer, "azure-client-id", now)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want ok, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
