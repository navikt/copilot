package source

import (
	"bytes"
	"strings"
)

// SplitFrontmatter splits a markdown file into YAML frontmatter and body.
// Returns (frontmatter, body, hasFrontmatter).
// Frontmatter is the content between the opening and closing "---" delimiters
// (without the delimiters themselves). Body is everything after the closing "---".
func SplitFrontmatter(data []byte) ([]byte, []byte, bool) {
	const delimiter = "---"

	// Normalize CRLF → LF for consistent parsing
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))

	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if !hasDelimiterPrefix(trimmed) {
		return nil, data, false
	}

	// Find the end of the opening delimiter line
	afterOpen := trimmed[len(delimiter):]
	// Allow trailing whitespace on the delimiter line
	afterOpen = bytes.TrimLeft(afterOpen, " \t")
	idx := bytes.IndexByte(afterOpen, '\n')
	if idx < 0 {
		// Only "---" with no closing delimiter
		return nil, data, false
	}
	afterOpen = afterOpen[idx+1:]

	// Find the closing "---" (allowing trailing whitespace)
	closeIdx := findClosingDelimiter(afterOpen)
	if closeIdx < 0 {
		// Check if afterOpen starts with closing delimiter (empty frontmatter)
		if isDelimiterLine(afterOpen) {
			delimEnd := len(delimiter)
			rest := afterOpen[delimEnd:]
			// Skip trailing whitespace and newline on delimiter line
			for len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
				rest = rest[1:]
			}
			if len(rest) > 0 && rest[0] == '\n' {
				rest = rest[1:]
			}
			return []byte{}, rest, true
		}
		return nil, data, false
	}

	// Include the trailing newline in frontmatter
	fm := afterOpen[:closeIdx+1]
	// Skip past \n---<optional whitespace>\n
	rest := afterOpen[closeIdx+1:]
	// Skip the --- and any trailing whitespace
	rest = rest[len(delimiter):]
	for len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
		rest = rest[1:]
	}

	// Skip the newline after closing ---
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}

	return fm, rest, true
}

// hasDelimiterPrefix checks if data starts with "---".
func hasDelimiterPrefix(data []byte) bool {
	return bytes.HasPrefix(data, []byte("---"))
}

// isDelimiterLine checks if data starts with "---" optionally followed by whitespace.
func isDelimiterLine(data []byte) bool {
	if !hasDelimiterPrefix(data) {
		return false
	}
	rest := data[3:]
	for len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
		rest = rest[1:]
	}
	return len(rest) == 0 || rest[0] == '\n'
}

// findClosingDelimiter finds the index of the newline before the closing "---" delimiter.
// Returns -1 if not found. Allows trailing whitespace after "---".
func findClosingDelimiter(data []byte) int {
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' && isDelimiterLine(data[i+1:]) {
			return i
		}
	}
	return -1
}

// StripFrontmatterKeys removes the specified top-level YAML keys (and their
// nested children) from frontmatter content.
func StripFrontmatterKeys(fm []byte, keys []string) []byte {
	if len(keys) == 0 || len(fm) == 0 {
		return fm
	}

	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}

	lines := bytes.Split(fm, []byte("\n"))
	var out [][]byte
	skipping := false

	for _, line := range lines {
		trimmed := bytes.TrimRight(line, " \t")

		// Check if this is a top-level key (no leading whitespace)
		if len(trimmed) > 0 && trimmed[0] != ' ' && trimmed[0] != '\t' {
			colonIdx := bytes.IndexByte(trimmed, ':')
			if colonIdx > 0 {
				if keySet[string(trimmed[:colonIdx])] {
					skipping = true
					continue
				}
			}
			skipping = false
		} else if skipping {
			// Indented line under a skipped key
			continue
		}

		if !skipping {
			out = append(out, line)
		}
	}

	result := bytes.Join(out, []byte("\n"))
	// Trim trailing empty lines that might remain
	result = bytes.TrimRight(result, "\n")
	if len(result) > 0 {
		result = append(result, '\n')
	}
	return result
}

// ExtractFrontmatterValue extracts the value of a simple top-level key from
// frontmatter. Returns ("", false) if not found. Only works for simple
// "key: value" pairs, not nested structures.
func ExtractFrontmatterValue(fm []byte, key string) (string, bool) {
	prefix := key + ":"
	lines := bytes.Split(fm, []byte("\n"))
	for _, line := range lines {
		trimmed := bytes.TrimRight(line, " \t")
		if bytes.HasPrefix(trimmed, []byte(prefix)) {
			val := string(trimmed[len(prefix):])
			val = strings.TrimSpace(val)
			// Remove surrounding quotes if present
			if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
				val = val[1 : len(val)-1]
			}
			return val, true
		}
	}
	return "", false
}

// openCodePrimaryAgents are the materialized agent names that opencode should
// treat as primary agents — i.e. selectable in the Tab/switch_agent picker and
// launchable via `opencode --agent <name>`. Every other Nav agent is exported
// as a subagent (invoked via @mention or delegated to by a primary agent),
// matching the @agent usage in their metadata examples.
var openCodePrimaryAgents = map[string]bool{
	"nav-pilot":      true,
	"nav-pilot-opus": true,
}

// OpenCodeAgentMode returns the opencode agent mode ("primary" or "subagent")
// for a materialized agent of the given name. Primary agents appear in the
// opencode agent selector and can be launched with `--agent <name>`.
func OpenCodeAgentMode(name string) string {
	if openCodePrimaryAgents[name] {
		return "primary"
	}
	return "subagent"
}

// BuildAgentFrontmatter generates OpenCode-compatible agent frontmatter.
// mode must be a valid opencode agent mode ("primary" or "subagent").
func BuildAgentFrontmatter(description, mode string) []byte {
	if mode == "" {
		mode = "subagent"
	}
	var buf bytes.Buffer
	buf.WriteString("description: " + yamlQuoteIfNeeded(description) + "\n")
	buf.WriteString("mode: " + mode + "\n")
	return buf.Bytes()
}

// yamlQuoteIfNeeded wraps a string in double quotes if it contains characters
// that are special in YAML.
func yamlQuoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	needsQuoting := false
	for _, c := range s {
		switch c {
		case ':', '#', '[', ']', '{', '}', ',', '&', '*', '?', '|', '-', '<', '>', '=', '!', '%', '@', '`', '\'', '"':
			needsQuoting = true
		}
	}
	if needsQuoting {
		escaped := strings.ReplaceAll(s, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return s
}

// TransformPromptFrontmatter strips the "name" key from prompt frontmatter
// since OpenCode derives the command name from the filename.
func TransformPromptFrontmatter(fm []byte) []byte {
	return StripFrontmatterKeys(fm, []string{"name"})
}

// Reassemble combines frontmatter and body back into a complete file.
// If fm is nil or empty, returns just the body.
func Reassemble(fm, body []byte) []byte {
	if len(fm) == 0 {
		return body
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fm)
	if len(fm) > 0 && fm[len(fm)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString("---\n")
	if len(body) > 0 {
		buf.WriteByte('\n')
		buf.Write(body)
	}
	return buf.Bytes()
}
