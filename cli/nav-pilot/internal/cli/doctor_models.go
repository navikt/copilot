package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// The client's model catalogue is per account and per plan, so a curated list
// in this repo can only ever be a guess about someone else's entitlement
// (#717). `known_models_gen.go` is generated from models.dev, which is global:
// it offered `claude-sonnet-4.6` for days after GitHub withdrew it, and three
// agents pinned to it failed at launch rather than at use (#715).
//
// So doctor asks the client instead of consulting a list.
//
// The probe is a launch that is designed to fail: `--model` with a sentinel the
// catalogue cannot contain. The client fetches the catalogue, rejects the
// model, and exits before running a completion, so it costs no AI credits
// (measured: the run prints no credits line). Debug logging puts the catalogue
// on disk, in a directory this process owns and removes.
//
// Everything here is warn-only. nav-pilot does not control the catalogue, and a
// developer who is offline, unauthenticated, or on a client that stops writing
// this log must get "could not check" rather than a red cross or, worse, a
// green tick that checked nothing. Same discipline as the cplt version skew.

// modelProbeSentinel is the model id the probe asks for. It has to be a value
// the catalogue can never hold and the client's own flag validation will not
// reject before the fetch: the shape is legal, the name is not real.
const modelProbeSentinel = "nav-pilot-model-probe"

// modelProbeTimeout bounds the catalogue probe. It fetches over the network, so
// it needs more room than the local version checks: measured at 2.3s, given 15
// to survive a slow link without hanging a health check.
const modelProbeTimeout = 15 * time.Second

// chatModelID matches an id inside a chat-model record in the client's debug
// log. The log holds the catalogue as escaped JSON on one line, so this reads
// the `"type":"chat"` marker and then the id that follows it within the same
// record. Anchoring on the type is what keeps embedding models out: they carry
// the same id shape and would otherwise be reported as available chat models.
//
// Derived from a real log (Copilot CLI 1.0.83-4), not from the documented
// shape: 29 chat models matched, zero embedding ids leaked.
var chatModelID = regexp.MustCompile(`type\\?":\\?"chat\\?".{0,60}?id\\?":\\?"([a-zA-Z0-9._-]+)`)

// clientChatModels asks the client which chat models this account can launch.
//
// A nil slice means the question could not be answered, which is different from
// an empty catalogue and is reported differently. The bool says which.
func clientChatModels(copilotPath string) ([]string, bool) {
	logDir, err := os.MkdirTemp("", "nav-pilot-models-")
	if err != nil {
		return nil, false
	}
	defer os.RemoveAll(logDir)

	// The error is expected: the sentinel is rejected by design. What matters is
	// whether the catalogue reached the log on the way there.
	//
	// Its own deadline, not cpltCommandTimeout. That one is 2s, tuned for `cplt
	// --version`, and this probe measured 2.3s on the machine it was written on:
	// under the shared budget it was killed just before writing the log, and
	// doctor reported "could not read the catalogue" every time. A network round
	// trip needs room that a local version string does not.
	ctx, cancel := context.WithTimeout(context.Background(), modelProbeTimeout)
	defer cancel()
	_ = exec.CommandContext(ctx, copilotPath,
		"--log-level", "debug", "--log-dir", logDir,
		"--model", modelProbeSentinel, "-p", "probe").Run()

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, false
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(logDir, e.Name()))
		if err != nil {
			continue
		}
		for _, m := range chatModelID.FindAllSubmatch(data, -1) {
			seen[string(m[1])] = true
		}
	}
	if len(seen) == 0 {
		return nil, false
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, true
}

// pinnedModel is one agent's model pin, as installed.
type pinnedModel struct {
	Agent string
	Label string // as written in frontmatter
	ID    string // resolved Copilot id, or "" when the label resolves to nothing
}

// installedModelPins reads the model pin of every agent installed in a scope.
// An agent with no pin inherits the client's model and is not listed: there is
// nothing that can be stale.
func installedModelPins(scope *InstallScope) []pinnedModel {
	dir := scope.DstPath(source.KindAgent.Dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var pins []pinnedModel
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), source.KindAgent.Suffix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		label := frontmatterModel(data)
		if label == "" {
			continue
		}
		id := strings.TrimPrefix(domain.OpenCodeModelForLabel(label), domain.OpenCodeProviderPrefix)
		pins = append(pins, pinnedModel{
			Agent: strings.TrimSuffix(e.Name(), source.KindAgent.Suffix),
			Label: label,
			ID:    id,
		})
	}
	sort.Slice(pins, func(i, j int) bool { return pins[i].Agent < pins[j].Agent })
	return pins
}

// frontmatterModelLine reads the `model:` key out of YAML frontmatter without
// pulling in a YAML parser for one field. It stops at the closing delimiter so
// a `model:` inside the prose body is never mistaken for a pin.
var frontmatterModelLine = regexp.MustCompile(`(?m)^model:[ \t]*(.+?)[ \t]*$`)

func frontmatterModel(data []byte) string {
	text := string(data)
	if !strings.HasPrefix(text, "---") {
		return ""
	}
	end := strings.Index(text[3:], "\n---")
	if end < 0 {
		return ""
	}
	m := frontmatterModelLine.FindStringSubmatch(text[:end+3])
	if m == nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(m[1]), `"'`)
}

// unavailablePins returns the pins the client cannot launch, given the
// catalogue it reported. A label that resolves to no known id is included too:
// the client is asked for the label verbatim in that case, and a name the
// catalogue does not carry fails the same way.
func unavailablePins(pins []pinnedModel, catalogue []string) []pinnedModel {
	have := make(map[string]bool, len(catalogue))
	for _, id := range catalogue {
		have[strings.ToLower(id)] = true
	}
	var bad []pinnedModel
	for _, p := range pins {
		want := p.ID
		if want == "" {
			want = p.Label
		}
		if !have[strings.ToLower(want)] {
			bad = append(bad, p)
		}
	}
	return bad
}

// reportModelPins prints the model-pin section of doctor.
//
// It never sets hasErrors. A stale pin is worth saying out loud, but nav-pilot
// neither owns the catalogue nor the agent files a user may have written
// themselves, and failing the whole health check over someone else's
// entitlement would teach people to ignore the exit code.
func reportModelPins() {
	scope, err := ScopeUser()
	if err != nil {
		fmt.Printf("    %s Could not resolve the user scope: %v\n", dim("-"), err)
		return
	}
	pins := installedModelPins(scope)
	if len(pins) == 0 {
		fmt.Printf("    %s No installed agent pins a model (all inherit the client's)\n", green("✓"))
		return
	}

	copilotPath, _ := exec.LookPath("copilot")
	if copilotPath == "" {
		fmt.Printf("    %s %d agent(s) pin a model; Copilot CLI is not on PATH, so availability was not checked\n",
			dim("-"), len(pins))
		return
	}

	catalogue, ok := clientChatModels(copilotPath)
	if !ok {
		// Offline, unauthenticated, or a client that no longer writes the
		// catalogue to its debug log. "Unknown" and never "fine".
		fmt.Printf("    %s %d agent(s) pin a model; could not read the client's model catalogue\n",
			dim("-"), len(pins))
		return
	}

	bad := unavailablePins(pins, catalogue)
	if len(bad) == 0 {
		fmt.Printf("    %s All %d pinned model(s) are available to this account\n", green("✓"), len(pins))
		return
	}
	fmt.Printf("    %s %d of %d pinned model(s) are not available to this account\n",
		yellow("⚠"), len(bad), len(pins))
	for _, p := range bad {
		fmt.Printf("      %s %s pins %q\n", yellow("⚠"), p.Agent, p.Label)
	}
	fmt.Printf("      %s The client launches these agents with a model it will reject. Repin them, or\n",
		yellow("Solution:"))
	fmt.Printf("          run %s to take the current pins from the source.\n", bold("nav-pilot sync --apply"))
}
