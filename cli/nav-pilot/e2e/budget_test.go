package e2e

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// budgetClientLog makes the test binary, run as cplt, copilot or opencode,
// stand in for the client: it answers --version at once and otherwise writes
// the moment it started to this file and exits.
const budgetClientLog = "NAV_PILOT_E2E_BUDGET_CLIENT_LOG"

// Budgets for nav-pilot's own time: what a user waits for before the
// client starts, after it exits, for --version and --help, and for a short
// command that, with a local source, does not need the network (config get,
// list). The network is a blackhole (every connection accepted, never
// answered): what a bad VPN or a firewall that drops packets looks like. None
// of these paths may wait on it.
//
// The test fails at budgetMargin times the budget, on the median of
// budgetRuns: CI runners are slower and noisier than a laptop, and a real
// regression here is a network wait of a second or more, not 20 ms.
//
// Neither telemetry nor the daily version check may cost anything at all.
// Every case runs three ways: with the version check due (its endpoint in the
// blackhole), and twice with it done for the day, once with telemetry on (its
// collector in the blackhole) and once with it off. The version check due
// must match it done, and telemetry on must match it off, within noise.
var budgets = map[string]time.Duration{
	"version": 50 * time.Millisecond,
	"help":    50 * time.Millisecond,
	"launch":  150 * time.Millisecond,
	"exit":    200 * time.Millisecond,
	"command": 100 * time.Millisecond,
}

const (
	budgetMargin = 3
	budgetRuns   = 6
	noise        = 40 * time.Millisecond
)

// mode is how a timed run is set up.
type mode int

const (
	budgetMode   mode = iota // telemetry on, version check due
	telemetryOn              // telemetry on, version check done for the day
	telemetryOff             // telemetry off, version check done for the day
	modes
)

// runBudgetClient is the fake client's whole program.
func runBudgetClient() {
	name := filepath.Base(os.Args[0])
	if slices.Contains(os.Args[1:], "--version") {
		switch name {
		case "cplt":
			fmt.Println("cplt 2026.09.24-192459-38642b4")
		case "opencode":
			fmt.Println("1.18.25")
		default:
			fmt.Println("GitHub Copilot CLI 1.0.40")
		}
		return
	}
	f, err := os.OpenFile(os.Getenv(budgetClientLog), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		fmt.Fprintf(f, "%d\n", time.Now().UnixNano())
		f.Close()
	}
}

// blackhole accepts connections and never answers them.
func blackhole(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var held []net.Conn
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range held {
			c.Close()
		}
	})
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, c)
			mu.Unlock()
		}
	}()
	return ln.Addr().String()
}

// TestLaunchBudget runs the launch and exit paths against fakes and a
// blackholed network, and fails when nav-pilot's own time goes over budget.
func TestLaunchBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	bin := binary(t)
	e := newEnv(t)
	src := e.pakke("plattform", "grillmester")
	repo := e.consumer("repo")
	if out, code := e.run(repo, "install", "plattform", "--source", src, "--repo", "--yes"); code != 0 {
		t.Fatalf("install: %d\n%s", code, out)
	}
	// opencode materializes the configured source; without one it fetches
	// navikt/copilot the first time, which is a wait on purpose and not what
	// this measures. TestResolveForLaunchCachesAndRefreshes covers the cache.
	if out, code := e.run(repo, "config", "set", "source", src); code != 0 {
		t.Fatalf("config set source: %d\n%s", code, out)
	}

	fake := filepath.Join(e.root, "fake")
	if err := os.MkdirAll(fake, 0o755); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// copilot is a copy, not a link: nav-pilot refuses a copilot that is
	// the same file as cplt (a disguised cplt).
	for _, name := range []string{"cplt", "opencode"} {
		if err := os.Symlink(self, filepath.Join(fake, name)); err != nil {
			t.Fatal(err)
		}
	}
	selfBytes, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "copilot"), selfBytes, 0o755); err != nil {
		t.Fatal(err)
	}
	// git only, not its directory: nothing else from the machine (gh, a real
	// copilot or opencode) may take part.
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(git, filepath.Join(fake, "git")); err != nil {
		t.Fatal(err)
	}
	hole := blackhole(t)
	clientLog := filepath.Join(e.root, "client.log")

	environ := append(os.Environ(),
		"PATH="+fake+string(os.PathListSeparator)+filepath.Dir(bin),
		"NO_COLOR=1",
		budgetClientLog+"="+clientLog,
		// The fake client is this test binary: under -race it would sleep a
		// second at exit, and that second is not nav-pilot's.
		"GORACE=atexit_sleep_ms=0",
		// Telemetry on, as for a user, with its collector in the blackhole.
		"NAV_PILOT_TELEMETRY_ENABLED=true", "DO_NOT_TRACK=",
		"NAV_PILOT_TELEMETRY_ENDPOINT=http://"+hole+"/v1/metrics",
		// A release build's version, so the daily release check runs too.
		"NAV_PILOT_E2E_VERSION=2026.09.01-120000-aaaaaaa",
		"HTTP_PROXY=http://"+hole, "HTTPS_PROXY=http://"+hole,
		"http_proxy=http://"+hole, "https_proxy=http://"+hole,
		"NO_PROXY=", "no_proxy=",
	)
	for k, v := range e.commandEnv() {
		environ = append(environ, k+"="+v)
	}
	// The same with telemetry off: the baseline telemetry is held to.
	environOff := append(slices.Clone(environ), "NAV_PILOT_TELEMETRY_ENABLED=false")

	// run times one invocation: the whole of it, and the moment the client
	// started (zero when none did).
	// The release check is due on every run in budget mode, so it is
	// measured too (it backs off an hour after a failure otherwise).
	checked := fmt.Sprintf(`{"last_checked":%q,"latest_version":"2026.09.01-120000-aaaaaaa"}`, time.Now().UTC().Format(time.RFC3339))
	run := func(m mode, args ...string) (total time.Duration, launch, exit time.Duration) {
		t.Helper()
		os.Remove(clientLog)
		cache := filepath.Join(e.home, "cache.json")
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		cmd.Env = environ
		switch m {
		case budgetMode:
			os.Remove(cache)
		case telemetryOn:
			os.WriteFile(cache, []byte(checked), 0o644)
		case telemetryOff:
			os.WriteFile(cache, []byte(checked), 0o644)
			cmd.Env = environOff
		}
		start := time.Now()
		out, err := cmd.CombinedOutput()
		end := time.Now()
		if err != nil {
			t.Fatalf("nav-pilot %v: %v\n%s", args, err, out)
		}
		if b, err := os.ReadFile(clientLog); err == nil {
			lines := strings.Fields(string(b))
			if len(lines) == 0 {
				t.Fatalf("nav-pilot %v: the client wrote no start time", args)
			}
			ns, _ := strconv.ParseInt(lines[len(lines)-1], 10, 64)
			started := time.Unix(0, ns)
			return end.Sub(start), started.Sub(start), end.Sub(started)
		}
		return end.Sub(start), 0, 0
	}
	median := func(d []time.Duration) time.Duration {
		slices.Sort(d)
		return d[len(d)/2]
	}
	check := func(name string, d [modes][]time.Duration) {
		t.Helper()
		budget := budgets[name]
		got, on, off := median(d[budgetMode]), median(d[telemetryOn]), median(d[telemetryOff])
		t.Logf("%-8s median %4d ms (budget %d ms); version check done: telemetry on %4d ms, off %4d ms", name, got.Milliseconds(), budget.Milliseconds(), on.Milliseconds(), off.Milliseconds())
		if got > budget*budgetMargin {
			t.Errorf("%s took %s (median of %d), budget %s: over %dx the budget. Something on this path waits on the network or does too much before the client starts; see docs/README.nav-pilot.md, «Ytelse»",
				name, got, budgetRuns, budget, budgetMargin)
		}
		if got > on+noise {
			t.Errorf("%s took %s with the version check due and %s with it done: something waits for the version check. It may not; see artifacts.AssessStalenessCached",
				name, got, on)
		}
		if on > off+noise {
			t.Errorf("%s took %s with telemetry on and %s with it off: something waits for telemetry. It may not; see internal/telemetry/spool.go",
				name, on, off)
		}
	}

	// The run right after one in budget mode is the slower one, whatever it
	// is (it pays for what that run left behind), so telemetry on and off
	// take turns being it.
	order := func(i int) []mode {
		if i%2 == 0 {
			return []mode{budgetMode, telemetryOn, telemetryOff}
		}
		return []mode{budgetMode, telemetryOff, telemetryOn}
	}

	// One untimed run of each: the first run of a new binary pays for the
	// OS's first look at it, and the first launch writes its one-time notices.
	for m := range modes {
		run(m, "--version")
		run(m, "--", "-p", "hei")
		run(m, "--client", "opencode", "--", "run", "hei")
	}

	for _, c := range []struct {
		name string
		args []string
	}{
		{"version", []string{"--version"}},
		{"help", []string{"--help"}},
		{"command", []string{"config", "get", "client"}},
		{"command", []string{"list"}},
	} {
		var d [modes][]time.Duration
		for i := range budgetRuns {
			for _, m := range order(i) {
				total, _, _ := run(m, c.args...)
				d[m] = append(d[m], total)
			}
		}
		check(c.name, d)
	}

	for _, client := range [][]string{
		{"--", "-p", "hei"},
		{"--client", "opencode", "--", "run", "hei"},
	} {
		var launches, exits [modes][]time.Duration
		for i := range budgetRuns {
			for _, m := range order(i) {
				_, launch, exit := run(m, client...)
				if launch == 0 {
					t.Fatalf("nav-pilot %v started no client", client)
				}
				launches[m], exits[m] = append(launches[m], launch), append(exits[m], exit)
			}
		}
		t.Logf("nav-pilot %v:", client)
		check("launch", launches)
		check("exit", exits)
	}
}
