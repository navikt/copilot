package local

// Bring your own endpoint: an OpenAI-compatible server nav-pilot did not start
// (Ollama, llama-server, LM Studio, vLLM), named by the local_endpoint config
// key. nav-pilot starts nothing and downloads nothing for it. It points the
// loop guard, the clients and decide at that URL, and says everywhere that the
// model is unsupported and unmeasured.
//
// The address is a trust boundary. Code, diffs and tool results go to it, and
// decide and ask bypass the guard. So only loopback and private addresses are
// accepted, twice: [ValidateEndpoint] refuses anything else when the key is
// set, and every connection to the server is checked again at dial time
// ([serverTransport]), after DNS, so a hostname that resolves somewhere else,
// or a redirect, cannot take a prompt off the machine or the private network.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// BackendEndpoint is the Model.Backend of the entry [EndpointManifest] makes.
// It never appears in a served manifest: checkModels refuses it there.
const BackendEndpoint = "endpoint"

var endpointURL, endpointModel string

// SetEndpoint puts this process in endpoint mode: base is the server's base
// URL as [ValidateEndpoint] returned it, model the id to ask it for. Empty base
// turns it off. nav-pilot calls it once at startup, beside [SetEnabled].
func SetEndpoint(base, model string) { endpointURL, endpointModel = base, model }

// Endpoint returns the configured endpoint and model, or two empty strings
// when this process runs the managed mlx-lm server.
func Endpoint() (base, model string) { return endpointURL, endpointModel }

// ValidateEndpoint checks a local_endpoint value and returns its base URL: the
// scheme, host, port and any path prefix, without a trailing /v1, since every
// caller appends /v1/... itself. The host must be localhost or a loopback or
// private IP address; a hostname is refused rather than resolved, because the
// answer can change after this check.
func ValidateEndpoint(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("local_endpoint %q is not an http(s) URL. Example: http://127.0.0.1:11434/v1", raw)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("local_endpoint %q has a user, a query or a fragment; give only scheme, host, port and path, as in http://127.0.0.1:11434/v1", raw)
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !allowedIP(ip)) {
		return "", fmt.Errorf(
			"local_endpoint %q is not on this machine or a private network. nav-pilot sends code, diffs and tool results to it, so it accepts only localhost and loopback or private IP addresses (127.0.0.0/8, ::1, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7). For a server on your network, use its IP address",
			raw)
	}
	path := strings.TrimSuffix(strings.TrimRight(u.EscapedPath(), "/"), "/v1")
	return u.Scheme + "://" + u.Host + path, nil
}

// allowedIP is the address rule both checks share.
func allowedIP(ip net.IP) bool { return ip.IsLoopback() || ip.IsPrivate() }

// checkDial refuses a connection to an address outside [allowedIP]. It runs
// after name resolution, on the address actually dialled, so it also covers a
// hostname such as localhost and any redirect a client follows.
func checkDial(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil || !allowedIP(ip) {
		return fmt.Errorf("nav-pilot refused to connect to %s: the local server must be on this machine or a private network", address)
	}
	return nil
}

// serverTransport carries every request nav-pilot makes to a local server,
// managed or not: no proxy (an HTTPS_PROXY would otherwise carry a prompt for a
// private address out through the proxy) and [checkDial] on every connection.
var serverTransport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, Control: checkDial}).DialContext
	return t
}()

// refuseRedirect is the guard's answer to a redirect from the server: an
// error, which the reverse proxy turns into a 502.
func refuseRedirect(r *http.Response) error {
	if r.StatusCode >= 300 && r.StatusCode < 400 {
		return fmt.Errorf("the local server answered %s with a redirect to %q; nav-pilot does not pass redirects on", r.Status, r.Header.Get("Location"))
	}
	return nil
}

// ServerClient is the HTTP client for requests to the local server. No
// timeout: a local completion at a large context legitimately takes minutes,
// and the caller's context bounds it.
//
// It follows no redirect: no completion server sends one, and a redirect
// would make one request into two, possibly to another service on the same
// network. The 3xx comes back to the caller as the answer.
var ServerClient = &http.Client{
	Transport:     serverTransport,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// EndpointManifest is the one-entry manifest endpoint mode answers from, so
// [IsLocal], [Lookup] and [Chosen] name the endpoint's model and nothing else.
// No capabilities: nothing about the model has been measured, so the dispatch
// policy gets the text written for a manifest without verdicts.
func EndpointManifest(model string) *Manifest {
	return &Manifest{Models: []Model{{
		Key:     "endpoint",
		Name:    model + " (own server, unmeasured)",
		Model:   model,
		Backend: BackendEndpoint,
		Default: true,
		Expect:  "Unsupported and unmeasured: a model on the developer's own server (local_endpoint) that nav-pilot has not benchmarked.",
	}}}
}

// ServedModel is the model the local server answers with: the endpoint's in
// endpoint mode, otherwise the recorded server's.
func ServedModel() (string, bool, error) {
	if endpointURL != "" {
		return endpointModel, true, nil
	}
	st, ok, err := LoadState()
	return st.Model, ok, err
}

// Backend names what serves local completions, for telemetry: "endpoint" or
// "mlx".
func Backend() string {
	if endpointURL != "" {
		return BackendEndpoint
	}
	return "mlx"
}

// TelemetryModel is a local model id as telemetry may carry it. An endpoint's
// id is whatever the developer typed, so it is recorded as "custom"; manifest
// ids are bounded and go through as they are.
func TelemetryModel(id string) string {
	if endpointURL != "" && id != "" {
		return "custom"
	}
	return id
}

// ErrEndpointDown is [EnsureOwnServer]'s answer in endpoint mode when the
// server does not answer.
var ErrEndpointDown = errors.New("the local endpoint is not answering")

// probeEndpoint is endpoint mode's stand-in for the ownership proof: nav-pilot
// owns nothing here, so it asks only whether the server answers.
func probeEndpoint() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL+"/v1/models", nil)
	if err != nil {
		return err
	}
	resp, err := ServerClient.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		err = errors.New(resp.Status)
	}
	start := "for example " + domain.Bold("ollama serve") + " or " + domain.Bold("llama-server --jinja -c 65536 -m <model.gguf>")
	if strings.HasSuffix(endpointURL, ":11434") {
		start = "Ollama's default port: " + domain.Bold("ollama serve")
	}
	return fmt.Errorf("%w at %s (%v).\n\n  Start your server (%s), then check it: %s",
		ErrEndpointDown, endpointURL, err, start, domain.Bold("nav-pilot alpha local doctor"))
}
