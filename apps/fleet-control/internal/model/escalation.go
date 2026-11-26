package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
)

// Owner decision 2026-11-26 (#2634): only a change that adds access or
// privileges waits for the platform team. Anything else an owner team
// submits publishes directly.

// riskRank orders tool risk: read < propose < write. A change to or from an
// unknown level escalates.
var riskRank = map[string]int{"read": 1, "propose": 2, "write": 3}

// dataRank orders spec.dataCategory (validateServerSpec's list). A change
// to or from an unknown or missing category escalates.
var dataRank = map[string]int{"Aapen": 1, "Intern": 2, "Fortrolig": 3}

// safeSpecFields never change who can call what. Every other field is
// compared below; a field this file does not know is an escalation.
var safeSpecFields = map[string]bool{"readme": true, "tools": true, "rosRef": true}

// safeRegistryFields are registry text that does not change who can call
// what. Any other registry key that changes (name, remotes, packages, ...)
// is an escalation.
var safeRegistryFields = map[string]bool{"description": true, "title": true, "version": true, "websiteUrl": true, "repository": true}

// Change is the result of Classify.
type Change struct {
	Escalates bool
	// Reasons names each escalation; empty when Escalates is false.
	Reasons []string
	// Changed lists the top-level spec fields that differ.
	Changed []string
}

// Classify compares two MCPServer specs as JSON objects. prev nil means no
// published generation: always an escalation. It fails closed: an
// unparsable spec or an unknown field that changed escalates.
func Classify(prev, next json.RawMessage) (c Change) {
	esc := func(f string, a ...any) { c.Reasons = append(c.Reasons, fmt.Sprintf(f, a...)) }
	defer func() { c.Escalates = len(c.Reasons) > 0 }()
	var p, n map[string]json.RawMessage
	if len(prev) == 0 {
		esc("first version")
		return c
	}
	if json.Unmarshal(prev, &p) != nil || json.Unmarshal(next, &n) != nil {
		esc("spec cannot be compared")
		return c
	}
	keys := map[string]bool{}
	for k := range p {
		keys[k] = true
	}
	for k := range n {
		keys[k] = true
	}
	for k := range keys {
		if !sameJSON(p[k], n[k]) {
			c.Changed = append(c.Changed, k)
		}
	}
	sort.Strings(c.Changed)
	toolsChecked := false
	for _, k := range c.Changed {
		switch k {
		case "readme", "tools", "rosRef":
		case "dataCategory":
			var a, b string
			_ = json.Unmarshal(p[k], &a)
			_ = json.Unmarshal(n[k], &b)
			if dataRank[a] == 0 || dataRank[b] == 0 || dataRank[b] > dataRank[a] {
				esc("data category raised from %q to %q", a, b)
			}
		case "upstream":
			esc("upstream changed")
		case "allowedTools", "toolRisk", "defaultRisk":
			if !toolsChecked {
				toolsChecked = true
				pt, pe := toolRisks(p)
				nt, ne := toolRisks(n)
				if pe != nil || ne != nil {
					esc("tools cannot be compared")
					continue
				}
				for _, t := range sortedKeys(nt) {
					old, ok := pt[t]
					switch {
					case !ok:
						esc("tool %s added", t)
					case riskRank[old] == 0 || riskRank[nt[t]] == 0 || riskRank[nt[t]] > riskRank[old]:
						esc("tool %s risk raised from %s to %s", t, old, nt[t])
					}
				}
			}
		case "registry":
			for _, r := range registryEscalations(p[k], n[k]) {
				esc("%s", r)
			}
		default:
			if !safeSpecFields[k] {
				esc("field %s changed", k)
			}
		}
	}
	return c
}

// toolRisks resolves allowedTools with toolRisk and defaultRisk; a tool
// without either is write.
func toolRisks(spec map[string]json.RawMessage) (map[string]string, error) {
	var tools []string
	var risk map[string]string
	var def string
	for k, v := range map[string]any{"allowedTools": &tools, "toolRisk": &risk, "defaultRisk": &def} {
		if raw, ok := spec[k]; ok {
			if err := json.Unmarshal(raw, v); err != nil {
				return nil, err
			}
		}
	}
	out := make(map[string]string, len(tools))
	for _, t := range tools {
		out[t] = "write"
		if def != "" {
			out[t] = def
		}
		if r, ok := risk[t]; ok {
			out[t] = r
		}
	}
	return out, nil
}

func registryEscalations(prev, next json.RawMessage) []string {
	var p, n map[string]json.RawMessage
	if len(prev) > 0 && json.Unmarshal(prev, &p) != nil || len(next) > 0 && json.Unmarshal(next, &n) != nil {
		return []string{"registry cannot be compared"}
	}
	var out []string
	keys := map[string]bool{}
	for k := range p {
		keys[k] = true
	}
	for k := range n {
		keys[k] = true
	}
	for _, k := range sortedKeys(keys) {
		if !safeRegistryFields[k] && !sameJSON(p[k], n[k]) {
			out = append(out, "registry "+k+" changed")
		}
	}
	return out
}

func sameJSON(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return bytes.Equal(a, b)
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return bytes.Equal(xa, ya)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
