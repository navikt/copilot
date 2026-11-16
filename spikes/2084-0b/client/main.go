// Test client for navikt/copilot#2084 phase 0b. Plays cplt's host broker until cplt#910 lands. Do not merge.
// It prints claims and timings only. Tokens stay in memory and are never printed or written to disk.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Port 62471: fixed loopback port for the pilot CIMD redirect_uri. See the PR for why.
const redirectURI = "http://127.0.0.1:62471/callback"

const mcpWhoamiBody = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whoami"}}`

type oauthConfig struct {
	targetURL     string
	resource      string
	clientID      string
	issuer        string
	authzEndpoint string
	tokenEndpoint string
	checkIss      bool
}

type prmDoc struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
}

type asMetaDoc struct {
	Issuer                   string `json:"issuer"`
	AuthorizationEndpoint    string `json:"authorization_endpoint"`
	TokenEndpoint            string `json:"token_endpoint"`
	AuthResponseIssSupported bool   `json:"authorization_response_iss_parameter_supported"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func main() {
	gw := flag.String("gw", "https://fleet-gw-0b.intern.dev.nav.no", "gateway base URL")
	resource := flag.String("resource", "", "target MCP resource URL (discovers AS via RFC 9728; default uses -gw/mcp)")
	clientID := flag.String("client-id", "", "OAuth client_id (default <gw>/oauth-client.json)")
	strategy := flag.String("strategy", "a", "a = offline_access + direct refresh, b = no refresh token")
	claims := flag.String("claims", "", "T6: Conditional Access claims JSON passed to Entra /authorize")
	calls := flag.Int("calls", 1, "number of /mcp calls")
	every := flag.Duration("every", time.Minute, "pause between calls")
	relogin := flag.Bool("relogin", false, "T3: on 401, log in again with prompt=none and time it")
	flag.Parse()

	cfg, err := resolveConfig(httpClient, *gw, *resource, *clientID)
	if err != nil {
		fail(err)
	}
	tok, err := login(httpClient, cfg, *strategy, *claims, "")
	if err != nil {
		fail(err)
	}
	for i := 0; i < *calls; i++ {
		if i > 0 {
			time.Sleep(*every)
		}
		status, body := call(httpClient, cfg.targetURL, tok)
		fmt.Printf("%s call %d: HTTP %d\n%s\n", time.Now().Format(time.TimeOnly), i+1, status, body)
		if status == 401 && *relogin {
			t0 := time.Now()
			if tok, err = login(httpClient, cfg, *strategy, *claims, "none"); err != nil {
				fail(err)
			}
			fmt.Printf("silent re-login took %s\n", time.Since(t0).Round(time.Millisecond))
		}
	}
}

func resolveConfig(hc *http.Client, gw, resource, clientID string) (oauthConfig, error) {
	base := strings.TrimRight(gw, "/")
	cid := clientID
	if cid == "" {
		cid = base + "/oauth-client.json"
	}
	if resource == "" {
		return oauthConfig{
			targetURL:     base + "/mcp",
			resource:      base + "/mcp",
			clientID:      cid,
			issuer:        base,
			authzEndpoint: base + "/authorize",
			tokenEndpoint: base + "/token",
			checkIss:      true,
		}, nil
	}
	return discover(hc, resource, cid)
}

func discover(hc *http.Client, resourceURL, clientID string) (oauthConfig, error) {
	u, err := url.Parse(resourceURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return oauthConfig{}, fmt.Errorf("invalid resource URL %q", resourceURL)
	}
	origin := u.Scheme + "://" + u.Host
	path := u.EscapedPath()
	if path == "/" {
		path = ""
	}

	prm, err := fetchPRM(hc, origin+"/.well-known/oauth-protected-resource"+path)
	if err != nil {
		if metaURL := probeResourceMetadata(hc, resourceURL); metaURL != "" {
			prm, err = fetchPRM(hc, metaURL)
		} else if path != "" {
			prm, err = fetchPRM(hc, origin+"/.well-known/oauth-protected-resource")
		}
	}
	if err != nil {
		return oauthConfig{}, fmt.Errorf("protected resource metadata discovery failed: %w", err)
	}

	asURL := prm.AuthorizationServers[0]
	oauthRes := resourceURL
	if prm.Resource != "" {
		oauthRes = prm.Resource
	}
	asMeta, err := fetchASMetadata(hc, asURL)
	if err != nil {
		return oauthConfig{}, fmt.Errorf("authorization server metadata discovery failed: %w", err)
	}
	issuer := asMeta.Issuer
	if issuer == "" {
		issuer = asURL
	}
	return oauthConfig{
		targetURL:     resourceURL,
		resource:      oauthRes,
		clientID:      clientID,
		issuer:        issuer,
		authzEndpoint: asMeta.AuthorizationEndpoint,
		tokenEndpoint: asMeta.TokenEndpoint,
		checkIss:      asMeta.AuthResponseIssSupported,
	}, nil
}

func fetchPRM(hc *http.Client, metaURL string) (prmDoc, error) {
	var prm prmDoc
	if err := getJSON(hc, metaURL, &prm); err != nil {
		return prm, err
	}
	if len(prm.AuthorizationServers) == 0 || prm.AuthorizationServers[0] == "" {
		return prm, fmt.Errorf("%s: missing authorization_servers", metaURL)
	}
	return prm, nil
}

func probeResourceMetadata(hc *http.Client, resourceURL string) string {
	req, err := http.NewRequest(http.MethodPost, resourceURL, strings.NewReader(mcpWhoamiBody))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusUnauthorized {
		return ""
	}
	for _, hdr := range resp.Header.Values("WWW-Authenticate") {
		if u := extractResourceMetadata(hdr); u != "" {
			return u
		}
	}
	return ""
}

func extractResourceMetadata(hdr string) string {
	lower := strings.ToLower(hdr)
	const key = "resource_metadata="
	idx := strings.Index(lower, key)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(hdr[idx+len(key):])
	if strings.HasPrefix(rest, `"`) {
		rest = rest[1:]
		if end := strings.IndexByte(rest, '"'); end >= 0 {
			return rest[:end]
		}
		return ""
	}
	if end := strings.IndexAny(rest, ", \t"); end >= 0 {
		return rest[:end]
	}
	return rest
}

func fetchASMetadata(hc *http.Client, asURL string) (asMetaDoc, error) {
	u, err := url.Parse(asURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return asMetaDoc{}, fmt.Errorf("invalid authorization server URL %q", asURL)
	}
	origin := u.Scheme + "://" + u.Host
	path := strings.TrimRight(u.EscapedPath(), "/")
	candidates := []string{
		origin + "/.well-known/oauth-authorization-server" + path,
		origin + path + "/.well-known/openid-configuration",
	}
	var lastErr error
	for _, c := range candidates {
		var meta asMetaDoc
		if err := getJSON(hc, c, &meta); err != nil {
			lastErr = err
			continue
		}
		if meta.AuthorizationEndpoint == "" || meta.TokenEndpoint == "" {
			lastErr = fmt.Errorf("%s: missing authorization_endpoint or token_endpoint", c)
			continue
		}
		return meta, nil
	}
	return asMetaDoc{}, lastErr
}

func getJSON(hc *http.Client, target string, dst any) error {
	resp, err := hc.Get(target)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", target, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(dst)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func rnd() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func login(hc *http.Client, cfg oauthConfig, strategy, claims, prompt string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:62471")
	if err != nil {
		return "", fmt.Errorf("cannot hold the loopback port: %w", err)
	}
	verifier, state := rnd(), rnd()
	sum := sha256.Sum256([]byte(verifier))

	authU, err := url.Parse(cfg.authzEndpoint)
	if err != nil {
		ln.Close()
		return "", fmt.Errorf("invalid authorization_endpoint %q: %w", cfg.authzEndpoint, err)
	}
	q := authU.Query()
	q.Set("client_id", cfg.clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
	q.Set("code_challenge_method", "S256")
	q.Set("resource", cfg.resource)
	if strategy != "" {
		q.Set("strategy", strategy)
	}
	if claims != "" {
		q.Set("claims", claims)
	}
	if prompt != "" {
		q.Set("prompt", prompt)
	}
	authU.RawQuery = q.Encode()

	type result struct {
		code string
		err  error
	}
	ch := make(chan result, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		p := r.URL.Query()
		var res result
		switch {
		case p.Get("state") != state:
			res.err = fmt.Errorf("state mismatch")
		case (cfg.checkIss || p.Get("iss") != "") && p.Get("iss") != cfg.issuer:
			res.err = fmt.Errorf("iss %q is not %q", p.Get("iss"), cfg.issuer)
		case p.Get("error") != "":
			res.err = fmt.Errorf("%s: %s", p.Get("error"), p.Get("error_description"))
		default:
			res.code = p.Get("code")
		}
		if res.err != nil {
			fmt.Fprintln(w, "Login failed:", res.err)
		} else {
			fmt.Fprintln(w, "Login complete. You can close this tab.")
		}
		select {
		case ch <- res:
		default:
		}
	})}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	u := authU.String()
	fmt.Println("opening browser for login")
	open := map[string]string{"darwin": "open", "windows": "explorer"}[runtime.GOOS]
	if open == "" {
		open = "xdg-open"
	}
	if exec.Command(open, u).Start() != nil {
		fmt.Println("open this URL:", u)
	}
	var res result
	select {
	case res = <-ch:
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("no callback within 5 minutes")
	}
	if res.err != nil {
		return "", res.err
	}
	resp, err := hc.PostForm(cfg.tokenEndpoint, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {res.code},
		"client_id":     {cfg.clientID},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
		"resource":      {cfg.resource},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	tok, _ := out["access_token"].(string)
	if resp.StatusCode != 200 || tok == "" {
		return "", fmt.Errorf("token endpoint: HTTP %d %v %v", resp.StatusCode, out["error"], out["error_description"])
	}
	fmt.Printf("logged in: strategy=%v expires_in=%vs\n", out["strategy"], out["expires_in"])
	return tok, nil
}

// call returns the resource's response, which holds claims only.
func call(hc *http.Client, targetURL, tok string) (int, string) {
	req, _ := http.NewRequest(http.MethodPost, targetURL, strings.NewReader(mcpWhoamiBody))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, err.Error()
	}
	var v any
	if json.Unmarshal(raw, &v) == nil {
		b, _ := json.MarshalIndent(v, "", "  ")
		return resp.StatusCode, string(b)
	}
	return resp.StatusCode, string(raw)
}
