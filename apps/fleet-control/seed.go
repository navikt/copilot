package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Route is a fleet-gateway route. Path is /mcp/<namespace>/<name>.
type Route struct {
	Path        string   `json:"path"`
	Target      string   `json:"target"`
	UpstreamURL string   `json:"upstream_url"`
	Enabled     bool     `json:"enabled"`
	Tools       []string `json:"tools"`
}

// Server is one published server version: a registry entry, a route, or both.
type Server struct {
	Namespace    string          `json:"namespace"`
	Name         string          `json:"name"`
	RegistryName string          `json:"registryName,omitempty"`
	Registry     json.RawMessage `json:"registry,omitempty"`
	Route        *Route          `json:"-"` // served only by /internal/v1/routes and the routes snapshot
	Status       string          `json:"-"`
	// gatewayPath is the route a registry entry's gateway remote points to.
	gatewayPath string
}

// Key is the server's identity, "<namespace>/<name>".
func (s Server) Key() string { return s.Namespace + "/" + s.Name }

var (
	dnsLabelRegex   = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	toolNameRegex   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
	serverNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*/[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	// internalHostSuffixes are reachable only inside Nav; the registry is public.
	internalHostSuffixes = []string{".intern.nav.no", ".intern.dev.nav.no", ".nais.adeo.no", ".adeo.no", ".svc.cluster.local", ".internal", ".local", ".localhost"}
)

const (
	gatewayRemotePrefix = "https://fleet-gateway.{{domain_internal}}"
	internalTemplate    = "{{domain_internal}}"
)

// routesFile is the pilot-routes.<env>.yaml format read by fleet-gateway.
type routesFile struct {
	Version int `yaml:"version"`
	Routes  []struct {
		Route        string   `yaml:"route"`
		Team         string   `yaml:"team"`
		Target       string   `yaml:"target"`
		UpstreamURL  string   `yaml:"upstream_url"`
		Enabled      *bool    `yaml:"enabled"`
		Tools        []string `yaml:"tools"`
		PassAllTools bool     `yaml:"passAllTools"`
	} `yaml:"routes"`
}

// seedServers turns today's Git files into servers. Both files are untrusted
// input: every name, URL and tool is validated, and any error stops the seed.
func seedServers(routesYAML, allowlistJSON []byte) ([]Server, error) {
	byKey := map[string]*Server{}

	routes, err := parseRoutes(routesYAML)
	if err != nil {
		return nil, err
	}
	for _, s := range routes {
		if byKey[s.Key()] != nil {
			return nil, fmt.Errorf("duplicate route for %s", s.Key())
		}
		byKey[s.Key()] = &s
	}

	entries, err := parseAllowlist(allowlistJSON)
	if err != nil {
		return nil, err
	}
	seenRegistry := map[string]string{}
	for _, e := range entries {
		key := e.Key()
		if other, ok := seenRegistry[key]; ok {
			return nil, fmt.Errorf("registry entries %q and %q both map to %s", other, e.RegistryName, key)
		}
		seenRegistry[key] = e.RegistryName
		if s := byKey[key]; s != nil {
			// Join only through a gateway remote naming this exact route.
			if s.Route == nil || e.gatewayPath != s.Route.Path {
				return nil, fmt.Errorf("registry entry %q maps to %s, which a route already uses without a gateway remote to it", e.RegistryName, key)
			}
			s.RegistryName, s.Registry = e.RegistryName, e.Registry
			continue
		}
		byKey[key] = &e
	}

	out := make([]Server, 0, len(byKey))
	for _, s := range byKey {
		s.Status = "published"
		out = append(out, *s)
	}
	slices.SortFunc(out, func(a, b Server) int { return strings.Compare(a.Key(), b.Key()) })
	return out, nil
}

func parseRoutes(raw []byte) ([]Server, error) {
	var doc routesFile
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse routes: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("routes file must contain a single YAML document")
	}
	if doc.Version != 2 {
		return nil, fmt.Errorf("unsupported routes version %d", doc.Version)
	}
	if len(doc.Routes) == 0 {
		return nil, errors.New("routes file has no routes")
	}
	out := make([]Server, 0, len(doc.Routes))
	for _, r := range doc.Routes {
		ns, name, err := splitRoutePath(r.Route)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", r.Route, err)
		}
		if r.PassAllTools {
			return nil, fmt.Errorf("route %q: passAllTools needs two approvals and cannot be seeded", r.Route)
		}
		if r.Team != ns {
			return nil, fmt.Errorf("route %q: team %q must equal namespace %q", r.Route, r.Team, ns)
		}
		parts := strings.Split(r.Target, ":")
		if len(parts) != 3 || parts[0] == "" || parts[1] != ns || !dnsLabelRegex.MatchString(parts[2]) {
			return nil, fmt.Errorf("route %q: target %q must be <cluster>:%s:<app>", r.Route, r.Target, ns)
		}
		if err := validateUpstreamURL(r.UpstreamURL, ns); err != nil {
			return nil, fmt.Errorf("route %q: upstream_url: %w", r.Route, err)
		}
		if err := validateTools(r.Tools); err != nil {
			return nil, fmt.Errorf("route %q: %w", r.Route, err)
		}
		out = append(out, Server{Namespace: ns, Name: name, Route: &Route{
			Path: r.Route, Target: r.Target, UpstreamURL: r.UpstreamURL,
			Enabled: r.Enabled == nil || *r.Enabled, Tools: r.Tools,
		}})
	}
	return out, nil
}

func splitRoutePath(p string) (ns, name string, err error) {
	segs := strings.Split(strings.TrimPrefix(p, "/mcp/"), "/")
	if !strings.HasPrefix(p, "/mcp/") || len(segs) != 2 || !dnsLabelRegex.MatchString(segs[0]) || !dnsLabelRegex.MatchString(segs[1]) {
		return "", "", errors.New("path must be /mcp/<namespace>/<name> with lowercase DNS labels")
	}
	return segs[0], segs[1], nil
}

func validateTools(tools []string) error {
	if len(tools) == 0 {
		return errors.New("tools must not be empty")
	}
	seen := map[string]bool{}
	for _, t := range tools {
		if !toolNameRegex.MatchString(t) {
			return fmt.Errorf("invalid tool name %q", t)
		}
		if seen[t] {
			return fmt.Errorf("duplicate tool %q", t)
		}
		seen[t] = true
	}
	return nil
}

// validateUpstreamURL accepts cluster-internal service DNS in namespace ns
// (http or https) and Nav internal ingresses over https, like fleet-gateway.
func validateUpstreamURL(raw, ns string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("must be http(s) without userinfo")
	}
	host, err := normalizeHost(u.Hostname())
	if err != nil {
		return err
	}
	if net.ParseIP(host) != nil || numericTLD(host) || strings.Contains(host, "xn--") {
		return errors.New("IP addresses are not allowed")
	}
	for _, suffix := range []string{"." + ns + ".svc.cluster.local", "." + ns} {
		if app, ok := strings.CutSuffix(host, suffix); ok && dnsLabelRegex.MatchString(app) {
			return nil
		}
	}
	if u.Scheme == "https" && (strings.HasSuffix(host, ".intern.nav.no") || strings.HasSuffix(host, ".intern.dev.nav.no") || strings.HasSuffix(host, ".nais.adeo.no")) {
		return nil
	}
	return fmt.Errorf("host %q is not an internal service in namespace %q", host, ns)
}

// parseAllowlist reads apps/mcp-registry/allowlist.json. Entries stay as the
// public JSON they are today, except that remotes on internal hosts other than
// fleet-gateway are dropped (owner decision on #2334).
func parseAllowlist(raw []byte) ([]Server, error) {
	var doc struct {
		Servers []json.RawMessage `json:"servers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse allowlist: %w", err)
	}
	if len(doc.Servers) == 0 {
		return nil, errors.New("allowlist has no servers")
	}
	out := make([]Server, 0, len(doc.Servers))
	for i, rawEntry := range doc.Servers {
		s, err := parseRegistryEntry(rawEntry)
		if err != nil {
			return nil, fmt.Errorf("allowlist server[%d]: %w", i, err)
		}
		out = append(out, s)
	}
	return out, nil
}

func parseRegistryEntry(raw json.RawMessage) (Server, error) {
	var entry map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&entry); err != nil {
		return Server{}, err
	}
	name, _ := entry["name"].(string)
	if len(name) < 3 || len(name) > 200 || !serverNameRegex.MatchString(name) {
		return Server{}, fmt.Errorf("invalid name %q", name)
	}
	if v, ok := entry["websiteUrl"].(string); ok {
		if err := validatePublicURL(v, false); err != nil {
			return Server{}, fmt.Errorf("%s: websiteUrl: %w", name, err)
		}
	}
	if repo, ok := entry["repository"].(map[string]any); ok {
		v, _ := repo["url"].(string)
		if err := validatePublicURL(v, false); err != nil {
			return Server{}, fmt.Errorf("%s: repository.url: %w", name, err)
		}
	}

	gatewayPath := ""
	ns, short := "copilot", strings.ToLower(name[strings.LastIndex(name, "/")+1:])
	if remotes, ok := entry["remotes"].([]any); ok {
		kept := remotes[:0]
		for _, r := range remotes {
			rm, _ := r.(map[string]any)
			u, _ := rm["url"].(string)
			if rest, ok := strings.CutPrefix(u, gatewayRemotePrefix); ok {
				var err error
				if gatewayPath != "" {
					return Server{}, fmt.Errorf("%s: more than one gateway remote", name)
				}
				if ns, short, err = splitRoutePath(rest); err != nil {
					return Server{}, fmt.Errorf("%s: gateway remote %q: %w", name, u, err)
				}
				gatewayPath = rest
				kept = append(kept, r)
				continue
			}
			if strings.Contains(u, internalTemplate) {
				slog.Info("seed drops internal remote; only the gateway may be internal", "server", name, "url", u)
				continue
			}
			if err := validatePublicURL(u, true); err != nil {
				return Server{}, fmt.Errorf("%s: remote: %w", name, err)
			}
			kept = append(kept, r)
		}
		entry["remotes"] = kept
	}
	if !dnsLabelRegex.MatchString(short) {
		return Server{}, fmt.Errorf("%s: derived server name %q is not a DNS label", name, short)
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return Server{}, err
	}
	return Server{Namespace: ns, Name: short, RegistryName: name, Registry: body, gatewayPath: gatewayPath}, nil
}

// validatePublicURL allows https on port 443 to a public DNS name, and plain
// http to loopback only when allowLoopback is set (local IDE MCP servers).
func validatePublicURL(raw string, allowLoopback bool) error {
	for i := 0; i < len(raw); i++ {
		if raw[i] <= ' ' || raw[i] > 126 || raw[i] == '{' {
			return errors.New("url must be printable ASCII without templates")
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.User != nil {
		return errors.New("userinfo is not allowed")
	}
	if strings.ContainsAny(u.Host, "%[]") && !allowLoopback {
		return errors.New("IP literals and zone IDs are not allowed")
	}
	host, err := normalizeHost(u.Hostname())
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); host == "localhost" || (ip != nil && ip.IsLoopback()) {
		if allowLoopback && u.Scheme == "http" {
			return nil
		}
		return errors.New("loopback is only allowed as plain http in remotes")
	}
	if u.Scheme != "https" || (u.Port() != "" && u.Port() != "443") {
		return errors.New("must be https on port 443")
	}
	if strings.Contains(host, "xn--") || net.ParseIP(host) != nil || !strings.Contains(host, ".") || numericTLD(host) {
		return fmt.Errorf("host %q must be an ASCII DNS name", host)
	}
	for _, suffix := range internalHostSuffixes {
		if strings.HasSuffix("."+host, suffix) {
			return fmt.Errorf("host %q is internal", host)
		}
	}
	return nil
}

// normalizeHost lowercases host and strips one trailing root dot, so
// "svc.intern.nav.no." is checked as "svc.intern.nav.no".
func normalizeHost(host string) (string, error) {
	h := strings.TrimSuffix(strings.ToLower(host), ".")
	if h == "" || strings.HasSuffix(h, ".") || strings.Contains(h, "..") {
		return "", fmt.Errorf("invalid host %q", host)
	}
	return h, nil
}

// numericTLD catches IP forms net.ParseIP misses, such as "127.1" or "0x7f.1".
func numericTLD(host string) bool {
	tld := host[strings.LastIndex(host, ".")+1:]
	return strings.Trim(tld, "0123456789abcdefx") == "" && strings.ContainsAny(tld, "0123456789")
}
