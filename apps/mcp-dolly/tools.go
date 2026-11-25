package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/navikt/copilot/apps/mcp-dolly/internal/dolly"
	"io"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maxToolResultBytes = 128 * 1024
	maxPageSize        = 50
	defaultPageSize    = dolly.DefaultPageSize
	maxPageNo          = 10000
	maxFragmentLen     = 100
	minOrderAntall     = 1
	maxOrderAntall     = 5
	maxOrderAge        = 120
	dailyPersonQuota   = 20
	quotaWindow        = 24 * time.Hour
)

// readOnlyAnnotations are client hints, never a security boundary.
var readOnlyAnnotations = map[string]bool{
	"readOnlyHint":    true,
	"destructiveHint": false,
	"idempotentHint":  true,
	"openWorldHint":   false,
}

var writeOrderAnnotations = map[string]bool{
	"readOnlyHint":    false,
	"destructiveHint": false,
	"idempotentHint":  false,
	"openWorldHint":   true,
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

func writeTool(name, desc string, required []string, props map[string]any) map[string]any {
	t := tool(name, desc, required, props)
	t["annotations"] = writeOrderAnnotations
	return t
}

func toolDefinitions(enableCreateOrder, elicit bool) []map[string]any {
	gruppeID := intProp("Dolly test group id (gruppeId).", 1, 1<<53)
	tools := []map[string]any{
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
	if enableCreateOrder {
		desc, props := createOrderDescTwoStep, createOrderProps(gruppeID)
		if elicit {
			// The client asks the user; the model never sees a token or a step 2.
			desc = createOrderDescElicit
			delete(props, confirmTokenKey)
		}
		tools = append(tools, writeTool("create_order", desc,
			[]string{"gruppeId", "antall", "environments"}, props))
	}
	return tools
}

const (
	createOrderDescTwoStep = "Create an order (bestilling) for 1-5 synthetic test persons in a Dolly group owned by the signed-in user. Two steps. Step 1: call with the order parameters; this only returns a preview (group, antall, environments, attributes, quota left) and a confirmationToken valid for 5 minutes, and orders nothing. Always confirm with the user by showing the preview. Step 2: after the user approves, call again with the identical parameters plus confirmationToken; only then is the order sent and quota charged. The token is single use and bound to the user and the exact parameters. Defaults are in the preview; only antall, environments, minAlder, maxAlder and kjoenn can be set."
	createOrderDescElicit  = "Create an order (bestilling) for 1-5 synthetic test persons in a Dolly group owned by the signed-in user. The user is asked to confirm the order in the client before anything is ordered; if they decline, nothing is ordered. Defaults are in the result; only antall, environments, minAlder, maxAlder and kjoenn can be set."
)

func createOrderProps(gruppeID map[string]any) map[string]any {
	return map[string]any{
		"gruppeId": gruppeID,
		"antall":   intProp("Number of synthetic test persons to create (1-5).", minOrderAntall, maxOrderAntall),
		"environments": map[string]any{
			"type":        "array",
			"description": "Target test environments (non-empty subset of q1, q2, q4).",
			"minItems":    1,
			"maxItems":    3,
			"uniqueItems": true,
			"items":       map[string]any{"type": "string", "enum": []string{"q1", "q2", "q4"}},
		},
		"minAlder":      intProp("Optional minimum age in years (0-120).", 0, maxOrderAge),
		"maxAlder":      intProp("Optional maximum age in years (0-120, >= minAlder).", 0, maxOrderAge),
		"kjoenn":        map[string]any{"type": "string", "enum": []string{"MANN", "KVINNE", "UKJENT"}, "description": "Optional gender (MANN, KVINNE, or UKJENT)."},
		confirmTokenKey: map[string]any{"type": "string", "description": "Token from the preview (step 2 only). Omit in step 1."},
	}
}

// orderDefaults is what Dolly applies to every created person (team Dolly, 2026-11-20).
const orderDefaults = "Each person is created with FNR and gets: fødselsnummer, name, gender, place of birth, date of birth, marital status, residential address, Norwegian citizenship and folkeregister personstatus=bosatt. " +
	"Unless set: age 30-60, even gender split; children 0-17, parents 60-80; spouse/partner within +/-3 years of the main person; every 20th couple (5%) is same-sex. " +
	"This tool can only set antall, environments, minAlder, maxAlder and kjoenn (main person); anything else requires the Dolly UI."

// orderDefaultsNB is orderDefaults in short Norwegian, for the confirmation form.
const orderDefaultsNB = "Alle får fødselsnummer, navn, kjønn, fødested, fødselsdato, sivilstand, bostedsadresse, norsk statsborgerskap og status bosatt. Uten valg: alder 30–60 og jevn kjønnsfordeling."

// elicitInput is set when the client declared form elicitation (2026-07-28 only).
// responses and state come from the client's retry, never from tool arguments.
type elicitInput struct {
	responses map[string]json.RawMessage
	state     string
}

const orderConfirmKey = "confirm_order"

const orderNeedsElicitation = "Ingenting er bestilt. create_order krever en klient som kan be brukeren bekrefte bestillingen (form elicitation), for eksempel Copilot CLI 1.0.95 eller nyere."

// confirmed reports the user's answer; answered is false when the retry carries no response.
func (el *elicitInput) confirmed() (ok, answered bool) {
	var r struct {
		Action  string `json:"action"`
		Content struct {
			Confirm bool `json:"confirm"`
		} `json:"content"`
	}
	raw, has := el.responses[orderConfirmKey]
	if !has || json.Unmarshal(raw, &r) != nil {
		return false, false
	}
	return r.Action == "accept" && r.Content.Confirm, true
}

// askOrderConfirmation returns an InputRequiredResult whose requestState carries
// the confirmation token. Clients pass requestState back without showing it to the model.
// https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr#inputrequiredresult
func (s *server) askOrderConfirmation(ctx context.Context, inboundToken, userHash, params string, gruppeID int64, req dolly.BestillingRequest, fields map[string]json.RawMessage) any {
	res := s.execWithOBO(ctx, inboundToken, func(tok string) (string, error) {
		return "", s.checkGroupOwner(ctx, tok, gruppeID)
	})
	if res["isError"] == true {
		return res
	}
	left, err := s.store.remaining(ctx, userHash, s.now())
	if err != nil {
		return toolErrorResult(err.Error())
	}
	ctok, _ := s.confirm.issue(userHash, params, s.now())
	msg := fmt.Sprintf("Bestille %d syntetiske testpersoner i Dolly-gruppe %d, miljø %s?", req.Antall, gruppeID, strings.Join(req.Environments, ", "))
	for _, k := range []string{"minAlder", "maxAlder", "kjoenn"} {
		if v, ok := fields[k]; ok {
			msg += fmt.Sprintf(" %s=%s.", k, v)
		}
	}
	msg += " " + orderDefaultsNB
	msg += fmt.Sprintf(" Kvote igjen %s: %d.", s.quotaPeriodNB(), left)
	return map[string]any{
		"resultType": "input_required",
		"inputRequests": map[string]any{orderConfirmKey: map[string]any{
			"method": "elicitation/create",
			"params": map[string]any{
				"mode":    "form",
				"message": msg,
				"requestedSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{"confirm": map[string]any{
						"type": "boolean", "title": "Bestill", "default": false,
					}},
					"required": []string{"confirm"},
				},
			},
		}},
		"requestState": ctok,
	}
}

func (s *server) handleToolCall(ctx context.Context, inboundToken, navIdent, toolName string, rawArgs json.RawMessage, el *elicitInput) (any, *mcpRPCError) {
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
			b, err := s.dolly.Get(ctx, tok, path, q)
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
			brukerID, err := s.dolly.CurrentBrukerID(ctx, tok)
			if err != nil {
				return "", err
			}
			q := url.Values{"brukerId": {brukerID}, "pageNo": {strconv.Itoa(pageNo)}, "pageSize": {strconv.Itoa(pageSize)}}
			b, err := s.dolly.Get(ctx, tok, "/api/v1/gruppe", q)
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

	case "create_order":
		if !s.cfg.EnableCreateOrder {
			return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "unknown tool"}
		}
		if !allow("gruppeId", "antall", "environments", "minAlder", "maxAlder", "kjoenn", confirmTokenKey) {
			return nil, rpcErr
		}
		gruppeID, orderReq, e := parseCreateOrder(fields, s.now())
		if e != nil {
			return nil, e
		}
		userHash := s.quota.hashUser(navIdent)
		params := paramsHash(fields)
		rawTok, hasTok := fields[confirmTokenKey]
		var token string
		consumed := false
		switch {
		case el == nil && !s.cfg.TwoStepOrder:
			// No token is issued or accepted: only the client may confirm with the user.
			return toolErrorResult(orderNeedsElicitation), nil
		case el != nil && hasTok:
			// The client confirms with the user; a token from the model is never accepted here.
			return toolErrorResult("confirmationToken is not accepted from this client; call create_order without it and the user confirms in the client"), nil
		case el != nil && el.state == "":
			return s.askOrderConfirmation(ctx, inboundToken, userHash, params, gruppeID, orderReq, fields), nil
		case el != nil:
			// Spend the state on every outcome, so a declined state cannot be replayed as accepted.
			if err := s.useToken(ctx, el.state, userHash, params); err != nil {
				return toolErrorResult(err.Error()), nil
			}
			consumed = true
			ok, answered := el.confirmed()
			if !answered {
				return s.askOrderConfirmation(ctx, inboundToken, userHash, params, gruppeID, orderReq, fields), nil
			}
			if !ok {
				return map[string]any{"content": []map[string]string{{"type": "text", "text": "The user did not confirm the order. Nothing was ordered."}}}, nil
			}
			token = el.state
		case !hasTok:
			// Step 1: preview only. Nothing is ordered and no quota is charged.
			attrs := map[string]json.RawMessage{}
			for _, k := range []string{"minAlder", "maxAlder", "kjoenn"} {
				if v, ok := fields[k]; ok {
					attrs[k] = v
				}
			}
			left, err := s.store.remaining(ctx, userHash, s.now())
			if err != nil {
				return toolErrorResult(err.Error()), nil
			}
			return s.execWithOBO(ctx, inboundToken, func(tok string) (string, error) {
				if err := s.checkGroupOwner(ctx, tok, gruppeID); err != nil {
					return "", err
				}
				ctok, exp := s.confirm.issue(userHash, params, s.now())
				preview, _ := json.Marshal(map[string]any{
					"status": "preview, NOT ordered",
					"order": map[string]any{
						"gruppeId":     gruppeID,
						"antall":       orderReq.Antall,
						"environments": orderReq.Environments,
						"attributes":   attrs,
					},
					"defaults":          orderDefaults,
					"quotaRemaining":    left,
					"confirmationToken": ctok,
					"expiresAt":         exp.UTC().Format(time.RFC3339),
					"next":              "Show this preview to the user. Only after they approve, call create_order again with the same parameters plus confirmationToken.",
				})
				return string(preview), nil
			}), nil
		default:
			var e *mcpRPCError
			if token, e = parseStrictString(rawTok, confirmTokenKey); e != nil {
				return nil, e
			}
		}
		if !consumed {
			if err := s.useToken(ctx, token, userHash, params); err != nil {
				return toolErrorResult(err.Error()), nil
			}
		}
		release, ok, err := s.store.reserve(ctx, userHash, orderReq.Antall, s.now())
		if err != nil {
			return toolErrorResult(err.Error()), nil
		}
		if !ok {
			return toolErrorResult(fmt.Sprintf("quota of %d test persons %s exceeded", dailyPersonQuota, s.quotaPeriodEN())), nil
		}
		committed := false
		defer func() {
			if !committed {
				release()
			}
		}()
		return s.execWithOBO(ctx, inboundToken, func(tok string) (string, error) {
			if err := s.checkGroupOwner(ctx, tok, gruppeID); err != nil {
				return "", err
			}
			b, refund, err := s.dolly.CreateOrder(ctx, tok, gruppeID, orderReq)
			if err != nil {
				if !refund {
					committed = true
				}
				return "", err
			}
			committed = true
			slog.Info("audit dolly create_order",
				"gruppeId", gruppeID,
				"antall", orderReq.Antall,
				"environments", orderReq.Environments,
			)
			return capResult(b)
		}), nil

	default:
		return nil, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "unknown tool"}
	}
}

// The quota counts per UTC day in Postgres mode and per rolling 24 h in memory mode.
func (s *server) quotaPeriodNB() string {
	if s.store.sharedSingleUse() {
		return "i dag (UTC-døgn)"
	}
	return "siste 24 timer"
}

func (s *server) quotaPeriodEN() string {
	if s.store.sharedSingleUse() {
		return "per UTC day"
	}
	return "per 24 hours"
}

func (s *server) useToken(ctx context.Context, token, userHash, params string) error {
	now := s.now() // one clock read, so verify and consume agree on expiry
	sig, exp, err := s.confirm.verify(token, userHash, params, now)
	if err != nil {
		return err
	}
	return s.store.useToken(ctx, userHash, sig, exp, now)
}

func (s *server) checkGroupOwner(ctx context.Context, tok string, gruppeID int64) error {
	brukerID, err := s.dolly.CurrentBrukerID(ctx, tok)
	if err != nil {
		return err
	}
	ownerID, err := s.dolly.GroupOwnerBrukerID(ctx, tok, gruppeID)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(ownerID), []byte(brukerID)) != 1 {
		return fmt.Errorf("group %d is not owned by the current user", gruppeID)
	}
	return nil
}

// shortcut: per-pod quota, move to Postgres if more replicas
type userQuota struct {
	mu      sync.Mutex
	hmacKey []byte
	nextID  uint64
	entries map[string][]quotaEntry
}

type quotaEntry struct {
	id     uint64
	at     time.Time
	antall int
}

func newUserQuota(key []byte) *userQuota {
	if len(key) < minKeyBytes {
		panic("userQuota needs USER_HASH_KEY")
	}
	return &userQuota{
		hmacKey: key,
		entries: make(map[string][]quotaEntry),
	}
}

func (q *userQuota) reserve(userHash string, antall int, now time.Time) (func(), bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	cutoff := now.Add(-quotaWindow)
	existing := q.entries[userHash]
	kept := existing[:0]
	used := 0
	for _, e := range existing {
		if e.at.After(cutoff) {
			kept = append(kept, e)
			used += e.antall
		}
	}
	if used+antall > dailyPersonQuota {
		if len(kept) == 0 {
			delete(q.entries, userHash)
		} else {
			q.entries[userHash] = kept
		}
		return nil, false
	}
	q.nextID++
	id := q.nextID
	q.entries[userHash] = append(kept, quotaEntry{id: id, at: now, antall: antall})
	return func() {
		q.mu.Lock()
		defer q.mu.Unlock()
		cur := q.entries[userHash]
		for i, e := range cur {
			if e.id == id {
				cur = append(cur[:i], cur[i+1:]...)
				break
			}
		}
		if len(cur) == 0 {
			delete(q.entries, userHash)
		} else {
			q.entries[userHash] = cur
		}
	}, true
}

func (q *userQuota) hashUser(navIdent string) string {
	mac := hmac.New(sha256.New, q.hmacKey)
	mac.Write([]byte(navIdent))
	return hex.EncodeToString(mac.Sum(nil))
}

func parseCreateOrder(fields map[string]json.RawMessage, now time.Time) (int64, dolly.BestillingRequest, *mcpRPCError) {
	gruppeID, e := parseID(fields, "gruppeId")
	if e != nil {
		return 0, dolly.BestillingRequest{}, e
	}

	rawAntall, ok := fields["antall"]
	if !ok {
		return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "antall is required"}
	}
	antall, e := parseStrictInt(rawAntall, "antall")
	if e != nil {
		return 0, dolly.BestillingRequest{}, e
	}
	if antall < minOrderAntall || antall > maxOrderAntall {
		return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("antall must be %d to %d", minOrderAntall, maxOrderAntall)}
	}

	rawEnvs, ok := fields["environments"]
	if !ok {
		return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "environments is required"}
	}
	var envItems []json.RawMessage
	if len(rawEnvs) == 0 || rawEnvs[0] != '[' || json.Unmarshal(rawEnvs, &envItems) != nil {
		return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "environments must be an array of strings"}
	}
	if len(envItems) < 1 || len(envItems) > 3 {
		return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "environments must contain 1 to 3 entries from q1, q2, q4"}
	}
	seenEnvs := make(map[string]bool, len(envItems))
	envs := make([]string, 0, len(envItems))
	for _, item := range envItems {
		env, envErr := parseStrictString(bytes.TrimSpace(item), "environments")
		if envErr != nil {
			return 0, dolly.BestillingRequest{}, envErr
		}
		// Map validated input to constants so only allowlisted literals reach the audit log.
		switch env {
		case "q1":
			env = "q1"
		case "q2":
			env = "q2"
		case "q4":
			env = "q4"
		default:
			return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "environments must be a subset of q1, q2, q4"}
		}
		if seenEnvs[env] {
			return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "environments must not contain duplicates"}
		}
		seenEnvs[env] = true
		envs = append(envs, env)
	}

	opprett := dolly.OpprettNyPerson{
		Identtype: "FNR",
		Syntetisk: true,
	}
	var minAlder, maxAlder int
	var hasMin, hasMax bool
	if rawMin, ok := fields["minAlder"]; ok {
		minAlder, e = parseStrictInt(rawMin, "minAlder")
		if e != nil {
			return 0, dolly.BestillingRequest{}, e
		}
		if minAlder < 0 || minAlder > maxOrderAge {
			return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("minAlder must be 0 to %d", maxOrderAge)}
		}
		hasMin = true
	}
	if rawMax, ok := fields["maxAlder"]; ok {
		maxAlder, e = parseStrictInt(rawMax, "maxAlder")
		if e != nil {
			return 0, dolly.BestillingRequest{}, e
		}
		if maxAlder < 0 || maxAlder > maxOrderAge {
			return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: fmt.Sprintf("maxAlder must be 0 to %d", maxOrderAge)}
		}
		hasMax = true
	}
	if hasMin && hasMax && minAlder > maxAlder {
		return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "minAlder must not exceed maxAlder"}
	}
	today := now.UTC()
	if hasMax {
		opprett.FoedtEtter = today.AddDate(-(maxAlder + 1), 0, 1).Format("2006-01-02T00:00:00")
	}
	if hasMin {
		opprett.FoedtFoer = today.AddDate(-minAlder, 0, 0).Format("2006-01-02T00:00:00")
	}

	var person *dolly.PersonDTO
	if rawKjoenn, ok := fields["kjoenn"]; ok {
		kjoenn, kErr := parseStrictString(rawKjoenn, "kjoenn")
		if kErr != nil {
			return 0, dolly.BestillingRequest{}, kErr
		}
		switch kjoenn {
		case "MANN", "KVINNE", "UKJENT":
			person = &dolly.PersonDTO{Kjoenn: []dolly.KjoennDTO{{Kjoenn: kjoenn}}}
		default:
			return 0, dolly.BestillingRequest{}, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "kjoenn must be MANN, KVINNE, or UKJENT"}
		}
	}

	return gruppeID, dolly.BestillingRequest{
		Antall:       antall,
		Environments: envs,
		Pdldata: dolly.PdlPerson{
			OpprettNyPerson: opprett,
			Person:          person,
		},
	}, nil
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
