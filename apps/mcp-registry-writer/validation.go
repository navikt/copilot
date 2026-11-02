package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

const (
	NameMinLength = 3
	NameMaxLength = 200
)

var (
	serverNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*/[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	ownerRegex      = regexp.MustCompile(`^[a-z0-9-]+$`)
	hostLabelRegex  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
)

var forbiddenHostSuffixes = []string{
	".intern.nav.no",
	".intern.dev.nav.no",
	".svc.cluster.local",
	".adeo.no",
	".internal",
	".local",
}

// Repository mirrors apps/mcp-registry's Repository shape.
type Repository struct {
	URL       string `json:"url"`
	Source    string `json:"source"`
	ID        string `json:"id,omitempty"`
	Subfolder string `json:"subfolder,omitempty"`
}

// RemoteTransport represents a remote transport with type and url only.
type RemoteTransport struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Submission models strict registry entry submission.
type Submission struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	WebsiteURL  string            `json:"websiteUrl,omitempty"`
	Repository  *Repository       `json:"repository,omitempty"`
	Owner       string            `json:"owner"`
	Remotes     []RemoteTransport `json:"remotes"`
}

// validateServerName matches apps/mcp-registry validation.go server name rules:
// length 3-200, exactly one slash, no leading '.' or '-', matches serverNameRegex.
func validateServerName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("server name is required and cannot be empty")
	}

	if len(name) < NameMinLength {
		return fmt.Errorf("server name must be at least %d characters", NameMinLength)
	}

	if len(name) > NameMaxLength {
		return fmt.Errorf("server name must be at most %d characters", NameMaxLength)
	}

	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "-") {
		return errors.New("server name cannot start with '.' or '-'")
	}

	if !strings.Contains(name, "/") {
		return errors.New("server name must be in reverse-DNS format with exactly one '/' (e.g., 'io.github.org/server-name')")
	}

	if strings.Count(name, "/") > 1 {
		return errors.New("server name cannot contain multiple slashes")
	}

	if !serverNameRegex.MatchString(name) {
		return fmt.Errorf("server name %q is invalid, must match pattern '^[a-zA-Z0-9][a-zA-Z0-9.-]*/[a-zA-Z0-9][a-zA-Z0-9._-]*$'", name)
	}

	return nil
}

// validateHTTPSURL validates a URL against strict security rules:
// https only; reject userinfo; host via u.Hostname(), lowercase, ASCII only (reject non-ASCII/IDN);
// must contain a dot and a non-numeric last label; reject any IP literal including zone IDs
// and integer/hex forms; reject localhost and forbidden suffixes; reject explicit non-443 ports.
func validateHTTPSURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return errors.New("url cannot be empty")
	}

	for i := 0; i < len(rawURL); i++ {
		if rawURL[i] > 127 {
			return errors.New("url contains non-ASCII characters")
		}
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid url format: %w", err)
	}

	if strings.ToLower(u.Scheme) != "https" {
		return fmt.Errorf("url must use https scheme, got %q", u.Scheme)
	}

	if u.User != nil {
		return errors.New("userinfo is not allowed in url")
	}

	if port := u.Port(); port != "" && port != "443" {
		return fmt.Errorf("explicit non-443 port %q is not allowed", port)
	}

	if strings.Contains(u.Host, "%") || strings.Contains(u.Host, "[") || strings.Contains(u.Host, "]") {
		return errors.New("IP literals or zone IDs are not allowed in host")
	}

	rawHost := u.Hostname()
	if rawHost == "" {
		return errors.New("url host cannot be empty")
	}

	for _, r := range rawHost {
		if r > 127 {
			return errors.New("host contains non-ASCII characters")
		}
	}

	host := strings.ToLower(rawHost)

	if strings.Contains(host, "xn--") {
		return errors.New("IDN (punycode) host is not allowed")
	}

	if strings.Contains(host, ":") {
		return errors.New("IPv6 literals are not allowed")
	}

	if net.ParseIP(host) != nil {
		return fmt.Errorf("IP literal %q is not allowed", host)
	}

	if !strings.Contains(host, ".") {
		return errors.New("host must contain at least one dot")
	}

	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
		return errors.New("host contains leading, trailing, or consecutive dots")
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if !hostLabelRegex.MatchString(label) {
			return fmt.Errorf("host label %q contains invalid characters", label)
		}
		if strings.HasPrefix(label, "0x") {
			return fmt.Errorf("hex IP form %q is not allowed", label)
		}
	}

	lastLabel := labels[len(labels)-1]
	isNumeric := true
	for i := 0; i < len(lastLabel); i++ {
		if lastLabel[i] < '0' || lastLabel[i] > '9' {
			isNumeric = false
			break
		}
	}
	if isNumeric {
		return fmt.Errorf("host last label %q cannot be numeric", lastLabel)
	}

	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("localhost is not allowed")
	}

	h := "." + host
	for _, suffix := range forbiddenHostSuffixes {
		if strings.HasSuffix(h, suffix) {
			return fmt.Errorf("internal host %q with suffix %q is not allowed", host, suffix)
		}
	}

	return nil
}

func validateOwner(owner string) error {
	if strings.TrimSpace(owner) == "" {
		return errors.New("owner is required and cannot be empty")
	}
	if !ownerRegex.MatchString(owner) {
		return fmt.Errorf("owner %q is invalid: must match ^[a-z0-9-]+$", owner)
	}
	return nil
}

// validateServerEntry validates that entryJSON matches the typed Submission struct strictly,
// rejecting any unknown fields. Returns the Submission, its re-marshaled canonical JSON, and any error.
func validateServerEntry(entryJSON []byte) (*Submission, []byte, error) {
	if len(entryJSON) == 0 {
		return nil, nil, errors.New("entry cannot be empty")
	}

	dec := json.NewDecoder(bytes.NewReader(entryJSON))
	dec.DisallowUnknownFields()

	var sub Submission
	if err := dec.Decode(&sub); err != nil {
		return nil, nil, fmt.Errorf("invalid entry JSON: %w", err)
	}

	if dec.More() {
		return nil, nil, errors.New("unexpected trailing data after JSON entry")
	}

	if err := validateServerName(sub.Name); err != nil {
		return nil, nil, err
	}

	if strings.TrimSpace(sub.Description) == "" {
		return nil, nil, errors.New("description is required and cannot be empty")
	}

	if strings.TrimSpace(sub.Version) == "" {
		return nil, nil, errors.New("version is required and cannot be empty")
	}

	if err := validateOwner(sub.Owner); err != nil {
		return nil, nil, err
	}

	if sub.WebsiteURL != "" {
		if err := validateHTTPSURL(sub.WebsiteURL); err != nil {
			return nil, nil, fmt.Errorf("invalid websiteUrl: %w", err)
		}
	}

	if sub.Repository != nil {
		if strings.TrimSpace(sub.Repository.Source) == "" {
			return nil, nil, errors.New("repository.source is required")
		}
		if err := validateHTTPSURL(sub.Repository.URL); err != nil {
			return nil, nil, fmt.Errorf("invalid repository.url: %w", err)
		}
	}

	if len(sub.Remotes) == 0 {
		return nil, nil, errors.New("at least one remote is required")
	}

	for i, remote := range sub.Remotes {
		if remote.Type != "streamable-http" && remote.Type != "sse" {
			return nil, nil, fmt.Errorf("remote[%d]: type must be 'streamable-http' or 'sse'", i)
		}
		if err := validateHTTPSURL(remote.URL); err != nil {
			return nil, nil, fmt.Errorf("remote[%d]: %w", i, err)
		}
	}

	canonicalJSON, err := json.Marshal(sub)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling canonical entry: %w", err)
	}

	return &sub, canonicalJSON, nil
}
