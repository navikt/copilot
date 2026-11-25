// Package routing loads and validates the pilot MCP routes and builds the
// route table the gateway serves.
package routing

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/navikt/copilot/fleet-gateway/internal/config"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// File is the versioned pilot route list loaded at startup.
type File struct {
	Version int     `yaml:"version" json:"version"`
	Routes  []Route `yaml:"routes" json:"routes"`
}

// Route defines one allowed MCP route and its upstream target.
type Route struct {
	Route       string `yaml:"route" json:"route"`
	Team        string `yaml:"team" json:"team"`
	Target      string `yaml:"target" json:"target"`
	UpstreamURL string `yaml:"upstream_url" json:"upstream_url"`
	Enabled     *bool  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	// Tools is the version 2 allowlist of tool names for tools/call.
	Tools []string `yaml:"tools,omitempty" json:"tools,omitempty"`
	// PassAllTools is operator-only: a version 2 route without Tools forwards
	// every tools/call.
	PassAllTools bool `yaml:"passAllTools,omitempty" json:"passAllTools,omitempty"`

	Cluster     string   `yaml:"-" json:"-"`
	Namespace   string   `yaml:"-" json:"-"`
	App         string   `yaml:"-" json:"-"`
	ParsedURL   *url.URL `yaml:"-" json:"-"`
	FullRouteID string   `yaml:"-" json:"-"`
	// AllowedTools is set when the route enforces a tool allowlist; nil passes all.
	AllowedTools map[string]bool `yaml:"-" json:"-"`
}

// Disabled reports whether the route is switched off with enabled: false.
func (r Route) Disabled() bool {
	return r.Enabled != nil && !*r.Enabled
}

// Load reads and validates the route file at filePath.
func Load(filePath string) ([]Route, error) {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" {
		return nil, errors.New("PILOT_ROUTES_FILE is required")
	}
	raw, err := os.ReadFile(trimmed) //nolint:gosec // configured at startup via PILOT_ROUTES_FILE
	if err != nil {
		return nil, fmt.Errorf("read pilot routes file %q: %w", trimmed, err)
	}
	return Parse(raw)
}

// Parse decodes and validates a route file.
func Parse(raw []byte) ([]Route, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("pilot routes config is empty")
	}

	var doc File
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse pilot routes: %w", err)
	}
	// Reject multi-document YAML streams so trailing documents cannot be ignored silently.
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("pilot routes config must contain a single YAML/JSON document")
	}
	if doc.Version == 1 {
		return nil, errors.New("pilot routes version 1 is no longer supported: it passed every tool call; set version: 2 and list tools per route (or passAllTools: true, operator only)")
	}
	if doc.Version != 2 {
		return nil, fmt.Errorf("unsupported pilot routes version %d", doc.Version)
	}
	for i := range doc.Routes {
		if err := ApplyToolPolicy(&doc.Routes[i]); err != nil {
			return nil, err
		}
	}
	return Validate(doc.Routes)
}

// mcpToolName is the MCP tool-name charset (spec 2025-11-25, "Tool names").
// Restricting to ASCII keeps names safe to render as agentgateway CEL literals.
var mcpToolName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

// ApplyToolPolicy validates tools and passAllTools and builds the allowlist.
// Version 2 denies by default: a route must list tools or be marked
// passAllTools by the operator, not both.
func ApplyToolPolicy(r *Route) error {
	name := strings.TrimSpace(r.Route)
	if r.Tools == nil {
		if !r.PassAllTools {
			return fmt.Errorf("route %q: version 2 routes must declare tools (or passAllTools, set by the operator)", name)
		}
		return nil
	}
	if r.PassAllTools {
		return fmt.Errorf("route %q: tools and passAllTools are mutually exclusive", name)
	}
	if len(r.Tools) == 0 {
		return fmt.Errorf("route %q: tools must not be empty", name)
	}
	r.AllowedTools = make(map[string]bool, len(r.Tools))
	for _, t := range r.Tools {
		if !mcpToolName.MatchString(t) {
			return fmt.Errorf("route %q: invalid tool name %q", name, t)
		}
		if r.AllowedTools[t] {
			return fmt.Errorf("route %q: duplicate tool %q", name, t)
		}
		r.AllowedTools[t] = true
	}
	return nil
}

// Validate checks every route and fills in the derived fields.
func Validate(routes []Route) ([]Route, error) {
	if len(routes) == 0 {
		return nil, errors.New("pilot routes list must not be empty")
	}

	seen := make(map[string]bool, len(routes))
	out := make([]Route, len(routes))
	for i, r := range routes {
		r.Route = strings.TrimSpace(r.Route)
		r.Team = strings.TrimSpace(r.Team)
		r.Target = strings.TrimSpace(r.Target)
		r.UpstreamURL = strings.TrimSpace(r.UpstreamURL)

		ns, err := validatePilotRoutePath(r.Route)
		if err != nil {
			return nil, fmt.Errorf("route[%d] invalid route %q: %w", i, r.Route, err)
		}
		if seen[r.Route] {
			return nil, fmt.Errorf("duplicate route %q", r.Route)
		}
		seen[r.Route] = true

		if r.Team == "" {
			return nil, fmt.Errorf("route %q: team is required", r.Route)
		}

		cluster, targetNS, app, err := ParseTarget(r.Target)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", r.Route, err)
		}
		// Path is /mcp/<namespace>/<name> (#2342 §3.3): the first segment is the owning namespace.
		if ns != targetNS {
			return nil, fmt.Errorf("route %q: path namespace %q must equal target namespace %q", r.Route, ns, targetNS)
		}
		r.Cluster = cluster
		r.Namespace = ns
		r.App = app

		parsed, err := ValidateUpstreamURL(r.UpstreamURL, ns)
		if err != nil {
			return nil, fmt.Errorf("route %q: invalid upstream_url %q: %w", r.Route, r.UpstreamURL, err)
		}
		r.ParsedURL = parsed

		out[i] = r
	}
	return out, nil
}

// validatePilotRoutePath checks /mcp/<namespace>/<name> and returns the namespace.
func validatePilotRoutePath(route string) (string, error) {
	if !strings.HasPrefix(route, "/mcp/") {
		return "", errors.New("must start with /mcp/")
	}
	segs := strings.Split(strings.TrimPrefix(route, "/mcp/"), "/")
	if len(segs) != 2 {
		return "", errors.New("must match /mcp/<namespace>/<name>")
	}
	for _, seg := range segs {
		if seg == "" || seg == "." || seg == ".." {
			return "", errors.New("invalid route segment")
		}
		for i := 0; i < len(seg); i++ {
			c := seg[i]
			isLower := c >= 'a' && c <= 'z'
			isUpper := c >= 'A' && c <= 'Z'
			isDigit := c >= '0' && c <= '9'
			if !isLower && !isUpper && !isDigit && c != '-' && c != '_' && c != '.' {
				return "", fmt.Errorf("invalid character %q in route segment", c)
			}
		}
	}
	return segs[0], nil
}

// ParseTarget splits a cluster:namespace:app target.
func ParseTarget(target string) (cluster, namespace, app string, err error) {
	parts := strings.Split(target, ":")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid target %q: expected cluster:namespace:app", target)
	}
	for _, part := range parts {
		if part == "" || strings.ContainsAny(part, " \t\r\n/") {
			return "", "", "", fmt.Errorf("invalid target %q: empty or malformed segment", target)
		}
	}
	return parts[0], parts[1], parts[2], nil
}

// ValidateUpstreamURL accepts only an in-cluster URL in the route's namespace.
func ValidateUpstreamURL(raw, namespace string) (*url.URL, error) {
	if err := config.ValidateHTTPURL(raw, false); err != nil {
		return nil, err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return nil, errors.New("missing hostname")
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return nil, fmt.Errorf("disallowed IP address %q", host)
		}
		if !ip.IsPrivate() {
			return nil, fmt.Errorf("upstream host %q must not be a public IP", host)
		}
		return nil, fmt.Errorf("disallowed IP address %q: raw IP addresses are not allowed", host)
	}

	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, fmt.Errorf("disallowed loopback host %q", host)
	}
	if strings.HasSuffix(host, ".internal") {
		return nil, fmt.Errorf("disallowed internal host %q", host)
	}
	if strings.Contains(host, "xn--") {
		return nil, fmt.Errorf("upstream host %q must not be punycode", host)
	}
	if strings.Contains(host, ".ekstern.") {
		return nil, fmt.Errorf("upstream host %q must not be public (.ekstern.)", host)
	}

	// Cluster-internal service DNS: only <app>.<namespace> or <app>.<namespace>.svc.cluster.local.
	if app, ok := strings.CutSuffix(host, "."+namespace+".svc.cluster.local"); ok && isSingleDNSLabel(app) {
		return u, nil
	}
	// Dial the short form as its FQDN: with namespace "com", evil.com would
	// otherwise resolve on the internet (#2848).
	if app, ok := strings.CutSuffix(host, "."+namespace); ok && isSingleDNSLabel(app) {
		fqdn := host + ".svc.cluster.local"
		if p := u.Port(); p != "" {
			fqdn = net.JoinHostPort(fqdn, p)
		}
		u.Host = fqdn
		return u, nil
	}

	// Internal Nav ingresses must use HTTPS.
	isInternalNav := strings.HasSuffix(host, ".intern.nav.no") ||
		strings.HasSuffix(host, ".intern.dev.nav.no") ||
		strings.HasSuffix(host, ".nais.adeo.no")
	if isInternalNav {
		if u.Scheme != "https" {
			return nil, fmt.Errorf("internal Nav host %q must use https", host)
		}
		return u, nil
	}

	return nil, fmt.Errorf("upstream host %q is public or not recognized as internal", host)
}

func isSingleDNSLabel(label string) bool {
	if label == "" || len(label) > 63 || strings.Contains(label, ".") {
		return false
	}
	for i := 0; i < len(label); i++ {
		c := label[i]
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if !isAlphaNum && c != '-' {
			return false
		}
		if (i == 0 || i == len(label)-1) && c == '-' {
			return false
		}
	}
	return true
}

// Table is the set of MCP routes the gateway serves. A route change replaces
// the whole table rather than editing it (#2641).
type Table struct {
	Routes           []Route // file order; Routes[0] is the primary resource
	RoutesByPath     map[string]Route
	AllowedResources map[string]bool
	PrimaryResource  string
}

// NewTable builds the table from validated routes, keeping their order.
// Disabled routes stay in it, so they answer 503 rather than 404.
func NewTable(routes []Route) *Table {
	rt := &Table{
		Routes:           append([]Route(nil), routes...),
		RoutesByPath:     make(map[string]Route, len(routes)),
		AllowedResources: make(map[string]bool, len(routes)),
	}
	for _, r := range rt.Routes {
		rt.RoutesByPath[r.Route] = r
		rt.AllowedResources[r.FullRouteID] = true
	}
	if len(rt.Routes) > 0 {
		rt.PrimaryResource = rt.Routes[0].FullRouteID
	}
	return rt
}

// ByResource finds the route whose resource URL is res.
func (rt *Table) ByResource(res string) (Route, bool) {
	for _, r := range rt.Routes {
		if r.FullRouteID == res {
			return r, true
		}
	}
	return Route{}, false
}
