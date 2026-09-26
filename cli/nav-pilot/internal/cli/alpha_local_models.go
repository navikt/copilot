package cli

// `nav-pilot alpha local models` and `use`: what the manifest offers this
// machine, and picking one. Both read the cached manifest, like status: they
// answer a question about this machine and should not wait on a network.
// Neither downloads nor restarts anything on its own.

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

func cachedManifest() (*local.Manifest, error) {
	m, _, _ := local.Cached()
	if m == nil {
		return nil, errors.New("no local-model manifest is available; the copy built into this nav-pilot is broken")
	}
	return m, nil
}

// runningModel is the model a live recorded server serves, or "".
func runningModel() string {
	if st, ok, _ := local.LoadState(); ok && local.Attach(st).Status().Health != local.HealthCrashed {
		return st.Model
	}
	return ""
}

func cmdLocalModels() error {
	m, err := cachedManifest()
	if err != nil {
		return err
	}
	printLocalModels(m)
	nudge(local.ReplacedNotice(m))
	nudge(local.PinnedAdvisory(m))
	return nil
}

// printLocalModels is the table models prints and use falls back to. `*` marks
// what a start would load, which is not always what local_model says: see
// [localSelection].
func printLocalModels(m *local.Manifest) {
	active, _, _, _ := localSelection(m)
	running := runningModel()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  \tKEY\tNAME\tSIZE\tCONTEXT\tRECOMMENDED\tSTATUS")
	row := func(e local.Model, status []string) {
		mark := ""
		if e.Model == active.Model {
			mark = "*"
		}
		size, ctx := "-", "-"
		if e.WeightsGB > 0 {
			size = fmt.Sprintf("%d GB", e.WeightsGB)
		}
		if n, err := strconv.Atoi(e.Params["MLX_OPENCODE_CONTEXT"]); err == nil && n > 0 {
			ctx = fmt.Sprintf("%dk", n/1024)
		}
		rec := "-"
		if r := e.Recommended(); len(r) > 0 {
			var short []string
			for _, x := range r {
				short = append(short, x.Short)
			}
			rec = strings.Join(short, ", ")
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\n", mark, e.Key, e.Name, size, ctx, rec, strings.Join(status, ", "))
	}
	for _, e := range m.Models {
		var status []string
		if e.Default {
			status = append(status, "default")
		}
		switch ok, err := local.WeightsPresent(e.Model); {
		case err != nil:
			status = append(status, "cache unreadable")
		case ok:
			status = append(status, "downloaded")
		default:
			status = append(status, "not downloaded")
		}
		if e.Model == running {
			status = append(status, "running")
		}
		if too := tooBigHere(e); too != "" {
			status = append(status, too)
		}
		row(e, status)
	}
	for _, wh := range m.Withheld {
		// Parse also withholds an unreadable min_nav_pilot, which names no
		// version to need.
		status := []string{"withheld: unreadable min_nav_pilot"}
		if strings.Contains(wh.Reason, "needs nav-pilot ≥") {
			status = []string{"withheld: needs nav-pilot ≥ " + string(wh.Model.MinNavPilot)}
		}
		// A server an older nav-pilot started can still serve it.
		if wh.Model.Model == running {
			status = append(status, "running")
		}
		row(wh.Model, status)
	}
	// A replaced id still in use because only its weights are here: not a
	// manifest entry, but what a start loads, so it gets the `*`.
	if !slices.ContainsFunc(m.Models, func(e local.Model) bool { return e.Model == active.Model }) {
		if repl, ok := m.ReplacedBy(active.Model); ok {
			status := []string{"downloaded", "replaced by " + repl.Key}
			if active.Model == running {
				status = append(status, "running")
			}
			row(active, status)
		}
	}
	_ = w.Flush()
	fmt.Printf("\n  Switch: %s\n", bold("nav-pilot alpha local use <key>"))
}

// tooBigHere is init's memory refusal as a table cell: [local.CheckWiredLimit]
// is what init and start refuse on, so the table says the same thing sooner.
// Only the refusals that are about this machine's memory; a manifest entry with
// no measured limit is start's to explain.
func tooBigHere(e local.Model) string {
	w, err := local.CheckWiredLimit(e)
	if err == nil || w.MachineRAMGB == 0 {
		return ""
	}
	if e.MinRAMGB > w.MachineRAMGB {
		return fmt.Sprintf("needs %d GB RAM", e.MinRAMGB)
	}
	return "too big for this machine"
}

func cmdLocalUse(args []string) error {
	m, err := cachedManifest()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		printLocalModels(m)
		fmt.Printf("  Usage:  %s\n", bold("nav-pilot alpha local use <key|model-id>"))
		return nil
	}
	arg := strings.TrimSpace(args[0])
	match := func(e local.Model) bool { return e.Key == arg || e.Model == arg }
	for _, w := range m.Withheld {
		if match(w.Model) {
			return errors.New(w.Reason)
		}
	}
	i := slices.IndexFunc(m.Models, match)
	if repl, ok := m.ReplacedBy(arg); ok && i < 0 {
		return fmt.Errorf("%s was replaced by %s. Use: %s", arg, repl.Key, bold("nav-pilot alpha local use "+repl.Key))
	}
	if i < 0 {
		var names []string
		for _, e := range m.Models {
			names = append(names, e.Key, e.Model)
		}
		if hint := suggest(arg, names); hint != "" {
			return fmt.Errorf("unknown local model: %s. Did you mean %s?", arg, bold("nav-pilot alpha local use "+hint))
		}
		return fmt.Errorf("unknown local model: %s. Run %s to see what is offered", arg, bold("nav-pilot alpha local models"))
	}
	e := m.Models[i]

	// Written even when it is the default, so the choice survives the manifest
	// moving its default to something else.
	if _, err := writeConfigKey("local_model", e.Model); err != nil {
		return err
	}
	fmt.Printf("%s local_model = %s %s\n", green("✓"), bold(e.Key), dim("("+e.Model+")"))
	// Just chosen on purpose, so the advisory this choice would trigger has
	// nothing to tell them.
	local.SetSelectedModel(e.Model)
	local.MarkSeen(local.PinnedAdvisory(m))

	present, err := local.WeightsPresent(e.Model)
	if err != nil {
		return err
	}
	if present {
		fmt.Printf("%s Downloaded.\n", green("✓"))
	} else {
		size := ""
		if e.WeightsGB > 0 {
			size = fmt.Sprintf(" (about %d GB)", e.WeightsGB)
		}
		// start refuses missing weights rather than fetching them, so init is
		// the command that downloads.
		fmt.Printf("%s Not downloaded yet%s. %s downloads it and starts it.\n",
			yellow("⚠"), size, bold("nav-pilot alpha local init"))
	}

	running := runningModel()
	if running == "" || running == e.Model {
		return nil
	}
	fmt.Printf("%s The running server still serves %s.\n", yellow("⚠"), bold(running))
	if !present {
		fmt.Printf("  After init, load it with %s\n", bold("nav-pilot alpha local restart"))
		return nil
	}
	if !providerpkg.IsTerminal(os.Stdin) {
		fmt.Printf("  Load it: %s\n", bold("nav-pilot alpha local restart"))
		return nil
	}
	restart := true
	if err := huh.NewConfirm().
		Title("Restart the server on " + e.Name + "?").
		Value(&restart).
		WithTheme(navTheme()).
		Run(); err != nil || !restart {
		fmt.Printf("  Later: %s\n", bold("nav-pilot alpha local restart"))
		return nil
	}
	return cmdLocalRestart()
}
