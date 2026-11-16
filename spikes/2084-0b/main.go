// Spike for navikt/copilot#2084 phase 0b: minimal OAuth authorization server in front of Entra. Do not merge.
// One binary, two roles: ROLE=gateway (fleet-gw-0b) and ROLE=team (fleet-team-0b).
// Responses and logs carry claims and errors only, never tokens or codes.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

var client = &http.Client{
	Timeout:       10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects refused") },
}

// ---------- gateway ----------

type pending struct { // an /authorize request waiting for consent or for Entra
	ClientID, RedirectURI, State, Challenge, Resource, Strategy, Prompt, Claims string
	CSRF                                                                        string
	Created                                                                     time.Time
}

type session struct {
	Strategy     string
	Resource     string
	RefreshToken string    // strategy a: Entra refresh token, memory only
	TeamToken    string    // strategy b: Entra access token for the team app
	TeamExpiry   time.Time // strategy b
	GotRefresh   bool
	Expiry       time.Time
}

type issued struct { // a gateway authorization code
	P       pending
	Session *session
	Created time.Time
}

type gateway struct {
	base         string // https://fleet-gw-0b.intern.dev.nav.no
	clientID     string // the one CIMD client_id accepted
	redirectURIs []string
	teamScope    string // api://dev-gcp.copilot.fleet-team-0b/.default
	teamURL      string
	entraID      string
	entraSecret  string
	entraToken   string
	entraAuthz   string

	mu       sync.Mutex
	pend     map[string]*pending // key: consent id, then Entra state
	codes    map[string]*issued
	sessions map[string]*session // key: gateway access token
	consent  map[string]string   // consent cookie -> client_id + " " + resource
}

const codeTTL = time.Minute

func newGateway(base string, redirectURIs []string) *gateway {
	return &gateway{
		base: base, clientID: base + "/oauth-client.json", redirectURIs: redirectURIs,
		pend: map[string]*pending{}, codes: map[string]*issued{}, sessions: map[string]*session{}, consent: map[string]string{},
	}
}

func (g *gateway) mcpResource() string { return g.base + "/mcp" }

func random() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func s256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func (g *gateway) cimd(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"client_id":                  g.clientID,
		"client_name":                "cplt (fleet-gateway phase 0b spike)",
		"redirect_uris":              g.redirectURIs,
		"grant_types":                []string{"authorization_code"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
}

func (g *gateway) prm(w http.ResponseWriter, _ *http.Request) { // RFC 9728
	writeJSON(w, 200, map[string]any{
		"resource":                 g.mcpResource(),
		"authorization_servers":    []string{g.base},
		"bearer_methods_supported": []string{"header"},
	})
}

func (g *gateway) asMetadata(w http.ResponseWriter, _ *http.Request) { // RFC 8414
	writeJSON(w, 200, map[string]any{
		"issuer":                                         g.base,
		"authorization_endpoint":                         g.base + "/authorize",
		"token_endpoint":                                 g.base + "/token",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"client_id_metadata_document_supported":          true,
		"authorization_response_iss_parameter_supported": true,
	})
}

func secureHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

// authorize validates the client request. Errors before redirect_uri is trusted are shown, never redirected.
func (g *gateway) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("client_id") != g.clientID {
		http.Error(w, "unknown client_id: only the pilot CIMD client is accepted", 400)
		return
	}
	if !slices.Contains(g.redirectURIs, q.Get("redirect_uri")) { // exact string match
		http.Error(w, "redirect_uri does not exactly match the client metadata document", 400)
		return
	}
	fail := func(e, desc string) {
		u, _ := url.Parse(q.Get("redirect_uri"))
		v := url.Values{"error": {e}, "error_description": {desc}, "iss": {g.base}}
		if s := q.Get("state"); s != "" {
			v.Set("state", s)
		}
		u.RawQuery = v.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
	}
	switch {
	case q.Get("response_type") != "code":
		fail("unsupported_response_type", "response_type must be code")
	case q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256":
		fail("invalid_request", "PKCE with S256 is required")
	case q.Get("state") == "":
		fail("invalid_request", "state is required")
	case q.Get("resource") != g.mcpResource():
		fail("invalid_target", "resource must be "+g.mcpResource())
	default:
		strategy := q.Get("strategy")
		if strategy == "" {
			strategy = "a"
		}
		if strategy != "a" && strategy != "b" {
			fail("invalid_request", "strategy must be a or b")
			return
		}
		p := &pending{ClientID: g.clientID, RedirectURI: q.Get("redirect_uri"), State: q.Get("state"),
			Challenge: q.Get("code_challenge"), Resource: g.mcpResource(), Strategy: strategy,
			Prompt: q.Get("prompt"), Claims: q.Get("claims"), CSRF: random(), Created: time.Now()}
		// Consent remembered from an earlier approval in this browser: skip the page (T3 silent re-login).
		if c, err := r.Cookie("fleet_consent"); err == nil {
			g.mu.Lock()
			ok := g.consent[c.Value] == p.ClientID+" "+p.Resource
			g.mu.Unlock()
			if ok {
				g.toEntra(w, r, p)
				return
			}
		}
		id := random()
		g.mu.Lock()
		g.pend["consent:"+id] = p
		g.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "fleet_csrf", Value: p.CSRF, Path: "/consent", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: 600})
		secureHeaders(w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = consentPage.Execute(w, map[string]string{"ID": id, "CSRF": p.CSRF, "Client": p.ClientID, "Redirect": p.RedirectURI, "Resource": p.Resource, "Strategy": p.Strategy})
	}
}

var consentPage = template.Must(template.New("c").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>fleet-gw-0b consent</title></head>
<body style="font-family:sans-serif;max-width:40em;margin:2em auto">
<h1>Allow access?</h1>
<p><b>{{.Client}}</b> wants to call <b>{{.Resource}}</b> as you.</p>
<p>It will receive the result at <code>{{.Redirect}}</code>. Strategy: {{.Strategy}}.</p>
<form method="post" action="/consent">
<input type="hidden" name="id" value="{{.ID}}"><input type="hidden" name="csrf" value="{{.CSRF}}">
<button name="decision" value="approve">Allow</button> <button name="decision" value="deny">Deny</button>
</form></body></html>`))

func (g *gateway) consentPost(w http.ResponseWriter, r *http.Request) {
	id, form := r.PostFormValue("id"), r.PostFormValue("csrf")
	c, err := r.Cookie("fleet_csrf")
	g.mu.Lock()
	p := g.pend["consent:"+id]
	delete(g.pend, "consent:"+id) // one attempt per consent id
	g.mu.Unlock()
	if p == nil || err != nil || form == "" || time.Since(p.Created) > 10*time.Minute ||
		subtle.ConstantTimeCompare([]byte(c.Value), []byte(form)) != 1 || form != p.CSRF {
		http.Error(w, "consent request expired or CSRF check failed; start the login again", 403)
		return
	}
	if r.PostFormValue("decision") != "approve" {
		u, _ := url.Parse(p.RedirectURI)
		u.RawQuery = url.Values{"error": {"access_denied"}, "state": {p.State}, "iss": {g.base}}.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}
	cv := random()
	g.mu.Lock()
	g.consent[cv] = p.ClientID + " " + p.Resource
	g.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "fleet_consent", Value: cv, Path: "/authorize", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: 12 * 3600})
	g.toEntra(w, r, p)
}

// toEntra stores the request under a fresh Entra state, only after consent, and redirects to Entra.
func (g *gateway) toEntra(w http.ResponseWriter, r *http.Request, p *pending) {
	authz, err := g.entraAuthorizeEndpoint()
	if err != nil {
		http.Error(w, "cannot read Entra metadata: "+err.Error(), 502)
		return
	}
	st := random()
	p.Created = time.Now()
	g.mu.Lock()
	g.pend["entra:"+st] = p
	g.mu.Unlock()
	v := url.Values{"client_id": {g.entraID}, "response_type": {"code"}, "redirect_uri": {g.base + "/oauth2/callback"},
		"response_mode": {"query"}, "state": {st}, "scope": {entraScope(p.Strategy, g.teamScope)}}
	if p.Prompt != "" {
		v.Set("prompt", p.Prompt)
	}
	if p.Claims != "" { // T6: Conditional Access claims passed through to Entra
		v.Set("claims", p.Claims)
	}
	http.Redirect(w, r, authz+"?"+v.Encode(), http.StatusFound)
}

func entraScope(strategy, teamScope string) string {
	if strategy == "a" { // refresh token first, team token later from it (no OBO)
		return "openid offline_access " + teamScope
	}
	return "openid " + teamScope // b: no refresh token
}

func (g *gateway) entraAuthorizeEndpoint() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.entraAuthz != "" {
		return g.entraAuthz, nil
	}
	resp, err := client.Get(os.Getenv("AZURE_APP_WELL_KNOWN_URL"))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var m struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil || m.AuthorizationEndpoint == "" {
		return "", errors.New("no authorization_endpoint in well-known")
	}
	g.entraAuthz = m.AuthorizationEndpoint
	return g.entraAuthz, nil
}

// entraErr keeps only safe fields of an Entra error response and explains AADSTS65001 (T4).
func entraErr(out map[string]any, status int) map[string]any {
	e := map[string]any{"status": status, "error": out["error"], "error_description": out["error_description"], "error_codes": out["error_codes"]}
	if c, ok := out["claims"]; ok {
		e["claims"] = c // T6: Conditional Access challenge
	}
	if hasCode(out, 65001) {
		e["explanation"] = "The team MCP does not allow the gateway: its accessPolicy.inbound is missing fleet-gw-0b. Ask the team to add it."
	}
	return e
}

func hasCode(out map[string]any, code float64) bool {
	if cs, ok := out["error_codes"].([]any); ok {
		for _, c := range cs {
			if c == code {
				return true
			}
		}
	}
	d, _ := out["error_description"].(string)
	return strings.Contains(d, "AADSTS65001")
}

func (g *gateway) entraTokenCall(form url.Values) (map[string]any, int, error) {
	form.Set("client_id", g.entraID)
	form.Set("client_secret", g.entraSecret)
	resp, err := client.PostForm(g.entraToken, form)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var out map[string]any
	err = json.NewDecoder(resp.Body).Decode(&out)
	return out, resp.StatusCode, err
}

func (g *gateway) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	g.mu.Lock()
	p := g.pend["entra:"+q.Get("state")]
	delete(g.pend, "entra:"+q.Get("state"))
	g.mu.Unlock()
	if p == nil || time.Since(p.Created) > 10*time.Minute {
		http.Error(w, "unknown or expired login; start again", 400)
		return
	}
	back := func(v url.Values) {
		v.Set("state", p.State)
		v.Set("iss", g.base)
		u, _ := url.Parse(p.RedirectURI)
		u.RawQuery = v.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
	}
	if e := q.Get("error"); e != "" {
		desc := q.Get("error_description")
		if strings.Contains(desc, "AADSTS65001") {
			desc = "AADSTS65001: the team MCP does not allow the gateway (accessPolicy.inbound). " + desc
		}
		log.Printf("audit login outcome=entra_error error=%q strategy=%s", e, p.Strategy)
		back(url.Values{"error": {e}, "error_description": {desc}})
		return
	}
	out, status, err := g.entraTokenCall(url.Values{"grant_type": {"authorization_code"}, "code": {q.Get("code")},
		"redirect_uri": {g.base + "/oauth2/callback"}, "scope": {entraScope(p.Strategy, g.teamScope)}})
	at, _ := out["access_token"].(string)
	if err != nil || status != 200 || at == "" {
		e := entraErr(out, status)
		log.Printf("audit login outcome=code_redeem_failed status=%d error=%v strategy=%s", status, out["error"], p.Strategy)
		d, _ := json.Marshal(e)
		back(url.Values{"error": {"server_error"}, "error_description": {string(d)}})
		return
	}
	s := &session{Strategy: p.Strategy, Resource: p.Resource, Expiry: time.Now().Add(time.Hour)}
	if rt, _ := out["refresh_token"].(string); rt != "" && p.Strategy == "a" {
		s.RefreshToken, s.GotRefresh = rt, true
	}
	if p.Strategy == "b" {
		s.TeamToken = at
		s.TeamExpiry = time.Now().Add(time.Duration(num(out["expires_in"])) * time.Second)
		s.Expiry = s.TeamExpiry // short session: gateway token dies with the Entra token
	}
	code := random()
	g.mu.Lock()
	g.codes[code] = &issued{P: *p, Session: s, Created: time.Now()}
	g.mu.Unlock()
	log.Printf("audit login outcome=ok strategy=%s refresh_token_received=%v", p.Strategy, s.GotRefresh)
	back(url.Values{"code": {code}})
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case string:
		var f float64
		_ = json.Unmarshal([]byte(n), &f)
		return f
	}
	return 0
}

func (g *gateway) token(w http.ResponseWriter, r *http.Request) {
	bad := func(e, d string) { writeJSON(w, 400, map[string]string{"error": e, "error_description": d}) }
	if r.PostFormValue("grant_type") != "authorization_code" {
		bad("unsupported_grant_type", "only authorization_code")
		return
	}
	g.mu.Lock()
	c := g.codes[r.PostFormValue("code")]
	delete(g.codes, r.PostFormValue("code")) // single use
	g.mu.Unlock()
	switch {
	case c == nil || time.Since(c.Created) > codeTTL:
		bad("invalid_grant", "unknown, used or expired code")
	case r.PostFormValue("client_id") != c.P.ClientID:
		bad("invalid_grant", "client_id mismatch")
	case r.PostFormValue("redirect_uri") != c.P.RedirectURI:
		bad("invalid_grant", "redirect_uri mismatch")
	case r.PostFormValue("resource") != "" && r.PostFormValue("resource") != c.P.Resource:
		bad("invalid_target", "resource mismatch")
	case subtle.ConstantTimeCompare([]byte(s256(r.PostFormValue("code_verifier"))), []byte(c.P.Challenge)) != 1:
		bad("invalid_grant", "PKCE verification failed")
	default:
		at := random()
		g.mu.Lock()
		g.sessions[at] = c.Session
		g.mu.Unlock()
		// shortcut: opaque token, no gateway refresh token; phase 1 decides format (#2090).
		writeJSON(w, 200, map[string]any{"access_token": at, "token_type": "Bearer",
			"expires_in": int(time.Until(c.Session.Expiry).Seconds()), "strategy": c.Session.Strategy})
	}
}

// mcp is the test MCP route: it gets a team token per strategy and calls fleet-team-0b.
func (g *gateway) mcp(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	unauth := func(desc string) {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+g.base+`/.well-known/oauth-protected-resource/mcp", error="invalid_token"`)
		writeJSON(w, 401, map[string]string{"error": "invalid_token", "error_description": desc})
	}
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	g.mu.Lock()
	s := g.sessions[tok]
	g.mu.Unlock()
	if !ok || s == nil {
		unauth("unknown token")
		return
	}
	if s.Resource != g.mcpResource() {
		unauth("token not issued for this resource")
		return
	}
	if time.Now().After(s.Expiry) {
		g.mu.Lock()
		delete(g.sessions, tok)
		g.mu.Unlock()
		unauth("session expired; log in again")
		return
	}
	res := map[string]any{"strategy": s.Strategy, "refresh_token_received": s.GotRefresh}
	team := s.TeamToken
	if s.Strategy == "a" {
		if s.RefreshToken == "" {
			res["error"] = "Entra issued no refresh token for offline_access"
			writeJSON(w, 502, res)
			return
		}
		out, status, err := g.entraTokenCall(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {s.RefreshToken}, "scope": {g.teamScope}})
		team, _ = out["access_token"].(string)
		if err != nil || status != 200 || team == "" {
			res["entra_error"] = entraErr(out, status)
			log.Printf("audit tool=whoami outcome=refresh_failed status=%d duration_ms=%d", status, time.Since(start).Milliseconds())
			writeJSON(w, 502, res)
			return
		}
		if rt, _ := out["refresh_token"].(string); rt != "" {
			res["refresh_token_rotated"] = rt != s.RefreshToken
			g.mu.Lock()
			s.RefreshToken = rt
			g.mu.Unlock()
		}
	} else {
		res["team_token_expires_in_s"] = int(time.Until(s.TeamExpiry).Seconds())
	}
	req, _ := http.NewRequest(http.MethodGet, g.teamURL, nil)
	req.Header.Set("Authorization", "Bearer "+team)
	resp, err := client.Do(req)
	if err != nil {
		res["team_error"] = err.Error()
		writeJSON(w, 502, res)
		return
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	res["team_status"], res["team"] = resp.StatusCode, body
	claims, _ := body["claims"].(map[string]any)
	log.Printf("audit navident=%v client=%s server=fleet-team-0b tool=whoami outcome=%d duration_ms=%d",
		claims["NAVident"], g.clientID, resp.StatusCode, time.Since(start).Milliseconds())
	writeJSON(w, resp.StatusCode, res)
}

func (g *gateway) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /oauth-client.json", g.cimd)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", g.prm)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", g.prm)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", g.asMetadata)
	mux.HandleFunc("GET /authorize", g.authorize)
	mux.HandleFunc("POST /consent", g.consentPost)
	mux.HandleFunc("GET /oauth2/callback", g.callback)
	mux.HandleFunc("POST /token", g.token)
	mux.HandleFunc("/mcp", g.mcp)
}

// ---------- team MCP ----------

// teamHandler validates via Texas introspection and returns claims only.
func teamHandler(w http.ResponseWriter, r *http.Request) {
	res := map[string]any{"app": os.Getenv("NAIS_APP_NAME")}
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		res["error"] = "no bearer token"
		writeJSON(w, 401, res)
		return
	}
	resp, err := client.PostForm(os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"), url.Values{"identity_provider": {"entra_id"}, "token": {tok}})
	var in map[string]any
	if err == nil {
		_ = json.NewDecoder(resp.Body).Decode(&in)
		resp.Body.Close()
	}
	if in["active"] != true {
		res["error"] = "introspection failed"
		res["introspection_error"] = in["error"]
		writeJSON(w, 401, res)
		return
	}
	claims := map[string]any{}
	for _, k := range []string{"aud", "azp", "azp_name", "NAVident", "scp", "idtyp", "tid", "groups", "exp", "iat"} {
		if v, ok := in[k]; ok {
			claims[k] = v
		}
	}
	res["claims"] = claims
	if msg := teamCheck(in, os.Getenv("AZURE_APP_CLIENT_ID"), gatewayClientID()); msg != "" {
		res["error"] = msg
		writeJSON(w, 403, res)
		return
	}
	writeJSON(w, 200, res)
}

func teamCheck(in map[string]any, self, gw string) string {
	switch {
	case in["aud"] != self:
		return "aud is not this app"
	case in["idtyp"] == "app":
		return "app token rejected"
	case gw == "" || in["azp"] != gw:
		return "azp is not the gateway"
	case in["NAVident"] == nil || in["NAVident"] == "":
		return "NAVident missing"
	}
	return ""
}

// gatewayClientID finds the gateway's client id among the pre-authorized apps Nais injects.
func gatewayClientID() string {
	var apps []struct{ Name, ClientID string }
	_ = json.Unmarshal([]byte(os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS")), &apps)
	for _, a := range apps {
		if a.Name == os.Getenv("GATEWAY_NAME") {
			return a.ClientID
		}
	}
	return ""
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/isalive", func(http.ResponseWriter, *http.Request) {})
	if os.Getenv("ROLE") == "gateway" {
		g := newGateway(os.Getenv("BASE_URL"), strings.Split(os.Getenv("REDIRECT_URIS"), ","))
		g.teamScope, g.teamURL = os.Getenv("TEAM_SCOPE"), os.Getenv("TEAM_URL")
		g.entraID, g.entraSecret, g.entraToken = os.Getenv("AZURE_APP_CLIENT_ID"), os.Getenv("AZURE_APP_CLIENT_SECRET"), os.Getenv("AZURE_OPENID_CONFIG_TOKEN_ENDPOINT")
		g.routes(mux)
	} else {
		mux.HandleFunc("/", teamHandler)
	}
	log.Fatal(http.ListenAndServe(":8080", mux))
}
