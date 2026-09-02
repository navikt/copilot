package cli

// `nav-pilot alpha local` — running a model on the developer's own machine.
//
// Under `alpha` because it is not the supported path and should not read as
// one. Every command here is inert for a developer who has not run init: the
// group provisions and enables, and until it has, [local.IsLocal] answers false
// everywhere and no other command behaves differently.
//
// The five commands split along what each one costs. init spends an afternoon
// of bandwidth and says so first. start loads the weights and blocks until the
// server has answered a real completion, because a port bind proves nothing.
// status spends one probe. stop and off spend nothing.
//
// Nothing here runs sudo. Raising the wired-memory limit is the one privileged
// action in the neighbourhood and it stays a command the developer types:
// internal/local reports what is needed and what is set, this prints it.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
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
	fmt.Fprint(os.Stderr, `nav-pilot alpha: features that are not supported yet

Usage:
  nav-pilot alpha local <command>

Local inference. Run a model on this machine instead of sending prompts to a
hosted one. Off until you run init, and invisible everywhere until then.

  init      Set it all up: environment, weights, memory limit, and a running server
  start     Start the server and wait until it answers a real completion
  stop      Stop the server
  status    Model, health, resident memory, the wired-memory limit and what it has done
  ask       Put one question straight to the local model: ask -p "..." (or pipe stdin)
  on        Dispatch to it again after off, without downloading anything
  off       Stop dispatching to it; the weights stay on disk
  purge     Remove the environment and the weights, after showing what and how big

Switching model:
  nav-pilot models                                  what is offered; local ones say (local)
  nav-pilot config set local_model <id>             pick one
  nav-pilot alpha local init                        download its weights, then start

The list refreshes on init and start, not on every command.
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
	case "on":
		return cmdLocalOn()
	case "off":
		return cmdLocalOff()
	case "ask":
		return cmdLocalAsk(args[2:])
	case "purge":
		return cmdLocalPurge(args[1:])
	case "", "help":
		alphaUsage()
		return nil
	default:
		if hint := suggest(sub, []string{"init", "start", "stop", "status", "ask", "on", "off", "purge"}); hint != "" {
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
	configured := ""
	if cfg, err := readConfig(); err == nil && cfg != nil && cfg.LocalModel != nil {
		configured = strings.TrimSpace(*cfg.LocalModel)
		local.SetSelectedModel(configured)
	}
	if entry, ok := local.Chosen(m); ok {
		if configured != "" && entry.Model != configured {
			fmt.Fprintf(os.Stderr, "%s local_model is %s, which this manifest does not offer. Using the default %s instead.\n",
				yellow("⚠"), bold(configured), bold(entry.Model))
		}
		return entry, nil
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

	fmt.Printf("%s  %s\n\n", bold("nav-pilot alpha local init"), dim("(alpha, unsupported)"))
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
		fmt.Printf("\n  %s Environment and weights are already here. Nothing to download.\n", green("✓"))
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

	// Said before the confirmation, and before 26 GB is spent, because on the
	// default client this is the whole shape of what the developer is buying.
	// It used to print after the download and after local inference was already
	// enabled, which told people what they had only once they had it.
	//
	// The client is reported, not set. Writing it looks like automation and is a
	// trap: config validation requires an opencode model in provider/model form,
	// so setting the client on a config whose model is a bare cloud id makes
	// every launch fail validation until someone edits the file by hand. The
	// launch converts bare ids on its own, so nothing needs writing here.
	if cfg, err := readConfig(); err == nil && cfg != nil && cfg.Client != nil && *cfg.Client == "copilot" {
		fmt.Printf("  %s\n", bold("Your client is the Copilot CLI, which has no sub-agent dispatch:"))
		fmt.Printf("    %s\n", dim("a local session runs entirely on the local model, or not at all."))
		fmt.Printf("    %s\n", dim("The Copilot CLI takes one model provider per process, so a cloud"))
		fmt.Printf("    %s\n", dim("main agent cannot hand scoped tasks to a local worker there."))
		fmt.Printf("  %s\n", dim("For a cloud agent with a local worker, switch clients first:"))
		fmt.Printf("    %s\n\n", dim("nav-pilot config set client opencode"))
	}

	// Said before the confirmation, not after it, and before anything is
	// downloaded. Collecting more than usual during an alpha is defensible only
	// if the developer is told what "more" means while they can still decline.
	fmt.Printf("  %s\n", bold("While this is alpha we measure it more closely than the rest of nav-pilot:"))
	fmt.Printf("    %s\n", dim("how many tasks each session hands to the local model, including none"))
	fmt.Printf("    %s\n", dim("which local model, how long it took to start, and when the server hangs"))
	fmt.Printf("  %s\n", dim("Never your prompts, your code, your file names or the model's output."))
	fmt.Printf("  %s\n", dim("DO_NOT_TRACK=1 turns all of it off, local included, as does"))
	fmt.Printf("  %s\n\n", dim("NAV_PILOT_TELEMETRY_ENABLED=false if you would rather set it per tool."))

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
	if err := local.SupportedPlatform(); err != nil {
		return err
	}
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

	// Setup is one command or it is not automated. The rest of this used to be a
	// list of four more for the developer to paste, which is a worse experience
	// than it looks: each one is a place to stop, and the sysctl in particular
	// arrived after a 25 GB download with no explanation of why it was needed.
	//
	// Everything below is announced as it happens rather than done silently, and
	// every part of it is reversible with `alpha local off` or `purge`.
	if !wired.Sufficient {
		fmt.Printf("%s Raising the wired-memory limit to %d GB (sudo; it resets at reboot)…\n",
			dim("→"), wired.RequiredGB)
		if err := local.RaiseWiredLimit(ctx, wired); err != nil {
			return err
		}
		fmt.Printf("%s Wired-memory limit raised.\n", green("✓"))
	}

	fmt.Printf("%s Starting the server (measured starts have been under a minute)…\n", dim("→"))
	if err := cmdLocalStart(); err != nil {
		return err
	}

	fmt.Printf("\n  %s\n", bold("Ready. Run nav-pilot in a repository and it will use the local worker."))
	fmt.Printf("  %s\n\n", dim("nav-pilot alpha local status  ·  nav-pilot alpha local off  ·  nav-pilot alpha local purge"))
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
		return fmt.Errorf("cancelled. The %d GB was not downloaded and local inference was not enabled", gb)
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
	// Interrupt has to reach the child. A start that wedges sits until the ten
	// minute readyTimeout, so an impatient Ctrl-C is the normal case, and
	// without this it killed nav-pilot while the server carried on loading: 21 GB
	// of resident memory holding a port, with no state file written yet, so
	// `stop` and `status` both reported nothing recorded. Cancelling here makes
	// Start return, and the cleanup below was already there waiting for it.
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	if !local.Installed() {
		return fmt.Errorf("the local-inference environment is not provisioned. Run %s first", bold("nav-pilot alpha local init"))
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

	// mlx-lm fetches a model it does not find, so without this `start` on a
	// machine that never ran init begins a 23 GB download with nothing on
	// screen saying so, and either finishes inside readyTimeout as a start that
	// looks pathologically slow or dies at ten minutes naming neither cause.
	// The autostart path has always refused this; `start` only claimed to.
	if present, err := local.WeightsPresent(model.Model); err != nil {
		return err
	} else if !present {
		return fmt.Errorf("the weights for %s are not on this machine.\n\n  Download them first:\n\n    %s",
			model.Model, bold("nav-pilot alpha local init"))
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
	fmt.Printf("  %s\n", dim("Ready means it answered a real completion, not that the port is open."))
	started := timeNow()
	srv := &local.Server{} // Port 0: Start asks the kernel for a free one.
	if err := srv.Start(ctx, model); err != nil {
		// The process may be up but not answering; do not leave it behind. This
		// is also the interrupt path: a cancelled context lands here.
		_ = srv.Stop()
		// Recorded here as well as on the way out. Measuring only the starts
		// that came up hides exactly the ones worth seeing: a start that runs
		// into readyTimeout is the slow tail, and leaving it out turned the
		// histogram into a distribution of the starts that worked, which is
		// how the docs came to quote a startup time the fleet contradicts.
		outcome := "failed"
		if ctx.Err() != nil {
			outcome = "interrupted"
		}
		telemetry.RecordLocalReadySeconds(model.Model, outcome, int64(timeNow().Sub(started).Seconds()))
		if ctx.Err() != nil {
			return fmt.Errorf("interrupted before the server was ready; it has been stopped")
		}
		return err
	}
	// Record only what actually came up. A pid written for a server that is
	// not ready is a pid `status` reports as crashed hours later, which reads
	// as "it died" rather than "it never started".
	status := srv.Status()
	// One closed vocabulary for the outcome: ready, failed, interrupted. This
	// is now the only record of how a start ended — nav_pilot_local_server_total
	// carried the same event from the same call site and was removed, since its
	// one distinctive value, `hung`, could never arrive here: Status() cannot
	// return it, only Health(ctx) produces it, and that is never fed in.
	readyOutcome := "ready"
	if status.Health != local.HealthReady || status.PID <= 0 {
		readyOutcome = "failed"
	}
	telemetry.RecordLocalReadySeconds(model.Model, readyOutcome, int64(timeNow().Sub(started).Seconds()))
	if status.Health != local.HealthReady || status.PID <= 0 {
		_ = srv.Stop()
		return fmt.Errorf("the local %s server did not come up (%s); nothing was recorded.\n\n  What it printed is in %s", model.Model, status.Health, local.LogPath())
	}
	if err := local.SaveState(local.State{
		PID:     status.PID,
		Model:   model.Model,
		Started: started,
		// The port the kernel handed this server. Every other process that
		// wants to reach it, the loop guard included, reads it from here.
		Port: srv.Port,
	}); err != nil {
		return err
	}

	// The opencode provider block is not written here any more. It carries the
	// loop guard's address, and the guard now takes a port per session, so a
	// block written at start time would name a port no guard is listening on by
	// the time anyone launches. Every launch writes it, which is where the live
	// address is known.

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
		"started by the launch below, not by this command. It ends a turn after %d identical tool calls in a row, and a client pointed straight at the address above goes unguarded",
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
	fmt.Printf("%s  %s\n\n", bold("nav-pilot alpha local status"), dim("(alpha, unsupported)"))

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
	if stats, err := local.ReadStats(); err == nil && stats.Requests > 0 {
		defer printLocalStats(stats)
	}
	fmt.Printf("  Model        %s\n", bold(st.Model))
	fmt.Printf("  Server       %s, %s\n", healthColour(health), dim(healthMeaning(health)))
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
		fmt.Printf("  Degraded     %d empty completions so far. Restart before it dies mid-request.\n", s.ZeroTokenReplies)
	}
	fmt.Println()
	return nil
}

func installedLabel() string {
	if local.Installed() {
		return green("provisioned")
	}
	return dim("not provisioned. Run 'nav-pilot alpha local init'")
}

func enabledLabel(on bool) string {
	if on {
		return green("enabled")
	}
	return dim("off. Local models are hidden and never launched")
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
		// The log path belongs here and not in a footnote: this is the one
		// state where the developer has nothing else to go on, and a crash
		// report that quotes only "the process is gone" is a report nobody can
		// act on.
		return "the process is gone; start it again. What it printed is in " + local.LogPath()
	case local.HealthHung:
		return "alive and accepting connections but not answering; it will not recover, restart it"
	}
	return ""
}

// ─── off ─────────────────────────────────────────────────────────────────────

// cmdLocalOn is the other half of off.
//
// `init` already brought dispatch back, and said so, but a developer who has just
// run `off` reaches for `on` and had to be told that provisioning is how you
// re-enable. Reprovisioning is a no-op on a machine that already has everything,
// so this was only ever a naming problem, and the fix is the name.
func cmdLocalOn() error {
	if !local.Installed() {
		return fmt.Errorf("local inference is not provisioned on this machine. Run %s first",
			bold("nav-pilot alpha local init"))
	}
	if _, err := writeConfigKey("local_enabled", "true"); err != nil {
		return err
	}
	fmt.Printf("%s Local dispatch is on.\n", green("✓"))

	if st, ok, _ := local.LoadState(); ok && local.Attach(st).Status().Health == local.HealthReady {
		fmt.Printf("%s The server is already up (pid %d).\n", dim("ℹ"), st.PID)
		return nil
	}
	fmt.Printf("%s Start the server when you need it: %s\n", dim("ℹ"), bold("nav-pilot alpha local start"))
	return nil
}

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
			fmt.Printf("%s model was %s, which only exists locally. Set back to %s.\n",
				green("✓"), *cfg.Model, bold("auto"))
		}
	}

	// And out of opencode's config — the claim first, then the thing it rests
	// on. nav-pilot's own config only decides what nav-pilot launches; both of
	// these live in a file opencode reads by itself, and they are two separate
	// writes to it. The dispatch policy tells every session that the worker
	// draws no AI credits, and only the binding taken out below makes
	// that true, so a crash between the two writes must not be able to leave
	// the fragment registered with the worker unbound: that is a session still
	// dispatching to a "free" worker that now bills every task to the session
	// model. It is the state the launch's fused write exists to rule out, and
	// unwinding in this order rules it out on the way back down. The reverse
	// leftover — a binding with nothing pointing at it — costs nothing.
	if err := providerpkg.RemoveOpenCodeLocalPolicy(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not remove the local dispatch policy from opencode: %v\n", yellow("⚠"), err)
	}

	// Then the provider block and the worker's binding that start wrote. Left
	// there, the model stays selectable and pointed at the guard's port — which
	// after `off` is whatever is listening on it. start writes it back.
	if err := providerpkg.RemoveOpenCodeLocalProvider(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not remove the local model from opencode: %v\n", yellow("⚠"), err)
	}

	if st, ok, _ := local.LoadState(); ok && local.Attach(st).Status().Health != local.HealthCrashed {
		fmt.Printf("%s The server is still running (pid %d). Run %s to free the memory.\n",
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
		// Said out loud, because the two halves come apart on their own: the
		// stamp pins exact mlx and mlx-lm versions, so a nav-pilot upgrade that
		// bumps a pin turns Installed() false while the config still says yes.
		// Silence there sends a local model id down the hosted path, where it
		// fails with an error about something else entirely.
		if r.LocalEnabled {
			fmt.Fprintf(os.Stderr, "%s Local dispatch is on but the environment is not provisioned to the versions this nav-pilot pins. Local models are hidden and launches go hosted. Run %s.\n",
				yellow("⚠"), bold("nav-pilot alpha local init"))
		}
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
	local.SetAutostart(r.LocalAutostart)
	if cfg.LocalModel != nil {
		local.SetSelectedModel(strings.TrimSpace(*cfg.LocalModel))
	}
}

// ─── purge ───────────────────────────────────────────────────────────────────

// cmdLocalPurge removes what init put on this machine, after saying what it is.
//
// It exists because `off` deliberately leaves the weights: turning dispatch off
// and back on should not cost a 23 GB download. That is right for a developer
// pausing and wrong for one leaving, who was otherwise left to find the Hugging
// Face cache layout themselves and delete it by hand.
//
// Lists first and deletes only with --yes, because the weights live in a shared
// cache that another MLX tool on the machine may be using.
func cmdLocalPurge(args []string) error {
	confirmed := slices.Contains(args, "--yes")

	if st, ok, _ := local.LoadState(); ok && local.Attach(st).Status().Health != local.HealthCrashed {
		return fmt.Errorf("the local server is still running (pid %d).\n\n  Stop it first:\n\n    %s",
			st.PID, bold("nav-pilot alpha local stop"))
	}

	// The recorded server names the model it loaded; the manifest names what
	// this machine would load next. Either identifies the weights to remove, and
	// a machine with neither has none to remove.
	var model string
	if st, ok, _ := local.LoadState(); ok {
		model = st.Model
	} else if m, _, err := local.Cached(); err == nil && m != nil {
		if chosen, ok := local.Chosen(m); ok {
			model = chosen.Model
		}
	}
	items := local.Removables(model)
	if len(items) == 0 {
		fmt.Printf("%s Nothing to remove. Local inference is not provisioned on this machine.\n", dim("ℹ"))
		return nil
	}

	var total int64
	for _, it := range items {
		total += it.Bytes
		fmt.Printf("  %s  %s\n      %s\n", bold(humanBytes(it.Bytes)), it.Path, dim(it.What))
	}
	fmt.Printf("\n  %s in total.\n\n", bold(humanBytes(total)))

	if !confirmed {
		fmt.Printf("%s Nothing was deleted. To go ahead:\n\n    %s\n\n",
			dim("ℹ"), bold("nav-pilot alpha local purge --yes"))
		return nil
	}

	if err := cmdLocalOff(); err != nil {
		return err
	}
	for _, it := range items {
		if err := os.RemoveAll(it.Path); err != nil {
			return fmt.Errorf("removing %s: %w", it.Path, err)
		}
	}
	fmt.Printf("%s Removed %s. %s puts it back.\n",
		green("✓"), bold(humanBytes(total)), bold("nav-pilot alpha local init"))
	return nil
}

// humanBytes is for a developer deciding whether to reclaim the space, so it is
// the unit their disk is measured in rather than an exact count.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%d MB", n/(1<<20))
	default:
		return fmt.Sprintf("%d kB", n/(1<<10))
	}
}

// printLocalStats shows what the local model has actually done on this
// machine. It is the developer's own number rather than the fleet's: how many
// requests it took, how many tokens it generated at no cost, and how long it
// spent.
//
// Requests without a usage block are named rather than hidden. A streaming
// client that never asks for stream_options.include_usage reports no tokens,
// and a token total that silently excluded those runs would read as "the model
// has barely done anything" when it had done all of it.
func printLocalStats(s local.Stats) {
	fmt.Printf("\n  %s\n", bold("On this machine"))
	fmt.Printf("  Requests     %d", s.Requests)
	if !s.Since.IsZero() {
		fmt.Printf(" %s", dim("since "+s.Since.Format("2 Jan 15:04")))
	}
	fmt.Println()
	if s.TokensOut > 0 || s.TokensIn > 0 {
		fmt.Printf("  Tokens       %s in, %s out, %s\n",
			thousands(s.TokensIn), thousands(s.TokensOut), dim("none of them billed"))
	}
	if s.Seconds > 0 {
		fmt.Printf("  Time         %s\n", dim(compactDuration(s.Seconds)))
	}
	if s.WithoutUsage > 0 {
		fmt.Printf("  %s\n", dim(fmt.Sprintf("%d of those reported no token count, so the totals are a floor",
			s.WithoutUsage)))
	}
}

func thousands(n int64) string {
	out := fmt.Sprintf("%d", n)
	for i := len(out) - 3; i > 0; i -= 3 {
		out = out[:i] + " " + out[i:]
	}
	return out
}

func compactDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second)).Round(time.Second)
	if d < time.Minute {
		return d.String()
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
