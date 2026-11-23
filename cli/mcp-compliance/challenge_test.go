package main

import "testing"

func TestBearerChallenges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		in     []string
		ok     bool
		errVal string
	}{
		{"bare", []string{"Bearer"}, true, ""},
		{"lowercase scheme", []string{`bearer realm="mcp"`}, true, ""},
		{"realm containing error=", []string{`Bearer realm="error=monitor"`}, true, ""},
		{"other scheme with prefix", []string{`BearerOther realm="x"`}, false, ""},
		{"unquoted error", []string{`Bearer error=invalid_token`}, true, "invalid_token"},
		{"quoted error", []string{`Bearer error="invalid_token"`}, true, "invalid_token"},
		{"quoted comma", []string{`Bearer realm="a, error=\"x\"", error="invalid_token"`}, true, "invalid_token"},
		{"escape", []string{`Bearer realm="a\"b", resource_metadata="u"`}, true, ""},
		{"second challenge", []string{`Basic realm="x", Bearer error="invalid_token"`}, true, "invalid_token"},
		{"second header value", []string{`Basic realm="x"`, `Bearer realm="mcp"`}, true, ""},
		{"error on other scheme only", []string{`Basic error="x", Bearer realm="m"`}, true, ""},
		{"spaces around =", []string{`Bearer error = "invalid_token"`}, true, "invalid_token"},
		{"spaces around = unquoted", []string{`Bearer realm="m", error = invalid_token`}, true, "invalid_token"},
		{"token68 then bearer", []string{`Negotiate YWJjZA==, Bearer error="invalid_token"`}, true, "invalid_token"},
		{"token68 no padding", []string{`Negotiate YWJj, Bearer realm="m"`}, true, ""},
		{"token68 slash", []string{`Negotiate YW/Jj+Q==, Bearer error="invalid_token"`}, true, "invalid_token"},
		{"token68 only", []string{`Negotiate YWJjZA==`}, false, ""},
		{"empty", nil, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c challenge
			cs := bearerChallenges(tc.in)
			if len(cs) > 0 {
				c = cs[0]
			}
			ok := len(cs) > 0
			if ok != tc.ok || c.params["error"] != tc.errVal {
				t.Fatalf("ok=%v error=%q, want ok=%v error=%q", ok, c.params["error"], tc.ok, tc.errVal)
			}
		})
	}
}

func TestBearerChallengesAll(t *testing.T) {
	cs := bearerChallenges([]string{`Bearer realm="a", Bearer realm="b", error="invalid_token"`, `Bearer error="x"`})
	if len(cs) != 3 || cs[1].params["error"] != "invalid_token" || cs[2].params["error"] != "x" {
		t.Fatalf("got %+v", cs)
	}
}
