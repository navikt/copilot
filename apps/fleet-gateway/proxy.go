package main

import (
	"context"
	"crypto/rand"
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
	"sync"
	"sync/atomic"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/audit"
	"github.com/navikt/copilot/fleet-gateway/internal/policy"
)

const (
	maxMCPRequestBodyBytes  = 1 << 20  // 1 MiB
	maxMCPResponseBodyBytes = 10 << 20 // 10 MiB for non-SSE responses
	maxAuditMethodLen       = policy.MaxAuditMethodLen
	maxAuditBatchTools      = policy.MaxAuditBatchTools
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

const (
	maxMCPParamHeaders     = policy.MaxMCPParamHeaders
	maxMCPParamHeaderBytes = policy.MaxMCPParamHeaderBytes
	maxMCPNameHeaderBytes  = policy.MaxMCPNameHeaderBytes
	// MaxHeaderBytes covers every Mcp-Param-* at its cap (128 KiB), a bearer token
	// and the other headers with room to spare; Go's default is 1 MiB.
	maxRequestHeaderBytes = 192 << 10
)

// Tool policy decision reasons, logged in the audit line.
const (
	reasonPassAll        = policy.ReasonPassAll
	reasonListed         = policy.ReasonListed
	reasonNotCall        = policy.ReasonNotCall
	reasonNotListed      = policy.ReasonNotListed
	reasonAmbiguous      = policy.ReasonAmbiguous
	reasonHeaderMismatch = policy.ReasonHeaderMismatch
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
			// Go's default is 10 MiB, outside the in-flight memory budget.
			MaxResponseHeaderBytes: 64 << 10,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("redirects refused")
		},
	}
}

func (g *Gateway) mcpHandler(route PilotRoute) http.HandlerFunc {
	fullRouteURL := g.cfg.BaseURL + route.Route
	prmURL := g.cfg.BaseURL + "/.well-known/oauth-protected-resource" + route.Route
	var routeInFlight slots // nil, like g.inFlight, for Gateways built without routes()
	if g.inFlight != nil {
		routeInFlight = make(slots, min(g.cfg.MCPMaxInFlightPerRoute, cap(g.inFlight)))
	}

	return func(w http.ResponseWriter, r *http.Request) {
		start := g.now()
		outcome := "ok"
		navIdent := ""
		clientID := ""
		rpcMethod := ""
		toolName := ""
		rawTool := ""
		decision := ""
		decisionReason := ""
		headerMismatch := false
		scanIncomplete := false

		requestID := rand.Text()

		// One OpenID AuthZEN-shaped record per decision (owner decision 2026-11-18, #2342).
		// decision is true only when the gateway's own checks let the request through;
		// context.outcome tells what happened after that.
		defer func() {
			observeMCPRequest(route, outcome, decision, decisionReason, toolName, rawTool, g.now().Sub(start))
			audit.LogMCP(audit.MCP{
				NAVIdent: navIdent, ClientID: clientID,
				Method: rpcMethod, Tool: toolName,
				Route: strings.TrimPrefix(route.Route, "/mcp/"), Backend: route.Target,
				RequestID: requestID, HTTPMethod: r.Method,
				ScanIncomplete: scanIncomplete, HeaderMismatch: headerMismatch,
				ProtocolVersion: policy.ProtocolVersion(r.Header.Get("Mcp-Protocol-Version")),
				HasMcpMethod:    r.Header.Get("Mcp-Method") != "",
				HasMcpName:      r.Header.Get("Mcp-Name") != "",
				Outcome:         outcome, DurationMS: g.now().Sub(start).Milliseconds(),
				Decision: decision, DecisionReason: decisionReason,
			})
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
			markShed(w, "disabled")
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
			writeUnauthorized(w, prmURL, r.Header.Get("Authorization") != "")
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
			writeUnauthorized(w, prmURL, true)
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
			if errors.Is(getErr, errSealUnknownKID) {
				// Fail closed: treating the row as missing would let any caller ride the owner's upstream session.
				outcome = "mcp_session_unverifiable"
				slog.Warn("mcp session binding sealed with unknown key id", "route", route.Route)
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error":   "forbidden",
					"message": "Mcp-Session-Id cannot be verified; start a new MCP session",
				})
				return
			}
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

		// After auth, so bogus bearers cannot take slots; before the body read,
		// since the cap bounds buffered memory. The per-user cap comes first so
		// one user cannot fill a route or pod; the route cap comes next so one
		// slow upstream cannot hold every global slot.
		userKey := g.mcpSessionUserKey(sess.NAVIdent)
		releaseUserSlot, ok := g.userInFlight.tryAcquire(userKey)
		if !ok {
			outcome = "user_cap"
			decision = "deny"
			decisionReason = "user_cap"
			markShed(w, "user_cap")
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error":   "too_many_requests",
				"message": "too many concurrent MCP requests for this user; retry shortly",
			})
			return
		}
		defer releaseUserSlot()
		releaseRouteSlot, ok := routeInFlight.tryAcquire()
		if !ok {
			outcome = "over_capacity"
			writeOverCapacity(w, "MCP requests on this route")
			return
		}
		defer releaseRouteSlot()
		releaseGlobalSlot, ok := g.inFlight.tryAcquire()
		if !ok {
			outcome = "over_capacity"
			writeOverCapacity(w, "MCP requests")
			return
		}
		defer releaseGlobalSlot()
		releaseSlot := func() { releaseGlobalSlot(); releaseRouteSlot(); releaseUserSlot() }

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

		if !policy.HeadersValid(r.Header) {
			outcome = "bad_request"
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "invalid_request",
				"message": "too many, too large, duplicate or non-ASCII Mcp-* headers",
			})
			return
		}

		pd := policy.Decide(r.Method, route.allowedTools, r.Header, reqBody)
		rpcMethod, toolName, scanIncomplete, headerMismatch = pd.Method, pd.Tool, pd.ScanIncomplete, pd.HeaderMismatch
		rawTool = pd.RawTool
		decision, decisionReason = "allow", pd.Reason
		if !pd.Allow {
			decision = "deny"
		}
		switch {
		case pd.NonPostBody:
			outcome = "bad_request"
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "invalid_request",
				"message": "request body is only accepted on POST",
			})
			return
		case pd.Reason == reasonHeaderMismatch:
			outcome = "tool_denied"
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"jsonrpc": "2.0",
				"id":      nil,
				"error":   map[string]any{"code": -32020, "message": "Mcp-Method or Mcp-Name does not match the request body"},
			})
			return
		case !pd.Allow:
			outcome = "tool_denied"
			writeToolDenied(w, pd.Reason)
			return
		}

		// Reserve the SSE slot before the upstream call: a 503 after the tool
		// already ran, or after a session was bound, would lie to the client.
		// A request that accepts SSE holds one until the response proves it JSON.
		releaseStream, streamReserved := func() {}, false
		if r.Method == http.MethodGet || acceptsEventStream(r.Header) {
			releaseUserStream, ok := g.userSSE.tryAcquire(userKey)
			if !ok {
				outcome = "user_sse_cap"
				decision = "deny"
				decisionReason = "user_sse_cap"
				markShed(w, "user_sse_cap")
				w.Header().Set("Retry-After", "1")
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{
					"error":   "too_many_requests",
					"message": "too many concurrent SSE streams for this user; retry shortly",
				})
				return
			}
			defer releaseUserStream()
			releaseGlobalStream, ok := g.sseStreams.tryAcquire()
			if !ok {
				// Shed, not a decision: same as over_capacity above.
				decision, decisionReason = "", ""
				outcome = "sse_over_capacity"
				writeOverCapacity(w, "SSE streams")
				return
			}
			defer releaseGlobalStream()
			releaseStream = func() { releaseGlobalStream(); releaseUserStream() }
			streamReserved = true
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
		var bodyBuf atomic.Pointer[[]byte]
		// Track the latest reader so a failed attempt cannot drop the buffer during a retry.
		var latestBodyDone atomic.Pointer[chan struct{}]
		newBodyReader := func() *droppableBytes {
			done := make(chan struct{})
			latestBodyDone.Store(&done)
			return &droppableBytes{b: &bodyBuf, done: done}
		}
		var bodyReader io.Reader
		if len(reqBody) > 0 {
			b := reqBody
			bodyBuf.Store(&b)
			bodyReader = newBodyReader()
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
		if len(reqBody) > 0 {
			upReq.ContentLength = int64(len(reqBody))
			upReq.GetBody = func() (io.ReadCloser, error) {
				return newBodyReader(), nil
			}
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
			// Count live rows: a counter drifted when session rows expired without decrementing.
			// shortcut: count-then-put is not atomic across pods, so the cap is approximate; fine as defence in depth.
			userKey := g.mcpSessionUserKey(sess.NAVIdent)
			count, cntErr := g.store.CountPrefix(r.Context(), kindMCPSessionUser, userKey+":")
			if cntErr != nil {
				slog.Warn("mcp session bind skipped: count failed", "route", route.Route, "error", cntErr)
				return
			}
			if count >= g.cfg.MCPSessionUserCap {
				slog.Warn("mcp session bind skipped: per-user cap reached", "route", route.Route, "navident", sanitizeLogValue(navIdent))
				return
			}
			if putErr := g.store.Put(r.Context(), kindMCPSession, key, []byte(sess.NAVIdent), mcpSessionTTL); putErr != nil {
				slog.Warn("mcp session bind failed", "route", route.Route, "error", putErr)
				return
			}
			_ = g.store.Put(r.Context(), kindMCPSessionUser, userKey+":"+key, []byte("1"), mcpSessionTTL)
		}
		// A terminated session frees its binding and the user's slot under the cap.
		if r.Method == http.MethodDelete && ownsReqSession && resp.StatusCode < 300 {
			// Pop, not Delete: only the request that removed the binding frees the slot,
			// so concurrent DELETEs of one session cannot decrement twice.
			key := route.Route + "\n" + reqSessionID
			if _, removed, delErr := g.store.Pop(r.Context(), kindMCPSession, key); delErr != nil {
				slog.Warn("mcp session unbind failed", "route", route.Route, "error", delErr)
			} else if removed {
				_ = g.store.Delete(r.Context(), kindMCPSessionUser, g.mcpSessionUserKey(sess.NAVIdent)+":"+key)
			}
		}
		// Bind only ids the upstream issued on a successful response (MCP streamable HTTP).
		// A client-supplied id is never bound; an unbound one passes through for stateless upstreams.
		// A DELETE ends the session, so an id echoed on its response is not bound again.
		// The binding waits until the reply is accepted, so a 502 below leaves none.
		bindUpstreamSessions := func() {
			if resp.StatusCode < 300 && r.Method != http.MethodDelete {
				for _, id := range resp.Header.Values("Mcp-Session-Id") {
					if id != "" && len(id) <= maxMCPSessionIDBytes {
						bindMCPSession(id)
					}
				}
			}
		}

		if isEventStream(resp.Header.Get("Content-Type")) {
			// A stream buffers nothing but may stay open for SSE_MAX_DURATION:
			// move it from the in-flight cap to its own so streams cannot starve tool calls.
			if !streamReserved {
				// The client did not accept SSE, so no slot was reserved (MCP forbids this reply).
				outcome = "upstream_unexpected_sse"
				writeJSON(w, http.StatusBadGateway, map[string]string{
					"error":   "upstream_error",
					"message": "upstream answered with an event stream the client did not accept",
				})
				return
			}
			bindUpstreamSessions()
			releaseSlot()
			// Drop references to the buffered request body (up to 1 MiB) before
			// streaming so long-lived SSE connections do not pin it on the heap
			// via locals, resp.Request -> upReq, or Transport's rewind copy in readLoop.
			// Wait for the latest reader to copy all bytes so an early-flushing upstream gets the full body.
			if donePtr := latestBodyDone.Load(); donePtr != nil {
				done := *donePtr
				select {
				case <-done:
					bodyBuf.Store(nil)
				default:
					go func() {
						select {
						case <-done:
						case <-upCtx.Done():
						}
						bodyBuf.Store(nil)
					}()
				}
			}
			reqBody = nil    //nolint:ineffassign // drop heap reference before long-lived SSE stream
			bodyReader = nil //nolint:ineffassign // drop heap reference before long-lived SSE stream
			upReq.Body = nil
			upReq.GetBody = nil
			copyAllowlistedHeaders(w.Header(), resp.Header, allowedUpstreamResponseHeaders)
			rc := http.NewResponseController(w)
			_ = rc.SetWriteDeadline(time.Time{})
			w.WriteHeader(resp.StatusCode)
			if streamErr := streamSSE(upCtx, w, rc, resp.Body, g.cfg.SSEMaxDuration, g.cfg.SSEIdleTimeout, g.cfg.SSEMaxBytes); streamErr != nil {
				outcome = "stream_error"
			} else if resp.StatusCode >= 400 {
				outcome = fmt.Sprintf("upstream_%d", resp.StatusCode)
			}
			return
		}

		releaseStream()

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
		respBody, respLen, err := readChunks(resp.Body, maxRespBytes+1)
		bodyTimer.Stop()
		if err != nil {
			outcome = "stream_error"
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "upstream_error",
				"message": "failed to read upstream response",
			})
			return
		}
		if respLen > maxRespBytes {
			outcome = "response_too_large"
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "response_too_large",
				"message": "upstream response exceeds maximum allowed size",
			})
			return
		}

		bindUpstreamSessions()
		copyAllowlistedHeaders(w.Header(), resp.Header, allowedUpstreamResponseHeaders)
		w.WriteHeader(resp.StatusCode)
		for _, chunk := range respBody {
			if _, err := w.Write(chunk); err != nil {
				outcome = "stream_error"
				return
			}
		}
		if resp.StatusCode >= 400 {
			outcome = fmt.Sprintf("upstream_%d", resp.StatusCode)
		}
	}
}

// readChunks reads up to limit bytes in fixed 32 KiB chunks. Unlike io.ReadAll
// it never reallocates, so a body costs its size in memory, not up to ~2x
// plus the discarded backing arrays.
func readChunks(r io.Reader, limit int64) ([][]byte, int64, error) {
	const chunkSize = 32 << 10
	var chunks [][]byte
	var total int64
	for total < limit {
		buf := make([]byte, min(chunkSize, limit-total))
		n := 0
		for n < len(buf) {
			m, err := r.Read(buf[n:])
			n += m
			// Only io.EOF is a clean end. io.ErrUnexpectedEOF means the body
			// stopped short of its Content-Length; io.ReadFull would hide that.
			if err == io.EOF { //nolint:errorlint // Read returns io.EOF itself, never wrapped
				total += int64(n)
				if n > 0 {
					chunks = append(chunks, buf[:n])
				}
				return chunks, total, nil
			}
			if err != nil {
				return nil, total + int64(n), err
			}
		}
		chunks = append(chunks, buf)
		total += int64(n)
	}
	return chunks, total, nil
}

// droppableBytes streams a buffered body via a shared slice pointer so the SSE
// branch can release the underlying array even when net/http.Transport cloned
// upReq in setupRewindBody and kept the clone on persistConn.readLoop.
type droppableBytes struct {
	b        *atomic.Pointer[[]byte]
	off      int
	done     chan struct{}
	doneOnce sync.Once
}

func (d *droppableBytes) markDone() {
	if d.done != nil {
		d.doneOnce.Do(func() { close(d.done) })
	}
}

func (d *droppableBytes) Read(p []byte) (int, error) {
	bp := d.b.Load()
	if bp == nil || d.off >= len(*bp) {
		d.markDone()
		return 0, io.EOF
	}
	n := copy(p, (*bp)[d.off:])
	d.off += n
	if d.off >= len(*bp) {
		d.markDone()
	}
	return n, nil
}

func (d *droppableBytes) Close() error {
	d.markDone()
	return nil
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

	cbDone := make(chan struct{})
	stopAfter := context.AfterFunc(streamCtx, func() {
		defer close(cbDone)
		// Set write deadline before waking body.Read so it cannot land after ServeHTTP returns.
		if rc != nil {
			_ = rc.SetWriteDeadline(time.Now())
		}
		_ = body.Close()
	})
	defer func() {
		// stopAfter does not wait for a running callback; wait here before releasing the conn.
		if !stopAfter() {
			<-cbDone
		}
	}()

	if ferr := flushSSE(rc); ferr != nil {
		return ferr
	}

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
					if ferr := flushSSE(rc); ferr != nil {
						return ferr
					}
				}
				return errors.New("sse max bytes exceeded")
			}
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			if ferr := flushSSE(rc); ferr != nil {
				return ferr
			}
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

func flushSSE(rc *http.ResponseController) error {
	if rc == nil {
		return nil
	}
	if err := rc.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
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

// writeUnauthorized sends the 401 challenge. RFC 6750 s3.1: error only when the
// request carried credentials; without any, a bare challenge. resource_metadata
// per RFC 9728 s5.1 and MCP authorization (Protected Resource Metadata Discovery).
func writeUnauthorized(w http.ResponseWriter, prmURL string, presented bool) {
	challenge := fmt.Sprintf(`Bearer resource_metadata="%s"`, prmURL)
	if presented {
		challenge = fmt.Sprintf(`Bearer error="invalid_token", resource_metadata="%s"`, prmURL)
	}
	w.Header().Set("WWW-Authenticate", challenge)
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

// copyMCPParamHeaders forwards Mcp-Param-* unchanged, as the spec requires of intermediaries.
// Callers must check policy.HeadersValid first.
func copyMCPParamHeaders(dst, src http.Header) {
	connTokens := connectionHeaderTokens(src)
	for k, vals := range src {
		ck := textproto.CanonicalMIMEHeaderKey(k)
		if !policy.IsMCPParamHeader(ck) || connTokens[ck] {
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

// acceptsEventStream reports whether the Accept header admits text/event-stream.
// The most specific matching range decides (RFC 9110 12.5.1), so
// "text/event-stream;q=0, */*;q=1" refuses a stream.
func acceptsEventStream(h http.Header) bool {
	best, bestQ := -1, 0.0
	for _, v := range h.Values("Accept") {
		for _, part := range strings.Split(v, ",") {
			mt, params, err := mime.ParseMediaType(strings.TrimSpace(part))
			if err != nil {
				continue
			}
			spec := map[string]int{"text/event-stream": 3, "text/*": 2, "*/*": 1}[mt]
			if spec == 0 {
				continue
			}
			q := 1.0
			if raw, ok := params["q"]; ok {
				if f, qerr := strconv.ParseFloat(raw, 64); qerr == nil {
					q = f
				}
			}
			if spec > best || (spec == best && q > bestQ) {
				best, bestQ = spec, q
			}
		}
	}
	return best > 0 && bestQ > 0
}

func isEventStream(contentType string) bool {
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "text/event-stream")
}
