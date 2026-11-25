package main

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxDescription = 1024

var annotationHints = []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"}

// hiddenInstruction matches wording that only an injection uses: FAIL.
// softInstruction matches imperative wording honest descriptions also use: WARN.
// Both are deliberately narrow.
var (
	hiddenInstruction = regexp.MustCompile(`(?i)ignore\s+(?:(?:all|any|the)\s+)?(?:previous|prior)(?:\s+instructions)?|system\s+prompt|(?:do\s+not|don't)\s+tell\s+the\s+user|<\s*/?\s*(?:system|instructions?)\s*>`)
	softInstruction   = regexp.MustCompile(`(?i)\b(?:you\s+must|always|never)\b`)
)

// invisibleRune reports zero-width, bidi-control and tag characters.
func invisibleRune(r rune) bool {
	switch {
	case r == 0x061C: // Arabic letter mark
		return true
	case r >= 0x200B && r <= 0x200F, r == 0x2060, r == 0xFEFF: // zero-width, LRM/RLM, word joiner, BOM
		return true
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069: // bidi embeddings and isolates
		return true
	case r >= 0xE0000 && r <= 0xE007F: // tag characters
		return true
	}
	return false
}

// softInvisibleRune reports characters that are often legitimate (soft hyphen,
// variation selectors) but can also hide text: WARN.
func softInvisibleRune(r rune) bool {
	return r == 0x00AD || r == 0x034F || r == 0x180E || r >= 0xFE00 && r <= 0xFE0F || r >= 0xE0100 && r <= 0xE01EF
}

func descriptions(tool map[string]any) []string {
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if d, ok := x["description"].(string); ok {
				out = append(out, d)
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(x[k])
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	for _, v := range []any{tool["description"], tool["title"]} {
		if d, ok := v.(string); ok {
			out = append(out, d)
		}
	}
	if a, ok := tool["annotations"].(map[string]any); ok {
		if d, ok := a["title"].(string); ok {
			out = append(out, d)
		}
	}
	walk(tool["inputSchema"])
	return out
}

// fragment returns the match with a little context; redact caps it later.
func fragment(s string, start, end int) string {
	from := max(0, start-20)
	to := min(len(s), end+20)
	for from > 0 && !utf8.RuneStart(s[from]) {
		from--
	}
	for to < len(s) && !utf8.RuneStart(s[to]) {
		to++
	}
	return strings.ToValidUTF8(s[from:to], "")
}

// lintTools checks tools/list output and returns one row per finding, or one
// PASS row per rule. only restricts the check to one tool (-tool).
func (cfg *config) lintTools() []result {
	resp, err := cfg.do(http.MethodPost, cfg.token, modernBody("tools/list", nil), modernHeaders("tools/list", ""))
	if e := expectStatus(resp, err, 200); e != nil {
		return []result{{name: "tool descriptions", status: "SKIP", detail: "tools/list: " + e.Error()}}
	}
	r, _ := resp.body["result"].(map[string]any)
	list, _ := r["tools"].([]any)
	rules := []string{"no hidden instructions in tool descriptions", "no invisible or bidi characters in tool descriptions", "tools have all four annotations", "tool descriptions at most 1024 characters"}
	found := map[string]bool{}
	var rs []result
	add := func(rule, status, tool, detail string) {
		found[rule] = true
		rs = append(rs, result{name: rule + ": " + tool, status: status, detail: detail})
	}
	for _, t := range list {
		tool, _ := t.(map[string]any)
		name := fmt.Sprint(tool["name"])
		if cfg.tool != "" && name != cfg.tool {
			continue
		}
		for _, d := range descriptions(tool) {
			if loc := hiddenInstruction.FindStringIndex(d); loc != nil {
				add(rules[0], "FAIL", name, fmt.Sprintf("%q", fragment(d, loc[0], loc[1])))
			}
			if loc := softInstruction.FindStringIndex(d); loc != nil {
				add(rules[0], "WARN", name, fmt.Sprintf("%q", fragment(d, loc[0], loc[1])))
			}
			soft := -1
			for i, c := range d {
				if invisibleRune(c) {
					add(rules[1], "FAIL", name, fmt.Sprintf("U+%04X at byte %d", c, i))
					soft = -2
					break
				}
				if soft == -1 && softInvisibleRune(c) {
					soft = i
				}
			}
			if soft >= 0 {
				add(rules[1], "WARN", name, fmt.Sprintf("U+%04X at byte %d", []rune(d[soft:])[0], soft))
			}
			if n := utf8.RuneCountInString(d); n > maxDescription {
				add(rules[3], "WARN", name, fmt.Sprintf("%d characters, over %d", n, maxDescription))
			}
		}
		ann, _ := tool["annotations"].(map[string]any)
		var missing []string
		for _, h := range annotationHints {
			if _, ok := ann[h].(bool); !ok {
				missing = append(missing, h)
			}
		}
		if len(missing) > 0 {
			add(rules[2], "FAIL", name, "missing "+strings.Join(missing, ", "))
		}
	}
	for _, rule := range rules {
		if !found[rule] {
			rs = append(rs, result{name: rule, status: "PASS"})
		}
	}
	if len(list) == 0 {
		return []result{{name: "tool descriptions", status: "SKIP", detail: "tools/list returned no tools"}}
	}
	return rs
}
