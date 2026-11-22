package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maxToolResultBytes = 32 * 1024
	maxFilterLen       = 64
	maxKodeLen         = 128
)

var (
	namePattern   = regexp.MustCompile(`^[A-Za-z0-9æøåÆØÅ_-]{1,100}$`)
	spraakPattern = regexp.MustCompile(`^[a-z]{2}$`)
)

// readOnlyAnnotations are client hints, never a security boundary.
var readOnlyAnnotations = map[string]bool{
	"readOnlyHint":    true,
	"destructiveHint": false,
	"idempotentHint":  true,
	"openWorldHint":   false,
}

type fieldSpec struct {
	name, typ string
	required  bool
}

// toolOutputs lists each tool's tool-specific structuredContent fields and
// whether the tool pages with offset. toolDefinitions builds outputSchema from it.
var toolOutputs = map[string]struct {
	hasOffset bool
	fields    []fieldSpec
}{
	"list_kodeverk": {true, []fieldSpec{
		{"kodeverksnavn", "array", true},
	}},
	"get_kodeverk_koder": {true, []fieldSpec{
		{"kodeverksnavn", "string", true},
		{"koder", "array", true},
	}},
	"get_kodeverk_betydninger": {true, []fieldSpec{
		{"kodeverksnavn", "string", true},
		{"betydninger", "object", true},
	}},
	"list_hierarkier": {true, []fieldSpec{
		{"hierarkinavn", "array", true},
	}},
	"get_hierarki_noder": {false, []fieldSpec{
		{"hierarkinavn", "string", true},
		{"hierarkinivaaer", "array", true},
		{"noder", "object", true},
	}},
	"search_kodeverk": {false, []fieldSpec{
		{"query", "string", true},
		{"kodeverksnavn", "string", false},
		{"kodeverk", "array", false},
		{"matches", "array", false},
	}},
}

func toolDefinitions() []map[string]any {
	defs := baseToolDefinitions()
	for _, d := range defs {
		o := toolOutputs[d["name"].(string)]
		props := map[string]any{
			"total":     map[string]any{"type": "integer"},
			"returned":  map[string]any{"type": "integer"},
			"truncated": map[string]any{"type": "boolean"},
		}
		required := []string{"total", "returned"}
		if o.hasOffset {
			props["offset"] = map[string]any{"type": "integer"}
			props["nextOffset"] = map[string]any{"type": "integer", "description": "Offset for the next page; absent on the last page."}
			required = append(required, "offset")
		}
		for _, f := range o.fields {
			props[f.name] = map[string]any{"type": f.typ}
			if f.required {
				required = append(required, f.name)
			}
		}
		d["outputSchema"] = map[string]any{"type": "object", "properties": props, "required": required}
	}
	return defs
}

func baseToolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "list_kodeverk",
			"description": "List published code sets (kodeverk) in Nav's Felles kodeverk with optional substring filtering and pagination. Example: {\"filter\": \"post\"} finds Postnummer.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"description": "Optional case-insensitive substring filter on kodeverk name.",
						"maxLength":   maxFilterLen,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of items to return (default 50, max 200).",
						"default":     50,
						"minimum":     1,
						"maximum":     200,
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of matching items to skip (default 0).",
						"default":     0,
						"minimum":     0,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "get_kodeverk_koder",
			"description": "List published code keys for a specific kodeverk with optional substring filtering and pagination. Example: {\"kodeverksnavn\": \"Landkoder\", \"filter\": \"NO\"}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"kodeverksnavn"},
				"properties": map[string]any{
					"kodeverksnavn": map[string]any{
						"type":        "string",
						"description": "Name of the kodeverk.",
						"pattern":     `^[A-Za-z0-9æøåÆØÅ_-]{1,100}$`,
					},
					"filter": map[string]any{
						"type":        "string",
						"description": "Optional case-insensitive substring filter on code value.",
						"maxLength":   maxFilterLen,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of codes to return (default 50, max 200).",
						"default":     50,
						"minimum":     1,
						"maximum":     200,
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of matching codes to skip (default 0).",
						"default":     0,
						"minimum":     0,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "get_kodeverk_betydninger",
			"description": "Get code meanings (betydninger) with validity dates and language descriptions for a kodeverk, paginated over sorted codes. Example: {\"kodeverksnavn\": \"Postnummer\", \"kode\": \"0150\"} returns the poststed for 0150.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"kodeverksnavn"},
				"properties": map[string]any{
					"kodeverksnavn": map[string]any{
						"type":        "string",
						"description": "Name of the kodeverk.",
						"pattern":     `^[A-Za-z0-9æøåÆØÅ_-]{1,100}$`,
					},
					"kode": map[string]any{
						"type":        "string",
						"description": "Optional exact code filter.",
						"maxLength":   maxKodeLen,
					},
					"ekskluderUgyldige": map[string]any{
						"type":        "boolean",
						"description": "Exclude meanings invalid on oppslagsdato (default true).",
						"default":     true,
					},
					"oppslagsdato": map[string]any{
						"type":        "string",
						"description": "Optional lookup date (YYYY-MM-DD) for validity filtering.",
					},
					"spraak": map[string]any{
						"type":        "array",
						"description": "Optional ISO 639-1 two-letter language codes (max 5).",
						"maxItems":    5,
						"items": map[string]any{
							"type":    "string",
							"pattern": `^[a-z]{2}$`,
						},
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of codes to return (default 50, max 100).",
						"default":     50,
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of sorted codes to skip (default 0).",
						"default":     0,
						"minimum":     0,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "list_hierarkier",
			"description": "List published hierarchies in Nav's Felles kodeverk with optional substring filtering and pagination. Example: {\"filter\": \"geografi\"}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"description": "Optional case-insensitive substring filter on hierarchy name.",
						"maxLength":   maxFilterLen,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of items to return (default 50, max 200).",
						"default":     50,
						"minimum":     1,
						"maximum":     200,
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of matching items to skip (default 0).",
						"default":     0,
						"minimum":     0,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "get_hierarki_noder",
			"description": "Get the node tree for a hierarchy, bounded by depth and root node limit. Example: {\"hierarkinavn\": \"Geografi\", \"maxDepth\": 2}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"hierarkinavn"},
				"properties": map[string]any{
					"hierarkinavn": map[string]any{
						"type":        "string",
						"description": "Name of the hierarchy.",
						"pattern":     `^[A-Za-z0-9æøåÆØÅ_-]{1,100}$`,
					},
					"spraak": map[string]any{
						"type":        "array",
						"description": "Optional ISO 639-1 two-letter language codes (max 5).",
						"maxItems":    5,
						"items": map[string]any{
							"type":    "string",
							"pattern": `^[a-z]{2}$`,
						},
					},
					"maxDepth": map[string]any{
						"type":        "integer",
						"description": "Maximum tree depth starting at 1 for root nodes (default 3, max 10).",
						"default":     3,
						"minimum":     1,
						"maximum":     10,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of root nodes to return (default 50, max 100).",
						"default":     50,
						"minimum":     1,
						"maximum":     100,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "search_kodeverk",
			"description": "Search kodeverk names or, when kodeverksnavn is specified, search codes and Norwegian Bokmål (nb) terms within that kodeverk. Example: {\"query\": \"Oslo\", \"kodeverksnavn\": \"Kommuner\"}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search substring (2-64 characters; letters, digits, space, '-', '_').",
						"minLength":   2,
						"maxLength":   64,
					},
					"kodeverksnavn": map[string]any{
						"type":        "string",
						"description": "Optional kodeverk name to search codes and terms inside. When omitted, searches kodeverk names.",
						"pattern":     `^[A-Za-z0-9æøåÆØÅ_-]{1,100}$`,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of matches to return (default 25, max 50).",
						"default":     25,
						"minimum":     1,
						"maximum":     50,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
	}
}

func (s *server) handleToolCall(ctx context.Context, inboundToken, toolName string, rawArgs json.RawMessage) (any, *mcpWireError) {
	switch toolName {
	case "list_kodeverk":
		args, rpcErr := parseListArgs(rawArgs, false)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			names, err := s.kodeverk.listKodeverk(ctx, oboToken)
			if err != nil {
				return "", err
			}
			return buildListKodeverkResult(names, args.filter, args.limit, args.offset)
		}), nil

	case "get_kodeverk_koder":
		args, rpcErr := parseListArgs(rawArgs, true)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			koder, err := s.kodeverk.getKodeverkKoder(ctx, oboToken, args.navn)
			if err != nil {
				return "", err
			}
			return buildGetKodeverkKoderResult(args.navn, koder, args.filter, args.limit, args.offset)
		}), nil

	case "get_kodeverk_betydninger":
		args, rpcErr := parseBetydningerArgs(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			betydninger, err := s.kodeverk.getKodeverkBetydninger(
				ctx, oboToken, args.kodeverksnavn, args.ekskluderUgyldige, args.oppslagsdato, args.spraak,
			)
			if err != nil {
				return "", err
			}
			return buildBetydningerResult(args.kodeverksnavn, args.kode, args.hasKode, args.limit, args.offset, betydninger)
		}), nil

	case "list_hierarkier":
		args, rpcErr := parseListArgs(rawArgs, false)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			names, err := s.kodeverk.listHierarkier(ctx, oboToken)
			if err != nil {
				return "", err
			}
			return buildListHierarkierResult(names, args.filter, args.limit, args.offset)
		}), nil

	case "get_hierarki_noder":
		args, rpcErr := parseHierarkiNoderArgs(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			resp, err := s.kodeverk.getHierarkiNoder(ctx, oboToken, args.hierarkinavn, args.spraak)
			if err != nil {
				return "", err
			}
			return buildHierarkiNoderResult(args.hierarkinavn, args.maxDepth, args.limit, resp)
		}), nil

	case "search_kodeverk":
		args, rpcErr := parseSearchArgs(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			if args.kodeverksnavn == "" {
				names, err := s.kodeverk.listKodeverk(ctx, oboToken)
				if err != nil {
					return "", err
				}
				return buildSearchKodeverkNamesResult(args.query, args.limit, names)
			}
			betydninger, err := s.kodeverk.getKodeverkBetydninger(ctx, oboToken, args.kodeverksnavn, true, "", []string{"nb"})
			if err != nil {
				return "", err
			}
			return buildSearchKodeverkCodesResult(args.query, args.kodeverksnavn, args.limit, betydninger)
		}), nil

	default:
		return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: "unknown tool"}
	}
}

func (s *server) execWithOBO(ctx context.Context, inboundToken string, fn func(oboToken string) (string, error)) map[string]any {
	if s.upstreamSem != nil {
		select {
		case s.upstreamSem <- struct{}{}:
			defer func() { <-s.upstreamSem }()
		case <-ctx.Done():
			return toolErrorResult(ctx.Err().Error())
		}
	}
	oboToken, err := s.texas().Exchange(ctx, inboundToken, s.cfg.KodeverkScope)
	if err != nil {
		return toolErrorResult(err.Error())
	}
	text, err := fn(oboToken)
	if err != nil {
		return toolErrorResult(err.Error())
	}
	return toolResult(text)
}

// toolResult returns the JSON text as structuredContent and as a text block
// for clients older than 2026-07-28, plus a hint when more items exist.
func toolResult(text string) map[string]any {
	var structured map[string]any
	if err := json.Unmarshal([]byte(text), &structured); err != nil {
		return map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
	}
	content := []map[string]string{{"type": "text", "text": text}}
	total, _ := structured["total"].(float64)
	returned, _ := structured["returned"].(float64)
	truncated, _ := structured["truncated"].(bool)
	offset, hasOffset := structured["offset"].(float64)
	if hasOffset && int(returned) > 0 {
		if rest := int(total) - int(offset) - int(returned); rest > 0 {
			next := int(offset) + int(returned)
			structured["nextOffset"] = next
			narrow := "filter"
			if _, ok := structured["betydninger"]; ok {
				narrow = "kode"
			}
			content = append(content, map[string]string{"type": "text", "text": fmt.Sprintf(
				"%d more items. Next page: call again with offset=%d, or narrow the request (%s).", rest, next, narrow)})
		}
	} else if truncated || (!hasOffset && total > returned) {
		content = append(content, map[string]string{"type": "text", "text": truncationHint(structured, int(returned), int(total))})
	}
	return map[string]any{"content": content, "structuredContent": structured}
}

func truncationHint(structured map[string]any, returned, total int) string {
	if _, ok := structured["noder"]; ok {
		if returned == total {
			return fmt.Sprintf("%d of %d root nodes shown; deeper levels omitted. Increase maxDepth or narrow spraak to see deeper nodes.", returned, total)
		}
		return fmt.Sprintf("%d of %d root nodes shown. Lower maxDepth, narrow spraak, or increase limit to see more root nodes.", returned, total)
	}
	if _, ok := structured["query"]; ok {
		return fmt.Sprintf("%d of %d shown. Narrow query or kodeverksnavn to see the rest.", returned, total)
	}
	if _, ok := structured["betydninger"]; ok {
		return fmt.Sprintf("%d of %d shown: single item exceeds the 32 KiB limit. Narrow with kode, spraak, or oppslagsdato, or advance offset.", returned, total)
	}
	return fmt.Sprintf("%d of %d shown: single item exceeds the 32 KiB limit. Narrow with filter or advance offset.", returned, total)
}

func toolErrorResult(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": msg}},
		"isError": true,
	}
}

// --- Argument parsing & validation ---

func decodeArgMap(raw json.RawMessage, allowed map[string]bool) (map[string]json.RawMessage, *mcpWireError) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]json.RawMessage{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	tok, err := dec.Token()
	if err != nil {
		return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: "arguments must be a JSON object"}
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: "arguments must be a JSON object"}
	}
	fields := make(map[string]json.RawMessage)
	for dec.More() {
		kTok, keyErr := dec.Token()
		if keyErr != nil {
			return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: "invalid arguments object"}
		}
		key, keyOK := kTok.(string)
		if !keyOK {
			return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: "invalid argument key"}
		}
		if !allowed[key] {
			return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("unknown argument %q", key)}
		}
		if _, dup := fields[key]; dup {
			return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("duplicate argument %q", key)}
		}
		var val json.RawMessage
		if valErr := dec.Decode(&val); valErr != nil {
			return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("invalid value for %q", key)}
		}
		fields[key] = bytes.TrimSpace(val)
	}
	endTok, endErr := dec.Token()
	if endErr != nil || endTok != json.Delim('}') {
		return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: "invalid arguments object"}
	}
	if _, eofErr := dec.Token(); eofErr != io.EOF {
		return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: "trailing data in arguments"}
	}
	return fields, nil
}

func parseStrictString(raw json.RawMessage, field string) (string, *mcpWireError) {
	if len(raw) == 0 || raw[0] != '"' {
		return "", &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be a string", field)}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be a string", field)}
	}
	return s, nil
}

func parseStrictInt(raw json.RawMessage, field string) (int, *mcpWireError) {
	if len(raw) == 0 || raw[0] == '"' || bytes.ContainsAny(raw, ".eE") {
		return 0, &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be an integer", field)}
	}
	v, err := strconv.Atoi(string(raw))
	if err != nil {
		return 0, &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be an integer", field)}
	}
	return v, nil
}

func parseStrictBool(raw json.RawMessage, field string) (bool, *mcpWireError) {
	if bytes.Equal(raw, []byte("true")) {
		return true, nil
	}
	if bytes.Equal(raw, []byte("false")) {
		return false, nil
	}
	return false, &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be a boolean", field)}
}

func parseStrictStringSlice(raw json.RawMessage, field string) ([]string, *mcpWireError) {
	if len(raw) == 0 || raw[0] != '[' {
		return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be an array of strings", field)}
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return nil, &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be an array of strings", field)}
	}
	out := make([]string, 0, len(elems))
	for _, elem := range elems {
		s, rpcErr := parseStrictString(bytes.TrimSpace(elem), field+" item")
		if rpcErr != nil {
			return nil, rpcErr
		}
		out = append(out, s)
	}
	return out, nil
}

func validateName(name, field string) *mcpWireError {
	if !namePattern.MatchString(name) {
		return &mcpWireError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must match ^[A-Za-z0-9æøåÆØÅ_-]{1,100}$", field)}
	}
	return nil
}

func validateSpraak(spraak []string) *mcpWireError {
	if len(spraak) > 5 {
		return &mcpWireError{Code: mcpCodeInvalidParams, Message: "spraak allows at most 5 entries"}
	}
	for _, s := range spraak {
		if !spraakPattern.MatchString(s) {
			return &mcpWireError{Code: mcpCodeInvalidParams, Message: "each spraak entry must match ^[a-z]{2}$"}
		}
	}
	return nil
}

func validateSearchQuery(q string) *mcpWireError {
	rc := utf8.RuneCountInString(q)
	if rc < 2 || rc > 64 {
		return &mcpWireError{Code: mcpCodeInvalidParams, Message: "query must be 2 to 64 characters"}
	}
	if utf8.RuneCountInString(strings.TrimSpace(q)) < 2 {
		return &mcpWireError{Code: mcpCodeInvalidParams, Message: "query must contain at least 2 non-whitespace characters"}
	}
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' {
			continue
		}
		return &mcpWireError{Code: mcpCodeInvalidParams, Message: "query contains disallowed characters"}
	}
	return nil
}

type listArgs struct {
	navn   string
	filter string
	limit  int
	offset int
}

func parseListArgs(raw json.RawMessage, requireKodeverksnavn bool) (listArgs, *mcpWireError) {
	allowed := map[string]bool{"filter": true, "limit": true, "offset": true}
	if requireKodeverksnavn {
		allowed["kodeverksnavn"] = true
	}
	fields, rpcErr := decodeArgMap(raw, allowed)
	if rpcErr != nil {
		return listArgs{}, rpcErr
	}
	out := listArgs{limit: 50, offset: 0}
	if requireKodeverksnavn {
		v, ok := fields["kodeverksnavn"]
		if !ok {
			return listArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "kodeverksnavn is required"}
		}
		s, err := parseStrictString(v, "kodeverksnavn")
		if err != nil {
			return listArgs{}, err
		}
		if err := validateName(s, "kodeverksnavn"); err != nil {
			return listArgs{}, err
		}
		out.navn = s
	}
	if v, ok := fields["filter"]; ok {
		s, err := parseStrictString(v, "filter")
		if err != nil {
			return listArgs{}, err
		}
		if utf8.RuneCountInString(s) > maxFilterLen {
			return listArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "filter must be at most 64 characters"}
		}
		out.filter = s
	}
	if v, ok := fields["limit"]; ok {
		n, err := parseStrictInt(v, "limit")
		if err != nil {
			return listArgs{}, err
		}
		if n < 1 || n > 200 {
			return listArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "limit must be between 1 and 200"}
		}
		out.limit = n
	}
	if v, ok := fields["offset"]; ok {
		n, err := parseStrictInt(v, "offset")
		if err != nil {
			return listArgs{}, err
		}
		if n < 0 {
			return listArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "offset must be >= 0"}
		}
		out.offset = n
	}
	return out, nil
}

type betydningerArgs struct {
	kodeverksnavn     string
	kode              string
	hasKode           bool
	ekskluderUgyldige bool
	oppslagsdato      string
	spraak            []string
	limit             int
	offset            int
}

func parseBetydningerArgs(raw json.RawMessage) (betydningerArgs, *mcpWireError) {
	allowed := map[string]bool{
		"kodeverksnavn":     true,
		"kode":              true,
		"ekskluderUgyldige": true,
		"oppslagsdato":      true,
		"spraak":            true,
		"limit":             true,
		"offset":            true,
	}
	fields, rpcErr := decodeArgMap(raw, allowed)
	if rpcErr != nil {
		return betydningerArgs{}, rpcErr
	}
	out := betydningerArgs{
		ekskluderUgyldige: true,
		limit:             50,
		offset:            0,
	}
	kvRaw, ok := fields["kodeverksnavn"]
	if !ok {
		return betydningerArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "kodeverksnavn is required"}
	}
	kv, err := parseStrictString(kvRaw, "kodeverksnavn")
	if err != nil {
		return betydningerArgs{}, err
	}
	if err := validateName(kv, "kodeverksnavn"); err != nil {
		return betydningerArgs{}, err
	}
	out.kodeverksnavn = kv

	if v, ok := fields["kode"]; ok {
		s, err := parseStrictString(v, "kode")
		if err != nil {
			return betydningerArgs{}, err
		}
		if s == "" {
			return betydningerArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "kode must not be empty"}
		}
		if utf8.RuneCountInString(s) > maxKodeLen {
			return betydningerArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "kode must be at most 128 characters"}
		}
		out.kode = s
		out.hasKode = true
	}
	if v, ok := fields["ekskluderUgyldige"]; ok {
		b, err := parseStrictBool(v, "ekskluderUgyldige")
		if err != nil {
			return betydningerArgs{}, err
		}
		out.ekskluderUgyldige = b
	}
	if v, ok := fields["oppslagsdato"]; ok {
		s, err := parseStrictString(v, "oppslagsdato")
		if err != nil {
			return betydningerArgs{}, err
		}
		if _, err := time.Parse(time.DateOnly, s); err != nil {
			return betydningerArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "oppslagsdato must be YYYY-MM-DD"}
		}
		out.oppslagsdato = s
	}
	if v, ok := fields["spraak"]; ok {
		sp, err := parseStrictStringSlice(v, "spraak")
		if err != nil {
			return betydningerArgs{}, err
		}
		if err := validateSpraak(sp); err != nil {
			return betydningerArgs{}, err
		}
		out.spraak = sp
	}
	if v, ok := fields["limit"]; ok {
		n, err := parseStrictInt(v, "limit")
		if err != nil {
			return betydningerArgs{}, err
		}
		if n < 1 || n > 100 {
			return betydningerArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "limit must be between 1 and 100"}
		}
		out.limit = n
	}
	if v, ok := fields["offset"]; ok {
		n, err := parseStrictInt(v, "offset")
		if err != nil {
			return betydningerArgs{}, err
		}
		if n < 0 {
			return betydningerArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "offset must be >= 0"}
		}
		out.offset = n
	}
	return out, nil
}

type hierarkiNoderArgs struct {
	hierarkinavn string
	spraak       []string
	maxDepth     int
	limit        int
}

func parseHierarkiNoderArgs(raw json.RawMessage) (hierarkiNoderArgs, *mcpWireError) {
	allowed := map[string]bool{
		"hierarkinavn": true,
		"spraak":       true,
		"maxDepth":     true,
		"limit":        true,
	}
	fields, rpcErr := decodeArgMap(raw, allowed)
	if rpcErr != nil {
		return hierarkiNoderArgs{}, rpcErr
	}
	out := hierarkiNoderArgs{maxDepth: 3, limit: 50}
	hRaw, ok := fields["hierarkinavn"]
	if !ok {
		return hierarkiNoderArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "hierarkinavn is required"}
	}
	h, err := parseStrictString(hRaw, "hierarkinavn")
	if err != nil {
		return hierarkiNoderArgs{}, err
	}
	if err := validateName(h, "hierarkinavn"); err != nil {
		return hierarkiNoderArgs{}, err
	}
	out.hierarkinavn = h

	if v, ok := fields["spraak"]; ok {
		sp, err := parseStrictStringSlice(v, "spraak")
		if err != nil {
			return hierarkiNoderArgs{}, err
		}
		if err := validateSpraak(sp); err != nil {
			return hierarkiNoderArgs{}, err
		}
		out.spraak = sp
	}
	if v, ok := fields["maxDepth"]; ok {
		n, err := parseStrictInt(v, "maxDepth")
		if err != nil {
			return hierarkiNoderArgs{}, err
		}
		if n < 1 || n > 10 {
			return hierarkiNoderArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "maxDepth must be between 1 and 10"}
		}
		out.maxDepth = n
	}
	if v, ok := fields["limit"]; ok {
		n, err := parseStrictInt(v, "limit")
		if err != nil {
			return hierarkiNoderArgs{}, err
		}
		if n < 1 || n > 100 {
			return hierarkiNoderArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "limit must be between 1 and 100"}
		}
		out.limit = n
	}
	return out, nil
}

type searchArgs struct {
	query         string
	kodeverksnavn string
	limit         int
}

func parseSearchArgs(raw json.RawMessage) (searchArgs, *mcpWireError) {
	allowed := map[string]bool{
		"query":         true,
		"kodeverksnavn": true,
		"limit":         true,
	}
	fields, rpcErr := decodeArgMap(raw, allowed)
	if rpcErr != nil {
		return searchArgs{}, rpcErr
	}
	out := searchArgs{limit: 25}
	qRaw, ok := fields["query"]
	if !ok {
		return searchArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "query is required"}
	}
	q, err := parseStrictString(qRaw, "query")
	if err != nil {
		return searchArgs{}, err
	}
	if err := validateSearchQuery(q); err != nil {
		return searchArgs{}, err
	}
	out.query = q

	if v, ok := fields["kodeverksnavn"]; ok {
		kv, err := parseStrictString(v, "kodeverksnavn")
		if err != nil {
			return searchArgs{}, err
		}
		if err := validateName(kv, "kodeverksnavn"); err != nil {
			return searchArgs{}, err
		}
		out.kodeverksnavn = kv
	}
	if v, ok := fields["limit"]; ok {
		n, err := parseStrictInt(v, "limit")
		if err != nil {
			return searchArgs{}, err
		}
		if n < 1 || n > 50 {
			return searchArgs{}, &mcpWireError{Code: mcpCodeInvalidParams, Message: "limit must be between 1 and 50"}
		}
		out.limit = n
	}
	return out, nil
}

// --- Filtering, pagination, and 32 KiB size capping ---

func filterAndPaginateStrings(items []string, filter string, limit, offset int) (page []string, total int) {
	sorted := make([]string, len(items))
	copy(sorted, items)
	slices.Sort(sorted)

	var filtered []string
	if filter == "" {
		filtered = sorted
	} else {
		lower := strings.ToLower(filter)
		for _, item := range sorted {
			if strings.Contains(strings.ToLower(item), lower) {
				filtered = append(filtered, item)
			}
		}
	}
	total = len(filtered)
	if offset >= total {
		return []string{}, total
	}
	end := min(offset+limit, total)
	page = filtered[offset:end]
	if page == nil {
		page = []string{}
	}
	return page, total
}

// fitCappedJSON serializes render(n, truncated) within maxToolResultBytes by
// binary-searching the largest item count n <= totalItems if the full payload exceeds the cap.
func fitCappedJSON(totalItems int, alreadyTruncated bool, render func(n int, truncated bool) any) (string, error) {
	full := render(totalItems, alreadyTruncated)
	b, err := json.Marshal(full)
	if err != nil {
		return "", err
	}
	if len(b) <= maxToolResultBytes {
		return string(b), nil
	}

	// Binary search for the largest k in [0, totalItems-1] with truncated=true that fits.
	lo, hi := 0, max(totalItems-1, 0)
	bestN := 0
	var bestBytes []byte
	for lo <= hi {
		mid := lo + (hi-lo)/2
		candidate, candErr := json.Marshal(render(mid, true))
		if candErr != nil {
			return "", candErr
		}
		if len(candidate) <= maxToolResultBytes {
			bestN = mid
			bestBytes = candidate
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if bestBytes != nil {
		return string(bestBytes), nil
	}
	// Fallback: zero items with truncated=true.
	_ = bestN
	fallback, err := json.Marshal(render(0, true))
	if err != nil {
		return "", err
	}
	if len(fallback) > maxToolResultBytes {
		return "", errors.New("tool response exceeds 32 KiB limit")
	}
	return string(fallback), nil
}

func nextPageOffset(offset, returned, total int) *int {
	if returned > 0 && total-offset-returned > 0 {
		next := offset + returned
		return &next
	}
	return nil
}

func buildListKodeverkResult(names []string, filter string, limit, offset int) (string, error) {
	page, total := filterAndPaginateStrings(names, filter, limit, offset)
	return fitCappedJSON(len(page), false, func(n int, truncated bool) any {
		return struct {
			Total         int      `json:"total"`
			Returned      int      `json:"returned"`
			Offset        int      `json:"offset"`
			NextOffset    *int     `json:"nextOffset,omitempty"`
			Truncated     bool     `json:"truncated,omitempty"`
			Kodeverksnavn []string `json:"kodeverksnavn"`
		}{
			Total:         total,
			Returned:      n,
			Offset:        offset,
			NextOffset:    nextPageOffset(offset, n, total),
			Truncated:     truncated,
			Kodeverksnavn: page[:n],
		}
	})
}

func buildGetKodeverkKoderResult(navn string, koder []string, filter string, limit, offset int) (string, error) {
	page, total := filterAndPaginateStrings(koder, filter, limit, offset)
	return fitCappedJSON(len(page), false, func(n int, truncated bool) any {
		return struct {
			Kodeverksnavn string   `json:"kodeverksnavn"`
			Total         int      `json:"total"`
			Returned      int      `json:"returned"`
			Offset        int      `json:"offset"`
			NextOffset    *int     `json:"nextOffset,omitempty"`
			Truncated     bool     `json:"truncated,omitempty"`
			Koder         []string `json:"koder"`
		}{
			Kodeverksnavn: navn,
			Total:         total,
			Returned:      n,
			Offset:        offset,
			NextOffset:    nextPageOffset(offset, n, total),
			Truncated:     truncated,
			Koder:         page[:n],
		}
	})
}

func buildListHierarkierResult(names []string, filter string, limit, offset int) (string, error) {
	page, total := filterAndPaginateStrings(names, filter, limit, offset)
	return fitCappedJSON(len(page), false, func(n int, truncated bool) any {
		return struct {
			Total        int      `json:"total"`
			Returned     int      `json:"returned"`
			Offset       int      `json:"offset"`
			NextOffset   *int     `json:"nextOffset,omitempty"`
			Truncated    bool     `json:"truncated,omitempty"`
			Hierarkinavn []string `json:"hierarkinavn"`
		}{
			Total:        total,
			Returned:     n,
			Offset:       offset,
			NextOffset:   nextPageOffset(offset, n, total),
			Truncated:    truncated,
			Hierarkinavn: page[:n],
		}
	})
}

func buildBetydningerResult(navn, kode string, hasKode bool, limit, offset int, all map[string][]betydning) (string, error) {
	keys := make([]string, 0, len(all))
	for k := range all {
		if hasKode && k != kode {
			continue
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)

	total := len(keys)
	var pageKeys []string
	if offset < total {
		end := min(offset+limit, total)
		pageKeys = keys[offset:end]
	}

	return fitCappedJSON(len(pageKeys), false, func(n int, truncated bool) any {
		subset := make(map[string][]betydning, n)
		for _, k := range pageKeys[:n] {
			subset[k] = all[k]
		}
		return struct {
			Kodeverksnavn string                 `json:"kodeverksnavn"`
			Total         int                    `json:"total"`
			Returned      int                    `json:"returned"`
			Offset        int                    `json:"offset"`
			NextOffset    *int                   `json:"nextOffset,omitempty"`
			Truncated     bool                   `json:"truncated,omitempty"`
			Betydninger   map[string][]betydning `json:"betydninger"`
		}{
			Kodeverksnavn: navn,
			Total:         total,
			Returned:      n,
			Offset:        offset,
			NextOffset:    nextPageOffset(offset, n, total),
			Truncated:     truncated,
			Betydninger:   subset,
		}
	})
}

func pruneHierarkiNode(node hierarkinode, depth, maxDepth int) (hierarkinode, bool) {
	out := hierarkinode{
		Kode:    node.Kode,
		Termer:  node.Termer,
		Tekster: node.Tekster,
	}
	if len(node.Undernoder) == 0 {
		return out, false
	}
	if depth >= maxDepth {
		return out, true
	}
	cut := false
	out.Undernoder = make(map[string]hierarkinode, len(node.Undernoder))
	for k, child := range node.Undernoder {
		prunedChild, childCut := pruneHierarkiNode(child, depth+1, maxDepth)
		out.Undernoder[k] = prunedChild
		if childCut {
			cut = true
		}
	}
	return out, cut
}

func buildHierarkiNoderResult(navn string, maxDepth, limit int, resp hierarkiNoderResponse) (string, error) {
	rootKeys := make([]string, 0, len(resp.Noder))
	for k := range resp.Noder {
		rootKeys = append(rootKeys, k)
	}
	slices.Sort(rootKeys)

	total := len(rootKeys)
	truncated := false
	if len(rootKeys) > limit {
		rootKeys = rootKeys[:limit]
		truncated = true
	}

	nivaaer := resp.Hierarkinivaaer
	if nivaaer == nil {
		nivaaer = []string{}
	}

	renderWithDepth := func(depth, n int, trunc bool) any {
		nodes := make(map[string]hierarkinode, n)
		for _, k := range rootKeys[:n] {
			pruned, _ := pruneHierarkiNode(resp.Noder[k], 1, depth)
			nodes[k] = pruned
		}
		return struct {
			Hierarkinavn    string                  `json:"hierarkinavn"`
			Hierarkinivaaer []string                `json:"hierarkinivaaer"`
			Total           int                     `json:"total"`
			Returned        int                     `json:"returned"`
			Truncated       bool                    `json:"truncated,omitempty"`
			Noder           map[string]hierarkinode `json:"noder"`
		}{
			Hierarkinavn:    navn,
			Hierarkinivaaer: nivaaer,
			Total:           total,
			Returned:        n,
			Truncated:       trunc,
			Noder:           nodes,
		}
	}

	depthCut := false
	for _, k := range rootKeys {
		if _, cut := pruneHierarkiNode(resp.Noder[k], 1, maxDepth); cut {
			depthCut = true
			break
		}
	}
	if depthCut {
		truncated = true
	}

	// First try full maxDepth with fitCappedJSON; if even 1 root node at maxDepth
	// exceeds 32 KiB, try reducing depth and fitting as many roots as possible.
	text, err := fitCappedJSON(len(rootKeys), truncated, func(n int, trunc bool) any {
		return renderWithDepth(maxDepth, n, trunc)
	})
	if err != nil {
		return "", err
	}
	if len(rootKeys) > 0 {
		var check struct {
			Returned int `json:"returned"`
		}
		if json.Unmarshal([]byte(text), &check) == nil && check.Returned == 0 {
			for d := maxDepth - 1; d >= 1; d-- {
				cand, candErr := fitCappedJSON(len(rootKeys), true, func(n int, trunc bool) any {
					return renderWithDepth(d, n, trunc)
				})
				if candErr != nil {
					return "", candErr
				}
				var candCheck struct {
					Returned int `json:"returned"`
				}
				if json.Unmarshal([]byte(cand), &candCheck) == nil && candCheck.Returned > 0 {
					return cand, nil
				}
			}
		}
	}
	return text, nil
}

type searchCodeMatch struct {
	Kode      string `json:"kode"`
	Term      string `json:"term,omitempty"`
	Tekst     string `json:"tekst,omitempty"`
	GyldigFra string `json:"gyldigFra,omitempty"`
	GyldigTil string `json:"gyldigTil,omitempty"`
}

func buildSearchKodeverkNamesResult(query string, limit int, names []string) (string, error) {
	sorted := make([]string, len(names))
	copy(sorted, names)
	slices.Sort(sorted)

	lower := strings.ToLower(query)
	matches := make([]string, 0)
	for _, name := range sorted {
		if strings.Contains(strings.ToLower(name), lower) {
			matches = append(matches, name)
		}
	}
	total := len(matches)
	truncated := false
	if len(matches) > limit {
		matches = matches[:limit]
		truncated = true
	}

	return fitCappedJSON(len(matches), truncated, func(n int, trunc bool) any {
		return struct {
			Query     string   `json:"query"`
			Total     int      `json:"total"`
			Returned  int      `json:"returned"`
			Truncated bool     `json:"truncated,omitempty"`
			Kodeverk  []string `json:"kodeverk"`
		}{
			Query:     query,
			Total:     total,
			Returned:  n,
			Truncated: trunc,
			Kodeverk:  matches[:n],
		}
	})
}

func pickBeskrivelse(beskrivelser map[string]beskrivelse) beskrivelse {
	if len(beskrivelser) == 0 {
		return beskrivelse{}
	}
	for _, lang := range []string{"nb", "nn", "no", "en"} {
		if d, ok := beskrivelser[lang]; ok {
			return d
		}
	}
	first := true
	var bestKey string
	for k := range beskrivelser {
		if first || k < bestKey {
			bestKey = k
			first = false
		}
	}
	return beskrivelser[bestKey]
}

func buildSearchKodeverkCodesResult(query, kodeverksnavn string, limit int, betydninger map[string][]betydning) (string, error) {
	codes := make([]string, 0, len(betydninger))
	for k := range betydninger {
		codes = append(codes, k)
	}
	sort.Strings(codes)

	lower := strings.ToLower(query)
	matches := make([]searchCodeMatch, 0)
	for _, code := range codes {
		codeMatch := strings.Contains(strings.ToLower(code), lower)
		bets := betydninger[code]
		if len(bets) == 0 {
			if codeMatch {
				matches = append(matches, searchCodeMatch{Kode: code})
			}
			continue
		}
		for _, bet := range bets {
			desc := pickBeskrivelse(bet.Beskrivelser)
			termMatch := strings.Contains(strings.ToLower(desc.Term), lower)
			if codeMatch || termMatch {
				matches = append(matches, searchCodeMatch{
					Kode:      code,
					Term:      desc.Term,
					Tekst:     desc.Tekst,
					GyldigFra: bet.GyldigFra,
					GyldigTil: bet.GyldigTil,
				})
			}
		}
	}

	total := len(matches)
	truncated := false
	if len(matches) > limit {
		matches = matches[:limit]
		truncated = true
	}

	return fitCappedJSON(len(matches), truncated, func(n int, trunc bool) any {
		return struct {
			Query         string            `json:"query"`
			Kodeverksnavn string            `json:"kodeverksnavn"`
			Total         int               `json:"total"`
			Returned      int               `json:"returned"`
			Truncated     bool              `json:"truncated,omitempty"`
			Matches       []searchCodeMatch `json:"matches"`
		}{
			Query:         query,
			Kodeverksnavn: kodeverksnavn,
			Total:         total,
			Returned:      n,
			Truncated:     trunc,
			Matches:       matches[:n],
		}
	})
}
