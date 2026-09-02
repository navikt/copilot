package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// errValidationFailed is returned when `nav-pilot validate` found violations.
// The findings are already printed, so the message stays short.
var errValidationFailed = errors.New("agentpakke validation failed")

// cmdValidate checks whether a source repo conforms to the agentpakke contract
// (A5). It is the command an agentpakke repo runs in its own CI, so it resolves
// the source exactly like install does, reports every finding instead of the
// first, and exits non-zero when any of them is a violation.
//
// A source without a manifest is not a failure: it is a legacy collection
// source, and it is validated as one.
func cmdValidate(ref, sourceRepo string, jsonOutput bool) error {
	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveSourceRaw(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	label := sourceLabelFor(src)
	kind, notes, findings := validateSourceTree(src)

	if jsonOutput {
		problems := make([]string, 0, len(findings))
		for _, f := range findings {
			problems = append(problems, f.Error())
		}
		if err := outputJSON(map[string]interface{}{
			"command":  "validate",
			"source":   label,
			"sha":      src.SHA,
			"kind":     kind,
			"valid":    len(findings) == 0,
			"notes":    notes,
			"problems": problems,
		}); err != nil {
			return err
		}
		if len(findings) > 0 {
			return errValidationFailed
		}
		return nil
	}

	fmt.Println()
	fmt.Printf("%s %s\n", bold("Validating:"), fmt.Sprintf("%s@%s", label, shortSHA(src.SHA)))
	fmt.Println()
	for _, n := range notes {
		fmt.Printf("  %s %s\n", dim("ℹ"), n)
	}
	if len(findings) == 0 {
		if kind == "legacy" {
			fmt.Printf("\n%s %s is a valid legacy collection source.\n", green("✓"), label)
			return nil
		}
		fmt.Printf("\n%s %s conforms to the agentpakke contract.\n", green("✓"), label)
		return nil
	}

	fmt.Printf("\n%s %d problem(s):\n", red("✗"), len(findings))
	for _, f := range findings {
		fmt.Printf("  - %s\n", strings.ReplaceAll(f.Error(), "\n", "\n    "))
	}
	fmt.Println()
	fmt.Printf("The agentpakke contract and its JSON Schema live in %s.\n",
		dim("navikt/copilot: cli/nav-pilot/schemas"))
	return errValidationFailed
}

// validateSourceTree runs the conformance checks for a resolved checkout and
// returns the source kind ("agentpakke" or "legacy"), informational notes, and
// every violation found.
func validateSourceTree(src *Source) (kind string, notes []string, findings []error) {
	m, err := agentpakke.Load(src.Dir)
	if err != nil {
		if !errors.Is(err, agentpakke.ErrNoManifest) {
			return "agentpakke", []string{
				fmt.Sprintf("manifest: %s", agentpakke.ManifestPath),
			}, []error{err}
		}
		return "legacy", []string{
			fmt.Sprintf("no manifest (legacy collection source) — %s is absent", agentpakke.ManifestPath),
		}, validateLegacySource(src)
	}

	notes = append(notes,
		fmt.Sprintf("manifest: %s", agentpakke.ManifestPath),
		fmt.Sprintf("agentpakke: %s (contract version %s)", m.Name, m.ContractVersion),
	)
	if clients := m.ClientIDs(); len(clients) > 0 {
		var parts []string
		for _, id := range clients {
			label := id
			if !agentpakke.IsKnownClient(id) {
				label += " (unknown to this nav-pilot — ignored)"
			} else {
				label += fmt.Sprintf(" (tier %d)", m.Tier(id))
			}
			parts = append(parts, label)
		}
		notes = append(notes, "clients: "+strings.Join(parts, ", "))
	}
	if m.MinNavPilotVersion != "" {
		notes = append(notes, "minNavPilotVersion: "+m.MinNavPilotVersion)
	}

	return "agentpakke", notes, agentpakke.ValidateSource(src.Dir)
}

// validateLegacySource checks a source that ships no manifest. It must still be
// installable the way every source is today: at least one collection whose
// manifest loads and whose entries resolve in the canonical layout.
func validateLegacySource(src *Source) []error {
	names, err := listCollectionDirs(src.Dir)
	if err != nil || len(names) == 0 {
		return []error{fmt.Errorf(
			"source ships neither %s nor a collections/ directory, so nav-pilot has nothing to install from it. "+
				"Add an agentpakke manifest, or ship collections/<name>/manifest.json",
			agentpakke.ManifestPath)}
	}

	resolver := resolverFor(src.Dir, agentpakke.SynthesizeLegacy(""))
	var findings []error
	for _, name := range names {
		m, err := loadManifest(src.Dir, name)
		if err != nil {
			findings = append(findings, err)
			continue
		}
		for _, entry := range []struct {
			kind  *ArtifactKind
			names []string
		}{
			{KindAgent, m.Agents},
			{KindSkill, m.Skills},
			{KindInstruction, m.Instructions},
			{KindPrompt, m.Prompts},
		} {
			for _, item := range entry.names {
				if _, ok := resolver.Get(entry.kind, item); !ok {
					findings = append(findings, fmt.Errorf(
						"collection %q lists %s %q, which does not exist in %s/",
						name, entry.kind.Name, item, entry.kind.Dir))
				}
			}
		}
	}
	return findings
}
