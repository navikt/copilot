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

func main() {
	gw := flag.String("gw", "https://fleet-gw-0b.intern.dev.nav.no", "gateway base URL")
	strategy := flag.String("strategy", "a", "a = offline_access + direct refresh, b = no refresh token")
	claims := flag.String("claims", "", "T6: Conditional Access claims JSON passed to Entra /authorize")
	calls := flag.Int("calls", 1, "number of /mcp calls")
	every := flag.Duration("every", time.Minute, "pause between calls")
	relogin := flag.Bool("relogin", false, "T3: on 401, log in again with prompt=none and time it")
	flag.Parse()

	tok, err := login(*gw, *strategy, *claims, "")
	if err != nil {
		fail(err)
	}
	for i := 0; i < *calls; i++ {
		if i > 0 {
			time.Sleep(*every)
		}
		status, body := call(*gw, tok)
		fmt.Printf("%s call %d: HTTP %d\n%s\n", time.Now().Format(time.TimeOnly), i+1, status, body)
		if status == 401 && *relogin {
			t0 := time.Now()
			if tok, err = login(*gw, *strategy, *claims, "none"); err != nil {
				fail(err)
			}
			fmt.Printf("silent re-login took %s\n", time.Since(t0).Round(time.Millisecond))
		}
	}
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

func login(gw, strategy, claims, prompt string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:62471")
	if err != nil {
		return "", fmt.Errorf("cannot hold the loopback port: %w", err)
	}
	verifier, state := rnd(), rnd()
	sum := sha256.Sum256([]byte(verifier))
	clientID, resource := gw+"/oauth-client.json", gw+"/mcp"
	q := url.Values{"client_id": {clientID}, "redirect_uri": {redirectURI}, "response_type": {"code"}, "state": {state},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"},
		"resource": {resource}, "strategy": {strategy}}
	if claims != "" {
		q.Set("claims", claims)
	}
	if prompt != "" {
		q.Set("prompt", prompt)
	}
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
		case p.Get("iss") != gw:
			res.err = fmt.Errorf("iss %q is not the gateway", p.Get("iss"))
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

	u := gw + "/authorize?" + q.Encode()
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
	resp, err := http.PostForm(gw+"/token", url.Values{"grant_type": {"authorization_code"}, "code": {res.code},
		"client_id": {clientID}, "redirect_uri": {redirectURI}, "code_verifier": {verifier}, "resource": {resource}})
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

// call returns the gateway's response, which holds claims only.
func call(gw, tok string) (int, string) {
	req, _ := http.NewRequest(http.MethodPost, gw+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whoami"}}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	var v any
	_ = json.NewDecoder(resp.Body).Decode(&v)
	b, _ := json.MarshalIndent(v, "", "  ")
	return resp.StatusCode, string(b)
}
