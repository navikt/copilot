package main

import (
	"net/http"
	"strings"
	"testing"
)

var policyTools = map[string]bool{"whoami": true}

func TestToolPolicy_Proxy(t *testing.T) {
	call := func(name string) string {
		return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `"}}`
	}
	cases := []struct {
		name, body, reason string
		code               int
	}{
		{"allowed", call("whoami"), reasonListed, http.StatusOK},
		{"non-call", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, reasonNotCall, http.StatusOK},
		{"denied", call("delete_all"), reasonNotListed, http.StatusForbidden},
		{"name case", call("WhoAmI"), reasonNotListed, http.StatusForbidden},
		{"case-variant keys", `{"jsonrpc":"2.0","id":1,"Method":"tools/call","Params":{"Name":"delete_all"}}`, reasonAmbiguous, http.StatusBadRequest},
		{"case-variant name", `{"method":"tools/call","params":{"name":"whoami","NAME":"delete_all"}}`, reasonAmbiguous, http.StatusBadRequest},
		{"fold variant params", `{"method":"tools/call","paramſ":{"name":"delete_all"},"params":{"name":"whoami"}}`, reasonAmbiguous, http.StatusBadRequest},
		{"method case", `{"method":"Tools/Call","params":{"name":"delete_all"}}`, reasonAmbiguous, http.StatusBadRequest},
		{"duplicate method", `{"method":"tools/list","method":"tools/call","params":{"name":"delete_all"}}`, reasonAmbiguous, http.StatusBadRequest},
		{"duplicate name", `{"method":"tools/call","params":{"name":"whoami","name":"delete_all"}}`, reasonAmbiguous, http.StatusBadRequest},
		{"trailing data", call("whoami") + "\n" + call("delete_all"), reasonAmbiguous, http.StatusBadRequest},
		{"unparseable", "\xef\xbb\xbf" + call("whoami"), reasonAmbiguous, http.StatusBadRequest},
		{"truncated", call("whoami")[:30], reasonAmbiguous, http.StatusBadRequest},
		{"batch allowed", "[" + call("whoami") + `,{"method":"notifications/initialized"}]`, reasonListed, http.StatusOK},
		{"batch mixed", "[" + call("whoami") + "," + call("delete_all") + "]", reasonNotListed, http.StatusForbidden},
		{"batch ambiguous", "[" + call("whoami") + `,{"METHOD":"tools/call","params":{"name":"delete_all"}}]`, reasonAmbiguous, http.StatusBadRequest},
		{"batch unclosed", "[" + call("whoami"), reasonAmbiguous, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			audit, got, code := auditPostTools(t, policyTools, tc.body)
			if code != tc.code || audit["decision_reason"] != tc.reason {
				t.Fatalf("code=%d reason=%v, want %d %s", code, audit["decision_reason"], tc.code, tc.reason)
			}
			wantDecision := "allow"
			if tc.code != http.StatusOK {
				wantDecision = "deny"
				if got != nil {
					t.Fatalf("denied request was forwarded: %q", got)
				}
			} else if string(got) != tc.body {
				t.Fatalf("allowed body changed: %q", got)
			}
			if audit["decision"] != wantDecision {
				t.Fatalf("decision=%v, want %s", audit["decision"], wantDecision)
			}
		})
	}
}

func TestToolPolicy_PassAllRouteForwards(t *testing.T) {
	audit, got, code := auditPostTools(t, nil, `{"method":"tools/call","params":{"name":"delete_all"}}`)
	if code != http.StatusOK || got == nil || audit["decision"] != "allow" || audit["decision_reason"] != reasonPassAll {
		t.Fatalf("pass-all route: code=%d forwarded=%v audit=%v/%v", code, got != nil, audit["decision"], audit["decision_reason"])
	}
}

func TestToolPolicy_RouteFile(t *testing.T) {
	route := func(extra string) string {
		return "version: 2\nroutes:\n  - route: /mcp/x\n    team: t\n    target: dev-gcp:t:x\n    upstream_url: http://x.t/\n" + extra
	}
	bad := map[string]string{
		"v2 no tools":        route(""),
		"v2 empty tools":     route("    tools: []\n"),
		"v2 blank tool":      route("    tools: [\" a\"]\n"),
		"v2 duplicate tool":  route("    tools: [a, a]\n"),
		"v2 tools + passAll": route("    tools: [a]\n    passAllTools: true\n"),
		"v1 with tools":      strings.Replace(route("    tools: [a]\n"), "version: 2", "version: 1", 1),
		"v1 with passAll":    strings.Replace(route("    passAllTools: true\n"), "version: 2", "version: 1", 1),
		"passAll false only": route("    passAllTools: false\n"),
	}
	for name, y := range bad {
		if _, err := parseAndValidatePilotRoutes([]byte(y)); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
	rs, err := parseAndValidatePilotRoutes([]byte(route("    tools: [a, b]\n")))
	if err != nil || !rs[0].allowedTools["a"] || !rs[0].allowedTools["b"] || len(rs[0].allowedTools) != 2 {
		t.Fatalf("tools route: %v %v", err, rs)
	}
	rs, err = parseAndValidatePilotRoutes([]byte(route("    passAllTools: true\n")))
	if err != nil || rs[0].allowedTools != nil {
		t.Fatalf("passAll route must not enforce: %v", err)
	}
	rs, err = parseAndValidatePilotRoutes([]byte(strings.Replace(route(""), "version: 2", "version: 1", 1)))
	if err != nil || rs[0].allowedTools != nil {
		t.Fatalf("v1 route must load as pass-all: %v", err)
	}
	for _, f := range []string{"pilot-routes.dev.yaml", "pilot-routes.prod.yaml", ""} {
		if _, err := (fileRouteSource{path: f}).Routes(); err != nil {
			t.Errorf("%q: %v", f, err)
		}
	}
}
