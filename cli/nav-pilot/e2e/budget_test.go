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
// client starts, after it exits, and for --version and --help. The network is
// a blackhole (every connection accepted, never answered): what a bad VPN or a
// firewall that drops packets looks like. None of these paths may wait on it.
//
// The test fails at budgetMargin times the budget, on the median of
// budgetRuns: CI runners are slower and noisier than a laptop, and a real
// regression here is a network wait of a second or more, not 20 ms.
var budgets = map[string]time.Duration{
	"version": 50 * time.Millisecond,
	"help":    50 * time.Millisecond,
	"launch":  150 * time.Millisecond,
	"exit":    200 * time.Millisecond,
}

const (
	budgetMargin = 3
	budgetRuns   = 5
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
	for _, name := range []string{"cplt", "copilot", "opencode"} {
		if err := os.Symlink(self, filepath.Join(fake, name)); err != nil {
			t.Fatal(err)
		}
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	hole := blackhole(t)
	clientLog := filepath.Join(e.root, "client.log")

	environ := append(os.Environ(),
		"PATH="+fake+string(os.PathListSeparator)+filepath.Dir(bin)+string(os.PathListSeparator)+filepath.Dir(git),
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

	// run times one invocation: the whole of it, and the moment the client
	// started (zero when none did).
	run := func(args ...string) (total time.Duration, launch, exit time.Duration) {
		t.Helper()
		os.Remove(clientLog)
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		cmd.Env = environ
		start := time.Now()
		out, err := cmd.CombinedOutput()
		end := time.Now()
		if err != nil {
			t.Fatalf("nav-pilot %v: %v\n%s", args, err, out)
		}
		if b, err := os.ReadFile(clientLog); err == nil {
			lines := strings.Fields(string(b))
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
	check := func(name string, got time.Duration) {
		t.Helper()
		budget := budgets[name]
		t.Logf("%-8s median %4d ms (budget %d ms)", name, got.Milliseconds(), budget.Milliseconds())
		if got > budget*budgetMargin {
			t.Errorf("%s took %s (median of %d), budget %s: over %dx the budget. Something on this path waits on the network or does too much before the client starts; see docs/README.nav-pilot.md, «Ytelse»",
				name, got, budgetRuns, budget, budgetMargin)
		}
	}

	// One untimed run of each: the first run of a new binary pays for the
	// OS's first look at it, and the first launch writes its one-time notices.
	run("--version")
	run("--", "-p", "hei")
	run("--client", "opencode", "--", "run", "hei")

	for _, c := range []struct {
		name string
		args []string
	}{
		{"version", []string{"--version"}},
		{"help", []string{"--help"}},
	} {
		var d []time.Duration
		for range budgetRuns {
			total, _, _ := run(c.args...)
			d = append(d, total)
		}
		check(c.name, median(d))
	}

	for _, client := range [][]string{
		{"--", "-p", "hei"},
		{"--client", "opencode", "--", "run", "hei"},
	} {
		var launches, exits []time.Duration
		for range budgetRuns {
			_, launch, exit := run(client...)
			if launch == 0 {
				t.Fatalf("nav-pilot %v started no client", client)
			}
			launches, exits = append(launches, launch), append(exits, exit)
		}
		t.Logf("nav-pilot %v:", client)
		check("launch", median(launches))
		check("exit", median(exits))
	}
}
