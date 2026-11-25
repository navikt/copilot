package registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var serverNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*/[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
var hostRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
var tagRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// DecodeStaticRegistry decodes allowlist.json and rejects unknown fields.
func DecodeStaticRegistry(data []byte) (StaticRegistryData, error) {
	var staticData StaticRegistryData
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&staticData); err != nil {
		return StaticRegistryData{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return StaticRegistryData{}, fmt.Errorf("unexpected trailing data after JSON object: %w", err)
		}
		return StaticRegistryData{}, errors.New("unexpected trailing data after JSON object")
	}
	return staticData, nil
}

// ValidateAllowListFile validates allowlist.json in the working directory.
func ValidateAllowListFile() error {
	data, err := os.ReadFile("allowlist.json")
	if err != nil {
		return fmt.Errorf("cannot read allowlist.json: %v", err)
	}

	staticData, err := DecodeStaticRegistry(data)
	if err != nil {
		return fmt.Errorf("invalid JSON format: %v", err)
	}

	return ValidateRegistry(&staticData)
}

// ValidateRegistry validates every entry of a static registry.
func ValidateRegistry(data *StaticRegistryData) error {
	return validateRegistryEntries(nil, data)
}

// ValidateSubstitutedRegistry validates a registry after domain substitution against its raw form.
func ValidateSubstitutedRegistry(raw, data *StaticRegistryData) error {
	if len(data.Servers) != len(raw.Servers) {
		return fmt.Errorf("server count changed after substitution (%d -> %d)", len(raw.Servers), len(data.Servers))
	}
	return validateRegistryEntries(raw, data)
}

func validateRegistryEntries(raw, data *StaticRegistryData) error {
	if len(data.Servers) == 0 {
		return fmt.Errorf("registry must contain at least one server")
	}

	serverNames := make(map[string]bool)

	for i := range data.Servers {
		var rawServer *StaticServerData
		if raw != nil {
			rawServer = &raw.Servers[i]
		}
		if err := validateServerEntry(&data.Servers[i], rawServer, i, serverNames); err != nil {
			return err
		}
		serverNames[data.Servers[i].Name] = true
	}

	return nil
}

func validateServerEntry(server, rawServer *StaticServerData, index int, existingNames map[string]bool) error {
	if err := ValidateName(server.Name, index); err != nil {
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

	if rawServer != nil && len(server.Remotes) != len(rawServer.Remotes) {
		return fmt.Errorf("server[%d]: remote count changed after substitution", index)
	}

	for j := range server.Remotes {
		var rawRemote *Transport
		if rawServer != nil {
			rawRemote = &rawServer.Remotes[j]
		}
		if err := validateTransport(&server.Remotes[j], rawRemote, index, j); err != nil {
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

	for j, host := range server.SandboxHosts {
		if !hostRegex.MatchString(host) {
			return fmt.Errorf("server[%d].sandboxHosts[%d]: '%s' must be a bare lowercase host name (e.g., 'api.figma.com')", index, j, host)
		}
	}

	for tool, risk := range server.ToolRisk {
		if !slices.Contains(server.Tools, tool) {
			return fmt.Errorf("server[%d].toolRisk: '%s' is not in the server's tools", index, tool)
		}
		// A read has no entry: toolRisk lists only what does more.
		if !slices.Contains([]string{"write", "external", "host-exec"}, risk) {
			return fmt.Errorf("server[%d].toolRisk.%s: '%s' must be write, external or host-exec (a read has no entry)", index, tool, risk)
		}
	}

	// A server an app runs on the user's machine works outside the sandbox,
	// so nav-pilot can only leave its risky tools off when it knows them.
	for j, remote := range server.Remotes {
		if u, err := url.Parse(remote.URL); err == nil && isLoopbackHost(u.Hostname()) && len(server.Tools) == 0 {
			return fmt.Errorf("server[%d].remotes[%d]: a localhost server must list its tools", index, j)
		}
	}

	if server.Owner != "" && !ownerRegex.MatchString(server.Owner) {
		return fmt.Errorf("server[%d].owner %q is invalid: must be a Nais team matching ^[a-z0-9-]+$", index, server.Owner)
	}
	if err := validateAudience(server.Audience); err != nil {
		return fmt.Errorf("server[%d].%w", index, err)
	}
	if err := validateProtocolVersions(server.ProtocolVersions); err != nil {
		return fmt.Errorf("server[%d].%w", index, err)
	}

	if server.Meta != nil && server.Meta.Broker != nil {
		if err := validateBroker(server.Meta.Broker, server.Tools, index); err != nil {
			return err
		}
	}

	for j := range server.SetupInstructions {
		if err := validateSetupInstruction(&server.SetupInstructions[j], index, j); err != nil {
			return err
		}
	}

	return nil
}

// brokerMechanism is what a credential mechanism pins: the server command
// cplt runs, and the login command nav-pilot runs. login.status is not
// pinned: nav-pilot probes with its own command. nav-pilot keeps the same
// table (cli/nav-pilot/internal/cli/mcp_broker.go);
// testdata/broker-mechanisms.json keeps the two in step.
type brokerMechanism struct {
	Command []string    `json:"command"`
	Login   BrokerLogin `json:"login"`
}

var brokerMechanisms = map[string]brokerMechanism{
	"nais-keychain": {
		Command: []string{"nais", "alpha", "mcp", "serve"},
		Login:   BrokerLogin{Run: "nais login -n"},
	},
}

var (
	brokerClasses   = []string{"read", "write", "external"} // host-exec is never brokered
	brokerArgChecks = []string{"graphql-read-only"}
)

// validateBroker checks the broker extension against what cplt and nav-pilot
// accept, so a registry entry never yields a spec either refuses.
func validateBroker(b *BrokerExtension, tools []string, index int) error {
	prefix := fmt.Sprintf("server[%d]._meta[io.github.navikt/broker]", index)
	if b.V != 1 {
		return fmt.Errorf("%s.v: %d is not a known version (1)", prefix, b.V)
	}
	m, ok := brokerMechanisms[b.Credential]
	if !ok {
		return fmt.Errorf("%s.credential: '%s' must be one of %v", prefix, b.Credential, slices.Sorted(maps.Keys(brokerMechanisms)))
	}
	if !slices.Equal(b.Command, m.Command) {
		return fmt.Errorf("%s.command: %s runs only %q", prefix, b.Credential, strings.Join(m.Command, " "))
	}
	// login.status may be anything or absent; nav-pilot probes with its own
	// pinned command. Builds before 2026.10.24 refuse an entry without it and
	// must be updated (navikt/copilot#1979).
	if b.Login.Run != m.Login.Run {
		return fmt.Errorf("%s.login.run: must be %q for %s (nav-pilot's pinned command)", prefix, m.Login.Run, b.Credential)
	}
	for j, host := range b.EgressHosts {
		if !hostRegex.MatchString(host) {
			return fmt.Errorf("%s.egressHosts[%d]: '%s' must be a bare lowercase host name", prefix, j, host)
		}
	}
	if len(b.Tools.Classes) == 0 {
		return fmt.Errorf("%s.tools.classes: must class at least one tool", prefix)
	}
	for tool, class := range b.Tools.Classes {
		if !slices.Contains(tools, tool) {
			return fmt.Errorf("%s.tools.classes: '%s' is not in the server's tools", prefix, tool)
		}
		if !slices.Contains(brokerClasses, class) {
			return fmt.Errorf("%s.tools.classes.%s: '%s' must be one of %v", prefix, tool, class, brokerClasses)
		}
	}
	for tool, check := range b.ArgChecks {
		if _, ok := b.Tools.Classes[tool]; !ok {
			return fmt.Errorf("%s.argChecks: '%s' is not in tools.classes", prefix, tool)
		}
		if !slices.Contains(brokerArgChecks, check) {
			return fmt.Errorf("%s.argChecks.%s: '%s' must be one of %v", prefix, tool, check, brokerArgChecks)
		}
	}
	return nil
}

const maxSetupCommandLength = 500

func validateSetupInstruction(instruction *SetupInstruction, serverIndex, instructionIndex int) error {
	prefix := fmt.Sprintf("server[%d].setupInstructions[%d]", serverIndex, instructionIndex)
	if strings.TrimSpace(instruction.Title) == "" {
		return fmt.Errorf("%s: 'title' is required and cannot be empty", prefix)
	}
	if strings.TrimSpace(instruction.Description) == "" {
		return fmt.Errorf("%s: 'description' is required and cannot be empty", prefix)
	}
	if len(instruction.Commands) == 0 {
		return fmt.Errorf("%s: 'commands' must contain at least one command", prefix)
	}
	for commandIndex, command := range instruction.Commands {
		if strings.TrimSpace(command) == "" {
			return fmt.Errorf("%s.commands[%d]: command cannot be empty", prefix, commandIndex)
		}
		if utf8.RuneCountInString(command) > maxSetupCommandLength {
			return fmt.Errorf("%s.commands[%d]: command exceeds %d characters", prefix, commandIndex, maxSetupCommandLength)
		}
		if strings.IndexFunc(command, unicode.IsControl) != -1 {
			return fmt.Errorf("%s.commands[%d]: command cannot contain control characters", prefix, commandIndex)
		}
	}
	return nil
}

// ValidateName checks a server name; index is the entry position used in errors.
func ValidateName(name string, index int) error {
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

func validateTransport(transport, rawTransport *Transport, serverIndex, remoteIndex int) error {
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
		var err error
		if rawTransport == nil {
			err = validateURL(transport.URL)
		} else {
			err = validateSubstitutedURL(rawTransport.URL, transport.URL)
		}
		if err != nil {
			return fmt.Errorf("server[%d].remotes[%d]: %v", serverIndex, remoteIndex, err)
		}
	}

	return nil
}

func validateURL(rawURL string) error {
	// Placeholders such as {{domain_internal}} are filled at serve time; the
	// host is checked with them replaced, so a literal internal host still
	// fails wherever a placeholder sits in the URL.
	u, err := url.Parse(templateVarRegex.ReplaceAllString(rawURL, "placeholder"))
	if err != nil {
		return fmt.Errorf("invalid url format: %v", err)
	}
	host, err := validateURLSchemeAndHost(u)
	if err != nil {
		return err
	}
	return validatePublicHost(host)
}

func validateSubstitutedURL(rawURL, subURL string) error {
	u, err := url.Parse(subURL)
	if err != nil {
		return fmt.Errorf("invalid url format: %v", err)
	}
	host, err := validateURLSchemeAndHost(u)
	if err != nil {
		return err
	}
	if u.User != nil {
		return errors.New("userinfo is not allowed in url")
	}
	if !isLoopbackHost(host) && !hostRegex.MatchString(strings.ToLower(host)) {
		return fmt.Errorf("url host %q is invalid", host)
	}
	if rawURL != subURL {
		if err := validateSubstitutionBoundary(rawURL, u); err != nil {
			return err
		}
	}
	return validateHost(host, urlHostHasInternalTemplate(rawURL))
}

func validateURLSchemeAndHost(u *url.URL) (string, error) {
	host := u.Hostname()
	if host == "" {
		return "", errors.New("url must have a host")
	}
	switch u.Scheme {
	case "https":
	case "http":
		// Loopback servers (e.g. local IDE MCP plugins) legitimately bind plain HTTP.
		if !isLoopbackHost(host) {
			return "", fmt.Errorf("url must use https scheme, got %q", u.Scheme)
		}
	default:
		return "", fmt.Errorf("url must use https scheme, got %q", u.Scheme)
	}
	return host, nil
}

func validateSubstitutionBoundary(rawURL string, subU *url.URL) error {
	rawU, err := url.Parse(templateVarRegex.ReplaceAllString(rawURL, "placeholder"))
	if err != nil {
		return fmt.Errorf("invalid raw url format: %v", err)
	}
	if subU.Scheme != rawU.Scheme {
		return fmt.Errorf("substituted url scheme %q does not match %q", subU.Scheme, rawU.Scheme)
	}
	if subU.Port() != rawU.Port() {
		return fmt.Errorf("substituted url port %q does not match %q", subU.Port(), rawU.Port())
	}
	if strings.Contains(rawU.Host, "placeholder") {
		if !strings.Contains(rawU.Path, "placeholder") && subU.Path != rawU.Path {
			return fmt.Errorf("substituted host changed url path from %q to %q", rawU.Path, subU.Path)
		}
		if !strings.Contains(rawU.RawQuery, "placeholder") && subU.RawQuery != rawU.RawQuery {
			return errors.New("substituted host changed url query")
		}
		if !strings.Contains(rawU.Fragment, "placeholder") && subU.Fragment != rawU.Fragment {
			return errors.New("substituted host changed url fragment")
		}
	}
	return nil
}

func urlHostHasInternalTemplate(rawURL string) bool {
	replaced := strings.ReplaceAll(rawURL, "{{domain_internal}}", "__domain_internal__")
	replaced = templateVarRegex.ReplaceAllString(replaced, "placeholder")
	u, err := url.Parse(replaced)
	if err != nil {
		return false
	}
	return strings.Contains(u.Hostname(), "__domain_internal__")
}

// internalHostSuffixes are domains only reachable inside Nav. The registry is
// public, so a remote on one of them would publish an internal hostname.
var internalHostSuffixes = []string{".intern.nav.no", ".intern.dev.nav.no", ".svc.cluster.local", ".adeo.no"}

func validatePublicHost(host string) error {
	return validateHost(host, false)
}

func validateHost(host string, allowNavInternal bool) error {
	h := "." + strings.TrimSuffix(strings.ToLower(host), ".")
	for _, suffix := range internalHostSuffixes {
		if strings.HasSuffix(h, suffix) {
			if allowNavInternal && (suffix == ".intern.nav.no" || suffix == ".intern.dev.nav.no") {
				continue
			}
			return fmt.Errorf("url host %q is internal (%s); team MCPs go through the gateway", host, strings.TrimPrefix(suffix, "."))
		}
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

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}

// Submission is the strict shape of an entry submitted through the write API.
// Static allowlist.json entries are PR-reviewed and may use the richer
// StaticServerData shape; submissions and the DB rows they produce may not.
type Submission struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	WebsiteURL  string      `json:"websiteUrl,omitempty"`
	Repository  *Repository `json:"repository,omitempty"`
	Owner       string      `json:"owner"`
	Audience    *Audience   `json:"audience,omitempty"`
	// ProtocolVersions is checked by validateProtocolVersions.
	ProtocolVersions []string           `json:"protocolVersions,omitempty"`
	Remotes          []SubmissionRemote `json:"remotes"`
}

// SubmissionRemote is a remote transport with type and url only.
type SubmissionRemote struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

var (
	ownerRegex     = regexp.MustCompile(`^[a-z0-9-]+$`)
	hostLabelRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	// submissionBlockedSuffixes extends internalHostSuffixes for unreviewed input.
	submissionBlockedSuffixes = append(slices.Clone(internalHostSuffixes), ".internal", ".local")
)

type validationFieldError struct {
	field string
	err   error
}

func (e *validationFieldError) Error() string { return e.err.Error() }
func (e *validationFieldError) Unwrap() error { return e.err }

// SubmissionErrorMessage returns a generic validation message naming only the
// failing field so rejected input is never echoed in responses or logs.
func SubmissionErrorMessage(err error) string {
	var fe *validationFieldError
	if errors.As(err, &fe) {
		return "invalid " + fe.field
	}
	return "invalid entry"
}

const (
	maxAudienceTeams = 50
	// MaxTeamSlugLen is the longest Nais team name.
	MaxTeamSlugLen = 63 // a Nais team is a Kubernetes namespace, a DNS label
)

// validateAudience is the one check for audience, shared by allowlist.json
// and the write API so the two cannot drift. nil means visible to everyone.
func validateAudience(a *Audience) error {
	if a == nil {
		return nil
	}
	if len(a.Teams) == 0 {
		return errors.New("audience.teams must list at least one team; leave out audience for everyone")
	}
	if len(a.Teams) > maxAudienceTeams {
		return fmt.Errorf("audience.teams has %d teams, at most %d allowed", len(a.Teams), maxAudienceTeams)
	}
	seen := make(map[string]bool, len(a.Teams))
	for i, team := range a.Teams {
		if len(team) > MaxTeamSlugLen || !ownerRegex.MatchString(team) {
			return fmt.Errorf("audience.teams[%d] is invalid: must be a Nais team matching ^[a-z0-9-]+$, at most %d characters", i, MaxTeamSlugLen)
		}
		if seen[team] {
			return fmt.Errorf("audience.teams[%d] is a duplicate", i)
		}
		seen[team] = true
	}
	return nil
}

const maxProtocolVersions = 10

// validateProtocolVersions is the one check for protocolVersions, shared by
// allowlist.json and the write API. nil means unknown. Format only: a new MCP
// revision must not need a registry release.
func validateProtocolVersions(v []string) error {
	if v == nil {
		return nil
	}
	if len(v) == 0 {
		return errors.New("protocolVersions must list at least one revision; leave it out if unknown")
	}
	if len(v) > maxProtocolVersions {
		return fmt.Errorf("protocolVersions has %d entries, at most %d allowed", len(v), maxProtocolVersions)
	}
	seen := make(map[string]bool, len(v))
	for i, rev := range v {
		if _, err := time.Parse(time.DateOnly, rev); err != nil {
			return fmt.Errorf("protocolVersions[%d] is invalid: must be a dated MCP revision YYYY-MM-DD", i)
		}
		if seen[rev] {
			return fmt.Errorf("protocolVersions[%d] is a duplicate", i)
		}
		seen[rev] = true
	}
	return nil
}

// ValidateSubmission decodes entryJSON strictly into a Submission, validates it
// and returns it with its re-marshaled canonical JSON. It guards both the
// write API and DB rows at reload.
func ValidateSubmission(entryJSON []byte) (*Submission, []byte, error) {
	dec := json.NewDecoder(bytes.NewReader(entryJSON))
	dec.DisallowUnknownFields()
	var sub Submission
	if err := dec.Decode(&sub); err != nil {
		return nil, nil, &validationFieldError{field: "entry", err: fmt.Errorf("invalid entry JSON: %w", err)}
	}
	if dec.More() {
		return nil, nil, &validationFieldError{field: "entry", err: errors.New("unexpected trailing data after JSON entry")}
	}
	if err := ValidateName(sub.Name, 0); err != nil {
		return nil, nil, &validationFieldError{field: "name", err: err}
	}
	if err := validateDescription(sub.Description, 0); err != nil {
		return nil, nil, &validationFieldError{field: "description", err: err}
	}
	if strings.TrimSpace(sub.Version) == "" {
		return nil, nil, &validationFieldError{field: "version", err: errors.New("version is required and cannot be empty")}
	}
	if !ownerRegex.MatchString(sub.Owner) {
		return nil, nil, &validationFieldError{field: "owner", err: fmt.Errorf("owner %q is invalid: must be a Nais team matching ^[a-z0-9-]+$", sub.Owner)}
	}
	if sub.WebsiteURL != "" {
		sub.WebsiteURL = strings.TrimSpace(sub.WebsiteURL)
		if err := validateSubmissionURL(sub.WebsiteURL); err != nil {
			return nil, nil, &validationFieldError{field: "websiteUrl", err: fmt.Errorf("invalid websiteUrl: %w", err)}
		}
	}
	if sub.Repository != nil {
		if strings.TrimSpace(sub.Repository.Source) == "" {
			return nil, nil, &validationFieldError{field: "repository.source", err: errors.New("repository.source is required")}
		}
		sub.Repository.URL = strings.TrimSpace(sub.Repository.URL)
		if err := validateSubmissionURL(sub.Repository.URL); err != nil {
			return nil, nil, &validationFieldError{field: "repository.url", err: fmt.Errorf("invalid repository.url: %w", err)}
		}
	}
	if err := validateAudience(sub.Audience); err != nil {
		return nil, nil, &validationFieldError{field: "audience", err: err}
	}
	if err := validateProtocolVersions(sub.ProtocolVersions); err != nil {
		return nil, nil, &validationFieldError{field: "protocolVersions", err: err}
	}
	if len(sub.Remotes) == 0 {
		return nil, nil, &validationFieldError{field: "remotes", err: errors.New("at least one remote is required")}
	}
	for i, remote := range sub.Remotes {
		if remote.Type != "streamable-http" && remote.Type != "sse" {
			return nil, nil, &validationFieldError{field: "remotes", err: fmt.Errorf("remote[%d]: type must be 'streamable-http' or 'sse'", i)}
		}
		sub.Remotes[i].URL = strings.TrimSpace(remote.URL)
		if err := validateSubmissionURL(sub.Remotes[i].URL); err != nil {
			return nil, nil, &validationFieldError{field: "remotes", err: fmt.Errorf("remote[%d]: %w", i, err)}
		}
	}
	canonical, err := json.Marshal(sub)
	if err != nil {
		return nil, nil, &validationFieldError{field: "entry", err: fmt.Errorf("marshaling canonical entry: %w", err)}
	}
	return &sub, canonical, nil
}

// validateSubmissionURL is stricter than validateHTTPSURL: https on port 443,
// no userinfo, a lowercase ASCII DNS name with a dot and a non-numeric TLD, no
// IP literal in any form, no localhost and no internal suffix.
func validateSubmissionURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("url cannot be empty")
	}
	for i := 0; i < len(rawURL); i++ {
		if rawURL[i] > 127 {
			return errors.New("url contains non-ASCII characters")
		}
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid url format: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("url must use https scheme, got %q", u.Scheme)
	}
	if u.User != nil {
		return errors.New("userinfo is not allowed in url")
	}
	if port := u.Port(); port != "" && port != "443" {
		return fmt.Errorf("explicit non-443 port %q is not allowed", port)
	}
	if strings.ContainsAny(u.Host, "%[]") {
		return errors.New("IP literals or zone IDs are not allowed in host")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("url host cannot be empty")
	}
	if host != strings.ToLower(host) {
		return errors.New("host must be lowercase")
	}
	if strings.Contains(host, "xn--") {
		return errors.New("IDN (punycode) host is not allowed")
	}
	if strings.Contains(host, ":") || net.ParseIP(host) != nil {
		return fmt.Errorf("IP literal %q is not allowed", host)
	}
	if !strings.Contains(host, ".") {
		return errors.New("host must contain at least one dot")
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if !hostLabelRegex.MatchString(label) {
			return fmt.Errorf("host label %q is invalid", label)
		}
		if strings.HasPrefix(label, "0x") {
			return fmt.Errorf("hex IP form %q is not allowed", label)
		}
	}
	if strings.Trim(labels[len(labels)-1], "0123456789") == "" {
		return fmt.Errorf("host last label %q cannot be numeric", labels[len(labels)-1])
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("localhost is not allowed")
	}
	h := "." + host
	for _, suffix := range submissionBlockedSuffixes {
		if strings.HasSuffix(h, suffix) {
			return fmt.Errorf("internal host %q (%s) is not allowed", host, strings.TrimPrefix(suffix, "."))
		}
	}
	return nil
}
