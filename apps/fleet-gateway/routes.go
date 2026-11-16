package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed pilot-routes.yaml
var defaultPilotRoutesYAML []byte

// PilotRoutesConfig is the versioned pilot route list loaded at startup.
type PilotRoutesConfig struct {
	Version int          `yaml:"version" json:"version"`
	Routes  []PilotRoute `yaml:"routes" json:"routes"`
}

// PilotRoute defines one allowed MCP route and its upstream target.
type PilotRoute struct {
	Route         string `yaml:"route" json:"route"`
	Team          string `yaml:"team" json:"team"`
	Target        string `yaml:"target" json:"target"`
	UpstreamURL   string `yaml:"upstream_url" json:"upstream_url"`
	RequiredGroup string `yaml:"required_group,omitempty" json:"required_group,omitempty"`

	Cluster     string   `yaml:"-" json:"-"`
	Namespace   string   `yaml:"-" json:"-"`
	App         string   `yaml:"-" json:"-"`
	ParsedURL   *url.URL `yaml:"-" json:"-"`
	FullRouteID string   `yaml:"-" json:"-"`
}

func loadPilotRoutes(filePath string) ([]PilotRoute, error) {
	raw := defaultPilotRoutesYAML
	if trimmed := strings.TrimSpace(filePath); trimmed != "" {
		data, err := os.ReadFile(trimmed) //nolint:gosec // configured at startup via PILOT_ROUTES_FILE
		if err != nil {
			return nil, fmt.Errorf("read pilot routes file %q: %w", trimmed, err)
		}
		raw = data
	}
	return parseAndValidatePilotRoutes(raw)
}

func parseAndValidatePilotRoutes(raw []byte) ([]PilotRoute, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("pilot routes config is empty")
	}

	var doc PilotRoutesConfig
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
	if doc.Version != 1 {
		return nil, fmt.Errorf("unsupported pilot routes version %d", doc.Version)
	}
	return validatePilotRoutes(doc.Routes)
}

func validatePilotRoutes(routes []PilotRoute) ([]PilotRoute, error) {
	if len(routes) == 0 {
		return nil, errors.New("pilot routes list must not be empty")
	}

	seen := make(map[string]bool, len(routes))
	out := make([]PilotRoute, len(routes))
	for i, r := range routes {
		r.Route = strings.TrimSpace(r.Route)
		r.Team = strings.TrimSpace(r.Team)
		r.Target = strings.TrimSpace(r.Target)
		r.UpstreamURL = strings.TrimSpace(r.UpstreamURL)
		r.RequiredGroup = strings.TrimSpace(r.RequiredGroup)

		if err := validatePilotRoutePath(r.Route); err != nil {
			return nil, fmt.Errorf("route[%d] invalid route %q: %w", i, r.Route, err)
		}
		if seen[r.Route] {
			return nil, fmt.Errorf("duplicate route %q", r.Route)
		}
		seen[r.Route] = true

		if r.Team == "" {
			return nil, fmt.Errorf("route %q: team is required", r.Route)
		}

		cluster, ns, app, err := parseTarget(r.Target)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", r.Route, err)
		}
		r.Cluster = cluster
		r.Namespace = ns
		r.App = app

		parsed, err := validateInternalUpstreamURL(r.UpstreamURL, ns)
		if err != nil {
			return nil, fmt.Errorf("route %q: invalid upstream_url %q: %w", r.Route, r.UpstreamURL, err)
		}
		r.ParsedURL = parsed

		if strings.ContainsAny(r.RequiredGroup, " \t\r\n") {
			return nil, fmt.Errorf("route %q: required_group must not contain whitespace", r.Route)
		}

		out[i] = r
	}
	return out, nil
}

func validatePilotRoutePath(route string) error {
	if !strings.HasPrefix(route, "/mcp/") {
		return errors.New("must start with /mcp/")
	}
	name := strings.TrimPrefix(route, "/mcp/")
	if name == "" || strings.Contains(name, "/") {
		return errors.New("must match /mcp/<name> with a single non-empty segment")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		if !isLower && !isUpper && !isDigit && c != '-' && c != '_' && c != '.' {
			return fmt.Errorf("invalid character %q in route name", c)
		}
	}
	if name == "." || name == ".." {
		return errors.New("invalid route segment")
	}
	return nil
}

func parseTarget(target string) (cluster, namespace, app string, err error) {
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

func validateInternalUpstreamURL(raw, namespace string) (*url.URL, error) {
	if err := validateHTTPURL(raw, false); err != nil {
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
	if strings.Contains(host, ".ekstern.") {
		return nil, fmt.Errorf("upstream host %q must not be public (.ekstern.)", host)
	}

	// Cluster-internal service DNS: only <app>.<namespace> or <app>.<namespace>.svc.cluster.local.
	if app, ok := strings.CutSuffix(host, "."+namespace+".svc.cluster.local"); ok && isSingleDNSLabel(app) {
		return u, nil
	}
	if app, ok := strings.CutSuffix(host, "."+namespace); ok && isSingleDNSLabel(app) {
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
