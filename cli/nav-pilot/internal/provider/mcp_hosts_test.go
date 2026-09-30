package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// testRegistry is Nav's registry in miniature: a public remote, a private one
// and a loopback one.
func testRegistry() mcpRegistry {
	reg := mcpRegistry{Remotes: map[string]bool{}, Packages: map[string]bool{}, Servers: map[string][]string{
		"com.figma/figma-mcp":             {"https://mcp.figma.com/mcp"},
		"io.github.navikt/mcp-onboarding": {"https://mcp-onboarding.intern.nav.no/mcp"},
		"io.github.navikt/aksel-arcade":   {"http://127.0.0.1:3846/mcp"},
	}}
	for _, urls := range reg.Servers {
		for _, u := range urls {
			reg.Remotes[normalizeMCPURL(u)] = true
		}
	}
	return reg
}

// mcpEnv isolates HOME and nav-pilot's state, writes a Copilot MCP config,
// and answers the registry questions without a network.
func mcpEnv(t *testing.T, copilotConfig string, reg mcpRegistry, regErr error) {
	t.Helper()
	proposeEnv(t)
	home, _ := os.UserHomeDir()
	if copilotConfig != "" {
		if err := os.MkdirAll(filepath.Join(home, ".copilot"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".copilot", "mcp-config.json"), []byte(copilotConfig), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// No project OpenCode config from the repo this test runs in.
	t.Chdir(t.TempDir())
	prevPolicy, prevReg, prevCur := fetchMCPPolicy, fetchMCPRegistry, currentMCPHosts
	fetchMCPPolicy = func() (string, error) { return "https://registry.test", nil }
	fetchMCPRegistry = func(string) (mcpRegistry, error) { return reg, regErr }
	currentMCPHosts = resolveMCPHosts // not memoized: each test asks afresh
	mcpRefreshOnce = sync.Once{}
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry, currentMCPHosts = prevPolicy, prevReg, prevCur })
	t.Cleanup(mcpRefreshing.Wait) // runs first: no read outlives the fakes
	// The registry's answer as a launch finds it: cached, or never read.
	if regErr == nil {
		if err := refreshMCPRegistry(); err != nil {
			t.Fatal(err)
		}
	}
}

// The security property: the host is the registry's. A config entry that names
// a registry server but points somewhere else gets the registry's host and
// never its own URL's; an entry the registry does not know gets nothing.
func TestMCPHostsComeFromTheRegistryNotTheConfig(t *testing.T) {
	mcpEnv(t, `{"mcpServers": {
		"com.figma/figma-mcp": {"type": "http", "url": "https://evil.example/mcp"},
		"io.github.navikt/aksel-arcade": {"type": "http", "url": "http://127.0.0.1:3846/mcp"},
		"my-own": {"type": "http", "url": "https://also-evil.example/mcp"}
	}}`, testRegistry(), nil)

	got, err := resolveMCPHosts()
	if err != nil {
		t.Fatal(err)
	}
	if names := got.Names(); !slices.Equal(names, []string{"mcp.figma.com"}) {
		t.Errorf("hosts = %v, want only the registry's mcp.figma.com", names)
	}
	if !slices.Equal(got.Loopback, []MCPLoopback{{"io.github.navikt/aksel-arcade", "3846"}}) {
		t.Errorf("loopback = %v, want aksel-arcade on 3846 and no grant", got.Loopback)
	}
}

// OpenCode names are the user's own, so a remote matches by a URL the
// registry lists, and the host is still the registry's spelling.
func TestOpenCodeMCPMatchesByRegistryURL(t *testing.T) {
	off := false
	got := matchMCPHosts(testRegistry(), nil, map[string]mcpServer{
		"figma":    {Type: "remote", URL: "HTTPS://MCP.FIGMA.COM/mcp/"},
		"evil":     {Type: "remote", URL: "https://evil.example/mcp"},
		"disabled": {Type: "remote", URL: "https://mcp-onboarding.intern.nav.no/mcp", Enabled: &off},
	})
	if names := got.Names(); !slices.Equal(names, []string{"mcp.figma.com"}) {
		t.Errorf("hosts = %v, want mcp.figma.com only", names)
	}
	if len(got.Hosts) == 1 && !slices.Equal(got.Hosts[0].Servers, []string{"com.figma/figma-mcp"}) {
		t.Errorf("servers = %v, want the registry's name, not the config's", got.Hosts[0].Servers)
	}
}

// A registry server's sandboxHosts (Figma's OAuth host) join its remotes in
// the consent set, under the same server, for both clients; a server the user
// did not configure brings none.
func TestMCPSandboxHostsJoinTheConsentSet(t *testing.T) {
	reg := testRegistry()
	reg.Entries = []MCPServerEntry{
		{Name: "com.figma/figma-mcp", SandboxHosts: []string{"api.figma.com"}},
		{Name: "io.github.navikt/mcp-onboarding", SandboxHosts: []string{"unused.example"}},
	}
	want := []MCPHost{
		{Host: "api.figma.com", Servers: []string{"com.figma/figma-mcp"}},
		{Host: "mcp.figma.com", Servers: []string{"com.figma/figma-mcp"}},
	}
	if got := matchMCPHosts(reg, []string{"com.figma/figma-mcp"}, nil); !slices.EqualFunc(got.Hosts, want, func(a, b MCPHost) bool {
		return a.Host == b.Host && slices.Equal(a.Servers, b.Servers)
	}) {
		t.Errorf("copilot hosts = %v, want %v", got.Hosts, want)
	}
	got := matchMCPHosts(reg, nil, map[string]mcpServer{"figma": {Type: "remote", URL: "https://mcp.figma.com/mcp"}})
	if names := got.Names(); !slices.Equal(names, []string{"api.figma.com", "mcp.figma.com"}) {
		t.Errorf("opencode hosts = %v, want api.figma.com and mcp.figma.com", names)
	}
}

func approvedMCP(t *testing.T, hosts []MCPHost) *artifacts.ProposalConsent {
	t.Helper()
	if err := RecordMCPHosts(hosts, true); err != nil {
		t.Fatal(err)
	}
	rec, err := readMCPRecord()
	if err != nil || rec == nil {
		t.Fatalf("record: %v %v", rec, err)
	}
	return rec
}

func set(hosts ...string) MCPHosts {
	var m MCPHosts
	for _, h := range hosts {
		m.Hosts = append(m.Hosts, MCPHost{Host: h, Servers: []string{"s"}})
	}
	return m
}

// One answer per host set: asked when a host is outside the approval, not
// asked again for the same set, and asked again when it grows. A set that
// shrinks needs no answer and grants only what is still needed.
func TestMCPConsentByHostSet(t *testing.T) {
	mcpEnv(t, "", mcpRegistry{}, nil)

	if st := mcpHostState(set("a.example"), nil, nil); st.Pending == nil || st.Grant != nil {
		t.Fatalf("no record: pending=%v grant=%v, want a question and no grant", st.Pending, st.Grant)
	}
	rec := approvedMCP(t, set("a.example", "b.example").Hosts)
	if rec.Hash != set("b.example", "a.example").Hash() {
		t.Errorf("the hash depends on order")
	}
	if st := mcpHostState(set("a.example", "b.example"), nil, rec); st.Pending != nil || len(st.Grant) != 2 {
		t.Errorf("same set: pending=%v grant=%v, want no question and both granted", st.Pending, st.Grant)
	}
	if st := mcpHostState(set("a.example"), nil, rec); st.Pending != nil || !slices.Equal(mcpHostNames(st.Grant), []string{"a.example"}) {
		t.Errorf("shrunk set: pending=%v grant=%v, want no question and a.example only", st.Pending, st.Grant)
	}
	st := mcpHostState(set("a.example", "b.example", "c.example"), nil, rec)
	if len(st.Pending) != 3 {
		t.Errorf("grown set: pending=%v, want the whole new set asked about", st.Pending)
	}
	if !slices.Equal(mcpHostNames(st.Grant), []string{"a.example", "b.example"}) {
		t.Errorf("grown set, unanswered: grant=%v, want the earlier approval kept", st.Grant)
	}

	if err := RecordMCPHosts(set("a.example", "b.example", "c.example").Hosts, false); err != nil {
		t.Fatal(err)
	}
	declined, _ := readMCPRecord()
	if st := mcpHostState(set("a.example", "b.example", "c.example"), nil, declined); st.Pending != nil || st.Grant != nil {
		t.Errorf("declined set: pending=%v grant=%v, want no question and no grant", st.Pending, st.Grant)
	}
	if st := mcpHostState(set("a.example", "b.example"), nil, declined); st.Pending != nil || st.Grant != nil {
		t.Errorf("shrunk declined set: pending=%v grant=%v, want no question and no grant", st.Pending, st.Grant)
	}
	if st := mcpHostState(set("a.example", "d.example"), nil, declined); st.Pending == nil {
		t.Errorf("a changed set after a decline was not asked about")
	}
}

// A registry that does not answer keeps the approved set, and a launch says so.
func TestMCPFetchFailureKeepsTheApprovedHosts(t *testing.T) {
	mcpEnv(t, `{"mcpServers": {"com.figma/figma-mcp": {}}}`, mcpRegistry{}, errors.New("offline"))
	protectingCplt(t)
	approvedMCP(t, []MCPHost{
		{Host: "mcp-onboarding.intern.nav.no", Servers: []string{"x"}, Private: true},
		{Host: "mcp.figma.com", Servers: []string{"y"}},
	})
	st, err := ReadMCPHostState()
	if err != nil || st.FetchErr == nil {
		t.Fatalf("state: %+v %v", st, err)
	}
	if st.Pending != nil || len(st.Grant) != 2 {
		t.Errorf("offline: pending=%v grant=%v, want the approved pair and no question", st.Pending, st.Grant)
	}
	if got := MCPAllowlistHosts(); !slices.Equal(got, []string{"mcp-onboarding.intern.nav.no", "mcp.figma.com"}) {
		t.Errorf("allowlist hosts offline = %v", got)
	}
	if got := cpltProposalFlags(); !slices.Equal(got, []string{"--allow-private-domain", "mcp-onboarding.intern.nav.no"}) {
		t.Errorf("offline flags = %q", got)
	}
}

// Private hosts, and hosts that do not resolve, get the exact-host waiver;
// public hosts only the allowlist file. All of them are on the allowlist,
// which cplt checks first.
func TestMCPPrivatePublicSplit(t *testing.T) {
	mcpEnv(t, `{"mcpServers": {"com.figma/figma-mcp": {}, "io.github.navikt/mcp-onboarding": {}}}`, testRegistry(), nil)
	protectingCplt(t)
	prev := lookupIPAddr
	lookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		switch host {
		case "mcp-onboarding.intern.nav.no":
			return []net.IPAddr{{IP: net.ParseIP("10.7.8.200")}}, nil
		case "mcp.figma.com":
			return []net.IPAddr{{IP: net.ParseIP("151.101.1.1")}}, nil
		}
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	t.Cleanup(func() { lookupIPAddr = prev })

	if got := cpltProposalFlags(); len(got) != 0 {
		t.Fatalf("flags before consent: %q", got)
	}
	st, err := ReadMCPHostState()
	if err != nil || len(st.Pending) != 2 {
		t.Fatalf("pending: %+v %v", st, err)
	}
	hosts := ClassifyMCPHosts(append(st.Pending, MCPHost{Host: "unresolvable.example"}))
	if err := RecordMCPHosts(hosts, true); err != nil {
		t.Fatal(err)
	}
	rec, _ := readMCPRecord()
	if !slices.Equal(rec.Hosts, []string{"mcp-onboarding.intern.nav.no", "unresolvable.example"}) {
		t.Errorf("recorded waiver = %v, want the private and the unresolvable host", rec.Hosts)
	}
	if got := cpltProposalFlags(); !slices.Equal(got, []string{"--allow-private-domain", "mcp-onboarding.intern.nav.no"}) {
		t.Errorf("flags = %q, want the one private host the registry still names, exact", got)
	}
	if got := MCPAllowlistHosts(); !slices.Equal(got, []string{"mcp-onboarding.intern.nav.no", "mcp.figma.com"}) {
		t.Errorf("allowlist hosts = %v", got)
	}
}

// A host the pakke waiver and an MCP server both need is one flag, and each
// source's record stands alone: the pakke's approval does not grant MCP hosts.
func TestPakkeAndMCPHostsCombine(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")
	prevCur := currentMCPHosts
	currentMCPHosts = func() (MCPHosts, error) {
		return MCPHosts{Hosts: []MCPHost{{Host: "cloud.nais.io"}, {Host: "mcp-onboarding.intern.nav.no"}}}, nil
	}
	t.Cleanup(func() { currentMCPHosts = prevCur })

	if got := cpltProposalFlags(); !slices.Equal(got, []string{"--allow-private-domain", "cloud.nais.io"}) {
		t.Fatalf("pakke only = %q", got)
	}
	approvedMCP(t, []MCPHost{
		{Host: "cloud.nais.io", Private: true},
		{Host: "mcp-onboarding.intern.nav.no", Private: true},
	})
	want := []string{"--allow-private-domain", "cloud.nais.io", "--allow-private-domain", "mcp-onboarding.intern.nav.no"}
	if got := cpltProposalFlags(); !slices.Equal(got, want) {
		t.Errorf("combined = %q, want %q", got, want)
	}
	// Revoking the pakke's answer keeps the host the MCP server still needs.
	if err := artifacts.WriteProposalConsent(artifacts.ProposalConsent{
		Scope: scope.Name, Root: scope.RootDir, Pakke: "nais-pilot", Hash: proposal.Hash(),
		CpltStamp: minCpltStampProtectingNavPilotState,
	}); err != nil {
		t.Fatal(err)
	}
	if got := cpltProposalFlags(); !slices.Equal(got, want) {
		t.Errorf("pakke revoked = %q, want the MCP pair still", got)
	}
	// Declining the MCP set leaves nothing.
	if err := RecordMCPHosts([]MCPHost{{Host: "cloud.nais.io"}, {Host: "mcp-onboarding.intern.nav.no"}}, false); err != nil {
		t.Fatal(err)
	}
	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("both revoked: %q", got)
	}
}

// mcp_hosts = off grants nothing, whatever the record says.
func TestMCPHostsOffGrantsNothing(t *testing.T) {
	mcpEnv(t, `{"mcpServers": {"com.figma/figma-mcp": {}}}`, mcpRegistry{}, errors.New("offline"))
	protectingCplt(t)
	approvedMCP(t, []MCPHost{{Host: "mcp-onboarding.intern.nav.no", Private: true}})
	MCPHostsOff = true
	t.Cleanup(func() { MCPHostsOff = false })
	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("flags under off = %q", got)
	}
	if got := MCPAllowlistHosts(); len(got) != 0 {
		t.Errorf("allowlist under off = %v", got)
	}
}

// An OpenCode launch asks the registry for its policy check and for the MCP
// hosts; the network is asked once.
func TestMCPRegistryIsFetchedOncePerProcess(t *testing.T) {
	calls := 0
	fetch := memoMCPRegistry(func(string) (mcpRegistry, error) { calls++; return mcpRegistry{}, nil })
	fetch("https://a")
	fetch("https://a")
	fetch("https://b")
	if calls != 2 {
		t.Errorf("fetched %d times, want once per registry", calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// offline makes gh and the registry fail the test if a launch asks them, from
// here on, through the real fetchers.
func offline(t *testing.T, transport roundTripFunc) {
	t.Helper()
	prev := mcpHTTPClient
	mcpHTTPClient = &http.Client{Transport: transport}
	fetchMCPRegistry = memoMCPRegistry(askMCPRegistry)
	fetchMCPPolicy = func() (string, error) { return "https://registry.test", nil }
	t.Cleanup(func() { mcpHTTPClient = prev })
}

// The launch path with a fresh cache asks nothing over the network, in the
// foreground or behind it, and a grant with no private host probes no cplt.
func TestLaunchMakesNoNetworkCallWithAFreshCache(t *testing.T) {
	mcpEnv(t, `{"mcpServers": {"com.figma/figma-mcp": {}, "io.github.navikt/mcp-onboarding": {}}}`, testRegistry(), nil)
	offline(t, func(r *http.Request) (*http.Response, error) {
		t.Errorf("the launch asked %s", r.URL)
		return nil, errors.New("offline")
	})
	protectingCplt(t)
	approvedMCP(t, []MCPHost{{Host: "mcp.figma.com"}, {Host: "mcp-onboarding.intern.nav.no"}})
	stubCpltVersion(t, func() (string, error) {
		t.Error("cplt probed for a grant that waives no private host")
		return cpltWithStateDeny, nil
	})
	st, err := ReadMCPHostState()
	if err != nil || st.FetchErr != nil || st.Pending != nil || len(st.Grant) != 2 {
		t.Fatalf("state: %+v %v", st, err)
	}
	if got := cpltProposalFlags(); len(got) != 0 {
		t.Errorf("flags = %q", got)
	}
	mcpRefreshing.Wait()
}

// A stale cache answers the launch as it is, and a registry that hangs is
// read behind it: the launch does not wait.
func TestStaleCacheDoesNotWaitForTheRegistry(t *testing.T) {
	mcpEnv(t, `{"mcpServers": {"com.figma/figma-mcp": {}}}`, testRegistry(), nil)
	c, _ := readMCPRegistryCache()
	c.At = c.At.Add(-2 * mcpRegistryTTL)
	data, _ := json.Marshal(c)
	if err := os.WriteFile(mcpRegistryCachePath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	release, asked := make(chan struct{}), make(chan struct{})
	offline(t, func(r *http.Request) (*http.Response, error) {
		close(asked)
		select {
		case <-release:
		case <-time.After(2 * time.Second):
			t.Error("the launch waited for a registry that hangs")
		}
		return nil, errors.New("hung up")
	})
	st, err := ReadMCPHostState()
	if err != nil || st.FetchErr != nil || !slices.Equal(st.Current.Names(), []string{"mcp.figma.com"}) {
		t.Fatalf("state: %+v %v", st, err)
	}
	<-asked // the read started, and the launch already has its answer
	close(release)
	mcpRefreshing.Wait()
}

// A repository's own opencode.json names no server for the consent screen,
// under either client: only the launched client's user config counts.
func TestProjectOpenCodeConfigAsksNothing(t *testing.T) {
	mcpEnv(t, "", testRegistry(), nil)
	t.Setenv("XDG_CONFIG_HOME", "")
	hostile := `{"mcp": {"github": {"type": "remote", "url": "https://mcp-onboarding.intern.nav.no/mcp"}}}`
	if err := os.WriteFile("opencode.json", []byte(hostile), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { MCPClient = "" })
	for _, client := range []string{"copilot", "opencode"} {
		MCPClient = client
		if got, err := resolveMCPHosts(); err != nil || len(got.Hosts) != 0 {
			t.Errorf("%s with a project opencode.json: hosts=%v err=%v, want none", client, got.Names(), err)
		}
	}
	// The same server in the user's own OpenCode config counts, for OpenCode
	// only.
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(hostile), 0o600); err != nil {
		t.Fatal(err)
	}
	MCPClient = "opencode"
	got, err := resolveMCPHosts()
	if err != nil || !slices.Equal(got.Names(), []string{"mcp-onboarding.intern.nav.no"}) {
		t.Fatalf("user opencode config: %v %v", got.Names(), err)
	}
	MCPClient = "copilot"
	if got, _ := resolveMCPHosts(); len(got.Hosts) != 0 {
		t.Errorf("copilot counted an OpenCode server: %v", got.Names())
	}
}

// A lookup that times out is no classification: the host is not private, and
// a later launch looks it up again behind the session and records what DNS
// says, under the same approval.
func TestMCPHostTimeoutIsReclassifiedLater(t *testing.T) {
	mcpEnv(t, "", mcpRegistry{}, nil)
	protectingCplt(t)
	prev := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = prev })
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return nil, &net.DNSError{Err: "i/o timeout", IsTimeout: true}
	}
	hosts := ClassifyMCPHosts([]MCPHost{{Host: "mcp-onboarding.intern.nav.no"}})
	if hosts[0].Private || !hosts[0].Unknown {
		t.Fatalf("timeout classified as %+v, want unknown and not private", hosts[0])
	}
	rec := approvedMCP(t, hosts)
	if len(rec.Hosts) != 0 {
		t.Errorf("waiver after a timeout = %v, want none", rec.Hosts)
	}
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("10.7.8.200")}}, nil
	}
	ReclassifyMCPHostsInBackground()
	mcpRefreshing.Wait()
	rec, _ = readMCPRecord()
	if !rec.Approved || !slices.Equal(rec.Hosts, []string{"mcp-onboarding.intern.nav.no"}) {
		t.Errorf("after the retry: approved=%v waiver=%v, want the host private and still approved", rec.Approved, rec.Hosts)
	}
}

// An approval recorded under a cplt that did not protect it is never applied,
// so it is no answer: the launch asks again instead of settling on it.
func TestUntrustworthyMCPApprovalAsksAgain(t *testing.T) {
	mcpEnv(t, `{"mcpServers": {"com.figma/figma-mcp": {}}}`, testRegistry(), nil)
	stubCpltVersion(t, func() (string, error) { return cpltBeforeStateDeny, nil })
	approvedMCP(t, []MCPHost{{Host: "mcp.figma.com"}})
	protectingCplt(t) // upgraded since
	st, err := ReadMCPHostState()
	if err != nil || st.Pending == nil || st.Grant != nil {
		t.Errorf("old-cplt approval: pending=%v grant=%v err=%v, want the question again", st.Pending, st.Grant, err)
	}
}

// Uninstalling the user scope's pakke removes its answers, not the MCP one.
func TestUninstallKeepsTheMCPAnswer(t *testing.T) {
	mcpEnv(t, "", mcpRegistry{}, nil)
	protectingCplt(t)
	approvedMCP(t, []MCPHost{{Host: "mcp.figma.com"}})
	user, _ := domain.ScopeUser()
	if _, err := artifacts.RemoveProposalConsentsIn(user); err != nil {
		t.Fatal(err)
	}
	if rec, err := readMCPRecord(); err != nil || rec == nil {
		t.Errorf("MCP answer after uninstall: %v %v", rec, err)
	}
}
