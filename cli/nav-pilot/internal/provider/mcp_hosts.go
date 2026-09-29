package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// The hosts the user's MCP servers need under cplt, from Nav's MCP registry.
//
// Which servers count is the launched client's user MCP config:
// ~/.copilot/mcp-config.json for Copilot, the user's own OpenCode files for
// OpenCode, never a project's. Which hosts they get is
// the registry's answer only. A server is matched by identity — its registry
// name, or for OpenCode a URL the registry lists — and the hosts are taken
// from the registry's remotes, never from a URL in the config. The agent can
// write those config files, so a config URL would be a host the agent picks.
//
// Nothing is granted without an answer. The answer lives in the pakke consent
// store (artifacts/consent.go) under [MCPConsentSource], one record in the
// user scope. Its hash is over the host set; its Block is the set itself, so
// the launch derives what it grants from the record, as for a pakke waiver.
// An approved set is a ceiling: a launch grants the hosts both the registry
// and the approval name, and only a host outside the approval asks again.
//
// A registry that does not answer changes nothing: the approved set stays in
// force and the launch says so in one line.

// MCPConsentSource is the consent-store key for the MCP registry hosts. The
// colon keeps it apart from every pakke name (^[a-z][a-z0-9-]*$).
const MCPConsentSource = "mcp:registry"

// MCPHost is one host the configured MCP servers need, and which of them.
type MCPHost struct {
	Host    string   `json:"host"`
	Servers []string `json:"servers"`
	// Private is a host that resolved to a private address, or does not
	// exist in DNS, when it was classified: it needs cplt's
	// --allow-private-domain.
	Private bool `json:"private,omitempty"`
	// Unknown is a host whose lookup failed another way, a timeout most
	// likely. Not private until a later lookup says so: see
	// [ReclassifyMCPHostsInBackground].
	Unknown bool `json:"unknown,omitempty"`
}

// MCPLoopback is a configured server the registry lists on this machine.
// Never granted: the user opens the port in cplt themselves.
type MCPLoopback struct {
	Server string
	Port   string
}

// MCPHosts is what the configured servers need, per the registry.
type MCPHosts struct {
	Hosts    []MCPHost // sorted by Host
	Loopback []MCPLoopback
}

// Names is the host names, sorted.
func (m MCPHosts) Names() []string { return mcpHostNames(m.Hosts) }

// Hash is the consent hash: the host set, nothing else. A server added or
// removed that changes no host asks nothing.
func (m MCPHosts) Hash() string { return mcpHostsHash(m.Hosts) }

func mcpHostNames(hosts []MCPHost) []string {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, h.Host)
	}
	sort.Strings(out)
	return out
}

func mcpHostsHash(hosts []MCPHost) string {
	sum := sha256.Sum256([]byte(strings.Join(mcpHostNames(hosts), "\n")))
	return hex.EncodeToString(sum[:])
}

// copilotMCPServerNames is the servers in ~/.copilot/mcp-config.json, by the
// name Copilot keys them on, which for a registry install is the registry's
// name for the server (com.figma/figma-mcp). Unreadable is none.
func copilotMCPServerNames() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".copilot", "mcp-config.json"))
	if err != nil {
		return nil
	}
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	var names []string
	for name := range cfg.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MCPClient is the client a launch runs, set by the cli: only that client's
// servers count. Empty is copilot.
var MCPClient string

// configuredMCPServers is the servers the user configured for [MCPClient], in
// the user's own config only. A project's opencode.json is the repository's
// say, not the user's: counted, a cloned repository would put a host of its
// choosing on the consent screen, and one Yes grants it to every launch.
func configuredMCPServers() (copilot []string, openCode map[string]mcpServer) {
	if MCPClient == "opencode" {
		return nil, openCodeUserMCPServers()
	}
	return copilotMCPServerNames(), nil
}

// matchMCPHosts is the registry's hosts for the configured servers. Pure: the
// security property — a config URL is never a host — is tested here.
func matchMCPHosts(reg mcpRegistry, copilot []string, openCode map[string]mcpServer) MCPHosts {
	type remote struct{ name, url string }
	byURL := map[string]remote{} // normalized registry remote -> registry's name and URL
	for name, urls := range reg.Servers {
		for _, u := range urls {
			byURL[normalizeMCPURL(u)] = remote{name, u}
		}
	}
	hosts := map[string][]string{}
	var loopback []MCPLoopback
	add := func(server, raw string) {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			return
		}
		host := strings.ToLower(u.Hostname())
		if ip := net.ParseIP(host); host == "localhost" || (ip != nil && ip.IsLoopback()) {
			port := u.Port()
			if port == "" {
				port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
			}
			if !slices.Contains(loopback, MCPLoopback{server, port}) {
				loopback = append(loopback, MCPLoopback{server, port})
			}
			return
		}
		if !slices.Contains(hosts[host], server) {
			hosts[host] = append(hosts[host], server)
		}
	}
	for _, name := range copilot {
		for _, u := range reg.Servers[name] {
			add(name, u)
		}
	}
	names := make([]string, 0, len(openCode))
	for name := range openCode {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := openCode[name]
		if s.Enabled != nil && !*s.Enabled {
			continue
		}
		if urls, ok := reg.Servers[name]; ok {
			for _, u := range urls {
				add(name, u)
			}
			continue
		}
		if s.Type == "remote" {
			// The registry's URL and name, not the config's: they match
			// once normalized, only the registry's URL is trusted, and the
			// consent screen says "(Nav's MCP registry)" after the name.
			if r, ok := byURL[normalizeMCPURL(s.URL)]; ok {
				add(r.name, r.url)
			}
		}
	}
	out := MCPHosts{Loopback: loopback}
	for host, servers := range hosts {
		sort.Strings(servers)
		out.Hosts = append(out.Hosts, MCPHost{Host: host, Servers: servers})
	}
	sort.Slice(out.Hosts, func(i, j int) bool { return out.Hosts[i].Host < out.Hosts[j].Host })
	return out
}

// resolveMCPHosts matches the configured servers against the cached registry
// answer. No configured server reads nothing more. It never waits on gh or the
// registry: a cache past [mcpRegistryTTL], or none, starts a read in the
// background for the next launch, and no cache counts as a registry that did
// not answer, so the approved set stays and nothing is asked.
func resolveMCPHosts() (MCPHosts, error) {
	copilot, openCode := configuredMCPServers()
	if len(copilot)+len(openCode) == 0 {
		return MCPHosts{}, nil
	}
	c, ok := readMCPRegistryCache()
	if !ok || time.Since(c.At) > mcpRegistryTTL {
		startMCPRegistryRefresh()
	}
	if !ok {
		return MCPHosts{}, errMCPRegistryNotRead
	}
	return matchMCPHosts(c.Registry, copilot, openCode), nil
}

// currentMCPHosts is resolveMCPHosts once per process: the launch pre-flight
// and the launch flags ask the same question. A var so tests answer it.
var currentMCPHosts = sync.OnceValues(resolveMCPHosts)

// The registry's answer is cached in nav-pilot's state directory, which cplt
// denies to the session like the consent record beside it. A day old is
// fresh enough: a grant is the approved set cut down to the cached one, and a
// refresh that grows the set asks at the next launch.
const mcpRegistryTTL = 24 * time.Hour

var errMCPRegistryNotRead = errors.New("Nav's MCP registry has not been read yet")

// mcpRegistryCache keeps the whole answer, every field of mcpRegistry, so
// anything that reads the registry can read the cache instead.
type mcpRegistryCache struct {
	At       time.Time   `json:"at"`
	Registry mcpRegistry `json:"registry"`
}

func mcpRegistryCachePath() string {
	p := artifacts.CacheFilePath()
	if p == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(p), "mcp-registry.json")
}

func readMCPRegistryCache() (mcpRegistryCache, bool) {
	var c mcpRegistryCache
	data, err := os.ReadFile(mcpRegistryCachePath())
	return c, err == nil && json.Unmarshal(data, &c) == nil && c.Registry.Servers != nil
}

// refreshMCPRegistry asks the org policy which registry, asks that registry,
// and caches the answer. A failure keeps the cache as it was.
func refreshMCPRegistry() error {
	// The registry the org policy names, which is the one Copilot enforces.
	// No policy, or no gh to ask, is Nav's own: the hosts are Nav-curated
	// either way, and a policy question nobody answered is not a reason to
	// drop an approved set.
	registry, err := fetchMCPPolicy()
	if err != nil || registry == "" {
		registry = agentpakke.MCPRegistryURL
	}
	reg, err := fetchMCPRegistry(registry)
	if err != nil {
		return fmt.Errorf("%s did not answer: %w", registry, err)
	}
	path := mcpRegistryCachePath()
	if path == "" {
		return errors.New("nav-pilot has no state directory to cache the MCP registry in")
	}
	data, err := json.Marshal(mcpRegistryCache{At: time.Now(), Registry: reg})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeConfigAtomically(path, data)
}

// startMCPRegistryRefresh reads the registry in the background, once per
// process. The launch does not wait for it; the process waits for nothing at
// exit either, so a session shorter than the read leaves it to the next one.
func startMCPRegistryRefresh() {
	mcpRefreshOnce.Do(func() { mcpRefreshing.Go(func() { _ = refreshMCPRegistry() }) })
}

var (
	mcpRefreshOnce sync.Once
	mcpRefreshing  sync.WaitGroup
)

// RefreshMCPRegistryIfDue is the background read done in the foreground, for
// doctor: nothing when no server is configured or the cache is fresh.
func RefreshMCPRegistryIfDue() error {
	if copilot, openCode := configuredMCPServers(); len(copilot)+len(openCode) == 0 {
		return nil
	}
	if c, ok := readMCPRegistryCache(); ok && time.Since(c.At) <= mcpRegistryTTL {
		return nil
	}
	return refreshMCPRegistry()
}

// lookupIPAddr resolves a host for classification. A var for tests.
var lookupIPAddr = net.DefaultResolver.LookupIPAddr

// ClassifyMCPHosts marks each host that resolves to a private address.
//
// A host DNS says does not exist counts as private. The intern.nav.no servers
// resolve only over naisdevice, so an answer given with naisdevice off would
// otherwise approve them without the waiver they need. The waiver is exact
// host, registry-curated and consented, so the cost of a public host marked
// private is cplt's DNS-rebinding guard lifted for that one name.
func ClassifyMCPHosts(hosts []MCPHost) []MCPHost {
	out := slices.Clone(hosts)
	var wg sync.WaitGroup
	for i := range out {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			addrs, err := lookupIPAddr(ctx, out[i].Host)
			var dnsErr *net.DNSError
			notFound := errors.As(err, &dnsErr) && dnsErr.IsNotFound
			// A timeout says nothing about the host, and a classification
			// is kept with the answer: a slow resolver must not lift the
			// guard for a public host for good.
			out[i].Unknown = err != nil && !notFound
			out[i].Private = notFound || (err == nil && len(addrs) == 0)
			for _, a := range addrs {
				if a.IP.IsPrivate() || a.IP.IsLoopback() || a.IP.IsLinkLocalUnicast() {
					out[i].Private = true
				}
			}
		}()
	}
	wg.Wait()
	return out
}

// ReclassifyMCPHostsInBackground looks up again, behind the launch, the
// approved hosts whose lookup failed when they were answered, and records
// what it finds under the same approval. Call it after any question, so the
// answer it rewrites is the latest.
func ReclassifyMCPHostsInBackground() {
	rec, err := readMCPRecord()
	if err != nil || rec == nil || !rec.Approved {
		return
	}
	hosts := recordedMCPHosts(rec)
	var unknown []MCPHost
	for _, h := range hosts {
		if h.Unknown {
			unknown = append(unknown, h)
		}
	}
	if len(unknown) == 0 {
		return
	}
	mcpRefreshing.Go(func() {
		changed := false
		for _, h := range ClassifyMCPHosts(unknown) {
			if !h.Unknown {
				i := slices.IndexFunc(hosts, func(o MCPHost) bool { return o.Host == h.Host })
				hosts[i], changed = h, true
			}
		}
		if changed {
			// ponytail: read-modify-write without a compare; a second
			// nav-pilot answering in the same second can lose to this.
			_ = RecordMCPHosts(hosts, true)
		}
	})
}

// MCPHostState is where the configured servers' hosts stand against the
// recorded answer.
type MCPHostState struct {
	Current  MCPHosts
	FetchErr error
	Record   *artifacts.ProposalConsent
	// Grant is what a launch applies now: the approved hosts the registry
	// still names, or every approved host when the registry did not answer.
	Grant []MCPHost
	// Pending is the set to ask about, nil when there is no question: the
	// current set has a host outside the approval and is not inside a set
	// already declined.
	Pending []MCPHost
	// Previous is the set the recorded answer was about, for the diff.
	Previous []MCPHost
}

// MCPHostsOff is the user's mcp_hosts = off: nothing is asked and nothing is
// granted, whatever a record says. Set by the cli before a launch.
var MCPHostsOff bool

// recordedMCPHosts decodes the host set a record answered about.
func recordedMCPHosts(rec *artifacts.ProposalConsent) []MCPHost {
	if rec == nil {
		return nil
	}
	var hosts []MCPHost
	if json.Unmarshal([]byte(rec.Block), &hosts) != nil {
		return nil
	}
	return hosts
}

// mcpHostState is the verdict. Pure.
func mcpHostState(cur MCPHosts, fetchErr error, rec *artifacts.ProposalConsent) MCPHostState {
	st := MCPHostState{Current: cur, FetchErr: fetchErr, Record: rec, Previous: recordedMCPHosts(rec)}
	var approved []MCPHost
	if rec != nil && rec.Approved {
		approved = recordedMCPHosts(rec)
	}
	if fetchErr != nil {
		st.Grant = approved
		return st
	}
	var outside bool
	for _, h := range cur.Hosts {
		i := slices.IndexFunc(approved, func(a MCPHost) bool { return a.Host == h.Host })
		if i < 0 {
			outside = true
			continue
		}
		// The classification the answer was given on, and today's reasons.
		g := approved[i]
		g.Servers = h.Servers
		st.Grant = append(st.Grant, g)
	}
	// A decline is a ceiling too: a set inside a declined one is declined,
	// or dropping one server would ask again on every launch.
	declined := rec != nil && !rec.Approved && !slices.ContainsFunc(cur.Hosts, func(h MCPHost) bool {
		return !slices.ContainsFunc(st.Previous, func(p MCPHost) bool { return p.Host == h.Host })
	})
	if outside && !declined {
		st.Pending = cur.Hosts
	}
	return st
}

// ReadMCPHostState resolves the configured servers' hosts (once per process)
// and reads the recorded answer. An unreadable record is an error and grants
// nothing.
func ReadMCPHostState() (MCPHostState, error) {
	cur, fetchErr := currentMCPHosts()
	rec, err := readMCPRecord()
	if err != nil {
		return MCPHostState{Current: cur, FetchErr: fetchErr}, err
	}
	return mcpHostState(cur, fetchErr, rec), nil
}

func readMCPRecord() (*artifacts.ProposalConsent, error) {
	user, err := domain.ScopeUser()
	if err != nil {
		return nil, err
	}
	return artifacts.ReadProposalConsent(user, MCPConsentSource)
}

// RecordMCPHosts records an answer about hosts as classified by
// [ClassifyMCPHosts]: the private ones are the launch's waiver flags.
func RecordMCPHosts(hosts []MCPHost, approve bool) error {
	user, err := domain.ScopeUser()
	if err != nil {
		return err
	}
	block, err := json.Marshal(hosts)
	if err != nil {
		return err
	}
	rec := artifacts.ProposalConsent{
		Scope:     user.Name,
		Root:      user.RootDir,
		Pakke:     MCPConsentSource,
		Hash:      mcpHostsHash(hosts),
		Block:     string(block),
		CpltStamp: CpltStamp(),
		Approved:  approve,
		At:        time.Now(),
	}
	if approve {
		rec.Hosts = privateMCPHosts(hosts)
	}
	return artifacts.WriteProposalConsent(rec)
}

func privateMCPHosts(hosts []MCPHost) []string {
	var out []string
	for _, h := range hosts {
		if h.Private {
			out = append(out, h.Host)
		}
	}
	return out
}

// MCPAllowlistHosts is the hosts nav-pilot's allowlist file carries for MCP
// servers: every granted host, private ones included, because under an
// allowlist cplt checks the list before it checks the address. Nil when
// nothing is approved or the record cannot be trusted.
func MCPAllowlistHosts() []string {
	grant := mcpGrant(false, false)
	return mcpHostNames(grant)
}

// mcpPrivateDomainFlags is the launch's --allow-private-domain flags for the
// MCP grant, less any host the pakke waiver already carries.
func mcpPrivateDomainFlags(already []string) []string {
	grant := mcpGrant(true, true)
	var hosts []string
	for _, h := range privateMCPHosts(grant) {
		if !slices.Contains(already, h) {
			hosts = append(hosts, h)
		}
	}
	if len(hosts) == 0 {
		return nil
	}
	fmt.Fprintf(os.Stderr, "%s MCP servers (%s): cplt may resolve %s to private addresses, approved for this user.\n",
		domain.Dim("ℹ"), strings.Join(mcpGrantServers(grant, hosts), ", "), strings.Join(hosts, ", "))
	return privateDomainFlags(hosts)
}

func mcpGrantServers(grant []MCPHost, hosts []string) []string {
	var out []string
	for _, h := range grant {
		if slices.Contains(hosts, h.Host) {
			for _, s := range h.Servers {
				if !slices.Contains(out, s) {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// mcpGrant is what a launch may apply. Never the network, and no cplt probe
// for a user with no approved record, which is almost everyone, nor with
// privateOnly for a record that waives no private host: a grant that would
// change nothing needs no trust check.
func mcpGrant(say, privateOnly bool) []MCPHost {
	if MCPHostsOff {
		return nil
	}
	rec, err := readMCPRecord()
	if err != nil {
		if say {
			fmt.Fprintf(os.Stderr, "%s MCP servers: no registry hosts are granted — %v\n", domain.Yellow("⚠"), err)
		}
		return nil
	}
	if rec == nil || !rec.Approved || (privateOnly && len(rec.Hosts) == 0) {
		return nil
	}
	if reason := untrustworthyRecord(rec); reason != "" {
		if say {
			fmt.Fprintf(os.Stderr, "%s MCP servers: the approved registry hosts are not applied — %s.\n", domain.Yellow("⚠"), reason)
		}
		return nil
	}
	cur, fetchErr := currentMCPHosts()
	st := mcpHostState(cur, fetchErr, rec)
	if fetchErr != nil && say && len(st.Grant) > 0 {
		fmt.Fprintf(os.Stderr, "%s MCP servers: %v; keeping the %d MCP host(s) approved before.\n",
			domain.Yellow("⚠"), fetchErr, len(st.Grant))
	}
	return st.Grant
}
