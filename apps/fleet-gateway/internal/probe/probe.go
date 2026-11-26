// Package probe serves POST /probe/v1/tools: it fetches the tool list of an
// MCP server that is not yet approved, so the /fleet wizard in my-copilot can
// prefill the registration form (#2985). The answer prefills a form only; it
// never grants access.
//
// Every input fails closed: the caller's Entra OBO token (iss, aud, azp, exp),
// owner-team membership, the target URL (same check as registration, private
// IPs only at dial time, no redirects), the forwarded methods (initialize,
// notifications/initialized and tools/list only), the response size and the
// per-user rate. The probe first calls the target without a credential. Only
// if it answers 401 does the probe exchange the caller's token (OBO, the same
// exchange a route uses) for one to the target app and retry as the user, so
// the target's own checks apply (owner decision 2026-11-26).
//
// A failure answers {"error": kind}, the kinds my-copilot's fleet-tools.ts
// handles (DiscoveryFailure):
//
//	unavailable  Texas, the Nais Console or the target failed (503, 5xx), or
//	             the caller hit the rate limit (429). Try again later.
//	auth         The caller's token was refused (401), or the target refused
//	             the user's OBO token too (502). Log in again.
//	access       The caller is not in the team (403), the target does not
//	             accept tokens from fleet-gateway (AADSTS65001), the target
//	             answered 403, or the connection was refused or timed out:
//	             usually its inbound accessPolicy lacks fleet-gateway.
//	noroute      No such app in the team's namespace (DNS), or 404 at the path.
//	toolarge     An answer was over 512 KiB.
//	invalid      The request (team, upstream_url) was refused (400, 422), or
//	             the target did not answer as an MCP server: a redirect, an
//	             unexpected status, not a JSON-RPC result, a bad tool name.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/teamtoken"

	"github.com/navikt/copilot/fleet-gateway/internal/audit"
	"github.com/navikt/copilot/fleet-gateway/internal/ratelimit"
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

const (
	// MaxResponseBytes caps each upstream answer (#2985).
	MaxResponseBytes = 512 << 10
	// Timeout bounds the whole probe, both upstream calls included.
	Timeout         = 10 * time.Second
	maxRequestBytes = 4 << 10
	protocolVersion = "2025-06-18"
)

// Members answers team membership from the same source as fleet-control.
type Members interface {
	IsMember(ctx context.Context, team, email string) (bool, error)
}

// Handler is the probe endpoint. Build it with New.
type Handler struct {
	introspectURL              string
	issuer, audience, allowAzp string
	members                    Members
	limiter                    *ratelimit.Limiter
	cluster                    string
	exchange                   Exchange
	texas, upstream            *http.Client
	validate                   func(raw, namespace string) (*url.URL, error)
	timeout                    time.Duration
	now                        func() time.Time
}

// Config holds the probe's dependencies. Every field is required except Now.
type Config struct {
	IntrospectURL string // Texas token introspection endpoint
	Issuer        string // expected iss: the Entra tenant issuer
	Audience      string // expected aud: the gateway's own client id
	AllowedAzp    string // expected azp: my-copilot's client id
	Members       Members
	Limiter       *ratelimit.Limiter
	Cluster       string   // NAIS_CLUSTER_NAME, the cluster of every probed app
	Exchange      Exchange // OBO exchange shared with the route proxy
	Now           func() time.Time
}

// Exchange trades the caller's token for one to target (cluster:namespace:app).
type Exchange func(ctx context.Context, userToken, target, app string) (string, error)

// New returns the handler, or an error when any required field is empty.
func New(c Config) (*Handler, error) {
	if c.IntrospectURL == "" || c.Issuer == "" || c.Audience == "" || c.AllowedAzp == "" || c.Members == nil || c.Limiter == nil ||
		c.Cluster == "" || c.Exchange == nil {
		return nil, errors.New("probe: introspection URL, issuer, audience, allowed azp, members, limiter, cluster and exchange are required")
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return &Handler{
		introspectURL: c.IntrospectURL, issuer: c.Issuer, audience: c.Audience, allowAzp: c.AllowedAzp,
		members: c.Members, limiter: c.Limiter, now: c.Now, cluster: c.Cluster, exchange: c.Exchange,
		texas:    &http.Client{Timeout: 5 * time.Second},
		upstream: newUpstreamClient(privateOnly),
		validate: routing.ValidateUpstreamURL,
		timeout:  Timeout,
	}, nil
}

// newUpstreamClient refuses redirects, ignores proxy env and checks every
// dialed address, so DNS that resolves outside the cluster is refused.
func newUpstreamClient(allow func(netip.Addr) bool) *http.Client {
	d := &net.Dialer{Timeout: 5 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip, err := netip.ParseAddr(host)
		if err != nil || !allow(ip.Unmap()) {
			return fmt.Errorf("probe: dial to %s refused", host)
		}
		return nil
	}}
	return &http.Client{
		Timeout:       Timeout,
		Transport:     &http.Transport{Proxy: nil, DialContext: d.DialContext, ForceAttemptHTTP2: true, TLSHandshakeTimeout: 5 * time.Second},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// privateOnly allows RFC 1918 and ULA addresses. IsPrivate excludes loopback,
// link-local (metadata server) and public addresses.
func privateOnly(ip netip.Addr) bool { return ip.IsPrivate() }

var (
	dnsLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	errAuth  = errors.New("invalid token")
)

type request struct {
	Team string `json:"team"`
	// UpstreamURL must be the team's own app service, <app>.<team> cluster DNS.
	UpstreamURL string `json:"upstream_url"`
}

// Failure kinds, see the package comment.
const (
	kindUnavailable = "unavailable"
	kindAuth        = "auth"
	kindAccess      = "access"
	kindNoRoute     = "noroute"
	kindTooLarge    = "toolarge"
	kindInvalid     = "invalid"
)

// statusError is a non-success HTTP status from the target.
type statusError struct{ code int }

func (e *statusError) Error() string { return fmt.Sprintf("status %d", e.code) }

var errTooLarge = fmt.Errorf("response over %d bytes", MaxResponseBytes)

// Tool is one entry in the answer. Annotations are hints for the form only.
type Tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Annotations json.RawMessage `json:"annotations,omitempty"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := h.now()
	var subject, team, host, outcome string
	tools := 0
	defer func() {
		slog.Info("audit probe", "subject", audit.Sanitize(subject), "team", audit.Sanitize(team),
			"upstream_host", audit.Sanitize(host), "outcome", outcome, "tools", tools,
			"duration_ms", h.now().Sub(start).Milliseconds())
	}()
	fail := func(status int, o, kind string) {
		outcome = o
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": kind})
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	ident, email, userToken, err := h.authenticate(ctx, r.Header.Get("Authorization"))
	if errors.Is(err, errAuth) {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		fail(http.StatusUnauthorized, "invalid_token", kindAuth)
		return
	} else if err != nil {
		slog.Error("probe: token introspection failed", "error", err)
		fail(http.StatusServiceUnavailable, "texas_unavailable", kindUnavailable)
		return
	}
	subject = ident
	if !h.limiter.Allow(ident) {
		fail(http.StatusTooManyRequests, "rate_limited", kindUnavailable)
		return
	}

	var req request
	dec := json.NewDecoder(io.LimitReader(r.Body, maxRequestBytes))
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil {
		fail(http.StatusBadRequest, "bad_request", kindInvalid)
		return
	}
	team = req.Team
	if !dnsLabel.MatchString(req.Team) {
		fail(http.StatusBadRequest, "bad_team", kindInvalid)
		return
	}
	ok, err := h.members.IsMember(ctx, req.Team, email)
	if err != nil {
		slog.Error("probe: team lookup failed", "error", err)
		fail(http.StatusServiceUnavailable, "teams_unavailable", kindUnavailable)
		return
	}
	if !ok {
		fail(http.StatusForbidden, "not_member", kindAccess)
		return
	}
	target, err := h.validate(req.UpstreamURL, req.Team)
	if err != nil {
		fail(http.StatusUnprocessableEntity, "bad_target", kindInvalid)
		return
	}
	host = target.Hostname()
	// Only the team's own app service: validate also accepts internal ingresses,
	// which are not tied to a namespace. It normalizes <app>.<team> to the FQDN.
	app, ok := strings.CutSuffix(host, "."+req.Team+".svc.cluster.local")
	if !ok || !dnsLabel.MatchString(app) {
		fail(http.StatusUnprocessableEntity, "bad_target", kindInvalid)
		return
	}

	list, err := h.fetchTools(ctx, target.String(), "")
	var se *statusError
	if errors.As(err, &se) && se.code == http.StatusUnauthorized {
		// Q2: the target wants Entra login; retry as the user with an OBO token for it.
		tok, xerr := h.exchange(ctx, userToken, h.cluster+":"+req.Team+":"+app, app)
		if xerr != nil {
			slog.Warn("probe: token exchange failed", "app", audit.Sanitize(app), "error", xerr)
			var missing *teamtoken.AccessMissingError
			if errors.As(xerr, &missing) {
				fail(http.StatusBadGateway, "token_access_missing", kindAccess)
			} else {
				fail(http.StatusBadGateway, "token_exchange_failed", kindUnavailable)
			}
			return
		}
		list, err = h.fetchTools(ctx, target.String(), tok)
	}
	if err != nil {
		slog.Warn("probe: upstream failed", "host", audit.Sanitize(host), "error", err)
		status, o, kind := upstreamFailure(ctx, err)
		fail(status, o, kind)
		return
	}
	tools, outcome = len(list), "ok"
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"tools": list})
}

// upstreamFailure maps a target failure to a status, audit outcome and kind.
func upstreamFailure(ctx context.Context, err error) (int, string, string) {
	var se *statusError
	var dnsErr *net.DNSError
	switch {
	case ctx.Err() != nil:
		return http.StatusGatewayTimeout, "upstream_timeout", kindAccess
	case errors.Is(err, errTooLarge):
		return http.StatusBadGateway, "upstream_too_large", kindTooLarge
	case errors.As(err, &se):
		switch {
		case se.code == http.StatusUnauthorized:
			return http.StatusBadGateway, "upstream_unauthorized", kindAuth
		case se.code == http.StatusForbidden:
			return http.StatusBadGateway, "upstream_forbidden", kindAccess
		case se.code == http.StatusNotFound:
			return http.StatusBadGateway, "upstream_not_found", kindNoRoute
		case se.code >= 500:
			return http.StatusBadGateway, "upstream_unavailable", kindUnavailable
		}
		return http.StatusBadGateway, "upstream_error", kindInvalid
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return http.StatusBadGateway, "upstream_no_such_app", kindNoRoute
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		// Refused or reset connection, or the dial guard.
		return http.StatusBadGateway, "upstream_unreachable", kindAccess
	}
	return http.StatusBadGateway, "upstream_error", kindInvalid
}

type tokenClaims struct {
	Active   bool            `json:"active"`
	Iss      string          `json:"iss"`
	Aud      json.RawMessage `json:"aud"`
	Azp      string          `json:"azp"`
	Exp      int64           `json:"exp"`
	Idtyp    string          `json:"idtyp"`
	NAVident string          `json:"NAVident"`
	Email    string          `json:"preferred_username"`
}

// authenticate validates the bearer token at Texas and rechecks iss, aud,
// azp and exp itself. errAuth for a bad token, another error for an outage.
func (h *Handler) authenticate(ctx context.Context, header string) (ident, email, tok string, err error) {
	tok, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || tok == "" || len(header) > 16<<10 || strings.ContainsAny(tok, " \t\r\n") {
		return "", "", "", errAuth
	}
	body, _ := json.Marshal(map[string]string{"identity_provider": "entra_id", "token": tok})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.introspectURL, bytes.NewReader(body))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.texas.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("introspection status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", "", err
	}
	var c tokenClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", "", "", fmt.Errorf("decode introspection: %w", err)
	}
	email = normEmail(c.Email)
	if !c.Active || c.Iss != h.issuer || !hasAudience(c.Aud, h.audience) || c.Azp != h.allowAzp ||
		c.Exp <= h.now().Unix() || c.Idtyp == "app" || c.NAVident == "" || email == "" {
		return "", "", "", errAuth
	}
	return c.NAVident, email, tok, nil
}

// hasAudience accepts aud as a string or an array, as JWT allows.
func hasAudience(raw json.RawMessage, want string) bool {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one == want
	}
	var many []string
	if json.Unmarshal(raw, &many) != nil {
		return false
	}
	for _, a := range many {
		if a == want {
			return true
		}
	}
	return false
}

// fetchTools sends initialize, notifications/initialized and tools/list, and
// nothing else. token is "" (no credential) or the OBO token for the target,
// never the caller's own token.
func (h *Handler) fetchTools(ctx context.Context, target, token string) ([]Tool, error) {
	_, session, err := h.call(ctx, target, token, "", 1, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "fleet-gateway-probe", "version": "1"},
	})
	if err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	if _, _, err = h.call(ctx, target, token, session, 0, notifyInitialized, nil); err != nil {
		return nil, fmt.Errorf("notifications/initialized: %w", err)
	}
	result, _, err := h.call(ctx, target, token, session, 2, "tools/list", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}
	var out struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("decode tools: %w", err)
	}
	for _, t := range out.Tools {
		if !routing.ValidToolName(t.Name) {
			return nil, errors.New("tool name outside the gateway charset")
		}
	}
	if out.Tools == nil {
		out.Tools = []Tool{}
	}
	return out.Tools, nil
}

// allowedMethods is the only place a method is chosen; call refuses the rest.
// The one notification is sent without an id and expects no result.
var allowedMethods = map[string]bool{"initialize": true, "tools/list": true}

const notifyInitialized = "notifications/initialized"

func (h *Handler) call(ctx context.Context, target, token, session string, id int, method string, params any) (json.RawMessage, string, error) {
	notify := method == notifyInitialized
	if !allowedMethods[method] && !notify {
		return nil, "", fmt.Errorf("method %q is not forwarded", method)
	}
	rpc := map[string]any{"jsonrpc": "2.0", "method": method}
	if !notify {
		rpc["id"], rpc["params"] = id, params
	}
	body, _ := json.Marshal(rpc)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if method != "initialize" {
		req.Header.Set("MCP-Protocol-Version", protocolVersion)
	}
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	resp, err := h.upstream.Do(req) //nolint:gosec // target passed routing.ValidateUpstreamURL; dial is private-only
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if notify {
		// Streamable HTTP answers a notification with 202; older servers send 200 or 204.
		if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			return nil, "", &statusError{resp.StatusCode}
		}
		return nil, "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", &statusError{resp.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(raw) > MaxResponseBytes {
		return nil, "", errTooLarge
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		raw = sseMessage(raw, id)
	}
	var msg struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil || msg.ID != id || msg.Error != nil || msg.Result == nil {
		return nil, "", errors.New("not a JSON-RPC result for the request")
	}
	return msg.Result, resp.Header.Get("Mcp-Session-Id"), nil
}

// sseMessage returns the data of the first SSE event whose JSON id is id.
func sseMessage(raw []byte, id int) []byte {
	for _, event := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n\n") {
		var data []string
		for _, line := range strings.Split(event, "\n") {
			if v, ok := strings.CutPrefix(line, "data:"); ok {
				data = append(data, strings.TrimPrefix(v, " "))
			}
		}
		var probe struct {
			ID int `json:"id"`
		}
		joined := []byte(strings.Join(data, "\n"))
		if json.Unmarshal(joined, &probe) == nil && probe.ID == id {
			return joined
		}
	}
	return nil
}
