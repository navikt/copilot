package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	maxMCPRequestBodyBytes  = 1 << 20  // 1 MiB
	maxMCPResponseBodyBytes = 10 << 20 // 10 MiB for non-SSE responses
	maxAuditMethodLen       = 64
	maxAuditBatchTools      = 10
	maxMCPSessionIDBytes    = 256
)

var allowedUpstreamRequestHeaders = map[string]bool{
	"Content-Type":         true,
	"Accept":               true,
	"Mcp-Session-Id":       true,
	"Mcp-Protocol-Version": true,
	"Last-Event-Id":        true,
	// MCP 2026-07-28 Streamable HTTP request metadata; upstream validates them against the body.
	"Mcp-Method": true,
	"Mcp-Name":   true,
}

// Mcp-Param-{Name} headers mirror tool arguments; the spec sets no limits, so the gateway does.
const (
	mcpParamHeaderPrefix   = "Mcp-Param-"
	maxMCPParamHeaders     = 32
	maxMCPParamHeaderBytes = 4096 // name + value
	// Mcp-Name carries a tool name, resource URI or prompt name; Mcp-Method a method.
	maxMCPNameHeaderBytes = 1024
	// MaxHeaderBytes covers every Mcp-Param-* at its cap (128 KiB), a bearer token
	// and the other headers with room to spare; Go's default is 1 MiB.
	maxRequestHeaderBytes = 192 << 10
)

var allowedUpstreamResponseHeaders = map[string]bool{
	"Content-Type":         true,
	"Cache-Control":        true,
	"Mcp-Session-Id":       true,
	"Mcp-Protocol-Version": true,
	"Last-Event-Id":        true,
}

var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Proxy-Connection":    true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func newHardenedProxyClient(headerTimeout time.Duration) *http.Client {
	if headerTimeout <= 0 {
		headerTimeout = defaultOutboundTimeout
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: headerTimeout,
			IdleConnTimeout:       90 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("redirects refused")
		},
	}
}

func (g *Gateway) mcpHandler(route PilotRoute) http.HandlerFunc {
	fullRouteURL := g.cfg.BaseURL + route.Route
	prmURL := g.cfg.BaseURL + "/.well-known/oauth-protected-resource" + route.Route

	return func(w http.ResponseWriter, r *http.Request) {
		start := g.now()
		outcome := "ok"
		navIdent := ""
		clientID := ""
		rpcMethod := ""
		toolName := ""
		decision := ""
		decisionReason := ""
		headerMismatch := false

		defer func() {
			slog.Info("audit mcp",
				"navident", sanitizeLogValue(navIdent),
				"client_id", sanitizeLogValue(clientID),
				"route", route.Route,
				"http_method", sanitizeLogValue(r.Method),
				"method", sanitizeLogValue(rpcMethod),
				"tool", sanitizeLogValue(toolName),
				"decision", decision,
				"decision_reason", decisionReason,
				"header_mismatch", headerMismatch,
				"outcome", outcome,
				"duration_ms", g.now().Sub(start).Milliseconds(),
			)
		}()

		switch r.Method {
		case http.MethodPost, http.MethodGet, http.MethodDelete:
		default:
			outcome = "method_not_allowed"
			w.Header().Set("Allow", "GET, POST, DELETE")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error":   "method_not_allowed",
				"message": "only GET, POST, and DELETE are supported",
			})
			return
		}

		if g.cfg.GatewayDisabled || route.isDisabled() {
			outcome = "disabled"
			slog.Warn("route disabled by kill switch", "route", route.Route, "global", g.cfg.GatewayDisabled)
			w.Header().Set("Retry-After", "300")
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error":   "temporarily_unavailable",
				"message": "this route is temporarily disabled by the gateway operator",
			})
			return
		}

		token, ok := extractBearerToken(r.Header.Get("Authorization"))
		if !ok {
			outcome = "unauthorized"
			writeUnauthorized(w, prmURL)
			return
		}

		tokenHash := hashToken(token)
		sess, ok, err := g.getSession(r.Context(), tokenHash)
		if err != nil {
			outcome = "store_error"
			slog.Warn("session lookup failed", "route", route.Route, "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error":   "temporarily_unavailable",
				"message": "session store unavailable",
			})
			return
		}
		if !ok || !g.now().Before(sess.Expiry) {
			outcome = "unauthorized"
			writeUnauthorized(w, prmURL)
			return
		}

		navIdent = sess.NAVIdent
		clientID = sess.ClientID

		if sess.Resource != fullRouteURL {
			outcome = "forbidden_resource"
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error":   "invalid_target",
				"message": "token is not bound to this route",
			})
			return
		}

		if len(r.Header.Values("Mcp-Session-Id")) > 1 {
			outcome = "bad_request"
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "invalid_request",
				"message": "only one Mcp-Session-Id header is allowed",
			})
			return
		}
		reqSessionID := r.Header.Get("Mcp-Session-Id")
		ownsReqSession := false
		if reqSessionID != "" {
			if len(reqSessionID) > maxMCPSessionIDBytes {
				outcome = "bad_request"
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error":   "invalid_request",
					"message": "Mcp-Session-Id exceeds maximum allowed length",
				})
				return
			}
			owner, found, getErr := g.store.Get(r.Context(), kindMCPSession, route.Route+"\n"+reqSessionID)
			if getErr != nil {
				outcome = "store_error"
				slog.Warn("mcp session lookup failed", "route", route.Route, "error", getErr)
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"error":   "temporarily_unavailable",
					"message": "session store unavailable",
				})
				return
			}
			if found {
				if string(owner) != sess.NAVIdent {
					outcome = "mcp_session_mismatch"
					slog.Warn("mcp session id belongs to another user",
						"navident", sanitizeLogValue(navIdent),
						"route", route.Route,
					)
					writeJSON(w, http.StatusForbidden, map[string]string{
						"error":   "forbidden",
						"message": "Mcp-Session-Id belongs to another user",
					})
					return
				}
				ownsReqSession = true
			}
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxMCPRequestBodyBytes)
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				outcome = "payload_too_large"
				writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
					"error":   "payload_too_large",
					"message": "request body exceeds maximum allowed size",
				})
				return
			}
			outcome = "bad_request"
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "invalid_request",
				"message": "failed to read request body",
			})
			return
		}

		if !mcpHeadersValid(r.Header) {
			outcome = "bad_request"
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "invalid_request",
				"message": "too many, too large, duplicate or non-ASCII Mcp-* headers",
			})
			return
		}

		decision, decisionReason = "allow", reasonPassAll
		if r.Method != http.MethodPost && route.allowedTools != nil {
			decisionReason = reasonNotCall
		}
		// MCP defines no body for GET/DELETE; reject one rather than strip it,
		// so a client sending tools/call that way learns it was never run.
		if r.Method != http.MethodPost && len(reqBody) > 0 {
			decision, decisionReason = "deny", reasonAmbiguous
			outcome = "bad_request"
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "invalid_request",
				"message": "request body is only accepted on POST",
			})
			return
		}
		if r.Method == http.MethodPost {
			rpcMethod, toolName = extractJSONRPCAudit(reqBody)
			headerMismatch = mcpHeadersMismatch(r.Header, rpcMethod, toolName)
			if route.allowedTools != nil {
				allow, reason := toolPolicyDecision(reqBody, route.allowedTools)
				decisionReason = reason
				if !allow {
					decision = "deny"
					outcome = "tool_denied"
					writeToolDenied(w, reason)
					return
				}
			}
		}

		if g.TeamTokens == nil {
			outcome = "upstream_token_error"
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "upstream_token_error",
				"message": "team token source is not configured",
			})
			return
		}

		teamToken, err := g.TeamTokens.TeamToken(r.Context(), tokenHash, fullRouteURL)
		if err != nil {
			var missingAccess *upstreamAccessMissingError
			if errors.As(err, &missingAccess) {
				outcome = "upstream_access_missing"
				writeJSON(w, http.StatusBadGateway, map[string]string{
					"error":   "upstream_access_missing",
					"message": missingAccess.Error(),
				})
				return
			}
			outcome = "upstream_token_error"
			slog.Warn("team token failed", "route", route.Route, "reason", err.Error())
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "upstream_token_error",
				"message": "failed to acquire upstream team token",
			})
			return
		}

		targetURL := buildUpstreamURL(route.ParsedURL, r.URL.RawQuery)
		var bodyReader io.Reader
		if len(reqBody) > 0 {
			bodyReader = bytes.NewReader(reqBody)
		}
		upCtx, upCancel := context.WithCancel(r.Context())
		defer upCancel()

		upReq, err := http.NewRequestWithContext(upCtx, r.Method, targetURL, bodyReader)
		if err != nil {
			outcome = "upstream_error"
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "upstream_error",
				"message": "failed to build upstream request",
			})
			return
		}

		copyAllowlistedHeaders(upReq.Header, r.Header, allowedUpstreamRequestHeaders)
		copyMCPParamHeaders(upReq.Header, r.Header)
		upReq.Header.Set("Authorization", "Bearer "+teamToken)

		client := g.proxyClient
		if client == nil {
			client = g.cfg.HTTPClient
		}
		resp, err := client.Do(upReq) //nolint:gosec // upstream URL validated from versioned pilot-routes config
		if err != nil {
			outcome = "upstream_error"
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "upstream_error",
				"message": "upstream request failed",
			})
			return
		}
		defer func() { _ = resp.Body.Close() }()

		mcpSessionTTL := g.cfg.SessionMaxAge
		if mcpSessionTTL <= 0 {
			mcpSessionTTL = defaultSessionMaxAge
		}
		bindMCPSession := func(id string) {
			key := route.Route + "\n" + id
			// Never take over an id another user already owns.
			owner, found, getErr := g.store.Get(r.Context(), kindMCPSession, key)
			if getErr != nil || (found && string(owner) != sess.NAVIdent) {
				slog.Warn("mcp session bind skipped", "route", route.Route, "owned_by_other", found, "error", getErr)
				return
			}
			if found {
				return // already ours; skipping keeps the per-user count exact
			}
			// shortcut: Get+Put on the counter is not atomic across pods, so the cap is approximate; fine as defence in depth.
			count := 0
			if raw, ok, cntErr := g.store.Get(r.Context(), kindMCPSessionUser, sess.NAVIdent); cntErr == nil && ok {
				count, _ = strconv.Atoi(string(raw))
			}
			if count >= g.cfg.MCPSessionUserCap {
				slog.Warn("mcp session bind skipped: per-user cap reached", "route", route.Route, "navident", sanitizeLogValue(navIdent))
				return
			}
			if putErr := g.store.Put(r.Context(), kindMCPSession, key, []byte(sess.NAVIdent), mcpSessionTTL); putErr != nil {
				slog.Warn("mcp session bind failed", "route", route.Route, "error", putErr)
				return
			}
			_ = g.store.Put(r.Context(), kindMCPSessionUser, sess.NAVIdent, []byte(strconv.Itoa(count+1)), mcpSessionTTL)
		}
		// A terminated session frees its binding and the user's slot under the cap.
		if r.Method == http.MethodDelete && ownsReqSession && resp.StatusCode < 300 {
			// Pop, not Delete: only the request that removed the binding frees the slot,
			// so concurrent DELETEs of one session cannot decrement twice.
			if _, removed, delErr := g.store.Pop(r.Context(), kindMCPSession, route.Route+"\n"+reqSessionID); delErr != nil {
				slog.Warn("mcp session unbind failed", "route", route.Route, "error", delErr)
			} else if raw, ok, cntErr := g.store.Get(r.Context(), kindMCPSessionUser, sess.NAVIdent); removed && cntErr == nil && ok {
				// shortcut: same non-atomic Get+Put as bindMCPSession; the cap stays approximate.
				if count, _ := strconv.Atoi(string(raw)); count > 0 {
					_ = g.store.Put(r.Context(), kindMCPSessionUser, sess.NAVIdent, []byte(strconv.Itoa(count-1)), mcpSessionTTL)
				}
			}
		}
		// Bind only ids the upstream issued on a successful response (MCP streamable HTTP).
		// A client-supplied id is never bound; an unbound one passes through for stateless upstreams.
		// A DELETE ends the session, so an id echoed on its response is not bound again.
		if resp.StatusCode < 300 && r.Method != http.MethodDelete {
			for _, id := range resp.Header.Values("Mcp-Session-Id") {
				if id != "" && len(id) <= maxMCPSessionIDBytes {
					bindMCPSession(id)
				}
			}
		}

		if isEventStream(resp.Header.Get("Content-Type")) {
			copyAllowlistedHeaders(w.Header(), resp.Header, allowedUpstreamResponseHeaders)
			rc := http.NewResponseController(w)
			_ = rc.SetWriteDeadline(time.Time{})
			w.WriteHeader(resp.StatusCode)
			_ = rc.Flush()
			if streamErr := streamSSE(upCtx, w, rc, resp.Body, g.cfg.SSEMaxDuration, g.cfg.SSEIdleTimeout, g.cfg.SSEMaxBytes); streamErr != nil {
				outcome = "stream_error"
			} else if resp.StatusCode >= 400 {
				outcome = fmt.Sprintf("upstream_%d", resp.StatusCode)
			}
			return
		}

		maxRespBytes := g.cfg.MaxResponseBodyBytes
		if maxRespBytes <= 0 {
			maxRespBytes = maxMCPResponseBodyBytes
		}
		if resp.ContentLength > maxRespBytes {
			outcome = "response_too_large"
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "response_too_large",
				"message": "upstream response exceeds maximum allowed size",
			})
			return
		}

		bodyTimeout := g.cfg.ProxyHeaderTimeout
		if bodyTimeout <= 0 {
			bodyTimeout = defaultOutboundTimeout
		}
		bodyTimer := time.AfterFunc(bodyTimeout, upCancel)
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes+1))
		bodyTimer.Stop()
		if err != nil {
			outcome = "stream_error"
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "upstream_error",
				"message": "failed to read upstream response",
			})
			return
		}
		if int64(len(respBody)) > maxRespBytes {
			outcome = "response_too_large"
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "response_too_large",
				"message": "upstream response exceeds maximum allowed size",
			})
			return
		}

		copyAllowlistedHeaders(w.Header(), resp.Header, allowedUpstreamResponseHeaders)
		w.WriteHeader(resp.StatusCode)
		if len(respBody) > 0 {
			if _, err := w.Write(respBody); err != nil {
				outcome = "stream_error"
				return
			}
		}
		if resp.StatusCode >= 400 {
			outcome = fmt.Sprintf("upstream_%d", resp.StatusCode)
		}
	}
}

func streamSSE(
	ctx context.Context,
	w io.Writer,
	rc *http.ResponseController,
	body io.ReadCloser,
	maxDuration time.Duration,
	idleTimeout time.Duration,
	maxBytes int64,
) error {
	streamCtx, cancelStream := context.WithCancelCause(ctx)
	defer cancelStream(nil)

	if maxDuration > 0 {
		maxTimer := time.AfterFunc(maxDuration, func() {
			cancelStream(errors.New("sse max duration exceeded"))
		})
		defer maxTimer.Stop()
	}

	var idleTimer *time.Timer
	if idleTimeout > 0 {
		idleTimer = time.AfterFunc(idleTimeout, func() {
			cancelStream(errors.New("sse idle timeout exceeded"))
		})
		defer idleTimer.Stop()
	}

	stopAfter := context.AfterFunc(streamCtx, func() {
		_ = body.Close()
	})
	defer stopAfter()

	var totalBytes int64
	buf := make([]byte, 32*1024)
	for {
		if streamCtx.Err() != nil {
			return context.Cause(streamCtx)
		}
		n, err := body.Read(buf)
		if n > 0 {
			if streamCtx.Err() != nil {
				return context.Cause(streamCtx)
			}
			if maxBytes > 0 && totalBytes+int64(n) > maxBytes {
				remaining := maxBytes - totalBytes
				if remaining > 0 {
					if _, werr := w.Write(buf[:remaining]); werr != nil {
						return werr
					}
					_ = rc.Flush()
				}
				return errors.New("sse max bytes exceeded")
			}
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			_ = rc.Flush()
			totalBytes += int64(n)
			if idleTimer != nil {
				idleTimer.Reset(idleTimeout)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if cause := context.Cause(streamCtx); cause != nil {
				return cause
			}
			return err
		}
	}
}

// writeToolDenied answers a denied POST with a JSON-RPC error and never
// forwards it. An ambiguous body is a malformed request: HTTP 400 with
// -32600 Invalid Request. A tool outside the allowlist is HTTP 403 with
// -32602, the code MCP uses for an unknown tool, so clients treat it as a
// tool that does not exist. The id is null: for a batch or an ambiguous body
// the gateway cannot name one id without guessing.
func writeToolDenied(w http.ResponseWriter, reason string) {
	status, code, msg := http.StatusForbidden, -32602, "tool is not allowed on this route"
	if reason == reasonAmbiguous {
		status, code, msg = http.StatusBadRequest, -32600, "invalid_request: ambiguous JSON-RPC body"
	}
	writeJSON(w, status, map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error":   map[string]any{"code": code, "message": msg},
	})
}

func writeUnauthorized(w http.ResponseWriter, prmURL string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s"`, prmURL))
	writeJSON(w, http.StatusUnauthorized, map[string]string{
		"error":   "invalid_token",
		"message": "missing, invalid, or expired bearer token",
	})
}

func extractBearerToken(authHeader string) (string, bool) {
	if len(authHeader) <= len("Bearer ") {
		return "", false
	}
	if !strings.EqualFold(authHeader[:len("Bearer ")], "Bearer ") {
		return "", false
	}
	tok := strings.TrimSpace(authHeader[len("Bearer "):])
	if tok == "" || strings.ContainsAny(tok, " \t\r\n") {
		return "", false
	}
	return tok, true
}

func buildUpstreamURL(base *url.URL, rawQuery string) string {
	u := *base
	if rawQuery != "" {
		if u.RawQuery != "" {
			u.RawQuery = u.RawQuery + "&" + rawQuery
		} else {
			u.RawQuery = rawQuery
		}
	}
	return u.String()
}

func copyAllowlistedHeaders(dst, src http.Header, allowlist map[string]bool) {
	connTokens := connectionHeaderTokens(src)
	for k, vals := range src {
		ck := textproto.CanonicalMIMEHeaderKey(k)
		if hopByHopHeaders[ck] || connTokens[ck] || !allowlist[ck] {
			continue
		}
		for _, v := range vals {
			dst.Add(ck, v)
		}
	}
}

func isMCPParamHeader(name string) bool {
	ck := textproto.CanonicalMIMEHeaderKey(name)
	return len(ck) > len(mcpParamHeaderPrefix) && strings.HasPrefix(ck, mcpParamHeaderPrefix)
}

// mcpHeadersValid caps Mcp-Param-*, Mcp-Name and Mcp-Method, rejects a repeated
// Mcp-Name or Mcp-Method, and rejects obs-text (0x80-0xFF) in any Mcp-* value:
// the spec requires ASCII, and upstreams that ignore these headers never check.
func mcpHeadersValid(h http.Header) bool {
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
		case isMCPParamHeader(ck):
			for _, v := range vals {
				n++
				if n > maxMCPParamHeaders || len(ck)+len(v) > maxMCPParamHeaderBytes {
					return false
				}
			}
		case ck == "Mcp-Name" || ck == "Mcp-Method":
			if len(vals) > 1 || len(vals[0]) > maxMCPNameHeaderBytes {
				return false
			}
		}
	}
	return true
}

// mcpHeadersMismatch reports whether Mcp-Method or Mcp-Name disagrees with the
// audited body. The body is the truth; the headers only feed this audit field.
// Mcp-Name is compared for tools/call only, where the audit has the tool name.
func mcpHeadersMismatch(h http.Header, method, tool string) bool {
	if m := h.Get("Mcp-Method"); m != "" && m != method {
		return true
	}
	name := h.Get("Mcp-Name")
	if name == "" || method != "tools/call" || tool == auditIncomplete {
		return false
	}
	return decodeMCPHeaderValue(name) != tool
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

// copyMCPParamHeaders forwards Mcp-Param-* unchanged, as the spec requires of intermediaries.
// Callers must check mcpHeadersValid first.
func copyMCPParamHeaders(dst, src http.Header) {
	connTokens := connectionHeaderTokens(src)
	for k, vals := range src {
		ck := textproto.CanonicalMIMEHeaderKey(k)
		if !isMCPParamHeader(ck) || connTokens[ck] {
			continue
		}
		for _, v := range vals {
			dst.Add(ck, v)
		}
	}
}

func connectionHeaderTokens(h http.Header) map[string]bool {
	tokens := make(map[string]bool)
	for _, val := range h.Values("Connection") {
		for _, part := range strings.Split(val, ",") {
			name := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(part))
			if name != "" {
				tokens[name] = true
			}
		}
	}
	return tokens
}

func isEventStream(contentType string) bool {
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "text/event-stream")
}

// auditIncomplete marks an audit line whose scan stopped before the end of the
// body, so a name the upstream reads past that point is never logged as absent.
const auditIncomplete = "...incomplete"

// extractJSONRPCAudit returns the JSON-RPC method and, for tools/call, the tool
// name. Arguments are never decoded into anything that is logged. A batch is
// logged as method "batch" with the names of up to maxAuditBatchTools
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
func extractJSONRPCAudit(body []byte) (method, tool string) {
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if err != nil {
		// A body the scan cannot start on (a UTF-8 BOM, say, which some upstream
		// parsers accept) must not log as if it carried no call.
		if len(body) > 0 {
			return "", auditIncomplete
		}
		return "", ""
	}
	switch tok {
	case json.Delim('{'):
		m, params, ok := auditScanObject(dec)
		if !ok || !auditAtEOF(dec) {
			return "", auditIncomplete
		}
		return auditMethodTool(m, params)
	case json.Delim('['):
		var tools []string
		complete := true
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
			// name is set only for tools/call, also when m is quoted as unusual.
			if m, name := auditMethodTool(m, params); name != "" || m == "tools/call" {
				if len(tools) == maxAuditBatchTools {
					tools = append(tools, "...")
					break
				}
				tools = append(tools, name)
			}
		}
		if len(tools) <= maxAuditBatchTools {
			// More() is also false on a syntax error, so require the closing ']'.
			if tok, err := dec.Token(); !complete || err != nil || tok != json.Delim(']') || !auditAtEOF(dec) {
				tools = append(tools, auditIncomplete)
			}
		}
		if len(tools) == 0 {
			return "batch", ""
		}
		list, _ := json.Marshal(tools)
		return "batch", string(list)
	}
	return "", ""
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
func auditMethodTool(rawMethod, params json.RawMessage) (method, tool string) {
	method, tool, ambiguous := rpcMethodTool(rawMethod, params)
	if ambiguous {
		return auditField(method), auditIncomplete
	}
	return auditField(method), auditField(tool)
}

// auditField returns s as is when it is short visible ASCII, which every MCP
// method and tool name is. Anything else (spaces, NUL, U+200B, U+FEFF, U+00A0)
// is logged Go-quoted and ASCII-escaped, so "whoami " can never read as the
// listed tool whoami and an invisible character never hides in the audit line.
func auditField(s string) string {
	visible := len(s) <= maxAuditMethodLen
	for i := 0; visible && i < len(s); i++ {
		visible = s[i] > 0x20 && s[i] < 0x7f
	}
	if visible {
		return s
	}
	trunc := ""
	if r := []rune(s); len(r) > maxAuditMethodLen {
		s, trunc = string(r[:maxAuditMethodLen]), "..."
	}
	return strconv.QuoteToASCII(s) + trunc
}

// rpcMethodTool returns the unsanitized method and, for tools/call, the
// unsanitized params.name, so audit and enforcement read the body the same way.
// ambiguous is set for a case-variant or duplicate name key, and for a method
// that only reads as tools/call once control characters or spaces are dropped.
func rpcMethodTool(rawMethod, params json.RawMessage) (method, tool string, ambiguous bool) {
	_ = json.Unmarshal(rawMethod, &method)
	if sanitizeAuditMethod(method) != "tools/call" {
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
	reasonPassAll   = "pass_all"
	reasonListed    = "listed"
	reasonNotCall   = "not_tools_call"
	reasonNotListed = "not_listed"
	reasonAmbiguous = "ambiguous"
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
		return false, reasonAmbiguous
	}
	sawCall, notListed := false, false
	member := func() (bool, string) {
		m, params, ok := auditScanObject(dec)
		if !ok {
			return false, reasonAmbiguous
		}
		method, tool, ambiguous := rpcMethodTool(m, params)
		switch {
		case ambiguous || (strings.EqualFold(strings.TrimSpace(method), "tools/call") && method != "tools/call"):
			return false, reasonAmbiguous
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
				return false, reasonAmbiguous
			}
			if tok != json.Delim('{') {
				if !auditSkip(dec, tok) {
					return false, reasonAmbiguous
				}
				continue
			}
			if ok, reason := member(); !ok {
				return false, reason
			}
		}
		// More() is also false on a syntax error, so require the closing ']'.
		if tok, err := dec.Token(); err != nil || tok != json.Delim(']') {
			return false, reasonAmbiguous
		}
	default:
		return false, reasonAmbiguous
	}
	if !auditAtEOF(dec) {
		return false, reasonAmbiguous
	}
	if notListed {
		return false, reasonNotListed
	}
	if sawCall {
		return true, reasonListed
	}
	return true, reasonNotCall
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
	b.Grow(min(len(method), maxAuditMethodLen))
	for _, r := range method {
		if b.Len() >= maxAuditMethodLen {
			break
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
