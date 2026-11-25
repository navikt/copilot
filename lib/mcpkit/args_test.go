package mcpkit

import (
	"encoding/json"
	"testing"
)

func TestDecodeArgMap(t *testing.T) {
	allowed := map[string]bool{"a": true, "b": true}
	for _, tc := range []struct {
		in      string
		wantErr string
	}{
		{"", ""},
		{"null", ""},
		{`{"a":1,"b":"x"}`, ""},
		{`[1]`, "arguments must be a JSON object"},
		{`{"c":1}`, `unknown argument "c"`},
		{`{"a":1,"a":2}`, `duplicate argument "a"`},
		{`{"a":1}{}`, "trailing data in arguments"},
		{`{"a":}`, `invalid value for "a"`},
	} {
		_, err := DecodeArgMap(json.RawMessage(tc.in), allowed)
		got := ""
		if err != nil {
			got = err.Message
			if err.Code != CodeInvalidParams {
				t.Errorf("%q: code %d", tc.in, err.Code)
			}
		}
		if got != tc.wantErr {
			t.Errorf("%q: err %q, want %q", tc.in, got, tc.wantErr)
		}
	}
}

func TestParseStrict(t *testing.T) {
	if s, err := ParseStrictString(json.RawMessage(`"hi"`), "f"); err != nil || s != "hi" {
		t.Fatalf("string: %q %v", s, err)
	}
	if _, err := ParseStrictString(json.RawMessage(`1`), "f"); err == nil || err.Message != "f must be a string" {
		t.Fatalf("string reject: %v", err)
	}
	if v, err := ParseStrictInt(json.RawMessage(`-3`), "n"); err != nil || v != -3 {
		t.Fatalf("int: %d %v", v, err)
	}
	for _, in := range []string{`"3"`, `3.0`, `1e2`, ``, `true`} {
		if _, err := ParseStrictInt(json.RawMessage(in), "n"); err == nil || err.Message != "n must be an integer" {
			t.Errorf("int %q: %v", in, err)
		}
	}
	if r := ToolErrorResult("boom"); r["isError"] != true {
		t.Fatalf("ToolErrorResult: %v", r)
	}
}
