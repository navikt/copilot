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
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxToolResultBytes = 32 * 1024
	maxFilterLen       = 64
	maxQueryLen        = 64
	defaultStatus      = "ACTIVE"
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	navIdentPattern = regexp.MustCompile(`^[A-Za-z][0-9]{6}$`)
	slugPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
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

type toolOutputSpec struct {
	paged     bool
	listCount bool
	fields    []fieldSpec
}

var toolOutputs = map[string]toolOutputSpec{
	"list_teams": {paged: true, listCount: true, fields: []fieldSpec{
		{"teams", "array", true},
	}},
	"search_teams": {listCount: true, fields: []fieldSpec{
		{"query", "string", true},
		{"teams", "array", true},
	}},
	"get_team": {fields: []fieldSpec{
		{"id", "string", true},
		{"name", "string", true},
		{"description", "string", false},
		{"slackChannel", "string", false},
		{"productAreaId", "string", false},
		{"avdelingNomId", "string", false},
		{"avdelingNavn", "string", false},
		{"teamType", "string", false},
		{"clusterIds", "array", false},
		{"naisTeams", "array", false},
		{"tags", "array", false},
		{"status", "string", false},
		{"memberCount", "integer", true},
		{"members", "array", true},
	}},
	"get_teams_by_nais_team": {listCount: true, fields: []fieldSpec{
		{"naisTeam", "string", true},
		{"teams", "array", true},
	}},
	"list_product_areas": {paged: true, listCount: true, fields: []fieldSpec{
		{"areas", "array", true},
	}},
	"get_product_area": {fields: []fieldSpec{
		{"id", "string", true},
		{"name", "string", true},
		{"areaType", "string", false},
		{"description", "string", false},
		{"slackChannel", "string", false},
		{"avdelingNomId", "string", false},
		{"avdelingNavn", "string", false},
		{"tags", "array", false},
		{"status", "string", false},
		{"isDefaultArea", "boolean", false},
		{"memberCount", "integer", true},
		{"members", "array", true},
	}},
	"list_clusters": {paged: true, listCount: true, fields: []fieldSpec{
		{"clusters", "array", true},
	}},
	"get_cluster": {fields: []fieldSpec{
		{"id", "string", true},
		{"name", "string", true},
		{"description", "string", false},
		{"slackChannel", "string", false},
		{"productAreaId", "string", false},
		{"avdelingNomId", "string", false},
		{"avdelingNavn", "string", false},
		{"tags", "array", false},
		{"status", "string", false},
		{"memberCount", "integer", true},
		{"members", "array", true},
	}},
	"get_member_memberships": {fields: []fieldSpec{
		{"name", "string", false},
		{"teams", "array", true},
		{"productAreas", "array", true},
		{"clusters", "array", true},
	}},
	"get_nais_team": {fields: []fieldSpec{
		{"team", "object", false},
		{"query", "string", false},
		{"total", "integer", false},
		{"returned", "integer", false},
		{"teams", "array", false},
	}},
}

func toolDefinitions() []map[string]any {
	defs := baseToolDefinitions()
	for _, d := range defs {
		o := toolOutputs[d["name"].(string)]
		props := map[string]any{
			"truncated": map[string]any{"type": "boolean"},
		}
		var required []string
		if o.listCount {
			props["total"] = map[string]any{"type": "integer"}
			props["returned"] = map[string]any{"type": "integer"}
			required = append(required, "total", "returned")
		}
		if o.paged {
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
		schema := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			schema["required"] = required
		}
		d["outputSchema"] = schema
	}
	return defs
}

func baseToolDefinitions() []map[string]any {
	statusProp := map[string]any{
		"type":        "string",
		"description": "Comma-separated status filter from ACTIVE, PLANNED, INACTIVE (default ACTIVE).",
		"default":     defaultStatus,
	}
	uuidProp := func(desc string) map[string]any {
		return map[string]any{
			"type":        "string",
			"description": desc,
			"pattern":     `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
		}
	}
	return []map[string]any{
		{
			"name":        "list_teams",
			"description": "List teams in Nav's Team Catalog with optional productAreaId, clusterId, status, substring filter, and pagination. Omits nested member lists (returns memberCount). Example: {\"status\": \"ACTIVE\", \"filter\": \"copilot\", \"limit\": 25}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"productAreaId": uuidProp("Optional product area UUID to filter teams by."),
					"clusterId":     uuidProp("Optional cluster UUID to filter teams by."),
					"status":        statusProp,
					"filter": map[string]any{
						"type":        "string",
						"description": "Optional case-insensitive substring filter on team name or description.",
						"maxLength":   maxFilterLen,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of teams to return (default 25, max 100).",
						"default":     25,
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of matching teams to skip (default 0).",
						"default":     0,
						"minimum":     0,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "search_teams",
			"description": "Search teams by name in Nav's Team Catalog. Omits nested member lists (returns memberCount). Example: {\"query\": \"copilot\", \"limit\": 25}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query (3-64 characters; letters, digits, space, '-', '_').",
						"minLength":   3,
						"maxLength":   maxQueryLen,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of teams to return (default 25, max 50).",
						"default":     25,
						"minimum":     1,
						"maximum":     50,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "get_team",
			"description": "Get details for a single team by UUID, with members projected to name and roles only. Example: {\"id\": \"11111111-2222-3333-4444-555555555555\"}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"id"},
				"properties": map[string]any{
					"id": uuidProp("Team UUID."),
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "get_teams_by_nais_team",
			"description": "List teams associated with a Nais team/namespace slug. Omits nested member lists. Example: {\"naisTeam\": \"copilot\"}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"naisTeam"},
				"properties": map[string]any{
					"naisTeam": map[string]any{
						"type":        "string",
						"description": "Nais team slug (letters, digits, '-', '_').",
						"pattern":     `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`,
					},
					"status": statusProp,
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "list_product_areas",
			"description": "List product areas in Nav's Team Catalog with optional status, substring filter, and pagination. Omits nested member lists (returns memberCount). Example: {\"status\": \"ACTIVE\", \"limit\": 50}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"status": statusProp,
					"filter": map[string]any{
						"type":        "string",
						"description": "Optional case-insensitive substring filter on product area name.",
						"maxLength":   maxFilterLen,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of product areas to return (default 50, max 100).",
						"default":     50,
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of matching product areas to skip (default 0).",
						"default":     0,
						"minimum":     0,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "get_product_area",
			"description": "Get details for a single product area by UUID, with members projected to name and roles only. Example: {\"id\": \"11111111-2222-3333-4444-555555555555\"}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"id"},
				"properties": map[string]any{
					"id": uuidProp("Product area UUID."),
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "list_clusters",
			"description": "List clusters in Nav's Team Catalog with optional status, substring filter, and pagination. Omits nested member lists (returns memberCount). Example: {\"status\": \"ACTIVE\", \"limit\": 50}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"status": statusProp,
					"filter": map[string]any{
						"type":        "string",
						"description": "Optional case-insensitive substring filter on cluster name.",
						"maxLength":   maxFilterLen,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of clusters to return (default 50, max 100).",
						"default":     50,
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of matching clusters to skip (default 0).",
						"default":     0,
						"minimum":     0,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "get_cluster",
			"description": "Get details for a single cluster by UUID, with members projected to name and roles only. Example: {\"id\": \"11111111-2222-3333-4444-555555555555\"}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"id"},
				"properties": map[string]any{
					"id": uuidProp("Cluster UUID."),
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "get_member_memberships",
			"description": "Get teams, product areas, and clusters a person belongs to by NAVident. Returns the person's name and their roles per unit without nested member lists. Example: {\"navIdent\": \"Z123456\"}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"navIdent"},
				"properties": map[string]any{
					"navIdent": map[string]any{
						"type":        "string",
						"description": "Nav employee identifier (one letter followed by six digits, e.g. Z123456).",
						"pattern":     `^[A-Za-z][0-9]{6}$`,
					},
				},
			},
			"annotations": readOnlyAnnotations,
		},
		{
			"name":        "get_nais_team",
			"description": "Get a Nais team by exact teamId or search Nais teams by query (supply exactly one of teamId or query). Example: {\"teamId\": \"copilot\"}.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"teamId": map[string]any{
						"type":        "string",
						"description": "Exact Nais team slug.",
						"pattern":     `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`,
					},
					"query": map[string]any{
						"type":        "string",
						"description": "Search query (3-64 characters; letters, digits, space, '-', '_').",
						"minLength":   3,
						"maxLength":   maxQueryLen,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of Nais teams to return when using query (default 25, max 50).",
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

func (s *server) handleToolCall(ctx context.Context, inboundToken, toolName string, rawArgs json.RawMessage) (any, *mcpRPCError) {
	switch toolName {
	case "list_teams":
		args, rpcErr := parseListTeamsArgs(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			teams, err := s.teamCatalog.listTeams(ctx, oboToken, args.productAreaID, args.clusterID, args.status)
			if err != nil {
				return "", err
			}
			return buildListTeamsResult(teams, args.filter, args.limit, args.offset)
		}), nil

	case "search_teams":
		args, rpcErr := parseSearchArgs(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			teams, err := s.teamCatalog.searchTeams(ctx, oboToken, args.query)
			if err != nil {
				return "", err
			}
			return buildSearchTeamsResult(args.query, teams, args.limit)
		}), nil

	case "get_team":
		id, rpcErr := parseUUIDArg(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			team, err := s.teamCatalog.getTeam(ctx, oboToken, id)
			if err != nil {
				return "", err
			}
			return buildGetTeamResult(team)
		}), nil

	case "get_teams_by_nais_team":
		args, rpcErr := parseTeamsByNaisTeamArgs(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			teams, err := s.teamCatalog.getTeamsByNaisTeam(ctx, oboToken, args.naisTeam, args.status)
			if err != nil {
				return "", err
			}
			return buildTeamsByNaisTeamResult(args.naisTeam, teams)
		}), nil

	case "list_product_areas":
		args, rpcErr := parseListCatalogArgs(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			areas, err := s.teamCatalog.listProductAreas(ctx, oboToken, args.status)
			if err != nil {
				return "", err
			}
			return buildListProductAreasResult(areas, args.filter, args.limit, args.offset)
		}), nil

	case "get_product_area":
		id, rpcErr := parseUUIDArg(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			area, err := s.teamCatalog.getProductArea(ctx, oboToken, id)
			if err != nil {
				return "", err
			}
			return buildGetProductAreaResult(area)
		}), nil

	case "list_clusters":
		args, rpcErr := parseListCatalogArgs(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			clusters, err := s.teamCatalog.listClusters(ctx, oboToken, args.status)
			if err != nil {
				return "", err
			}
			return buildListClustersResult(clusters, args.filter, args.limit, args.offset)
		}), nil

	case "get_cluster":
		id, rpcErr := parseUUIDArg(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			cluster, err := s.teamCatalog.getCluster(ctx, oboToken, id)
			if err != nil {
				return "", err
			}
			return buildGetClusterResult(cluster)
		}), nil

	case "get_member_memberships":
		navIdent, rpcErr := parseNavIdentArg(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			m, err := s.teamCatalog.getMemberMemberships(ctx, oboToken, navIdent)
			if err != nil {
				return "", err
			}
			return buildMemberMembershipsResult(navIdent, m)
		}), nil

	case "get_nais_team":
		args, rpcErr := parseNaisTeamArgs(rawArgs)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.execWithOBO(ctx, inboundToken, func(oboToken string) (string, error) {
			if args.teamID != "" {
				nt, err := s.teamCatalog.getNaisTeam(ctx, oboToken, args.teamID)
				if err != nil {
					return "", err
				}
				return buildGetNaisTeamResult(nt)
			}
			teams, err := s.teamCatalog.searchNaisTeams(ctx, oboToken, args.query)
			if err != nil {
				return "", err
			}
			return buildSearchNaisTeamsResult(args.query, teams, args.limit)
		}), nil

	default:
		return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "unknown tool"}
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
	oboToken, err := s.exchangeToken(ctx, inboundToken)
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
	total, hasTotal := structured["total"].(float64)
	returned, _ := structured["returned"].(float64)
	truncated, _ := structured["truncated"].(bool)
	offset, hasOffset := structured["offset"].(float64)
	if hasOffset && int(returned) > 0 {
		if rest := int(total) - int(offset) - int(returned); rest > 0 {
			next := int(offset) + int(returned)
			structured["nextOffset"] = next
			content = append(content, map[string]string{"type": "text", "text": fmt.Sprintf(
				"%d more items. Next page: call again with offset=%d, or narrow the request (filter).", rest, next)})
		}
	} else if truncated || (hasTotal && !hasOffset && total > returned) {
		content = append(content, map[string]string{"type": "text", "text": truncationHint(structured, int(returned), int(total), hasTotal)})
	}
	return map[string]any{"content": content, "structuredContent": structured}
}

func truncationHint(structured map[string]any, returned, total int, hasTotal bool) string {
	if _, ok := structured["query"]; ok {
		return fmt.Sprintf("%d of %d shown. Narrow query to see the rest.", returned, total)
	}
	if _, ok := structured["offset"]; ok {
		return fmt.Sprintf("%d of %d shown: single item exceeds the 32 KiB limit. Narrow with filter or advance offset.", returned, total)
	}
	if hasTotal {
		return fmt.Sprintf("%d of %d shown: output capped at 32 KiB.", returned, total)
	}
	return "Output truncated to fit the 32 KiB limit."
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

func validateUUID(val, field string) (string, *mcpRPCError) {
	if !uuidPattern.MatchString(val) {
		return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must be a valid UUID", field)}
	}
	return strings.ToLower(val), nil
}

func validateSlug(val, field string) *mcpRPCError {
	if !slugPattern.MatchString(val) {
		return &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("%s must match ^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$", field)}
	}
	return nil
}

func validateStatus(raw string) (string, *mcpRPCError) {
	parts := strings.Split(raw, ",")
	if len(parts) == 0 || len(parts) > 3 {
		return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: "status must list 1 to 3 values from ACTIVE, PLANNED, INACTIVE"}
	}
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.ToUpper(strings.TrimSpace(p))
		switch s {
		case "ACTIVE", "PLANNED", "INACTIVE":
		default:
			return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: "status must contain only ACTIVE, PLANNED, INACTIVE"}
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return strings.Join(out, ","), nil
}

func validateSearchQuery(q string) *mcpRPCError {
	rc := utf8.RuneCountInString(q)
	if rc < 3 || rc > maxQueryLen {
		return &mcpRPCError{Code: mcpCodeInvalidParams, Message: "query must be 3 to 64 characters"}
	}
	if utf8.RuneCountInString(strings.TrimSpace(q)) < 3 {
		return &mcpRPCError{Code: mcpCodeInvalidParams, Message: "query must contain at least 3 non-whitespace characters"}
	}
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' {
			continue
		}
		return &mcpRPCError{Code: mcpCodeInvalidParams, Message: "query contains disallowed characters"}
	}
	return nil
}

type listTeamsArgs struct {
	productAreaID string
	clusterID     string
	status        string
	filter        string
	limit         int
	offset        int
}

func parseListTeamsArgs(raw json.RawMessage) (listTeamsArgs, *mcpRPCError) {
	allowed := map[string]bool{
		"productAreaId": true,
		"clusterId":     true,
		"status":        true,
		"filter":        true,
		"limit":         true,
		"offset":        true,
	}
	fields, rpcErr := decodeArgMap(raw, allowed)
	if rpcErr != nil {
		return listTeamsArgs{}, rpcErr
	}
	out := listTeamsArgs{status: defaultStatus, limit: 25, offset: 0}
	if v, ok := fields["productAreaId"]; ok {
		s, err := parseStrictString(v, "productAreaId")
		if err != nil {
			return listTeamsArgs{}, err
		}
		norm, err := validateUUID(s, "productAreaId")
		if err != nil {
			return listTeamsArgs{}, err
		}
		out.productAreaID = norm
	}
	if v, ok := fields["clusterId"]; ok {
		s, err := parseStrictString(v, "clusterId")
		if err != nil {
			return listTeamsArgs{}, err
		}
		norm, err := validateUUID(s, "clusterId")
		if err != nil {
			return listTeamsArgs{}, err
		}
		out.clusterID = norm
	}
	if v, ok := fields["status"]; ok {
		s, err := parseStrictString(v, "status")
		if err != nil {
			return listTeamsArgs{}, err
		}
		norm, err := validateStatus(s)
		if err != nil {
			return listTeamsArgs{}, err
		}
		out.status = norm
	}
	if v, ok := fields["filter"]; ok {
		s, err := parseStrictString(v, "filter")
		if err != nil {
			return listTeamsArgs{}, err
		}
		if utf8.RuneCountInString(s) > maxFilterLen {
			return listTeamsArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "filter must be at most 64 characters"}
		}
		out.filter = s
	}
	if v, ok := fields["limit"]; ok {
		n, err := parseStrictInt(v, "limit")
		if err != nil {
			return listTeamsArgs{}, err
		}
		if n < 1 || n > 100 {
			return listTeamsArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "limit must be between 1 and 100"}
		}
		out.limit = n
	}
	if v, ok := fields["offset"]; ok {
		n, err := parseStrictInt(v, "offset")
		if err != nil {
			return listTeamsArgs{}, err
		}
		if n < 0 {
			return listTeamsArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "offset must be >= 0"}
		}
		out.offset = n
	}
	return out, nil
}

type listCatalogArgs struct {
	status string
	filter string
	limit  int
	offset int
}

func parseListCatalogArgs(raw json.RawMessage) (listCatalogArgs, *mcpRPCError) {
	allowed := map[string]bool{
		"status": true,
		"filter": true,
		"limit":  true,
		"offset": true,
	}
	fields, rpcErr := decodeArgMap(raw, allowed)
	if rpcErr != nil {
		return listCatalogArgs{}, rpcErr
	}
	out := listCatalogArgs{status: defaultStatus, limit: 50, offset: 0}
	if v, ok := fields["status"]; ok {
		s, err := parseStrictString(v, "status")
		if err != nil {
			return listCatalogArgs{}, err
		}
		norm, err := validateStatus(s)
		if err != nil {
			return listCatalogArgs{}, err
		}
		out.status = norm
	}
	if v, ok := fields["filter"]; ok {
		s, err := parseStrictString(v, "filter")
		if err != nil {
			return listCatalogArgs{}, err
		}
		if utf8.RuneCountInString(s) > maxFilterLen {
			return listCatalogArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "filter must be at most 64 characters"}
		}
		out.filter = s
	}
	if v, ok := fields["limit"]; ok {
		n, err := parseStrictInt(v, "limit")
		if err != nil {
			return listCatalogArgs{}, err
		}
		if n < 1 || n > 100 {
			return listCatalogArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "limit must be between 1 and 100"}
		}
		out.limit = n
	}
	if v, ok := fields["offset"]; ok {
		n, err := parseStrictInt(v, "offset")
		if err != nil {
			return listCatalogArgs{}, err
		}
		if n < 0 {
			return listCatalogArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "offset must be >= 0"}
		}
		out.offset = n
	}
	return out, nil
}

type searchArgs struct {
	query string
	limit int
}

func parseSearchArgs(raw json.RawMessage) (searchArgs, *mcpRPCError) {
	allowed := map[string]bool{"query": true, "limit": true}
	fields, rpcErr := decodeArgMap(raw, allowed)
	if rpcErr != nil {
		return searchArgs{}, rpcErr
	}
	out := searchArgs{limit: 25}
	qRaw, ok := fields["query"]
	if !ok {
		return searchArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "query is required"}
	}
	q, err := parseStrictString(qRaw, "query")
	if err != nil {
		return searchArgs{}, err
	}
	if err := validateSearchQuery(q); err != nil {
		return searchArgs{}, err
	}
	out.query = strings.TrimSpace(q)

	if v, ok := fields["limit"]; ok {
		n, err := parseStrictInt(v, "limit")
		if err != nil {
			return searchArgs{}, err
		}
		if n < 1 || n > 50 {
			return searchArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "limit must be between 1 and 50"}
		}
		out.limit = n
	}
	return out, nil
}

func parseUUIDArg(raw json.RawMessage) (string, *mcpRPCError) {
	fields, rpcErr := decodeArgMap(raw, map[string]bool{"id": true})
	if rpcErr != nil {
		return "", rpcErr
	}
	idRaw, ok := fields["id"]
	if !ok {
		return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: "id is required"}
	}
	s, err := parseStrictString(idRaw, "id")
	if err != nil {
		return "", err
	}
	return validateUUID(s, "id")
}

type teamsByNaisTeamArgs struct {
	naisTeam string
	status   string
}

func parseTeamsByNaisTeamArgs(raw json.RawMessage) (teamsByNaisTeamArgs, *mcpRPCError) {
	fields, rpcErr := decodeArgMap(raw, map[string]bool{"naisTeam": true, "status": true})
	if rpcErr != nil {
		return teamsByNaisTeamArgs{}, rpcErr
	}
	out := teamsByNaisTeamArgs{status: defaultStatus}
	ntRaw, ok := fields["naisTeam"]
	if !ok {
		return teamsByNaisTeamArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "naisTeam is required"}
	}
	s, err := parseStrictString(ntRaw, "naisTeam")
	if err != nil {
		return teamsByNaisTeamArgs{}, err
	}
	if err := validateSlug(s, "naisTeam"); err != nil {
		return teamsByNaisTeamArgs{}, err
	}
	out.naisTeam = s

	if v, ok := fields["status"]; ok {
		st, err := parseStrictString(v, "status")
		if err != nil {
			return teamsByNaisTeamArgs{}, err
		}
		norm, err := validateStatus(st)
		if err != nil {
			return teamsByNaisTeamArgs{}, err
		}
		out.status = norm
	}
	return out, nil
}

func parseNavIdentArg(raw json.RawMessage) (string, *mcpRPCError) {
	fields, rpcErr := decodeArgMap(raw, map[string]bool{"navIdent": true})
	if rpcErr != nil {
		return "", rpcErr
	}
	v, ok := fields["navIdent"]
	if !ok {
		return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: "navIdent is required"}
	}
	s, err := parseStrictString(v, "navIdent")
	if err != nil {
		return "", err
	}
	if !navIdentPattern.MatchString(s) {
		return "", &mcpRPCError{Code: mcpCodeInvalidParams, Message: "navIdent must match ^[A-Za-z][0-9]{6}$"}
	}
	return strings.ToUpper(s), nil
}

type naisTeamArgs struct {
	teamID string
	query  string
	limit  int
}

func parseNaisTeamArgs(raw json.RawMessage) (naisTeamArgs, *mcpRPCError) {
	fields, rpcErr := decodeArgMap(raw, map[string]bool{"teamId": true, "query": true, "limit": true})
	if rpcErr != nil {
		return naisTeamArgs{}, rpcErr
	}
	out := naisTeamArgs{limit: 25}
	if v, ok := fields["teamId"]; ok {
		s, err := parseStrictString(v, "teamId")
		if err != nil {
			return naisTeamArgs{}, err
		}
		if err := validateSlug(s, "teamId"); err != nil {
			return naisTeamArgs{}, err
		}
		out.teamID = s
	}
	if v, ok := fields["query"]; ok {
		s, err := parseStrictString(v, "query")
		if err != nil {
			return naisTeamArgs{}, err
		}
		if err := validateSearchQuery(s); err != nil {
			return naisTeamArgs{}, err
		}
		out.query = strings.TrimSpace(s)
	}
	if (out.teamID == "" && out.query == "") || (out.teamID != "" && out.query != "") {
		return naisTeamArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "provide exactly one of teamId or query"}
	}
	if v, ok := fields["limit"]; ok {
		n, err := parseStrictInt(v, "limit")
		if err != nil {
			return naisTeamArgs{}, err
		}
		if n < 1 || n > 50 {
			return naisTeamArgs{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "limit must be between 1 and 50"}
		}
		out.limit = n
	}
	return out, nil
}

// --- PII Projection & Output Builders ---

// personRole is the only person representation exposed by this server.
// Email, leave status, employment dates, stale flags, resource types,
// teamPercent, and NAVidents are never projected.
type personRole struct {
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

type teamSummary struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	SlackChannel  string   `json:"slackChannel,omitempty"`
	ProductAreaID string   `json:"productAreaId,omitempty"`
	AvdelingNomID string   `json:"avdelingNomId,omitempty"`
	AvdelingNavn  string   `json:"avdelingNavn,omitempty"`
	TeamType      string   `json:"teamType,omitempty"`
	ClusterIDs    []string `json:"clusterIds,omitempty"`
	NaisTeams     []string `json:"naisTeams,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Status        string   `json:"status,omitempty"`
	MemberCount   int      `json:"memberCount"`
}

type teamDetail struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Description   string       `json:"description,omitempty"`
	SlackChannel  string       `json:"slackChannel,omitempty"`
	ProductAreaID string       `json:"productAreaId,omitempty"`
	AvdelingNomID string       `json:"avdelingNomId,omitempty"`
	AvdelingNavn  string       `json:"avdelingNavn,omitempty"`
	TeamType      string       `json:"teamType,omitempty"`
	ClusterIDs    []string     `json:"clusterIds,omitempty"`
	NaisTeams     []string     `json:"naisTeams,omitempty"`
	Tags          []string     `json:"tags,omitempty"`
	Status        string       `json:"status,omitempty"`
	MemberCount   int          `json:"memberCount"`
	Members       []personRole `json:"members"`
	Truncated     bool         `json:"truncated,omitempty"`
}

type productAreaSummary struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	AreaType      string   `json:"areaType,omitempty"`
	Description   string   `json:"description,omitempty"`
	SlackChannel  string   `json:"slackChannel,omitempty"`
	AvdelingNomID string   `json:"avdelingNomId,omitempty"`
	AvdelingNavn  string   `json:"avdelingNavn,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Status        string   `json:"status,omitempty"`
	IsDefaultArea bool     `json:"isDefaultArea,omitempty"`
	MemberCount   int      `json:"memberCount"`
}

type productAreaDetail struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	AreaType      string       `json:"areaType,omitempty"`
	Description   string       `json:"description,omitempty"`
	SlackChannel  string       `json:"slackChannel,omitempty"`
	AvdelingNomID string       `json:"avdelingNomId,omitempty"`
	AvdelingNavn  string       `json:"avdelingNavn,omitempty"`
	Tags          []string     `json:"tags,omitempty"`
	Status        string       `json:"status,omitempty"`
	IsDefaultArea bool         `json:"isDefaultArea,omitempty"`
	MemberCount   int          `json:"memberCount"`
	Members       []personRole `json:"members"`
	Truncated     bool         `json:"truncated,omitempty"`
}

type clusterSummary struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	SlackChannel  string   `json:"slackChannel,omitempty"`
	ProductAreaID string   `json:"productAreaId,omitempty"`
	AvdelingNomID string   `json:"avdelingNomId,omitempty"`
	AvdelingNavn  string   `json:"avdelingNavn,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Status        string   `json:"status,omitempty"`
	MemberCount   int      `json:"memberCount"`
}

type clusterDetail struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Description   string       `json:"description,omitempty"`
	SlackChannel  string       `json:"slackChannel,omitempty"`
	ProductAreaID string       `json:"productAreaId,omitempty"`
	AvdelingNomID string       `json:"avdelingNomId,omitempty"`
	AvdelingNavn  string       `json:"avdelingNavn,omitempty"`
	Tags          []string     `json:"tags,omitempty"`
	Status        string       `json:"status,omitempty"`
	MemberCount   int          `json:"memberCount"`
	Members       []personRole `json:"members"`
	Truncated     bool         `json:"truncated,omitempty"`
}

type membershipUnit struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status,omitempty"`
	Roles       []string `json:"roles"`
	MemberCount int      `json:"memberCount"`
}

func projectPersonName(m upstreamMember) string {
	if m.Resource == nil {
		return ""
	}
	if full := strings.TrimSpace(m.Resource.FullName); full != "" {
		return full
	}
	return strings.TrimSpace(strings.TrimSpace(m.Resource.GivenName) + " " + strings.TrimSpace(m.Resource.FamilyName))
}

func sortedRoles(roles []string) []string {
	if len(roles) == 0 {
		return []string{}
	}
	out := make([]string, len(roles))
	copy(out, roles)
	slices.Sort(out)
	return out
}

func projectMembers(raw []upstreamMember) []personRole {
	out := make([]personRole, 0, len(raw))
	for _, m := range raw {
		out = append(out, personRole{
			Name:  projectPersonName(m),
			Roles: sortedRoles(m.Roles),
		})
	}
	slices.SortFunc(out, func(a, b personRole) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(strings.Join(a.Roles, ","), strings.Join(b.Roles, ","))
	})
	return out
}

func projectTeamSummary(t upstreamTeam) teamSummary {
	return teamSummary{
		ID:            t.ID,
		Name:          t.Name,
		Description:   t.Description,
		SlackChannel:  t.SlackChannel,
		ProductAreaID: t.ProductAreaID,
		AvdelingNomID: t.AvdelingNomID,
		AvdelingNavn:  t.AvdelingNavn,
		TeamType:      t.TeamType,
		ClusterIDs:    t.ClusterIDs,
		NaisTeams:     t.NaisTeams,
		Tags:          t.Tags,
		Status:        t.Status,
		MemberCount:   len(t.Members),
	}
}

func projectProductAreaSummary(a upstreamProductArea) productAreaSummary {
	return productAreaSummary{
		ID:            a.ID,
		Name:          a.Name,
		AreaType:      a.AreaType,
		Description:   a.Description,
		SlackChannel:  a.SlackChannel,
		AvdelingNomID: a.AvdelingNomID,
		AvdelingNavn:  a.AvdelingNavn,
		Tags:          a.Tags,
		Status:        a.Status,
		IsDefaultArea: a.IsDefaultArea,
		MemberCount:   len(a.Members),
	}
}

func projectClusterSummary(c upstreamCluster) clusterSummary {
	return clusterSummary{
		ID:            c.ID,
		Name:          c.Name,
		Description:   c.Description,
		SlackChannel:  c.SlackChannel,
		ProductAreaID: c.ProductAreaID,
		AvdelingNomID: c.AvdelingNomID,
		AvdelingNavn:  c.AvdelingNavn,
		Tags:          c.Tags,
		Status:        c.Status,
		MemberCount:   len(c.Members),
	}
}

func findMemberInList(members []upstreamMember, navIdent string) (name string, roles []string) {
	for _, m := range members {
		if strings.EqualFold(m.NavIdent, navIdent) {
			return projectPersonName(m), sortedRoles(m.Roles)
		}
	}
	return "", []string{}
}

// --- Pagination and 32 KiB size capping ---

func fitCappedJSON(totalItems int, alreadyTruncated bool, render func(n int, truncated bool) any) (string, error) {
	full := render(totalItems, alreadyTruncated)
	b, err := json.Marshal(full)
	if err != nil {
		return "", err
	}
	if len(b) <= maxToolResultBytes {
		return string(b), nil
	}

	lo, hi := 0, max(totalItems-1, 0)
	var bestBytes []byte
	for lo <= hi {
		mid := lo + (hi-lo)/2
		candidate, candErr := json.Marshal(render(mid, true))
		if candErr != nil {
			return "", candErr
		}
		if len(candidate) <= maxToolResultBytes {
			bestBytes = candidate
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if bestBytes != nil {
		return string(bestBytes), nil
	}
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

func sortTeamsByName(items []teamSummary) {
	slices.SortFunc(items, func(a, b teamSummary) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
}

func buildListTeamsResult(raw []upstreamTeam, filter string, limit, offset int) (string, error) {
	lower := strings.ToLower(filter)
	filtered := make([]teamSummary, 0, len(raw))
	for _, t := range raw {
		if lower != "" && !strings.Contains(strings.ToLower(t.Name), lower) && !strings.Contains(strings.ToLower(t.Description), lower) {
			continue
		}
		filtered = append(filtered, projectTeamSummary(t))
	}
	sortTeamsByName(filtered)

	total := len(filtered)
	var page []teamSummary
	if offset < total {
		page = filtered[offset:min(offset+limit, total)]
	} else {
		page = []teamSummary{}
	}

	return fitCappedJSON(len(page), false, func(n int, truncated bool) any {
		return struct {
			Total      int           `json:"total"`
			Returned   int           `json:"returned"`
			Offset     int           `json:"offset"`
			NextOffset *int          `json:"nextOffset,omitempty"`
			Truncated  bool          `json:"truncated,omitempty"`
			Teams      []teamSummary `json:"teams"`
		}{
			Total:      total,
			Returned:   n,
			Offset:     offset,
			NextOffset: nextPageOffset(offset, n, total),
			Truncated:  truncated,
			Teams:      page[:n],
		}
	})
}

func buildSearchTeamsResult(query string, raw []upstreamTeam, limit int) (string, error) {
	items := make([]teamSummary, 0, len(raw))
	for _, t := range raw {
		items = append(items, projectTeamSummary(t))
	}
	sortTeamsByName(items)

	total := len(items)
	truncated := false
	if len(items) > limit {
		items = items[:limit]
		truncated = true
	}

	return fitCappedJSON(len(items), truncated, func(n int, trunc bool) any {
		return struct {
			Query     string        `json:"query"`
			Total     int           `json:"total"`
			Returned  int           `json:"returned"`
			Truncated bool          `json:"truncated,omitempty"`
			Teams     []teamSummary `json:"teams"`
		}{
			Query:     query,
			Total:     total,
			Returned:  n,
			Truncated: trunc,
			Teams:     items[:n],
		}
	})
}

func buildGetTeamResult(t upstreamTeam) (string, error) {
	members := projectMembers(t.Members)
	return fitCappedJSON(len(members), false, func(n int, truncated bool) any {
		return teamDetail{
			ID:            t.ID,
			Name:          t.Name,
			Description:   t.Description,
			SlackChannel:  t.SlackChannel,
			ProductAreaID: t.ProductAreaID,
			AvdelingNomID: t.AvdelingNomID,
			AvdelingNavn:  t.AvdelingNavn,
			TeamType:      t.TeamType,
			ClusterIDs:    t.ClusterIDs,
			NaisTeams:     t.NaisTeams,
			Tags:          t.Tags,
			Status:        t.Status,
			MemberCount:   len(members),
			Members:       members[:n],
			Truncated:     truncated,
		}
	})
}

func buildTeamsByNaisTeamResult(naisTeamSlug string, raw []upstreamTeam) (string, error) {
	items := make([]teamSummary, 0, len(raw))
	for _, t := range raw {
		items = append(items, projectTeamSummary(t))
	}
	sortTeamsByName(items)
	total := len(items)

	return fitCappedJSON(len(items), false, func(n int, truncated bool) any {
		return struct {
			NaisTeam  string        `json:"naisTeam"`
			Total     int           `json:"total"`
			Returned  int           `json:"returned"`
			Truncated bool          `json:"truncated,omitempty"`
			Teams     []teamSummary `json:"teams"`
		}{
			NaisTeam:  naisTeamSlug,
			Total:     total,
			Returned:  n,
			Truncated: truncated,
			Teams:     items[:n],
		}
	})
}

func buildListProductAreasResult(raw []upstreamProductArea, filter string, limit, offset int) (string, error) {
	lower := strings.ToLower(filter)
	filtered := make([]productAreaSummary, 0, len(raw))
	for _, a := range raw {
		if lower != "" && !strings.Contains(strings.ToLower(a.Name), lower) && !strings.Contains(strings.ToLower(a.Description), lower) {
			continue
		}
		filtered = append(filtered, projectProductAreaSummary(a))
	}
	slices.SortFunc(filtered, func(a, b productAreaSummary) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})

	total := len(filtered)
	var page []productAreaSummary
	if offset < total {
		page = filtered[offset:min(offset+limit, total)]
	} else {
		page = []productAreaSummary{}
	}

	return fitCappedJSON(len(page), false, func(n int, truncated bool) any {
		return struct {
			Total      int                  `json:"total"`
			Returned   int                  `json:"returned"`
			Offset     int                  `json:"offset"`
			NextOffset *int                 `json:"nextOffset,omitempty"`
			Truncated  bool                 `json:"truncated,omitempty"`
			Areas      []productAreaSummary `json:"areas"`
		}{
			Total:      total,
			Returned:   n,
			Offset:     offset,
			NextOffset: nextPageOffset(offset, n, total),
			Truncated:  truncated,
			Areas:      page[:n],
		}
	})
}

func buildGetProductAreaResult(a upstreamProductArea) (string, error) {
	members := projectMembers(a.Members)
	return fitCappedJSON(len(members), false, func(n int, truncated bool) any {
		return productAreaDetail{
			ID:            a.ID,
			Name:          a.Name,
			AreaType:      a.AreaType,
			Description:   a.Description,
			SlackChannel:  a.SlackChannel,
			AvdelingNomID: a.AvdelingNomID,
			AvdelingNavn:  a.AvdelingNavn,
			Tags:          a.Tags,
			Status:        a.Status,
			IsDefaultArea: a.IsDefaultArea,
			MemberCount:   len(members),
			Members:       members[:n],
			Truncated:     truncated,
		}
	})
}

func buildListClustersResult(raw []upstreamCluster, filter string, limit, offset int) (string, error) {
	lower := strings.ToLower(filter)
	filtered := make([]clusterSummary, 0, len(raw))
	for _, c := range raw {
		if lower != "" && !strings.Contains(strings.ToLower(c.Name), lower) && !strings.Contains(strings.ToLower(c.Description), lower) {
			continue
		}
		filtered = append(filtered, projectClusterSummary(c))
	}
	slices.SortFunc(filtered, func(a, b clusterSummary) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})

	total := len(filtered)
	var page []clusterSummary
	if offset < total {
		page = filtered[offset:min(offset+limit, total)]
	} else {
		page = []clusterSummary{}
	}

	return fitCappedJSON(len(page), false, func(n int, truncated bool) any {
		return struct {
			Total      int              `json:"total"`
			Returned   int              `json:"returned"`
			Offset     int              `json:"offset"`
			NextOffset *int             `json:"nextOffset,omitempty"`
			Truncated  bool             `json:"truncated,omitempty"`
			Clusters   []clusterSummary `json:"clusters"`
		}{
			Total:      total,
			Returned:   n,
			Offset:     offset,
			NextOffset: nextPageOffset(offset, n, total),
			Truncated:  truncated,
			Clusters:   page[:n],
		}
	})
}

func buildGetClusterResult(c upstreamCluster) (string, error) {
	members := projectMembers(c.Members)
	return fitCappedJSON(len(members), false, func(n int, truncated bool) any {
		return clusterDetail{
			ID:            c.ID,
			Name:          c.Name,
			Description:   c.Description,
			SlackChannel:  c.SlackChannel,
			ProductAreaID: c.ProductAreaID,
			AvdelingNomID: c.AvdelingNomID,
			AvdelingNavn:  c.AvdelingNavn,
			Tags:          c.Tags,
			Status:        c.Status,
			MemberCount:   len(members),
			Members:       members[:n],
			Truncated:     truncated,
		}
	})
}

func buildMemberMembershipsResult(navIdent string, m upstreamMembership) (string, error) {
	var personName string
	teams := make([]membershipUnit, 0, len(m.Teams))
	for _, t := range m.Teams {
		name, roles := findMemberInList(t.Members, navIdent)
		if personName == "" && name != "" {
			personName = name
		}
		teams = append(teams, membershipUnit{
			ID:          t.ID,
			Name:        t.Name,
			Status:      t.Status,
			Roles:       roles,
			MemberCount: len(t.Members),
		})
	}
	areas := make([]membershipUnit, 0, len(m.ProductAreas))
	for _, a := range m.ProductAreas {
		name, roles := findMemberInList(a.Members, navIdent)
		if personName == "" && name != "" {
			personName = name
		}
		areas = append(areas, membershipUnit{
			ID:          a.ID,
			Name:        a.Name,
			Status:      a.Status,
			Roles:       roles,
			MemberCount: len(a.Members),
		})
	}
	clusters := make([]membershipUnit, 0, len(m.Clusters))
	for _, c := range m.Clusters {
		name, roles := findMemberInList(c.Members, navIdent)
		if personName == "" && name != "" {
			personName = name
		}
		clusters = append(clusters, membershipUnit{
			ID:          c.ID,
			Name:        c.Name,
			Status:      c.Status,
			Roles:       roles,
			MemberCount: len(c.Members),
		})
	}

	sortUnits := func(units []membershipUnit) {
		slices.SortFunc(units, func(a, b membershipUnit) int {
			if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
				return c
			}
			return strings.Compare(a.ID, b.ID)
		})
	}
	sortUnits(teams)
	sortUnits(areas)
	sortUnits(clusters)

	totalItems := len(teams) + len(areas) + len(clusters)
	return fitCappedJSON(totalItems, false, func(n int, truncated bool) any {
		rem := n
		tTake := min(rem, len(teams))
		rem -= tTake
		aTake := min(rem, len(areas))
		rem -= aTake
		cTake := min(rem, len(clusters))
		return struct {
			Name         string           `json:"name,omitempty"`
			Teams        []membershipUnit `json:"teams"`
			ProductAreas []membershipUnit `json:"productAreas"`
			Clusters     []membershipUnit `json:"clusters"`
			Truncated    bool             `json:"truncated,omitempty"`
		}{
			Name:         personName,
			Teams:        teams[:tTake],
			ProductAreas: areas[:aTake],
			Clusters:     clusters[:cTake],
			Truncated:    truncated,
		}
	})
}

func buildGetNaisTeamResult(nt naisTeam) (string, error) {
	b, err := json.Marshal(struct {
		Team naisTeam `json:"team"`
	}{Team: nt})
	if err != nil {
		return "", err
	}
	if len(b) > maxToolResultBytes {
		return "", errors.New("tool response exceeds 32 KiB limit")
	}
	return string(b), nil
}

func buildSearchNaisTeamsResult(query string, raw []naisTeam, limit int) (string, error) {
	items := make([]naisTeam, len(raw))
	copy(items, raw)
	slices.SortFunc(items, func(a, b naisTeam) int {
		return strings.Compare(strings.ToLower(a.Slug), strings.ToLower(b.Slug))
	})

	total := len(items)
	truncated := false
	if len(items) > limit {
		items = items[:limit]
		truncated = true
	}

	return fitCappedJSON(len(items), truncated, func(n int, trunc bool) any {
		return struct {
			Query     string     `json:"query"`
			Total     int        `json:"total"`
			Returned  int        `json:"returned"`
			Truncated bool       `json:"truncated,omitempty"`
			Teams     []naisTeam `json:"teams"`
		}{
			Query:     query,
			Total:     total,
			Returned:  n,
			Truncated: trunc,
			Teams:     items[:n],
		}
	})
}
