package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxToolResultBytes = 128 * 1024
	maxPageSize        = 50
	defaultPageSize    = 20
	maxPageNo          = 10000
	maxFragmentLen     = 100
)

// readOnlyAnnotations are client hints, never a security boundary.
var readOnlyAnnotations = map[string]bool{
	"readOnlyHint":    true,
	"destructiveHint": false,
	"idempotentHint":  true,
	"openWorldHint":   false,
}

func intProp(desc string, minimum, maximum int) map[string]any {
	return map[string]any{"type": "integer", "description": desc, "minimum": minimum, "maximum": maximum}
}

var pagingProps = map[string]any{
	"pageNo":   intProp("Zero-based page number (default 0).", 0, maxPageNo),
	"pageSize": intProp("Page size (default 20, max 50).", 1, maxPageSize),
}

func withPaging(props map[string]any) map[string]any {
	for k, v := range pagingProps {
		props[k] = v
	}
	return props
}

func tool(name, desc string, required []string, props map[string]any) map[string]any {
	schema := map[string]any{"type": "object", "additionalProperties": false, "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return map[string]any{"name": name, "description": desc, "inputSchema": schema, "annotations": readOnlyAnnotations}
}

func toolDefinitions() []map[string]any {
	gruppeID := intProp("Dolly test group id (gruppeId).", 1, 1<<53)
	return []map[string]any{
		tool("dolly_current_user", "Get the signed-in user's Dolly profile (brukerId, team).", nil, map[string]any{}),
		tool("dolly_list_groups", "List the signed-in user's Dolly test groups (testgrupper), paginated.", nil, withPaging(map[string]any{})),
		tool("dolly_get_group", "Get one Dolly test group with a page of its test persons (identer).", []string{"gruppeId"},
			withPaging(map[string]any{"gruppeId": gruppeID})),
		tool("dolly_search", "Search Dolly test groups or orders (bestillinger) by name or id fragment.", []string{"fragment", "kind"}, map[string]any{
			"fragment": map[string]any{"type": "string", "minLength": 1, "maxLength": maxFragmentLen, "description": "Search text (1-100 characters)."},
			"kind":     map[string]any{"type": "string", "enum": []string{"gruppe", "bestilling"}, "description": "What to search: gruppe or bestilling."},
		}),
		tool("dolly_get_order", "Get status of one Dolly order (bestilling) by id.", []string{"bestillingId"}, map[string]any{
			"bestillingId": intProp("Dolly order id (bestillingId).", 1, 1<<53),
		}),
		tool("dolly_list_group_orders", "List orders (bestillinger) with status for a Dolly test group, paginated.", []string{"gruppeId"},
			withPaging(map[string]any{"gruppeId": gruppeID})),
	}
}

func (s *server) handleToolCall(ctx context.Context, inboundToken, toolName string, rawArgs json.RawMessage) (any, *mcpRPCError) {
	var fields map[string]json.RawMessage
	var rpcErr *mcpRPCError
	allow := func(keys ...string) bool {
		m := map[string]bool{}
		for _, k := range keys {
			m[k] = true
		}
		fields, rpcErr = decodeArgMap(rawArgs, m)
		return rpcErr == nil
	}
	get := func(path string, q url.Values) any {
		return s.execWithOBO(ctx, inboundToken, func(tok string) (string, error) {
			b, err := s.dolly.get(ctx, tok, path, q)
			if err != nil {
				return "", err
			}
			return capResult(b)
		})
	}

	switch toolName {
	case "dolly_current_user":
		if !allow() {
			return nil, rpcErr
		}
		return get("/api/v1/bruker/current", nil), nil

	case "dolly_list_groups":
		if !allow("pageNo", "pageSize") {
			return nil, rpcErr
		}
		pageNo, pageSize, e := parsePaging(fields)
		if e != nil {
			return nil, e
		}
		return s.execWithOBO(ctx, inboundToken, func(tok string) (string, error) {
			brukerID, err := s.dolly.currentBrukerID(ctx, tok)
			if err != nil {
				return "", err
			}
			q := url.Values{"brukerId": {brukerID}, "pageNo": {strconv.Itoa(pageNo)}, "pageSize": {strconv.Itoa(pageSize)}}
			b, err := s.dolly.get(ctx, tok, "/api/v1/gruppe", q)
			if err != nil {
				return "", err
			}
			return capResult(b)
		}), nil

	case "dolly_get_group":
		if !allow("gruppeId", "pageNo", "pageSize") {
			return nil, rpcErr
		}
		id, e := parseID(fields, "gruppeId")
		if e != nil {
			return nil, e
		}
		pageNo, pageSize, e := parsePaging(fields)
		if e != nil {
			return nil, e
		}
		path := fmt.Sprintf("/api/v1/gruppe/%d/page/%d", id, pageNo)
		return get(path, url.Values{"pageSize": {strconv.Itoa(pageSize)}}), nil

	case "dolly_search":
		if !allow("fragment", "kind") {
			return nil, rpcErr
		}
		fragment, e := parseFragment(fields)
		if e != nil {
			return nil, e
		}
		kindRaw, ok := fields["kind"]
		if !ok {
			return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "kind is required"}
		}
		kind, e := parseStrictString(kindRaw, "kind")
		if e != nil {
			return nil, e
		}
		var path string
		switch kind {
		case "gruppe":
			path = "/api/v1/gruppe/soekGruppe"
		case "bestilling":
			path = "/api/v1/bestilling/soekBestilling"
		default:
			return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "kind must be gruppe or bestilling"}
		}
		return get(path, url.Values{"fragment": {fragment}}), nil

	case "dolly_get_order":
		if !allow("bestillingId") {
			return nil, rpcErr
		}
		id, e := parseID(fields, "bestillingId")
		if e != nil {
			return nil, e
		}
		return get(fmt.Sprintf("/api/v1/bestilling/%d", id), nil), nil

	case "dolly_list_group_orders":
		if !allow("gruppeId", "pageNo", "pageSize") {
			return nil, rpcErr
		}
		id, e := parseID(fields, "gruppeId")
		if e != nil {
			return nil, e
		}
		pageNo, pageSize, e := parsePaging(fields)
		if e != nil {
			return nil, e
		}
		q := url.Values{"page": {strconv.Itoa(pageNo)}, "pageSize": {strconv.Itoa(pageSize)}}
		return get(fmt.Sprintf("/api/v1/bestilling/gruppe/%d", id), q), nil

	default:
		return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "unknown tool"}
	}
}

func capResult(b []byte) (string, error) {
	if len(b) > maxToolResultBytes {
		return "", fmt.Errorf("result exceeds %d bytes; use a smaller pageSize or a narrower search", maxToolResultBytes)
	}
	return string(b), nil
}

func parseID(fields map[string]json.RawMessage, field string) (int64, *mcpRPCError) {
	raw, ok := fields[field]
	if !ok {
		return 0, &mcpRPCError{Code: mcpCodeInvalidParams, Message: field + " is required"}
	}
	if len(raw) == 0 || raw[0] == '"' || raw[0] == '-' || bytes.ContainsAny(raw, ".eE") {
		return 0, &mcpRPCError{Code: mcpCodeInvalidParams, Message: field + " must be a positive integer"}
	}
	v, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || v < 1 {
		return 0, &mcpRPCError{Code: mcpCodeInvalidParams, Message: field + " must be a positive integer"}
	}
	return v, nil
}

func parsePaging(fields map[string]json.RawMessage) (pageNo, pageSize int, rpcErr *mcpRPCError) {
	pageSize = defaultPageSize
	if raw, ok := fields["pageNo"]; ok {
		if pageNo, rpcErr = parseStrictInt(raw, "pageNo"); rpcErr != nil {
			return 0, 0, rpcErr
		}
		if pageNo < 0 || pageNo > maxPageNo {
			return 0, 0, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("pageNo must be 0 to %d", maxPageNo)}
		}
	}
	if raw, ok := fields["pageSize"]; ok {
		if pageSize, rpcErr = parseStrictInt(raw, "pageSize"); rpcErr != nil {
			return 0, 0, rpcErr
		}
		if pageSize < 1 || pageSize > maxPageSize {
			return 0, 0, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("pageSize must be 1 to %d", maxPageSize)}
		}
	}
	return pageNo, pageSize, nil
}

func parseFragment(fields map[string]json.RawMessage) (string, *mcpRPCError) {
	raw, ok := fields["fragment"]
	if !ok {
		return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: "fragment is required"}
	}
	s, rpcErr := parseStrictString(raw, "fragment")
	if rpcErr != nil {
		return "", rpcErr
	}
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < 1 || n > maxFragmentLen {
		return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("fragment must be 1 to %d characters", maxFragmentLen)}
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: "fragment contains control characters"}
		}
	}
	return s, nil
}

// --- Shared helpers (from mcp-kodeverk) ---

func (s *server) execWithOBO(ctx context.Context, inboundToken string, fn func(oboToken string) (string, error)) map[string]any {
	if s.upstreamSem != nil {
		select {
		case s.upstreamSem <- struct{}{}:
			defer func() { <-s.upstreamSem }()
		case <-ctx.Done():
			return toolErrorResult(ctx.Err().Error())
		}
	}
	oboToken, err := s.exchangeToken(ctx, inboundToken)
	if err != nil {
		return toolErrorResult(err.Error())
	}
	text, err := fn(oboToken)
	if err != nil {
		return toolErrorResult(err.Error())
	}
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": text}},
	}
}

func toolErrorResult(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": msg}},
		"isError": true,
	}
}

// --- Argument parsing & validation ---

func decodeArgMap(raw json.RawMessage, allowed map[string]bool) (map[string]json.RawMessage, *mcpRPCError) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]json.RawMessage{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	tok, err := dec.Token()
	if err != nil {
		return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "arguments must be a JSON object"}
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "arguments must be a JSON object"}
	}
	fields := make(map[string]json.RawMessage)
	for dec.More() {
		kTok, keyErr := dec.Token()
		if keyErr != nil {
			return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "invalid arguments object"}
		}
		key, keyOK := kTok.(string)
		if !keyOK {
			return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "invalid argument key"}
		}
		if !allowed[key] {
			return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("unknown argument %q", key)}
		}
		if _, dup := fields[key]; dup {
			return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("duplicate argument %q", key)}
		}
		var val json.RawMessage
		if valErr := dec.Decode(&val); valErr != nil {
			return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("invalid value for %q", key)}
		}
		fields[key] = bytes.TrimSpace(val)
	}
	endTok, endErr := dec.Token()
	if endErr != nil || endTok != json.Delim('}') {
		return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "invalid arguments object"}
	}
	if _, eofErr := dec.Token(); eofErr != io.EOF {
		return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "trailing data in arguments"}
	}
	return fields, nil
}

func parseStrictString(raw json.RawMessage, field string) (string, *mcpRPCError) {
	if len(raw) == 0 || raw[0] != '"' {
		return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be a string", field)}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be a string", field)}
	}
	return s, nil
}

func parseStrictInt(raw json.RawMessage, field string) (int, *mcpRPCError) {
	if len(raw) == 0 || raw[0] == '"' || bytes.ContainsAny(raw, ".eE") {
		return 0, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be an integer", field)}
	}
	v, err := strconv.Atoi(string(raw))
	if err != nil {
		return 0, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be an integer", field)}
	}
	return v, nil
}
