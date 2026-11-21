// Command mcp-compliance checks an MCP server behind fleet-gateway against the
// MCP 2026-07-28 rules and the auth rules the Nav MCP servers share. The checks
// mirror runMCP2026Compliance and the auth tests in apps/mcp-kodeverk,
// apps/mcp-team-catalog and apps/fleet-gateway/echo.
//
// Tokens come from environment variables only, so they never show up in a
// process list or shell history, and the tool never prints them.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	modernVersion         = "2026-07-28"
	codeHeaderMismatch    = -32020
	codeUnsupportedVer    = -32022
	codeInvalidRequest    = -32600
	metaProtocolVersion   = "io.modelcontextprotocol/protocolVersion"
	metaServerInfo        = "io.modelcontextprotocol/serverInfo"
	envToken              = "MCP_COMPLIANCE_TOKEN" // #nosec G101 -- variable name, not a credential
	envAppToken           = "MCP_COMPLIANCE_APP_TOKEN"
	invalidToken          = "mcp-compliance-invalid-token"
	defaultMaxRequestSize = 1 << 20
)

type config struct {
	url      string
	tool     string
	token    string // valid user token; empty for a server without auth
	appToken string // token with idtyp=app; empty skips that check
	maxBytes int
	legacy   bool // server is inside its legacy window: skip GET/DELETE 405
	client   *http.Client
}

type result struct {
	name   string
	status string // PASS, FAIL or SKIP
	detail string
}

type response struct {
	status int
	header http.Header
	body   map[string]any
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Getenv))
}

func run(args []string, out io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("mcp-compliance", flag.ContinueOnError)
	fs.SetOutput(out)
	cfg := config{client: &http.Client{Timeout: 30 * time.Second}}
	fs.StringVar(&cfg.url, "url", "", "MCP endpoint URL, e.g. http://localhost:8080/mcp (required)")
	fs.StringVar(&cfg.tool, "tool", "", "a tool callable with empty arguments (default: first tool from tools/list)")
	fs.IntVar(&cfg.maxBytes, "max-bytes", defaultMaxRequestSize, "request size limit the server must enforce")
	fs.BoolVar(&cfg.legacy, "legacy", false, "server runs inside its legacy window (MCP_LEGACY_PROTOCOL_UNTIL): skip the GET/DELETE 405 checks")
	fs.Usage = func() {
		fmt.Fprintf(out, "Usage: mcp-compliance -url URL [flags]\n\nEnvironment:\n  %s      valid user token for the server (optional)\n  %s  app token (idtyp=app) that must be rejected (optional)\n\n", envToken, envAppToken)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if cfg.url == "" || cfg.maxBytes <= 0 {
		fs.Usage()
		return 2
	}
	cfg.token = getenv(envToken)
	cfg.appToken = getenv(envAppToken)

	results, abort := checks(&cfg)
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "RESULT\tCHECK\tDETAIL\n")
	failed := 0
	for _, r := range results {
		if r.status == "FAIL" {
			failed++
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.status, r.name, redact(r.detail, cfg.token, cfg.appToken))
	}
	_ = tw.Flush()
	if abort != nil {
		fmt.Fprintf(out, "\nstopped: %v\n", abort)
		return 1
	}
	if failed > 0 {
		fmt.Fprintf(out, "\n%d of %d checks failed\n", failed, len(results))
		return 1
	}
	fmt.Fprintf(out, "\nall %d checks passed or skipped\n", len(results))
	return 0
}

func checks(cfg *config) ([]result, error) {
	var rs []result
	add := func(name string, err error) {
		r := result{name: name, status: "PASS"}
		var skip skipError
		var warn warnError
		switch {
		case errors.As(err, &skip):
			r.status, r.detail = "SKIP", string(skip)
		case errors.As(err, &warn):
			r.status, r.detail = "WARN", string(warn)
		case err != nil:
			r.status, r.detail = "FAIL", err.Error()
		}
		rs = append(rs, r)
	}

	discover := modernBody("server/discover", nil)
	discoverName := "server/discover lists " + modernVersion
	add(discoverName, func() error {
		resp, err := cfg.do(http.MethodPost, cfg.token, discover, modernHeaders("server/discover", ""))
		if e := expectStatus(resp, err, 200); e != nil {
			return e
		}
		r, _ := resp.body["result"].(map[string]any)
		v, _ := r["supportedVersions"].([]any)
		found := false
		for _, x := range v {
			found = found || x == modernVersion
		}
		switch {
		case !found:
			return fmt.Errorf("supportedVersions = %v, want %s", v, modernVersion)
		case r["capabilities"] == nil || r["ttlMs"] == nil || r["cacheScope"] == nil:
			return errors.New("result lacks capabilities, ttlMs or cacheScope")
		case r["resultType"] != "complete":
			return fmt.Errorf("resultType = %v, want complete", r["resultType"])
		}
		meta, _ := r["_meta"].(map[string]any)
		if meta[metaServerInfo] == nil {
			return errors.New("result _meta lacks " + metaServerInfo)
		}
		return nil
	}())

	if slices.ContainsFunc(rs, func(r result) bool { return r.name == discoverName && r.status == "FAIL" }) {
		// Stop here: without a working endpoint every other row would mean nothing,
		// and a wrong URL that answers 401 everywhere would pass the auth rows.
		return rs, errors.New("server/discover did not answer 200; check -url and " + envToken)
	}
	add("missing token is 401", func() error {
		return cfg.expectChallenge("")
	}())
	add("invalid token is 401", func() error {
		return cfg.expectChallenge(invalidToken)
	}())
	add("app token is 403", func() error {
		if cfg.appToken == "" {
			return skipError(envAppToken + " not set")
		}
		return cfg.expect(http.MethodPost, cfg.appToken, discover, modernHeaders("server/discover", ""), 403)
	}())

	tool, toolErr := cfg.pickTool()
	callBody := func() map[string]any {
		return modernBody("tools/call", map[string]any{"name": tool, "arguments": map[string]any{}})
	}
	mismatch := func(name string, body map[string]any, hdr map[string]string, code int) {
		add(name, func() error {
			resp, err := cfg.do(http.MethodPost, cfg.token, body, hdr)
			return expectRPCError(resp, err, 400, code)
		}())
	}
	mismatch("missing MCP-Protocol-Version is -32020", modernBody("tools/list", nil), with(modernHeaders("tools/list", ""), "MCP-Protocol-Version", ""), codeHeaderMismatch)
	unsupported := modernBody("tools/list", nil)
	unsupported["params"].(map[string]any)["_meta"].(map[string]any)[metaProtocolVersion] = "2099-01-01"
	mismatch("unsupported MCP-Protocol-Version is -32022", unsupported, with(modernHeaders("tools/list", ""), "MCP-Protocol-Version", "2099-01-01"), codeUnsupportedVer)
	mismatch("Mcp-Method mismatch is -32020", modernBody("tools/list", nil), modernHeaders("server/discover", ""), codeHeaderMismatch)
	mismatch("Mcp-Method missing is -32020", modernBody("tools/list", nil), with(modernHeaders("tools/list", ""), "Mcp-Method", ""), codeHeaderMismatch)
	differing := modernBody("tools/list", nil)
	differing["params"].(map[string]any)["_meta"].(map[string]any)[metaProtocolVersion] = "2025-06-18"
	mismatch("_meta protocolVersion differing from header is -32020", differing, modernHeaders("tools/list", ""), codeHeaderMismatch)
	if toolErr == nil {
		mismatch("Mcp-Name mismatch is -32020", callBody(), modernHeaders("tools/call", tool+"_other"), codeHeaderMismatch)
		mismatch("Mcp-Name missing is -32020", callBody(), modernHeaders("tools/call", ""), codeHeaderMismatch)
	} else {
		add("Mcp-Name mismatch is -32020", toolErr)
	}

	mismatch("missing method is -32600", map[string]any{"jsonrpc": "2.0", "id": 1}, modernHeaders("", ""), codeInvalidRequest)
	mismatch("null id is -32600", map[string]any{"jsonrpc": "2.0", "id": nil, "method": "tools/list"}, modernHeaders("tools/list", ""), codeInvalidRequest)

	for _, verb := range []string{http.MethodGet, http.MethodDelete} {
		add(verb+" is 405 with Allow: POST", func() error {
			if cfg.legacy {
				return skipError("-legacy set")
			}
			resp, err := cfg.do(verb, cfg.token, nil, nil)
			if e := expectStatus(resp, err, 405); e != nil {
				return e
			}
			if a := resp.header.Get("Allow"); a != http.MethodPost {
				return fmt.Errorf("allow header = %q, want POST", a)
			}
			return nil
		}())
	}

	add(fmt.Sprintf("body over %d bytes is 413", cfg.maxBytes), func() error {
		big := modernBody("tools/list", map[string]any{"pad": strings.Repeat("a", cfg.maxBytes)})
		return cfg.expect(http.MethodPost, cfg.token, big, modernHeaders("tools/list", ""), 413)
	}())
	return rs, nil
}

type skipError string

// warnError marks a SHOULD that is not met; it does not fail the run.
type warnError string

func (w warnError) Error() string { return string(w) }

func (s skipError) Error() string { return string(s) }

// pickTool returns -tool, or the first tool tools/list reports.
func (cfg *config) pickTool() (string, error) {
	if cfg.tool != "" {
		return cfg.tool, nil
	}
	resp, err := cfg.do(http.MethodPost, cfg.token, modernBody("tools/list", nil), modernHeaders("tools/list", ""))
	if e := expectStatus(resp, err, 200); e != nil {
		return "", fmt.Errorf("tools/list for a tool name: %w", e)
	}
	r, _ := resp.body["result"].(map[string]any)
	tools, _ := r["tools"].([]any)
	if len(tools) > 0 {
		if m, _ := tools[0].(map[string]any); m["name"] != nil {
			return fmt.Sprint(m["name"]), nil
		}
	}
	return "", errors.New("tools/list returned no tools; pass -tool")
}

func (cfg *config) do(method, token string, body map[string]any, hdr map[string]string) (*response, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, cfg.url, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// JSON only: the tool cannot parse SSE, and the Nav servers do not require text/event-stream.
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range hdr {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	res, err := cfg.client.Do(req) // #nosec G704 -- the operator picks the URL to test
	if err != nil {
		// The URL error carries no headers, so the token cannot leak here.
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	out := &response{status: res.StatusCode, header: res.Header}
	_ = json.Unmarshal(raw, &out.body)
	return out, nil
}

func expectStatus(resp *response, err error, want int) error {
	if err != nil {
		return err
	}
	if resp.status != want {
		return fmt.Errorf("status %d, want %d", resp.status, want)
	}
	return nil
}

func expectRPCError(resp *response, err error, status, code int) error {
	if e := expectStatus(resp, err, status); e != nil {
		return e
	}
	e, _ := resp.body["error"].(map[string]any)
	if c, _ := e["code"].(float64); c != float64(code) {
		return fmt.Errorf("error code %v, want %d", e["code"], code)
	}
	return nil
}

func modernBody(method string, params map[string]any) map[string]any {
	if params == nil {
		params = map[string]any{}
	}
	params["_meta"] = map[string]any{
		metaProtocolVersion:                          modernVersion,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "mcp-compliance", "version": "0"},
	}
	return map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
}

func modernHeaders(method, name string) map[string]string {
	return map[string]string{"MCP-Protocol-Version": modernVersion, "Mcp-Method": method, "Mcp-Name": name}
}

func with(h map[string]string, k, v string) map[string]string {
	out := map[string]string{}
	for kk, vv := range h {
		out[kk] = vv
	}
	out[k] = v
	return out
}

func (cfg *config) expect(method, token string, body map[string]any, hdr map[string]string, want int) error {
	resp, err := cfg.do(method, token, body, hdr)
	return expectStatus(resp, err, want)
}

// expectChallenge wants 401 with an RFC 6750 invalid_token challenge, which a
// wrong URL behind a generic 401 does not send.
func (cfg *config) expectChallenge(token string) error {
	resp, err := cfg.do(http.MethodPost, token, modernBody("server/discover", nil), modernHeaders("server/discover", ""))
	if e := expectStatus(resp, err, 401); e != nil {
		return e
	}
	if h := resp.header.Get("WWW-Authenticate"); !strings.Contains(h, "invalid_token") {
		// The reference servers send no challenge yet; discover-first already guards a wrong URL.
		return warnError(`no WWW-Authenticate with error="invalid_token" (RFC 6750 SHOULD)`)
	}
	return nil
}

// redact keeps server-controlled text from leaking a token or flooding the terminal.
func redact(s string, tokens ...string) string {
	for _, t := range tokens {
		if t != "" {
			s = strings.ReplaceAll(s, t, "[REDACTED]")
		}
	}
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
	if len(s) > 200 {
		s = strings.ToValidUTF8(s[:200], "") + "..."
	}
	return s
}
