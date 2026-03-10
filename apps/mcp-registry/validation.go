package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var serverNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*/[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
var tagRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validateAllowListFile() error {
	data, err := os.ReadFile("allowlist.json")
	if err != nil {
		return fmt.Errorf("cannot read allowlist.json: %v", err)
	}

	var staticData StaticRegistryData
	if err := json.Unmarshal(data, &staticData); err != nil {
		return fmt.Errorf("invalid JSON format: %v", err)
	}

	return validateRegistry(&staticData)
}

func validateRegistry(data *StaticRegistryData) error {
	if len(data.Servers) == 0 {
		return fmt.Errorf("registry must contain at least one server")
	}

	serverNames := make(map[string]bool)

	for i := range data.Servers {
		if err := validateServerEntry(&data.Servers[i], i, serverNames); err != nil {
			return err
		}
		serverNames[data.Servers[i].Name] = true
	}

	return nil
}

func validateServerEntry(server *StaticServerData, index int, existingNames map[string]bool) error {
	if err := validateName(server.Name, index); err != nil {
		return err
	}

	if existingNames[server.Name] {
		return fmt.Errorf("server[%d]: duplicate server name '%s'", index, server.Name)
	}

	if err := validateDescription(server.Description, index); err != nil {
		return err
	}

	if strings.TrimSpace(server.Version) == "" {
		return fmt.Errorf("server[%d]: 'version' is required and cannot be empty", index)
	}

	if server.Status != "" {
		if err := validateStatus(server.Status, index); err != nil {
			return err
		}
	}

	for j := range server.Remotes {
		if err := validateTransport(&server.Remotes[j], index, j); err != nil {
			return err
		}
	}

	for j := range server.Packages {
		if err := validatePackage(&server.Packages[j], index, j); err != nil {
			return err
		}
	}

	if server.PublishedAt != "" {
		if _, err := time.Parse(time.RFC3339, server.PublishedAt); err != nil {
			return fmt.Errorf("server[%d]: invalid publishedAt format, must be RFC3339: %v", index, err)
		}
	}

	if server.WebsiteURL != "" {
		if err := validateHTTPSURL(server.WebsiteURL); err != nil {
			return fmt.Errorf("server[%d]: invalid websiteUrl: %v", index, err)
		}
	}

	if server.Repository != nil {
		if err := validateRepository(server.Repository, index); err != nil {
			return err
		}
	}

	for j, tool := range server.Tools {
		if strings.TrimSpace(tool) == "" {
			return fmt.Errorf("server[%d].tools[%d]: tool name cannot be empty", index, j)
		}
	}

	for j, tag := range server.Tags {
		if !tagRegex.MatchString(tag) {
			return fmt.Errorf("server[%d].tags[%d]: tag '%s' must be lowercase kebab-case (e.g., 'browser-automation')", index, j, tag)
		}
	}

	return nil
}

func validateName(name string, index int) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("server[%d]: 'name' is required and cannot be empty", index)
	}

	if len(name) < NameMinLength {
		return fmt.Errorf("server[%d]: 'name' must be at least %d characters", index, NameMinLength)
	}

	if len(name) > NameMaxLength {
		return fmt.Errorf("server[%d]: 'name' must be at most %d characters", index, NameMaxLength)
	}

	if !strings.Contains(name, "/") {
		return fmt.Errorf("server[%d]: 'name' must be in reverse-DNS format with exactly one '/' (e.g., 'io.github.org/server-name')", index)
	}

	if strings.Count(name, "/") > 1 {
		return fmt.Errorf("server[%d]: 'name' cannot contain multiple slashes", index)
	}

	if !serverNameRegex.MatchString(name) {
		return fmt.Errorf("server[%d]: 'name' format is invalid, must match pattern '^[a-zA-Z0-9][a-zA-Z0-9.-]*/[a-zA-Z0-9][a-zA-Z0-9._-]*$'", index)
	}

	return nil
}

func validateDescription(description string, index int) error {
	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("server[%d]: 'description' is required and cannot be empty", index)
	}

	if len(description) < DescriptionMinLength {
		return fmt.Errorf("server[%d]: 'description' must be at least %d character", index, DescriptionMinLength)
	}

	if len(description) > DescriptionMaxLength {
		return fmt.Errorf("server[%d]: 'description' must be at most %d characters", index, DescriptionMaxLength)
	}

	return nil
}

func validateStatus(status string, index int) error {
	switch status {
	case StatusActive, StatusDeprecated, StatusDeleted:
		return nil
	default:
		return fmt.Errorf("server[%d]: 'status' must be one of: %s, %s, %s", index, StatusActive, StatusDeprecated, StatusDeleted)
	}
}

var templateVarRegex = regexp.MustCompile(`\{\{[a-zA-Z_][a-zA-Z0-9_]*\}\}`)

func validateTransport(transport *Transport, serverIndex, remoteIndex int) error {
	if strings.TrimSpace(transport.Type) == "" {
		return fmt.Errorf("server[%d].remotes[%d]: 'type' is required and cannot be empty", serverIndex, remoteIndex)
	}

	switch transport.Type {
	case TransportTypeStreamableHTTP, TransportTypeSSE, TransportTypeStdio:
	default:
		return fmt.Errorf("server[%d].remotes[%d]: 'type' must be one of: %s, %s, %s",
			serverIndex, remoteIndex, TransportTypeStreamableHTTP, TransportTypeSSE, TransportTypeStdio)
	}

	if transport.Type == TransportTypeStreamableHTTP || transport.Type == TransportTypeSSE {
		if strings.TrimSpace(transport.URL) == "" {
			return fmt.Errorf("server[%d].remotes[%d]: 'url' is required for %s transport", serverIndex, remoteIndex, transport.Type)
		}
		if err := validateURL(transport.URL); err != nil {
			return fmt.Errorf("server[%d].remotes[%d]: %v", serverIndex, remoteIndex, err)
		}
	}

	return nil
}

func validateURL(rawURL string) error {
	if templateVarRegex.MatchString(rawURL) {
		testURL := templateVarRegex.ReplaceAllString(rawURL, "placeholder")
		if _, err := url.Parse(testURL); err != nil {
			return fmt.Errorf("invalid url format (after template substitution): %v", err)
		}
		return nil
	}
	if _, err := url.Parse(rawURL); err != nil {
		return fmt.Errorf("invalid url format: %v", err)
	}
	return nil
}

func validatePackage(pkg *Package, serverIndex, pkgIndex int) error {
	if strings.TrimSpace(pkg.RegistryType) == "" {
		return fmt.Errorf("server[%d].packages[%d]: 'registryType' is required", serverIndex, pkgIndex)
	}

	switch pkg.RegistryType {
	case RegistryTypeNPM, RegistryTypePyPI, RegistryTypeOCI, RegistryTypeNuGet, RegistryTypeMCPB:
	default:
		return fmt.Errorf("server[%d].packages[%d]: 'registryType' must be one of: %s, %s, %s, %s, %s",
			serverIndex, pkgIndex, RegistryTypeNPM, RegistryTypePyPI, RegistryTypeOCI, RegistryTypeNuGet, RegistryTypeMCPB)
	}

	if strings.TrimSpace(pkg.Identifier) == "" {
		return fmt.Errorf("server[%d].packages[%d]: 'identifier' is required", serverIndex, pkgIndex)
	}

	if strings.TrimSpace(pkg.Transport.Type) == "" {
		return fmt.Errorf("server[%d].packages[%d]: 'transport.type' is required", serverIndex, pkgIndex)
	}

	switch pkg.Transport.Type {
	case TransportTypeStdio, TransportTypeStreamableHTTP, TransportTypeSSE:
	default:
		return fmt.Errorf("server[%d].packages[%d]: 'transport.type' must be one of: %s, %s, %s",
			serverIndex, pkgIndex, TransportTypeStdio, TransportTypeStreamableHTTP, TransportTypeSSE)
	}

	for j, arg := range pkg.PackageArguments {
		if err := validateArgument(&arg, serverIndex, pkgIndex, j, "packageArguments"); err != nil {
			return err
		}
	}

	for j, arg := range pkg.RuntimeArguments {
		if err := validateArgument(&arg, serverIndex, pkgIndex, j, "runtimeArguments"); err != nil {
			return err
		}
	}

	return nil
}

func validateArgument(arg *Argument, serverIndex, pkgIndex, argIndex int, field string) error {
	switch arg.Type {
	case "named", "positional":
	default:
		return fmt.Errorf("server[%d].packages[%d].%s[%d]: 'type' must be 'named' or 'positional'",
			serverIndex, pkgIndex, field, argIndex)
	}

	if arg.Type == "named" && strings.TrimSpace(arg.Name) == "" {
		return fmt.Errorf("server[%d].packages[%d].%s[%d]: 'name' is required for named arguments",
			serverIndex, pkgIndex, field, argIndex)
	}

	if arg.Type == "positional" && strings.TrimSpace(arg.Value) == "" && strings.TrimSpace(arg.ValueHint) == "" {
		return fmt.Errorf("server[%d].packages[%d].%s[%d]: 'value' or 'valueHint' is required for positional arguments",
			serverIndex, pkgIndex, field, argIndex)
	}

	return nil
}

func validateHTTPSURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("URL must use https scheme, got '%s'", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	return nil
}

func validateRepository(repo *Repository, index int) error {
	if err := validateHTTPSURL(repo.URL); err != nil {
		return fmt.Errorf("server[%d]: invalid repository.url: %v", index, err)
	}
	if strings.TrimSpace(repo.Source) == "" {
		return fmt.Errorf("server[%d]: repository.source is required", index)
	}
	return nil
}
