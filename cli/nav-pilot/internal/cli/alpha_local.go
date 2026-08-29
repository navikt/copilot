package cli

// `nav-pilot alpha local` — running a model on the developer's own machine.
//
// Under `alpha` because it is not the supported path and should not read as
// one. Every command here is inert for a developer who has not run init: the
// group provisions and enables, and until it has, [local.IsLocal] answers false
// everywhere and no other command behaves differently.
//
// The five commands split along what each one costs. init spends an afternoon
// of bandwidth and says so first. start spends minutes loading weights and
// blocks until the server has answered a real completion, because a port bind
// proves nothing. status spends one probe. stop and off spend nothing.
//
// Nothing here runs sudo. Raising the wired-memory limit is the one privileged
// action in the neighbourhood and it stays a command the developer types:
// internal/local reports what is needed and what is set, this prints it.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// envDownloadGB is roughly what the Python environment costs to fetch: a
// managed 3.12 interpreter plus the mlx and mlx-lm wheels. Approximate on
// purpose — it is stated so a developer can decide whether to start this on a
// hotel connection, and the weights beside it are twenty times larger.
const envDownloadGB = 1

func alphaUsage() {
	fmt.Fprint(os.Stderr, `nav-pilot alpha — features that are not supported yet

Usage:
  nav-pilot alpha local <command>

Local inference — run a model on this machine instead of sending prompts to a
hosted one. Off until you run init, and invisible everywhere until then.

  init      Provision the environment and download the weights
  start     Start the server and wait until it answers a real completion
  stop      Stop the server
  status    Model, health, resident memory and the wired-memory limit
  off       Stop dispatching to it; the weights stay on disk
`)
}

// cmdAlpha dispatches the alpha groups. There is one.
func cmdAlpha(args []string) error {
	if len(args) == 0 || args[0] == "help" {
		alphaUsage()
		return nil
	}
	if args[0] != "local" {
		return fmt.Errorf("unknown alpha group: %s. Run %s for usage", args[0], bold("nav-pilot alpha help"))
	}
	sub := ""
	if len(args) > 1 {
		sub = args[1]
	}
	switch sub {
	case "init":
		return cmdLocalInit()
	case "start":
		return cmdLocalStart()
	case "stop":
		return cmdLocalStop()
	case "status":
		return cmdLocalStatus()
	case "off":
		return cmdLocalOff()
	case "", "help":
		alphaUsage()
		return nil
	default:
		if hint := suggest(sub, []string{"init", "start", "stop", "status", "off"}); hint != "" {
			return fmt.Errorf("unknown command: nav-pilot alpha local %s. Did you mean %s?", sub, hint)
		}
		return fmt.Errorf("unknown command: nav-pilot alpha local %s. Run %s for usage", sub, bold("nav-pilot alpha help"))
	}
}

// activeManifest resolves the served manifest and installs it as the one the
// predicates answer from. Only init and start call it: they act on the
// manifest, so they should have the freshest one. status and off answer
// questions about this machine and read [local.Active] instead, so neither
// waits on a network. The resolve error is advisory — it says which
// fallback was used — so it is printed and the manifest is used anyway. A nil
// manifest is the only fatal case, and it means the copy built into this binary
// is broken.
func activeManifest() (*local.Manifest, error) {
	m, src, err := local.Resolve()
	if m == nil {
		return nil, err
	}
	local.SetActive(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", yellow("⚠"), err)
	} else if src != local.SourceNetwork {
		fmt.Fprintf(os.Stderr, "%s Using the %s local-model manifest.\n", dim("ℹ"), src)
	}
	return m, nil
}

// localModel picks the model these commands act on: the configured one when it
// is a manifest entry, otherwise the manifest's default. Lookup rather than
// IsLocal — this runs before anything is enabled, which is exactly when init
// needs to read the manifest.
func localModel(m *local.Manifest) (local.Model, error) {
	if cfg, err := readConfig(); err == nil && cfg != nil && cfg.Model != nil {
		if entry, ok := local.Lookup(*cfg.Model); ok {
			return entry, nil
		}
	}
	for _, entry := range m.Models {
		if entry.Default {
			return entry, nil
		}
	}
	// Unreachable: Parse refuses a manifest without exactly one default.
	return local.Model{}, errors.New("the local-model manifest names no default model")
}

// ─── init ────────────────────────────────────────────────────────────────────

func cmdLocalInit() error {
	ctx := context.Background()
	m, err := activeManifest()
	if err != nil {
		return err
	}
	model, err := localModel(m)
	if err != nil {
		return err
	}

	// Before anything is downloaded: a machine that cannot run the model must
	// not spend an afternoon of bandwidth finding out.
	wired, err := local.CheckWiredLimit(model)
	if err != nil {
		return err
	}

	present, err := local.WeightsPresent(model.Model)
	if err != nil {
		return err
	}

	fmt.Printf("%s  %s\n\n", bold("nav-pilot alpha local init"), dim("(alpha — unsupported)"))
	fmt.Printf("  Model    %s\n", bold(model.Name))
	fmt.Printf("           %s\n", dim(model.Model))
	if model.Role != "" {
		fmt.Printf("  For      %s\n", model.Role)
	}
	if model.Expect != "" {
		fmt.Printf("  Expect   %s\n", wrapIndent(model.Expect, "           ", 78))
	}
	fmt.Printf("  Needs    %d GB RAM, a %d GB wired-memory limit (this machine has %d GB)\n",
		model.MinRAMGB, wired.RequiredGB, wired.MachineRAMGB)

	download := envDownloadGB
	if !present {
		download += model.WeightsGB
	}
	switch {
	case present && local.Installed():
		fmt.Printf("\n  %s Environment and weights are already here — nothing to download.\n", green("✓"))
		download = 0
	case present:
		fmt.Printf("\n  %s Weights are already here. Download: about %d GB (the Python environment).\n",
			green("✓"), download)
	default:
		fmt.Printf("\n  %s Download: about %d GB (%d GB of weights plus the Python environment).\n",
			yellow("⚠"), download, model.WeightsGB)
		fmt.Printf("    %s\n", dim("The weights need huggingface.co, cas-server.xethub.hf.co and transfer.xethub.hf.co."))
		fmt.Printf("    %s\n", dim("Behind a TLS-inspecting proxy the first works and the other two hang at 0%."))
	}
	fmt.Println()

	// The kernel's answer, not the file mode's. isInteractive reads
	// os.ModeCharDevice, which is set for /dev/null too — so a scripted init
	// put its question to /dev/null, got an error back, printed "Cancelled" and
	// exited 0. The caller was told nothing, and carried on against an
	// environment that had never been provisioned.
	if download > 0 && providerpkg.IsTerminal(os.Stdin) {
		if err := confirmDownload(download, func() (bool, error) {
			var proceed bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Download about %d GB and provision the local-inference environment?", download)).
				Value(&proceed).
				WithTheme(navTheme()).
				Run()
			return proceed, err
		}); err != nil {
			return err
		}
	}

	fmt.Printf("%s Provisioning the Python environment…\n", dim("→"))
	if err := local.EnsureEnv(ctx); err != nil {
		return err
	}
	fmt.Printf("%s Environment ready.\n", green("✓"))

	if present {
		fmt.Printf("%s Weights already downloaded.\n", green("✓"))
	} else {
		fmt.Printf("%s Downloading weights…\n", dim("→"))
		if err := local.DownloadWeights(ctx, model.Model, progressLine); err != nil {
			fmt.Println()
			return err
		}
		fmt.Printf("\r%s\r%s Weights downloaded.\n", strings.Repeat(" ", 78), green("✓"))
	}

	if _, err := writeConfigKey("local_enabled", "true"); err != nil {
		return err
	}
	fmt.Printf("%s Local inference enabled.\n\n", green("✓"))

	// Deliberately not set for you. init provisions and enables; which model a
	// launch uses and which client runs it are the developer's settings, and
	// silently rewriting them is how someone ends up wondering why every
	// session got slower.
	fmt.Println("  Next:")
	if !wired.Sufficient {
		fmt.Printf("    %s\n", bold(wired.Command))
		fmt.Printf("      %s\n", dim("raises the wired-memory limit; it resets at reboot"))
	}
	if cfg, err := readConfig(); err == nil && (cfg == nil || cfg.Client == nil || *cfg.Client != "opencode") {
		fmt.Printf("    %s\n", bold("nav-pilot config set client opencode"))
		fmt.Printf("      %s\n", dim("local models run under opencode; the Copilot CLI resolves models through GitHub"))
	}
	fmt.Printf("    %s\n", bold("nav-pilot config set model "+model.Model))
	fmt.Printf("    %s\n", bold("nav-pilot alpha local start"))
	fmt.Println()
	return nil
}

// confirmDownload reports a refusal as an error rather than as a clean exit.
// init used to print "Cancelled. Nothing was downloaded." and return nil, so a
// script that ran init and carried on carried on regardless — and every command
// after it behaved as though an environment existed that did not.
//
// ask is a parameter so a test can be the developer who says no, without a
// terminal to say it from.
func confirmDownload(gb int, ask func() (bool, error)) error {
	proceed, err := ask()
	if err != nil || !proceed {
		return fmt.Errorf("cancelled — the %d GB was not downloaded and local inference was not enabled", gb)
	}
	return nil
}

// progressLine redraws one line of downloader output in place. The downloader
// already redraws with \r, so this only has to keep it inside one terminal
// line.
func progressLine(line string) {
	const width = 78
	if len(line) > width {
		line = line[:width]
	}
	fmt.Printf("\r  %-*s", width, line)
}

// wrapIndent breaks prose onto continuation lines under a fixed indent. The
// manifest's Expect field is a paragraph, and a paragraph printed as one line
// is a paragraph nobody reads.
func wrapIndent(s, indent string, width int) string {
	var out strings.Builder
	col := 0
	for i, word := range strings.Fields(s) {
		if i > 0 {
			if col+1+len(word) > width {
				out.WriteString("\n" + indent)
				col = 0
			} else {
				out.WriteString(" ")
				col++
			}
		}
		out.WriteString(word)
		col += len(word)
	}
	return out.String()
}

// ─── start ───────────────────────────────────────────────────────────────────

func cmdLocalStart() error {
	ctx := context.Background()
	if !local.Installed() {
		return fmt.Errorf("the local-inference environment is not provisioned — run %s first", bold("nav-pilot alpha local init"))
	}
	m, err := activeManifest()
	if err != nil {
		return err
	}
	model, err := localModel(m)
	if err != nil {
		return err
	}

	// An already-running server is reported, not replaced. Two mlx-lm processes
	// on one machine is two copies of the weights resident at once, which on a
	// machine sized for one is the memory failure this feature is trying to
	// stay under.
	if st, ok, err := local.LoadState(); err != nil {
		return err
	} else if ok {
		if local.Attach(st).Status().Health != local.HealthCrashed {
			fmt.Printf("%s A local server is already running: %s (pid %d, %s)\n",
				green("✓"), bold(st.Model), st.PID, local.ServerURL())
			return nil
		}
		if err := local.ClearState(); err != nil {
			return err
		}
	}

	wired, err := local.CheckWiredLimit(model)
	if err != nil {
		return err
	}
	if !wired.Sufficient {
		// Not raised here, and not warned past either. The measurement is that
		// a model over the cap is refused by its own server before it produces
		// a token, and that a cap raised too far takes the compositor with it —
		// so the number belongs to a command the developer types.
		return fmt.Errorf(
			"%s needs a %d GB wired-memory limit; this machine has %s.\n\n  Raise it, then start again (it resets at reboot):\n\n    %s",
			model.Model, wired.RequiredGB, currentWiredLabel(wired), bold(wired.Command))
	}

	fmt.Printf("%s Starting %s…\n", dim("→"), bold(model.Name))
	fmt.Printf("  %s\n", dim("Ready means it answered a real completion, not that the port is open — minutes on a cold cache."))
	started := timeNow()
	srv := &local.Server{Port: local.DefaultPort}
	if err := srv.Start(ctx, model); err != nil {
		// The process may be up but not answering; do not leave it behind.
		_ = srv.Stop()
		return err
	}
	// Record only what actually came up. A pid written for a server that is
	// not ready is a pid `status` reports as crashed hours later, which reads
	// as "it died" rather than "it never started".
	status := srv.Status()
	if status.Health != local.HealthReady || status.PID <= 0 {
		_ = srv.Stop()
		return fmt.Errorf("the local %s server did not come up (%s); nothing was recorded", model.Model, status.Health)
	}
	if err := local.SaveState(local.State{
		PID:     status.PID,
		Model:   model.Model,
		Port:    status.Port,
		Started: started,
	}); err != nil {
		return err
	}

	// Register it with opencode. Non-fatal: the server is up either way, and a
	// developer who keeps their own opencode config should be told rather than
	// have their session refused.
	if err := providerpkg.EnsureOpenCodeLocalProvider(model); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not register the local model with opencode: %v\n", yellow("⚠"), err)
	}

	fmt.Print(startSummary(model, srv.URL(), status.PID, wired, timeNow().Sub(started)))
	return nil
}

// startSummary is what start reports once the server is up.
//
// It builds a string rather than printing as it goes so a test can hold it to
// the rule it used to break: every address named here is one a client can
// connect to right now. It named the loop guard's port, and the guard is
// started by the launch — an in-process listener that lives exactly as long as
// the client session, with no daemon to keep it up between commands. So the
// address was never live when start printed it, and a developer who pasted it
// into curl reached nothing, or worse, went to the unguarded server instead.
//
// The guard is still named, because a developer needs to know a turn can be
// ended for them. It is named as something the launch does, which is what it
// is.
func startSummary(model local.Model, serverURL string, pid int, wired local.WiredLimit, took time.Duration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s %s is ready after %s.\n", green("✓"), bold(model.Name), took.Round(time.Second))
	fmt.Fprintf(&b, "  Serving  %s (pid %d)\n", serverURL, pid)
	fmt.Fprintf(&b, "  Guard    %s\n", dim(wrapIndent(fmt.Sprintf(
		"started by the launch below, not by this command: it ends a turn after %d identical tool calls in a row, and a client pointed straight at the address above goes unguarded",
		local.LoopGuardRepeat()), "           ", 78)))
	fmt.Fprintf(&b, "  Wired    %d GB required, %d GB set\n\n", wired.RequiredGB, wired.CurrentGB)
	fmt.Fprintf(&b, "  Launch:  %s\n", bold("nav-pilot --client opencode --model "+model.Model))
	fmt.Fprintf(&b, "  Stop:    %s\n\n", bold("nav-pilot alpha local stop"))
	return b.String()
}

// currentWiredLabel names what the cap is now. Unset is the macOS default, not
// zero, and saying "0 GB" would read as a broken machine.
func currentWiredLabel(w local.WiredLimit) string {
	if w.CurrentGB == 0 {
		return "no limit set, so the macOS default (roughly 75% of RAM) applies"
	}
	return fmt.Sprintf("%d GB", w.CurrentGB)
}

// ─── stop ────────────────────────────────────────────────────────────────────

func cmdLocalStop() error {
	st, ok, err := local.LoadState()
	if err != nil {
		return err
	}
	if !ok {
		fmt.Printf("%s No local server is recorded as running.\n", dim("ℹ"))
		return nil
	}
	srv := local.Attach(st)
	if srv.Status().Health == local.HealthCrashed {
		fmt.Printf("%s The recorded server (pid %d) is already gone.\n", yellow("⚠"), st.PID)
	} else if err := srv.Stop(); err != nil {
		return err
	} else {
		fmt.Printf("%s Stopped %s (pid %d).\n", green("✓"), bold(st.Model), st.PID)
	}
	return local.ClearState()
}

// ─── status ──────────────────────────────────────────────────────────────────

func cmdLocalStatus() error {
	ctx := context.Background()
	fmt.Printf("%s  %s\n\n", bold("nav-pilot alpha local status"), dim("(alpha — unsupported)"))

	cfg, _ := readConfig()
	enabled := cfg != nil && cfg.LocalEnabled != nil && *cfg.LocalEnabled
	fmt.Printf("  Environment  %s\n", installedLabel())
	fmt.Printf("  Dispatch     %s\n", enabledLabel(enabled))

	st, ok, err := local.LoadState()
	if err != nil {
		return err
	}
	if !ok {
		fmt.Printf("  Server       %s %s\n\n", local.HealthNotStarted, dim("(nothing recorded)"))
		fmt.Printf("  Start it: %s\n\n", bold("nav-pilot alpha local start"))
		return nil
	}

	srv := local.Attach(st)
	health := srv.Health(ctx)
	fmt.Printf("  Model        %s\n", bold(st.Model))
	fmt.Printf("  Server       %s %s\n", healthColour(health), dim("— "+healthMeaning(health)))
	fmt.Printf("  Process      pid %d, up %s, listening on %s\n",
		st.PID, timeNow().Sub(st.Started).Round(time.Second), local.ServerURL())
	if rss := local.ResidentMemoryMB(ctx, st.PID); rss > 0 {
		fmt.Printf("  Resident     %.1f GB\n", float64(rss)/1024)
	} else {
		fmt.Printf("  Resident     %s\n", dim("unreadable"))
	}

	// The wired limit is per model and measured, so it is only meaningful for a
	// model still in the manifest. Read from the manifest already in hand, not
	// a fresh one: status answers a question about this machine, and it should
	// not wait on a network to do it.
	if model, found := local.Lookup(st.Model); found {
		if wired, werr := local.CheckWiredLimit(model); werr == nil {
			fmt.Printf("  Wired limit  %d GB required, %s\n", wired.RequiredGB, currentWiredLabel(wired))
			if !wired.Sufficient {
				fmt.Printf("               %s %s\n", yellow("⚠"), bold(wired.Command))
			}
		}
	}

	if s := srv.Status(); s.ZeroTokenReplies > 0 {
		// Counted because it climbs before the signal death, so it is worth
		// acting on before the crash rather than after.
		fmt.Printf("  Degraded     %d empty completions so far — restart before it dies mid-request\n", s.ZeroTokenReplies)
	}
	fmt.Println()
	return nil
}

func installedLabel() string {
	if local.Installed() {
		return green("provisioned")
	}
	return dim("not provisioned — run 'nav-pilot alpha local init'")
}

func enabledLabel(on bool) string {
	if on {
		return green("enabled")
	}
	return dim("off — local models are hidden and never launched")
}

func healthColour(h local.Health) string {
	switch h {
	case local.HealthReady:
		return green(string(h))
	case local.HealthCrashed, local.HealthHung:
		return red(string(h))
	default:
		return yellow(string(h))
	}
}

// healthMeaning says what to do, because the five states differ in exactly that
// and a colour does not carry it.
func healthMeaning(h local.Health) string {
	switch h {
	case local.HealthNotStarted:
		return "nothing is running"
	case local.HealthStarting:
		return "alive, still mapping weights; the port opens before the model is loaded"
	case local.HealthReady:
		return "answered a real completion"
	case local.HealthCrashed:
		return "the process is gone; start it again"
	case local.HealthHung:
		return "alive and accepting connections but not answering; it will not recover, restart it"
	}
	return ""
}

// ─── off ─────────────────────────────────────────────────────────────────────

func cmdLocalOff() error {
	if _, err := writeConfigKey("local_enabled", "false"); err != nil {
		return err
	}
	fmt.Printf("%s Local dispatch is off. Launches go to the hosted model again.\n", green("✓"))

	// A configured local model with dispatch off would be sent to a hosted
	// provider as a model id it has never heard of, which fails somewhere
	// downstream with an error about something else. Reset it here, against the
	// manifest already in hand: turning local off must work on a train.
	if cfg, err := readConfig(); err == nil && cfg != nil && cfg.Model != nil {
		if _, isLocalModel := local.Lookup(*cfg.Model); isLocalModel {
			if _, err := writeConfigKey("model", "auto"); err != nil {
				return err
			}
			fmt.Printf("%s model was %s, which only exists locally — set back to %s.\n",
				green("✓"), *cfg.Model, bold("auto"))
		}
	}

	// And out of opencode's config. nav-pilot's own config only decides what
	// nav-pilot launches; the provider block start wrote lives in a file
	// opencode reads by itself, so leaving it there leaves the model selectable
	// and pointed at the guard's port — which after `off` is whatever is
	// listening on it. start writes it back.
	if err := providerpkg.RemoveOpenCodeLocalProvider(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not remove the local model from opencode: %v\n", yellow("⚠"), err)
	}

	if st, ok, _ := local.LoadState(); ok && local.Attach(st).Status().Health != local.HealthCrashed {
		fmt.Printf("%s The server is still running (pid %d) — %s to free the memory.\n",
			yellow("⚠"), st.PID, bold("nav-pilot alpha local stop"))
	}
	fmt.Printf("%s Weights are left on disk. %s brings it back without downloading them again.\n\n",
		dim("ℹ"), bold("nav-pilot alpha local init"))
	return nil
}

// ─── config ──────────────────────────────────────────────────────────────────

// localLoopGuard is the configured loop-guard threshold, or the built-in
// default. Unset and out-of-range both mean the default: validateConfig refuses
// a value below 2, so this only has to answer for a config that was never set.
func localLoopGuard(r ResolvedConfig) int {
	if r.LocalLoopGuard >= 2 {
		return r.LocalLoopGuard
	}
	return local.DefaultLoopGuardRepeat
}

// applyLocalConfig arms local dispatch for this process, and is the only place
// that does. Both halves have to hold: the developer enabled it, and the
// environment is actually on disk. Either missing means [local.IsLocal] stays
// false and nothing anywhere behaves differently — which is the promise made to
// everyone who never runs the alpha.
func applyLocalConfig() {
	cfg, err := readConfig()
	if err != nil {
		return
	}
	r := resolve(cfg, CLIOverrides{})
	local.SetLoopGuardRepeat(localLoopGuard(r))
	if !r.LocalEnabled || !local.Installed() {
		return
	}
	// Cached, never Resolve: this runs on every nav-pilot invocation, including
	// `config get`, and the package doc's "never blocking" is a promise about
	// exactly that. A fetch here put a connect timeout on the front of every
	// command for anyone behind a captive portal. init and start act on the
	// manifest and pay for a fresh one; nothing else does.
	if m, _, _ := local.Cached(); m != nil {
		local.SetActive(m)
	}
	local.SetEnabled(true)
}
