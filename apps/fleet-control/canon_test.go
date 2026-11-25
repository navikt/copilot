package main

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// Vectors from RFC 8785 section 3.2.2 and appendix B.
func TestJCS(t *testing.T) {
	for in, want := range map[string]string{
		`{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001],"literals":[null,true,false]}`: `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27]}`,
		// Keys sort by UTF-16 code units: U+1F600 (surrogates D83D) sorts before U+FB33.
		`{"\u20ac":1,"\r":2,"1":3,"\ud83d\ude00":4,"\u0080":5,"\u00f6":6,"\ufb33":7}`: "{\"\\r\":2,\"1\":3,\"\u0080\":5,\"\u00f6\":6,\"\u20ac\":1,\"\U0001f600\":4,\"\ufb33\":7}",
		`[-0, 1e21, 1e-7, 123456789012345680000, 9007199254740992]`:                   `[0,1e+21,1e-7,123456789012345680000,9007199254740992]`,
		// Only quote, backslash and control characters are escaped; <&> and / are not.
		`"<&>/\u000f\n\"\\"`: `"<&>/\u000f\n\"\\"`,
	} {
		var v any
		if err := json.Unmarshal([]byte(in), &v); err != nil {
			t.Fatal(err)
		}
		got, err := jcs(v)
		if err != nil || string(got) != want {
			t.Errorf("jcs(%s)\n got %s %v\nwant %s", in, got, err, want)
		}
	}
}

// A set in another order gives the same hash; a new tool does not.
func TestServerSpecHashNormalises(t *testing.T) {
	spec := func(tools ...string) string {
		s, _, err := validateServerSpec("dev-gcp", "", "team-a", "svc", MCPServerSpec{DataCategory: "Intern", Upstream: &Upstream{App: "svc", URL: "http://svc.team-a/mcp"}, AllowedTools: tools})
		if err != nil {
			t.Fatal(err)
		}
		h, _ := specHash(s)
		return h
	}
	if spec("b", "a") != spec("a", "b") || spec("a", "b") == spec("a", "b", "c") {
		t.Fatal("allowedTools is a set for the hash")
	}
}

// A spec from before v0.8 (no readme, no tools) keeps its stored hash.
// The value is SHA-256 over the JCS text, computed outside this code.
func TestServerSpecHashStableForOldSpecs(t *testing.T) {
	s, _, err := validateServerSpec("dev-gcp", "", "team-a", "svc", MCPServerSpec{DataCategory: "Intern", Upstream: &Upstream{App: "svc", URL: "http://svc.team-a/mcp"}, AllowedTools: []string{"b", "a"}, Tools: []ToolDoc{}, ToolRisk: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := specHash(s); h != "v1:sha256:4b421d525ace75dd45814de801bbe840e3e0d514318127a939bf72c0e9988001" {
		t.Fatalf("old spec hash changed: %s", h)
	}
	// readme and tool descriptions are content: a change needs approval again.
	base, _ := specHash(s)
	for _, mod := range []func(*MCPServerSpec){
		func(m *MCPServerSpec) { m.Readme = "# Hei" },
		func(m *MCPServerSpec) { m.Tools = []ToolDoc{{Name: "a", Description: "x"}} },
		func(m *MCPServerSpec) { m.ToolRisk = map[string]string{"a": "read"} },
		func(m *MCPServerSpec) { m.RosRef = "ROS-1" },
		func(m *MCPServerSpec) { m.DefaultRisk = "read" },
	} {
		m := s
		mod(&m)
		if h, _ := specHash(m); h == base {
			t.Errorf("hash ignores %+v", m)
		}
	}
}

func TestServerSpecV08Caps(t *testing.T) {
	ok := MCPServerSpec{DataCategory: "Intern", Upstream: &Upstream{App: "svc", URL: "http://svc.team-a/mcp"}, AllowedTools: []string{"a", "b"},
		Readme: strings.Repeat("x", maxReadmeBytes), Tools: []ToolDoc{{Name: "b", Description: strings.Repeat("x", maxToolDescriptionBytes)}, {Name: "a"}}}
	s, _, err := validateServerSpec("dev-gcp", "", "team-a", "svc", ok)
	if err != nil {
		t.Fatalf("at the cap: %v", err)
	}
	if s.Tools[0].Name != "a" {
		t.Errorf("tools not sorted: %+v", s.Tools)
	}
	for name, mod := range map[string]func(*MCPServerSpec){
		"readme": func(m *MCPServerSpec) { m.Readme += "x" },
		"description": func(m *MCPServerSpec) {
			m.Tools = []ToolDoc{{Name: "a", Description: strings.Repeat("x", maxToolDescriptionBytes+1)}}
		},
		"unknown": func(m *MCPServerSpec) { m.Tools = []ToolDoc{{Name: "c"}} },
		"too many": func(m *MCPServerSpec) {
			m.Tools, m.AllowedTools = nil, nil
			for i := range maxServerTools + 1 {
				m.AllowedTools = append(m.AllowedTools, "t"+strconv.Itoa(i))
			}
		},
		"twice": func(m *MCPServerSpec) { m.Tools = []ToolDoc{{Name: "a"}, {Name: "a"}} },
	} {
		m := ok
		mod(&m)
		var se *statusError
		if _, _, err := validateServerSpec("dev-gcp", "", "team-a", "svc", m); !errors.As(err, &se) || se.code != http.StatusUnprocessableEntity {
			t.Errorf("%s: want 422, got %v", name, err)
		}
	}
}

// #2903: toolRisk names allowed tools only; outside dev-* a write tool (a
// tool without risk included) needs rosRef, and an unset cluster is prod.
func TestServerSpecToolRiskAndRosRef(t *testing.T) {
	base := MCPServerSpec{DataCategory: "Intern", Upstream: &Upstream{App: "svc", URL: "http://svc.team-a/mcp"}, AllowedTools: []string{"a", "b"}}
	for _, c := range []struct {
		name, cluster string
		mod           func(*MCPServerSpec)
		ok            bool
		field         string // 422 cause field, "" for any
	}{
		{"dev write without ros", "dev-gcp", func(*MCPServerSpec) {}, true, ""},
		{"prod unset risk is write", "prod-gcp", func(*MCPServerSpec) {}, false, "spec.rosRef"},
		{"unset cluster is prod", "", func(*MCPServerSpec) {}, false, "spec.rosRef"},
		{"unknown cluster is prod", "prod-fss", func(*MCPServerSpec) {}, false, "spec.rosRef"},
		{"prod one write", "prod-gcp", func(m *MCPServerSpec) { m.ToolRisk = map[string]string{"a": "read"} }, false, "spec.rosRef"},
		{"prod all read", "prod-gcp", func(m *MCPServerSpec) { m.ToolRisk = map[string]string{"a": "read", "b": "read"} }, true, ""},
		{"prod write with ros", "prod-gcp", func(m *MCPServerSpec) { m.RosRef = "https://ros.example/1" }, true, ""},
		{"unknown tool", "dev-gcp", func(m *MCPServerSpec) { m.ToolRisk = map[string]string{"c": "read"} }, false, ""},
		{"bad risk", "dev-gcp", func(m *MCPServerSpec) { m.ToolRisk = map[string]string{"a": "Read"} }, false, ""},
		{"prod default read", "prod-gcp", func(m *MCPServerSpec) { m.DefaultRisk = "read" }, true, ""},
		{"prod propose needs no ros", "prod-gcp", func(m *MCPServerSpec) { m.DefaultRisk = "propose" }, true, ""},
		{"prod toolRisk propose", "prod-gcp", func(m *MCPServerSpec) { m.ToolRisk = map[string]string{"a": "propose", "b": "read"} }, true, ""},
		{"prod default read, one write", "prod-gcp", func(m *MCPServerSpec) { m.DefaultRisk = "read"; m.ToolRisk = map[string]string{"a": "write"} }, false, "spec.rosRef"},
		{"prod default write", "prod-gcp", func(m *MCPServerSpec) { m.DefaultRisk = "write"; m.ToolRisk = map[string]string{"a": "read"} }, false, "spec.rosRef"},
		{"bad default", "dev-gcp", func(m *MCPServerSpec) { m.DefaultRisk = "Read" }, false, "spec.defaultRisk"},
		{"unknown default", "dev-gcp", func(m *MCPServerSpec) { m.DefaultRisk = "admin" }, false, "spec.defaultRisk"},
		{"rosRef spaces", "dev-gcp", func(m *MCPServerSpec) { m.RosRef = " x" }, false, ""},
		{"rosRef too long", "dev-gcp", func(m *MCPServerSpec) { m.RosRef = strings.Repeat("x", maxRosRefBytes+1) }, false, ""},
	} {
		m := base
		c.mod(&m)
		_, sv, err := validateServerSpec(c.cluster, "", "team-a", "svc", m)
		var se *statusError
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case !c.ok && (!errors.As(err, &se) || se.code != http.StatusUnprocessableEntity):
			t.Errorf("%s: want 422, got %v", c.name, err)
		case !c.ok && c.field != "" && (len(se.causes) != 1 || se.causes[0].Field != c.field):
			t.Errorf("%s: want cause %s, got %+v", c.name, c.field, se.causes)
		case c.ok && len(m.ToolRisk) > 0 && sv.Route.ToolRisk["a"] != m.ToolRisk["a"]:
			t.Errorf("%s: route lacks toolRisk: %+v", c.name, sv.Route)
		}
	}
	// #2967: the route carries the resolved level per tool; toolRisk wins.
	m := base
	m.DefaultRisk, m.ToolRisk = "propose", map[string]string{"a": "read"}
	if _, sv, err := validateServerSpec("dev-gcp", "", "team-a", "svc", m); err != nil || !maps.Equal(sv.Route.ToolRisk, map[string]string{"a": "read", "b": "propose"}) {
		t.Errorf("resolved toolRisk: %v %+v", err, sv.Route)
	}
}

func TestToolRiskOfFailsClosed(t *testing.T) {
	for in, want := range map[string]string{"read": "read", "propose": "propose", "write": "write", "": "write", "admin": "write", "Read": "write"} {
		if got := toolRiskOf(map[string]string{"t": in}, "t"); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	if riskRank["read"] >= riskRank["propose"] || riskRank["propose"] >= riskRank["write"] {
		t.Error("ranking must be read < propose < write")
	}
}
