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
//
// Those two are compared run by run, not median against median. The three
// runs of one round follow each other within a second, so whatever else the
// machine does at that moment (another check, Spotlight, the endpoint
// scanner) lands on all three; across rounds a launch here varied from 80 ms
// to 1.2 s on a busy laptop, far more than noise. The median of the per-round
// differences cancels that; two medians of independent samples do not.
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

// pairedDiff is the median of a[i]-b[i]: how much slower a is than b, run
// for run.
func pairedDiff(a, b []time.Duration) time.Duration {
	d := make([]time.Duration, len(a))
	for i := range a {
		d[i] = a[i] - b[i]
	}
	return median(d)
}

// median is the true median: with an even count, the mean of the middle two.
// The upper of the two would side with whichever run of a pair is slower,
// and where a run sits in its round moves its time (see order).
func median(d []time.Duration) time.Duration {
	d = slices.Sorted(slices.Values(d))
	n := len(d)
	return (d[(n-1)/2] + d[n/2]) / 2
}

// runBudgetClient is the fake client's whole program.
func runBudgetClient() {
	name := filepath.Base(os.Args[0])
	if name == "git" {
		if f, err := os.OpenFile(os.Getenv(budgetClientLog), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, "git %v\n", os.Args[1:])
			f.Close()
		}
		os.Exit(1)
	}
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
	if name == "opencode" && slices.Contains(os.Args[1:], "session") {
		fmt.Println("[]")
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
	// The source is this repository's own checkout when the test runs in
	// one: opencode reads the whole source at launch, and a user's is
	// navikt/copilot (#1276), not the two files of src.
	launchSrc := src
	if root, err := filepath.Abs(filepath.Join("..", "..", "..")); err == nil {
		if _, err := os.Stat(filepath.Join(root, "skills")); err == nil {
			launchSrc = root
		}
	}
	if out, code := e.run(repo, "config", "set", "source", launchSrc); code != 0 {
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
	// the same file as cplt (a disguised cplt). git is a link too so any git
	// spawn on a cached path is caught and fails the test.
	for _, name := range []string{"cplt", "opencode", "git"} {
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
			if strings.Contains(string(b), "git ") {
				t.Fatalf("nav-pilot %v spawned git (%s); cached runs must read .git/HEAD directly", args, strings.TrimSpace(string(b)))
			}
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
	var summaryRows []string
	// extra is what telemetry costs on purpose on this path; telemetry on may
	// exceed off by noise plus extra.
	check := func(name, label string, d [modes][]time.Duration, extra time.Duration) {
		t.Helper()
		budget := budgets[name]
		ceil := budget * budgetMargin
		got, on, off := median(d[budgetMode]), median(d[telemetryOn]), median(d[telemetryOff])
		vDiff := pairedDiff(d[budgetMode], d[telemetryOn])
		tDiff := pairedDiff(d[telemetryOn], d[telemetryOff])
		tNoise := noise + extra
		t.Logf("%-20s median %4d ms (budget %3d ms, ceiling %3d ms); version check done: telemetry on %4d ms, off %4d ms",
			label, got.Milliseconds(), budget.Milliseconds(), ceil.Milliseconds(), on.Milliseconds(), off.Milliseconds())
		summaryRows = append(summaryRows, fmt.Sprintf(
			"| `%s` | %d ms | %d ms (%d ms) | %+d ms (<= %d ms) | %+d ms (<= %d ms) |",
			label, got.Milliseconds(), budget.Milliseconds(), ceil.Milliseconds(),
			vDiff.Milliseconds(), noise.Milliseconds(), tDiff.Milliseconds(), tNoise.Milliseconds(),
		))
		if got > ceil {
			t.Errorf("%s took %s (median of %d), budget %s: over %dx the budget. Something on this path waits on the network or does too much before the client starts; see docs/README.nav-pilot.md, «Ytelse»",
				label, got, budgetRuns, budget, budgetMargin)
		}
		if vDiff > noise {
			t.Errorf("%s took %s more with the version check due than with it done (median of %d paired runs; medians %s and %s): something waits for the version check. It may not; see artifacts.AssessStalenessCached",
				label, vDiff, budgetRuns, got, on)
		}
		if tDiff > tNoise {
			t.Errorf("%s took %s more with telemetry on than off (median of %d paired runs; medians %s and %s): something waits for telemetry. It may not; see internal/telemetry/spool.go",
				label, tDiff, budgetRuns, on, off)
		}
	}

	// Where a run sits in its round moves its time: the run right after one
	// in budget mode pays for what that run left behind. So the rounds go
	// through all six orders of the three modes, and every mode holds every
	// place equally often (budgetRuns is a multiple of six).
	orders := [][]mode{
		{budgetMode, telemetryOn, telemetryOff},
		{budgetMode, telemetryOff, telemetryOn},
		{telemetryOn, budgetMode, telemetryOff},
		{telemetryOff, budgetMode, telemetryOn},
		{telemetryOn, telemetryOff, budgetMode},
		{telemetryOff, telemetryOn, budgetMode},
	}
	order := func(i int) []mode { return orders[i%len(orders)] }

	// One untimed run of each: the first run of a new binary pays for the
	// OS's first look at it, and the first launch writes its one-time notices.
	for m := range modes {
		run(m, "--version")
		run(m, "--", "-p", "hei")
		run(m, "--client", "opencode", "--", "run", "hei")
	}

	for _, c := range []struct {
		name  string
		label string
		args  []string
	}{
		{"version", "--version", []string{"--version"}},
		{"help", "--help", []string{"--help"}},
		{"command", "config get client", []string{"config", "get", "client"}},
		{"command", "list", []string{"list"}},
	} {
		var d [modes][]time.Duration
		for i := range budgetRuns {
			for _, m := range order(i) {
				total, _, _ := run(m, c.args...)
				d[m] = append(d[m], total)
			}
		}
		check(c.name, c.label, d, 0)
	}

	// Both clients run in each mode of each round, next to each other, and
	// which goes first alternates by round. The opencode launch is held to
	// the Copilot one in the same pairs, so neither is always the run that
	// follows the other.
	clients := []struct {
		name string
		args []string
	}{
		{"copilot", []string{"--", "-p", "hei"}},
		{"opencode", []string{"--client", "opencode", "--", "run", "hei"}},
	}
	var launches, exits [2][modes][]time.Duration
	for i := range budgetRuns {
		for _, m := range order(i) {
			for _, c := range []int{i % 2, 1 - i%2} {
				args := clients[c].args
				_, launch, exit := run(m, args...)
				if launch == 0 {
					t.Fatalf("nav-pilot %v started no client", args)
				}
				launches[c][m], exits[c][m] = append(launches[c][m], launch), append(exits[c][m], exit)
			}
		}
	}
	// With telemetry on, an opencode exit reads the session's usage through
	// opencode's CLI on purpose: one `session list` spawn (the fake lists no
	// sessions, so no export follows). That is a fixed cost, not a wait, and
	// on a loaded Mac one spawn alone can exceed the noise floor (#1957). So
	// the opencode exit may cost two measured spawns of the fake on top.
	spawns := make([]time.Duration, budgetRuns)
	for i := range spawns {
		cmd := exec.Command(filepath.Join(fake, "opencode"), "session", "list")
		cmd.Env = environ
		start := time.Now()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fake opencode session list: %v\n%s", err, out)
		}
		spawns[i] = time.Since(start)
	}
	spawn := median(spawns)
	t.Logf("one spawn of the fake opencode: %d ms (median of %d)", spawn.Milliseconds(), budgetRuns)
	for c, cl := range clients {
		t.Logf("nav-pilot %v:", cl.args)
		check("launch", cl.name+" launch", launches[c], 0)
		var extra time.Duration
		if cl.name == "opencode" {
			extra = 2 * spawn
		}
		check("exit", cl.name+" exit", exits[c], extra)
	}
	// What opencode does on top of the Copilot launch must fit in the launch
	// budget, with the same margin as the rest. Pairing cancels load that
	// lands on both runs, not load that scales with the work: opencode reads
	// the whole source at launch, so its extra time grows with CPU and disk
	// contention, and with several checks running it reached 190 ms. What this
	// catches is a wait: once seconds on macOS's automounter for /home (#1276).
	ocDiff := pairedDiff(launches[1][telemetryOff], launches[0][telemetryOff])
	ocCeil := budgets["launch"] * budgetMargin
	if ocDiff > ocCeil {
		t.Errorf("an opencode launch took %s more than a Copilot launch (median of %d paired runs; medians %s and %s): over %dx the %s launch budget. Something on its path waits (a lookup, a lock, the network) or does too much",
			ocDiff, budgetRuns, median(launches[1][telemetryOff]), median(launches[0][telemetryOff]), budgetMargin, budgets["launch"])
	}
	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" {
		var b strings.Builder
		b.WriteString("### `nav-pilot` Startup & Exit Latency (`TestLaunchBudget`)\n\n")
		b.WriteString("| Path | Median | Target (3x Ceiling) | Version-Check Δ | Telemetry Δ |\n")
		b.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
		for _, row := range summaryRows {
			b.WriteString(row + "\n")
		}
		fmt.Fprintf(&b, "\nOpenCode vs Copilot launch overhead (paired median): **%+d ms** (ceiling <= %d ms)\n\n",
			ocDiff.Milliseconds(), ocCeil.Milliseconds())
		if f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			_, _ = f.WriteString(b.String())
			_ = f.Close()
		}
	}
}
