package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var serverNameRegex = regexp.MustCompile(`^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$`)

var internalHostSuffixes = []string{
	".intern.nav.no",
	".intern.dev.nav.no",
	".svc.cluster.local",
	".adeo.no",
}

// RemoteTransport represents the minimal remote transport object in an entry.
type RemoteTransport struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type serverEntryHeader struct {
	Name    string            `json:"name"`
	Remotes []RemoteTransport `json:"remotes"`
}

// validateServerName checks that name matches ^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$.
func validateServerName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("server name is required")
	}
	if !serverNameRegex.MatchString(name) {
		return fmt.Errorf("server name %q is invalid: must match ^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$", name)
	}
	return nil
}

// validateServerEntry validates that the entry JSON contains a valid server name
// and at least one remote with an https URL whose host is not internal.
func validateServerEntry(entryJSON []byte) (string, error) {
	if len(entryJSON) == 0 {
		return "", errors.New("entry cannot be empty")
	}

	var header serverEntryHeader
	if err := json.Unmarshal(entryJSON, &header); err != nil {
		return "", fmt.Errorf("invalid entry JSON: %w", err)
	}

	if err := validateServerName(header.Name); err != nil {
		return "", err
	}

	if len(header.Remotes) == 0 {
		return "", errors.New("at least one remote is required")
	}

	for i, remote := range header.Remotes {
		if err := validateRemoteTransport(&remote, i); err != nil {
			return "", err
		}
	}

	return header.Name, nil
}

func validateRemoteTransport(remote *RemoteTransport, index int) error {
	trimmedURL := strings.TrimSpace(remote.URL)
	if trimmedURL == "" {
		return fmt.Errorf("remote[%d]: url is required", index)
	}

	u, err := url.Parse(trimmedURL)
	if err != nil {
		return fmt.Errorf("remote[%d]: invalid url format: %w", index, err)
	}

	if strings.ToLower(u.Scheme) != "https" {
		return fmt.Errorf("remote[%d]: url must use https scheme", index)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("remote[%d]: url host cannot be empty", index)
	}

	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("remote[%d]: localhost is not allowed", index)
	}

	if net.ParseIP(host) != nil {
		return fmt.Errorf("remote[%d]: IP literal %q is not allowed", index, host)
	}

	h := "." + strings.TrimSuffix(strings.ToLower(host), ".")
	for _, suffix := range internalHostSuffixes {
		if strings.HasSuffix(h, suffix) {
			return fmt.Errorf("remote[%d]: internal host %q is not allowed", index, host)
		}
	}

	return nil
}
