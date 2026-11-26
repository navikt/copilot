package model

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var (
	// DNSLabel is a lowercase RFC 1123 label: namespaces, names and apps.
	DNSLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	// ToolName is the tool names a route may allow; no "*" can pass.
	ToolName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
)

// ValidateRoute is the one validator for routes, whether they come from an
// API write or the seed files (#2894). Both are untrusted input. r.Path is
// /mcp/<ns>/<name>; the target must be <cluster>:<ns>:<app> in the gateway's
// own cluster, and the upstream must be that app's service DNS: the gateway
// resolves service DNS in its cluster and sends the target's token there.
func ValidateRoute(cluster string, r Route) error {
	ns, _, err := SplitRoutePath(r.Path)
	if err != nil {
		return fmt.Errorf("route %q: %w", r.Path, err)
	}
	parts := strings.Split(r.Target, ":")
	if len(parts) != 3 || !DNSLabel.MatchString(parts[0]) || parts[1] != ns || !DNSLabel.MatchString(parts[2]) {
		return fmt.Errorf("route.target %q must be <cluster>:%s:<app>", r.Target, ns)
	}
	if parts[0] != cluster {
		return fmt.Errorf("route.target must be in cluster %q, where the gateway resolves service DNS", cluster)
	}
	if err := validateUpstreamURL(r.UpstreamURL, ns, parts[2]); err != nil {
		return fmt.Errorf("route.upstream_url: %w", err)
	}
	if err := ValidateTools(r.Tools); err != nil {
		return fmt.Errorf("route: %w", err)
	}
	return nil
}

// SplitRoutePath splits /mcp/<namespace>/<name>.
func SplitRoutePath(p string) (ns, name string, err error) {
	segs := strings.Split(strings.TrimPrefix(p, "/mcp/"), "/")
	if !strings.HasPrefix(p, "/mcp/") || len(segs) != 2 || !DNSLabel.MatchString(segs[0]) || !DNSLabel.MatchString(segs[1]) {
		return "", "", errors.New("path must be /mcp/<namespace>/<name> with lowercase DNS labels")
	}
	return segs[0], segs[1], nil
}

// ValidateTools requires a non-empty list of unique, well-formed tool names.
func ValidateTools(tools []string) error {
	if len(tools) == 0 {
		return errors.New("tools must not be empty")
	}
	seen := map[string]bool{}
	for _, t := range tools {
		if !ToolName.MatchString(t) {
			return fmt.Errorf("invalid tool name %q", t)
		}
		if seen[t] {
			return fmt.Errorf("duplicate tool %q", t)
		}
		seen[t] = true
	}
	return nil
}

// validateUpstreamURL allows only app's own service DNS in namespace ns, over
// http or https, without userinfo, query string or fragment. An ingress name
// proves nothing about which app answers. fleet-gateway rejects query strings
// and fragments, so they would block the whole snapshot.
func validateUpstreamURL(raw, ns, app string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("must be http(s) without userinfo")
	}
	host, err := NormalizeHost(u.Hostname())
	if err != nil {
		return err
	}
	if net.ParseIP(host) != nil || NumericTLD(host) {
		return errors.New("IP addresses are not allowed")
	}
	if strings.Contains(host, "xn--") {
		return errors.New("punycode hosts are not allowed")
	}
	if host != app+"."+ns && host != app+"."+ns+".svc.cluster.local" {
		return fmt.Errorf("host must be %s.%s or %s.%s.svc.cluster.local", app, ns, app, ns)
	}
	if u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return errors.New("query strings and fragments are not allowed")
	}
	return nil
}

// NormalizeHost rejects non-ASCII, lowercases host and strips one trailing root dot, so
// "svc.intern.nav.no." is checked as "svc.intern.nav.no".
func NormalizeHost(host string) (string, error) {
	// Non-ASCII is refused before lowercasing: Unicode folding maps U+212A
	// KELVIN SIGN to "k", so a different host could compare equal.
	for i := range len(host) {
		if host[i] >= 0x80 {
			return "", fmt.Errorf("invalid host %q: must be ASCII", host)
		}
	}
	h := strings.TrimSuffix(strings.ToLower(host), ".")
	if h == "" || strings.HasSuffix(h, ".") || strings.Contains(h, "..") {
		return "", fmt.Errorf("invalid host %q", host)
	}
	return h, nil
}

// NumericTLD catches IP forms net.ParseIP misses, such as "127.1" or "0x7f.1".
func NumericTLD(host string) bool {
	tld := host[strings.LastIndex(host, ".")+1:]
	return strings.Trim(tld, "0123456789abcdefx") == "" && strings.ContainsAny(tld, "0123456789")
}
