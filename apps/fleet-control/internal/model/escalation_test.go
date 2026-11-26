package model

import (
	"encoding/json"
	"testing"
)

func TestClassify(t *testing.T) {
	base := `{"dataCategory":"Intern","upstream":{"app":"a","url":"http://a"},"allowedTools":["t1","t2"],"toolRisk":{"t1":"read","t2":"propose"},"registry":{"name":"no.nav.x/a","description":"d","version":"1","remotes":[{"url":"u"}]},"readme":"r","tools":[{"name":"t1"}],"rosRef":"ros"}`
	with := func(edit func(m map[string]any)) string {
		var m map[string]any
		_ = json.Unmarshal([]byte(base), &m)
		edit(m)
		b, _ := json.Marshal(m)
		return string(b)
	}
	reg := func(k string, v any) func(m map[string]any) {
		return func(m map[string]any) { m["registry"].(map[string]any)[k] = v }
	}
	for _, tc := range []struct {
		name       string
		prev, next string
		escalates  bool
	}{
		{"first version", "", base, true},
		{"unchanged", base, base, false},
		{"tool added", base, with(func(m map[string]any) { m["allowedTools"] = []string{"t1", "t2", "t3"} }), true},
		{"tool removed", base, with(func(m map[string]any) {
			m["allowedTools"] = []string{"t1"}
			delete(m["toolRisk"].(map[string]any), "t2")
		}), false},
		{"risk raised read to propose", base, with(func(m map[string]any) { m["toolRisk"].(map[string]any)["t1"] = "propose" }), true},
		{"risk raised to write by dropping its entry", base, with(func(m map[string]any) { delete(m["toolRisk"].(map[string]any), "t2") }), true},
		{"risk lowered", base, with(func(m map[string]any) { m["toolRisk"].(map[string]any)["t2"] = "read" }), false},
		{"defaultRisk raises a tool", with(func(m map[string]any) { m["toolRisk"] = map[string]string{"t1": "read"}; m["defaultRisk"] = "read" }),
			with(func(m map[string]any) { m["toolRisk"] = map[string]string{"t1": "read"}; m["defaultRisk"] = "write" }), true},
		{"defaultRisk lowers a tool", with(func(m map[string]any) { m["toolRisk"] = map[string]string{"t1": "read"} }),
			with(func(m map[string]any) { m["toolRisk"] = map[string]string{"t1": "read"}; m["defaultRisk"] = "propose" }), false},
		{"unknown data category before", `{"dataCategory":""}`, `{"dataCategory":"Aapen"}`, true},
		{"unknown risk counts as write", base, with(func(m map[string]any) { m["toolRisk"].(map[string]any)["t1"] = "admin" }), true},
		{"data category raised", base, with(func(m map[string]any) { m["dataCategory"] = "Fortrolig" }), true},
		{"data category lowered", base, with(func(m map[string]any) { m["dataCategory"] = "Aapen" }), false},
		{"unknown data category", base, with(func(m map[string]any) { m["dataCategory"] = "Ny" }), true},
		{"upstream url changed", base, with(func(m map[string]any) { m["upstream"] = map[string]string{"app": "a", "url": "http://b"} }), true},
		{"upstream app changed", base, with(func(m map[string]any) { m["upstream"] = map[string]string{"app": "b", "url": "http://a"} }), true},
		{"upstream removed", base, with(func(m map[string]any) { delete(m, "upstream") }), true},
		{"readme changed", base, with(func(m map[string]any) { m["readme"] = "new" }), false},
		{"tool docs changed", base, with(func(m map[string]any) { m["tools"] = []map[string]string{{"name": "t1", "description": "x"}} }), false},
		{"rosRef changed", base, with(func(m map[string]any) { m["rosRef"] = "ros2" }), false},
		{"registry description", base, with(reg("description", "new")), false},
		{"registry version", base, with(reg("version", "2")), false},
		{"registry title added", base, with(reg("title", "T")), false},
		{"registry name", base, with(reg("name", "no.nav.x/b")), true},
		{"registry remotes", base, with(reg("remotes", []map[string]string{{"url": "v"}})), true},
		{"registry unknown key", base, with(reg("packages", []string{"p"})), true},
		{"registry removed", base, with(func(m map[string]any) { delete(m, "registry") }), true},
		{"unknown spec field", base, with(func(m map[string]any) { m["passAllTools"] = true }), true},
		{"unparsable prev", "{", base, true},
	} {
		var prev json.RawMessage
		if tc.prev != "" {
			prev = json.RawMessage(tc.prev)
		}
		c := Classify(prev, json.RawMessage(tc.next))
		if c.Escalates != tc.escalates {
			t.Errorf("%s: escalates = %v, want %v (reasons %v, changed %v)", tc.name, c.Escalates, tc.escalates, c.Reasons, c.Changed)
		}
		if c.Escalates != (len(c.Reasons) > 0) {
			t.Errorf("%s: Escalates and Reasons disagree", tc.name)
		}
	}
}
