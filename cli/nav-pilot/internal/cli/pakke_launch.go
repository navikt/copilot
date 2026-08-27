package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// stagedMaxAge is how long a staged payload tree may survive its launch before
// the next staged launch sweeps it. A tree is normally removed when the client
// exits; anything older than this was left behind by a crash.
//
// ponytail: naive age heuristic. A session running longer than a day would have
// its config swept out from under it — visible at the next config read, not
// unsafe. Upgrade to owner-pid files only if that turns out to happen.
const stagedMaxAge = 24 * time.Hour

// stagedRoot is where verified Tier 2 payloads are staged: ~/.nav-pilot/staged,
// a sibling of config.toml. It follows configPath(), so NAV_PILOT_CONFIG
// relocates it too.
func stagedRoot() string {
	return filepath.Join(filepath.Dir(configPath()), "staged")
}

// tryPakkeLaunch runs the Tier 2 (payload) launch path when — and only when —
// the resolved source ships an agentpakke manifest that declares pre-built
// payloads for the client being launched. It reports whether it handled the
// launch; (false, nil) means the caller should launch the legacy way.
//
// The first check is the no-regression gate: with no source selected (the
// default for every user who has never run `config set source` or passed
// --source) it returns immediately, having resolved nothing, read nothing, and
// left the active agentpakke on the built-in default.
func tryPakkeLaunch(resolved ResolvedConfig) (bool, error) {
	if resolved.Source == "" {
		return false, payloadContextUnsupported(resolved, defaultSourceRepo)
	}

	// The CLI's source funnel: applies source precedence and attaches the
	// schema-validated manifest.
	//
	// A resolve failure lands *before* the tier gate, so nothing yet says this
	// launch is Tier 2 — the source may well declare no payloads at all. Being
	// offline, or having a stale repo name in config, must therefore not block
	// a launch that worked before Tier 2 staging existed: warn and take the
	// legacy path, as EnsureOpenCodeNavContext has always done. Fail-closed
	// starts at the tier gate below, where the payload is known.
	src, err := resolveSource("", resolved.Source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not resolve source %s: %v — launching without agentpakke context.\n",
			yellow("⚠"), resolved.Source, err)
		return false, payloadContextUnsupported(resolved, resolved.Source)
	}

	if src.Pakke == nil || src.Pakke.Tier(resolved.Client) != agentpakke.TierPayload {
		// Manifest-less or Tier 1: exactly today's path, built-in default
		// agentpakke still active.
		src.Cleanup()
		return false, payloadContextUnsupported(resolved, resolved.Source)
	}

	pakke := src.Pakke
	context := resolved.PayloadContext
	if context == "" {
		context = pakke.DefaultContext(resolved.Client)
	}
	payload, ok := pakke.Payload(resolved.Client, context)
	if !ok {
		src.Cleanup()
		return true, fmt.Errorf("agentpakke %q declares no %q payload for %s (declared contexts: %s)",
			pakke.Name, context, resolved.Client, strings.Join(declaredContexts(pakke, resolved.Client), ", "))
	}

	root := stagedRoot()
	// Best-effort sweep of trees a crashed session left behind. It costs one
	// directory read on a path that is about to do far more I/O than that.
	_ = agentpakke.GCStaged(root, stagedMaxAge)

	stagedDir, stageErr := agentpakke.StagePayload(
		filepath.Join(src.Dir, filepath.FromSlash(payload.Path)),
		filepath.Join(src.Dir, filepath.FromSlash(payload.ManifestPath())),
		root)
	// The staged tree carries its own manifest, so the checkout — which may be
	// a temp clone — is no longer needed either way.
	src.Cleanup()
	if stageErr != nil {
		// Fail-closed (G2): a Tier 2 launch never falls back to the legacy path.
		return true, fmt.Errorf("staging the %q payload of agentpakke %q for %s: %w",
			context, pakke.Name, resolved.Client, stageErr)
	}
	defer func() {
		// A tree that survives is verified config left on disk; say so, and
		// let the 24h sweep pick it up.
		if err := agentpakke.CleanupStaged(stagedDir); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", yellow("⚠"), err)
		}
	}()

	// The only SetActivePakke call site: past the tier gate the manifest is
	// known to declare this client, which is the invariant provider.PrimaryAgent
	// relies on.
	providerpkg.SetActivePakke(pakke)

	staged := providerpkg.StagedLaunch{Dir: stagedDir, PakkeName: pakke.Name, Context: context}
	switch resolved.Client {
	case "opencode":
		return true, providerpkg.LaunchOpenCodeStaged(resolved, staged)
	case "copilot":
		return true, providerpkg.LaunchCopilotStaged(resolved, staged)
	default:
		return true, fmt.Errorf("agentpakke %q declares payloads for %s, but this nav-pilot cannot launch staged payloads for that client",
			pakke.Name, resolved.Client)
	}
}

// payloadContextUnsupported turns an explicit --payload-context into an error
// when the launch resolves to the legacy path. Ignoring the flag silently would
// mask what the user asked for, the same policy unknown config keys get.
func payloadContextUnsupported(resolved ResolvedConfig, label string) error {
	if resolved.PayloadContext == "" {
		return nil
	}
	return fmt.Errorf("--payload-context requires an agentpakke with pre-built payloads; source %s declares none for client %s",
		label, resolved.Client)
}

// declaredContexts lists a client's payload context ids, sorted.
func declaredContexts(m *agentpakke.Manifest, client string) []string {
	entry, ok := m.Client(client)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(entry.Payloads))
	for id := range entry.Payloads {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
