package mcpkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// ToolErrorResult is a tools/call result that reports msg as a tool error.
func ToolErrorResult(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": msg}},
		"isError": true,
	}
}

// DecodeArgMap decodes a tools/call arguments object, rejecting unknown and duplicate keys and trailing data.
func DecodeArgMap(raw json.RawMessage, allowed map[string]bool) (map[string]json.RawMessage, *WireError) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]json.RawMessage{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	tok, err := dec.Token()
	if err != nil {
		return nil, &WireError{Code: CodeInvalidParams, Message: "arguments must be a JSON object"}
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, &WireError{Code: CodeInvalidParams, Message: "arguments must be a JSON object"}
	}
	fields := make(map[string]json.RawMessage)
	for dec.More() {
		kTok, keyErr := dec.Token()
		if keyErr != nil {
			return nil, &WireError{Code: CodeInvalidParams, Message: "invalid arguments object"}
		}
		key, keyOK := kTok.(string)
		if !keyOK {
			return nil, &WireError{Code: CodeInvalidParams, Message: "invalid argument key"}
		}
		if !allowed[key] {
			return nil, &WireError{Code: CodeInvalidParams, Message: fmt.Sprintf("unknown argument %q", key)}
		}
		if _, dup := fields[key]; dup {
			return nil, &WireError{Code: CodeInvalidParams, Message: fmt.Sprintf("duplicate argument %q", key)}
		}
		var val json.RawMessage
		if valErr := dec.Decode(&val); valErr != nil {
			return nil, &WireError{Code: CodeInvalidParams, Message: fmt.Sprintf("invalid value for %q", key)}
		}
		fields[key] = bytes.TrimSpace(val)
	}
	endTok, endErr := dec.Token()
	if endErr != nil || endTok != json.Delim('}') {
		return nil, &WireError{Code: CodeInvalidParams, Message: "invalid arguments object"}
	}
	if _, eofErr := dec.Token(); eofErr != io.EOF {
		return nil, &WireError{Code: CodeInvalidParams, Message: "trailing data in arguments"}
	}
	return fields, nil
}

// ParseStrictString accepts only a JSON string.
func ParseStrictString(raw json.RawMessage, field string) (string, *WireError) {
	if len(raw) == 0 || raw[0] != '"' {
		return "", &WireError{Code: CodeInvalidParams, Message: fmt.Sprintf("%s must be a string", field)}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", &WireError{Code: CodeInvalidParams, Message: fmt.Sprintf("%s must be a string", field)}
	}
	return s, nil
}

// ParseStrictInt accepts only a JSON integer literal.
func ParseStrictInt(raw json.RawMessage, field string) (int, *WireError) {
	if len(raw) == 0 || raw[0] == '"' || bytes.ContainsAny(raw, ".eE") {
		return 0, &WireError{Code: CodeInvalidParams, Message: fmt.Sprintf("%s must be an integer", field)}
	}
	v, err := strconv.Atoi(string(raw))
	if err != nil {
		return 0, &WireError{Code: CodeInvalidParams, Message: fmt.Sprintf("%s must be an integer", field)}
	}
	return v, nil
}
