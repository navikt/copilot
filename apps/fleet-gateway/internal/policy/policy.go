// Package policy holds the gateway's pure request checks: JSON-RPC parsing for
// the audit line, the per-route tool allowlist, and Mcp-* header validation and
// header-versus-body mismatch. It writes no response and keeps no state.
package policy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"unicode"
)

// Audit display caps: method and tool length, and tools/call names listed per batch.
const (
	MaxAuditMethodLen  = 64
	MaxAuditBatchTools = 10
)

// Mcp-Param-{Name} headers mirror tool arguments; the spec sets no limits, so the gateway does.
const (
	mcpParamHeaderPrefix   = "Mcp-Param-"
	MaxMCPParamHeaders     = 32
	MaxMCPParamHeaderBytes = 4096 // name + value
	// Mcp-Name carries a tool name, resource URI or prompt name; Mcp-Method a method.
	MaxMCPNameHeaderBytes = 1024
)

// IsMCPParamHeader reports whether name is an Mcp-Param-{Name} header.
func IsMCPParamHeader(name string) bool {
	ck := textproto.CanonicalMIMEHeaderKey(name)
	return len(ck) > len(mcpParamHeaderPrefix) && strings.HasPrefix(ck, mcpParamHeaderPrefix)
}

// HeadersValid caps Mcp-Param-*, Mcp-Name and Mcp-Method, rejects a repeated
// Mcp-Name or Mcp-Method, and rejects obs-text (0x80-0xFF) in any Mcp-* value:
// the spec requires ASCII, and upstreams that ignore these headers never check.
func HeadersValid(h http.Header) bool {
	n := 0
	for k, vals := range h {
		ck := textproto.CanonicalMIMEHeaderKey(k)
		if !strings.HasPrefix(ck, "Mcp-") {
			continue
		}
		for _, v := range vals {
			for i := 0; i < len(v); i++ {
				if v[i] >= 0x80 {
					return false
				}
			}
		}
		switch {
		case IsMCPParamHeader(ck):
			for _, v := range vals {
				n++
				if n > MaxMCPParamHeaders || len(ck)+len(v) > MaxMCPParamHeaderBytes {
					return false
				}
			}
		case ck == "Mcp-Name" || ck == "Mcp-Method":
			if len(vals) > 1 || len(vals[0]) > MaxMCPNameHeaderBytes {
				return false
			}
		}
	}
	return true
}

// mcpHeadersMismatch reports whether Mcp-Method or Mcp-Name disagrees with the
// audited body. The body is the truth; the headers only feed this audit field.
// Mcp-Name is compared for tools/call only, where the audit has the tool name.
// Headers are compared with the raw body values, never the quoted audit form.
// Only POST is checked: GET and DELETE carry no body (one is rejected above),
// so there is nothing for the headers to disagree with. Mcp-Method and Mcp-Name
// are forwarded either way and never decide policy, so a Connection-listed
// Mcp-* header on those methods changes no decision and needs no audit flag.
func mcpHeadersMismatch(h http.Header, a rpcAudit) bool {
	if m := h.Get("Mcp-Method"); m != "" && decodeMCPHeaderValue(m) != a.rawMethod {
		return true
	}
	name := h.Get("Mcp-Name")
	if name == "" || a.rawMethod != "tools/call" || a.incomplete {
		return false
	}
	return decodeMCPHeaderValue(name) != a.rawTool
}

// decodeMCPHeaderValue undoes the spec's =?base64?...?= wrapping for non-ASCII values.
func decodeMCPHeaderValue(v string) string {
	if inner, ok := strings.CutPrefix(v, "=?base64?"); ok {
		if inner, ok = strings.CutSuffix(inner, "?="); ok {
			if b, err := base64.StdEncoding.DecodeString(inner); err == nil {
				return string(b)
			}
		}
	}
	return v
}

// auditIncomplete is the display marker for a scan that stopped before the end
// of the body, so a name the upstream reads past that point is never logged as
// absent. Code reads rpcAudit.incomplete, never this string: a tool may be
// named "...incomplete".
const auditIncomplete = "...incomplete"

// rpcAudit is what the audit line records about a JSON-RPC body. method and
// tool are display values (quoted by auditField); rawMethod and rawTool are the
// decoded body values, for comparison with Mcp-Method and Mcp-Name.
type rpcAudit struct {
	method, tool       string
	rawMethod, rawTool string
	incomplete         bool
}

// scanJSONRPCAudit returns the JSON-RPC method and, for tools/call, the tool
// name. Arguments are never decoded into anything that is logged. A batch is
// logged as method "batch" with the names of up to MaxAuditBatchTools
// tools/call entries as a JSON array, so a name cannot forge extra entries. The
// body is already capped at maxMCPRequestBodyBytes and is read only, so
// upstream receives it byte for byte.
//
// Upstream Go servers match keys case-insensitively, last wins, so a
// case-variant or duplicate method, params or name key, or data after the
// first top-level value, logs as incomplete rather than guessing. One token
// decoder walks the whole body and keeps only method and params, never a slice
// of members or a map of keys: a 1 MiB body of {} members or of 50k keys
// otherwise cost tens of MiB, near the pod's 64 MiB limit.
func scanJSONRPCAudit(body []byte) rpcAudit {
	incomplete := rpcAudit{tool: auditIncomplete, incomplete: true}
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		// A body the scan cannot start on (a UTF-8 BOM, say, which some upstream
		// parsers accept) must not log as if it carried no call.
		if len(body) > 0 {
			return incomplete
		}
		return rpcAudit{}
	}
	switch tok {
	case json.Delim('{'):
		m, params, ok := auditScanObject(dec)
		if !ok || !auditAtEOF(dec) {
			return incomplete
		}
		return auditMethodTool(m, params)
	case json.Delim('['):
		var tools []string
		complete := true
		batchIncomplete := false
		for complete && dec.More() {
			tok, err := dec.Token()
			if err != nil {
				complete = false
				break
			}
			if tok != json.Delim('{') {
				complete = auditSkip(dec, tok)
				continue
			}
			m, params, ok := auditScanObject(dec)
			if !ok {
				complete = false
				break
			}
			if a := auditMethodTool(m, params); a.rawMethod == "tools/call" || a.incomplete {
				name := a.tool
				batchIncomplete = batchIncomplete || a.incomplete
				// The cap limits only the logged list; the scan reads every member.
				switch {
				case len(tools) < MaxAuditBatchTools:
					tools = append(tools, name)
				case len(tools) == MaxAuditBatchTools:
					tools = append(tools, "...")
				}
			}
		}
		batch := rpcAudit{method: "batch", rawMethod: "batch", incomplete: batchIncomplete}
		// More() is also false on a syntax error, so require the closing ']'.
		if tok, err := dec.Token(); !complete || err != nil || tok != json.Delim(']') || !auditAtEOF(dec) {
			tools = append(tools, auditIncomplete)
			batch.incomplete = true
		}
		if len(tools) > 0 {
			list, _ := json.Marshal(tools)
			batch.tool = string(list)
		}
		return batch
	}
	return rpcAudit{}
}

// auditAtEOF reports whether only whitespace follows the top-level value.
func auditAtEOF(dec *json.Decoder) bool {
	_, err := dec.Token()
	return err == io.EOF
}

// auditKeyAmbiguous reports whether key is a case variant of want, or a repeat
// of want already seen; upstream may read either copy.
func auditKeyAmbiguous(key json.Token, want string, seen *bool) bool {
	k, _ := key.(string)
	if k == want {
		dup := *seen
		*seen = true
		return dup
	}
	return strings.EqualFold(k, want)
}

// auditMethodTool decodes the raw method and, for tools/call, params.name. A
// field of the wrong type is left empty without dropping the other.
func auditMethodTool(rawMethod, params json.RawMessage) rpcAudit {
	method, tool, ambiguous := rpcMethodTool(rawMethod, params)
	if ambiguous {
		return rpcAudit{method: auditField(method), tool: auditIncomplete, rawMethod: method, incomplete: true}
	}
	return rpcAudit{method: auditField(method), tool: auditField(tool), rawMethod: method, rawTool: tool}
}

// auditField returns s as is when it is short visible ASCII without a quote or
// backslash, which every MCP method and tool name is. A quote or backslash is
// quoted too, or a literal name could read like the quoted form of another. Anything else (spaces, NUL, U+200B, U+FEFF, U+00A0)
// is logged Go-quoted and ASCII-escaped, so "whoami " can never read as the
// listed tool whoami and an invisible character never hides in the audit line.
func auditField(s string) string {
	visible := len(s) <= MaxAuditMethodLen
	for i := 0; visible && i < len(s); i++ {
		visible = s[i] > 0x20 && s[i] < 0x7f && s[i] != '"' && s[i] != '\\'
	}
	if visible {
		return s
	}
	trunc := ""
	if r := []rune(s); len(r) > MaxAuditMethodLen {
		s, trunc = string(r[:MaxAuditMethodLen]), "..."
	}
	return strconv.QuoteToASCII(s) + trunc
}

// rpcMethodTool returns the unsanitized method and, for tools/call, the
// unsanitized params.name, so audit and enforcement read the body the same way.
// ambiguous is set for a case-variant or duplicate name key, and for a method
// that only reads as tools/call once control characters or spaces are dropped.
func rpcMethodTool(rawMethod, params json.RawMessage) (method, tool string, ambiguous bool) {
	_ = json.Unmarshal(rawMethod, &method)
	if !strings.EqualFold(sanitizeAuditMethod(method), "tools/call") {
		return method, "", false
	}
	if method != "tools/call" {
		return method, "", true
	}
	if len(params) == 0 {
		return method, "", false
	}
	dec := json.NewDecoder(bytes.NewReader(params))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return method, "", false
	}
	var skip, name json.RawMessage
	var seen bool
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return method, "", false
		}
		if auditKeyAmbiguous(key, "name", &seen) {
			return method, "", true
		}
		dst := &skip
		if key == "name" {
			dst = &name
		}
		if dec.Decode(dst) != nil {
			return method, "", false
		}
	}
	_ = json.Unmarshal(name, &tool)
	return method, tool, false
}

// Tool policy decision reasons, logged in the audit line.
const (
	ReasonPassAll   = "pass_all"
	ReasonListed    = "listed"
	ReasonNotCall   = "not_tools_call"
	ReasonNotListed = "not_listed"
	ReasonAmbiguous = "ambiguous"
	// ReasonHeaderMismatch: Mcp-Method/Mcp-Name disagree with the body on a route with an allowlist.
	ReasonHeaderMismatch = "header_mismatch"
)

// toolPolicyDecision decides a POST body against a route's tool allowlist,
// walking it like extractJSONRPCAudit. Anything the scan cannot read without
// guessing (case-variant or duplicate method/params/name keys, trailing data,
// unparseable or incomplete JSON) is denied as ambiguous, because upstream Go
// servers match keys case-insensitively and may run a tool the scan never saw.
// A batch is denied whole if any member is ambiguous or not listed.
func toolPolicyDecision(body []byte, allowed map[string]bool) (allow bool, reason string) {
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		return false, ReasonAmbiguous
	}
	sawCall, notListed := false, false
	member := func() (bool, string) {
		m, params, ok := auditScanObject(dec)
		if !ok {
			return false, ReasonAmbiguous
		}
		method, tool, ambiguous := rpcMethodTool(m, params)
		switch {
		case ambiguous || (strings.EqualFold(strings.TrimSpace(method), "tools/call") && method != "tools/call"):
			return false, ReasonAmbiguous
		case method != "tools/call":
			return true, ""
		case !allowed[tool]:
			// Keep scanning: ambiguity later in the body outranks not_listed.
			notListed = true
			return true, ""
		}
		sawCall = true
		return true, ""
	}
	switch tok {
	case json.Delim('{'):
		if ok, reason := member(); !ok {
			return false, reason
		}
	case json.Delim('['):
		for dec.More() {
			tok, err := dec.Token()
			if err != nil {
				return false, ReasonAmbiguous
			}
			if tok != json.Delim('{') {
				if !auditSkip(dec, tok) {
					return false, ReasonAmbiguous
				}
				continue
			}
			if ok, reason := member(); !ok {
				return false, reason
			}
		}
		// More() is also false on a syntax error, so require the closing ']'.
		if tok, err := dec.Token(); err != nil || tok != json.Delim(']') {
			return false, ReasonAmbiguous
		}
	default:
		return false, ReasonAmbiguous
	}
	if !auditAtEOF(dec) {
		return false, ReasonAmbiguous
	}
	if notListed {
		return false, ReasonNotListed
	}
	if sawCall {
		return true, ReasonListed
	}
	return true, ReasonNotCall
}

// auditScanObject reads one object whose '{' is consumed and returns the raw
// method and params values. Other values are skipped into a reused buffer.
func auditScanObject(dec *json.Decoder) (method, params json.RawMessage, ok bool) {
	var skip json.RawMessage
	var seenM, seenP bool
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, nil, false
		}
		if auditKeyAmbiguous(key, "method", &seenM) || auditKeyAmbiguous(key, "params", &seenP) {
			return nil, nil, false
		}
		dst := &skip
		switch key {
		case "method":
			dst = &method
		case "params":
			dst = &params
		}
		if dec.Decode(dst) != nil {
			return nil, nil, false
		}
	}
	_, err := dec.Token() // '}'
	return method, params, err == nil
}

// auditSkip consumes the rest of a non-object batch member whose first token
// is tok. It gives up past maxAuditSkipDepth: the decoder's delimiter stack
// grew to 27 MiB on a 1 MiB body of [, and no JSON-RPC member nests that deep.
func auditSkip(dec *json.Decoder, tok json.Token) bool {
	const maxAuditSkipDepth = 64
	depth := 0
	for depth <= maxAuditSkipDepth {
		switch tok {
		case json.Delim('['), json.Delim('{'):
			depth++
		case json.Delim(']'), json.Delim('}'):
			depth--
		}
		if depth == 0 {
			return true
		}
		var err error
		if tok, err = dec.Token(); err != nil {
			return false
		}
	}
	return false
}

func sanitizeAuditMethod(method string) string {
	method = strings.TrimSpace(method)
	if method == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(min(len(method), MaxAuditMethodLen))
	for _, r := range method {
		if b.Len() >= MaxAuditMethodLen {
			break
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ProtocolVersion caps MCP-Protocol-Version at 32 bytes and quotes it like
// other audit values; it records which revision clients send before enforcing one.
func ProtocolVersion(v string) string {
	if len(v) > 32 {
		v = v[:32]
	}
	return auditField(v)
}

// Decision is the policy outcome for one MCP request. Method, Tool,
// ScanIncomplete and HeaderMismatch feed the audit line and are set for POST
// only. NonPostBody marks a GET or DELETE that carried a body.
type Decision struct {
	Allow        bool
	Reason       string
	Method, Tool string
	// RawTool is the full parsed tool name (Tool is the truncated display value), for matching against an allowlist.
	RawTool        string
	ScanIncomplete bool
	HeaderMismatch bool
	NonPostBody    bool
}

// Decide applies the tool policy to a request whose Mcp-* headers already
// passed HeadersValid. allowed is the route's tool allowlist; nil passes all.
func Decide(httpMethod string, allowed map[string]bool, h http.Header, body []byte) Decision {
	d := Decision{Allow: true, Reason: ReasonPassAll}
	if httpMethod != http.MethodPost && allowed != nil {
		d.Reason = ReasonNotCall
	}
	// MCP defines no body for GET/DELETE; reject one rather than strip it,
	// so a client sending tools/call that way learns it was never run.
	if httpMethod != http.MethodPost && len(body) > 0 {
		return Decision{Reason: ReasonAmbiguous, NonPostBody: true}
	}
	if httpMethod != http.MethodPost {
		return d
	}
	audit := scanJSONRPCAudit(body)
	d.Method, d.Tool, d.ScanIncomplete = audit.method, audit.tool, audit.incomplete
	d.RawTool = audit.rawTool
	d.HeaderMismatch = mcpHeadersMismatch(h, audit)
	if allowed == nil {
		return d
	}
	d.Allow, d.Reason = toolPolicyDecision(body, allowed)
	if !d.Allow {
		return d
	}
	// The upstream gets these headers unchanged; one that dispatches on Mcp-Name
	// would run a tool the allowlist never saw. MCP 2026-07-28 answers -32020.
	// Mcp-Name only has a meaning on a single complete tools/call; on a batch or any other method it is spoofing.
	if d.HeaderMismatch || (h.Get("Mcp-Name") != "" && (audit.rawMethod != "tools/call" || audit.incomplete)) {
		d.Allow, d.Reason = false, ReasonHeaderMismatch
	}
	return d
}
