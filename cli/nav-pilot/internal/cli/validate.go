package cli

import (
	"errors"
	"fmt"
	"os"
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
		fmt.Fprintln(os.Stderr, dim("Resolving source..."))
	}
	src, err := resolveSourceRaw(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	label := sourceLabelFor(src)
	kind, notes, warnings, findings := validateSourceTree(src)

	if jsonOutput {
		// Both lists are always arrays in the JSON, never null: the documented
		// --json contract types them as arrays, and a consumer should not need
		// a null case for the ordinary "nothing to report" run.
		if warnings == nil {
			warnings = []string{}
		}
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
			"warnings": warnings,
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
	// Warnings get their own channel rather than riding along as notes: notes
	// are neutral facts about the manifest, and something the author may want
	// to fix must not read like one. They never fail the command.
	for _, w := range warnings {
		fmt.Printf("  %s %s\n", yellow("⚠"), strings.ReplaceAll(w, "\n", "\n    "))
	}
	if len(findings) == 0 {
		if kind == "legacy" {
			fmt.Printf("\n%s %s is a valid source without a manifest.\n", green("✓"), label)
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
// returns the source kind ("agentpakke" or "legacy"), informational notes,
// warnings that do not fail the command, and every violation found.
func validateSourceTree(src *Source) (kind string, notes []string, warnings []string, findings []error) {
	m, err := agentpakke.Load(src.Dir)
	if err != nil {
		if !errors.Is(err, agentpakke.ErrNoManifest) {
			return "agentpakke", []string{
				fmt.Sprintf("manifest: %s", agentpakke.ManifestPath),
			}, nil, []error{err}
		}
		return "legacy", []string{
			fmt.Sprintf("no manifest — %s is absent; read as collections/<name>/manifest.json", agentpakke.ManifestPath),
		}, nil, validateLegacySource(src)
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

	warnings = m.ModelWarnings()
	if w := inertReuseWarning(m, src.Dir); w != "" {
		warnings = append(warnings, w)
	}
	findings = agentpakke.ValidateSource(src.Dir)
	// Membership in the MCP registry is a live question, so it is asked here
	// rather than in the schema: the registry gains and retires servers without
	// a nav-pilot release. It fails open — see [agentpakke.Manifest.MCPServerFindings].
	mcpFindings, mcpWarning := m.MCPServerFindings()
	findings = append(findings, mcpFindings...)
	if mcpWarning != "" {
		warnings = append(warnings, mcpWarning)
	}
	return "agentpakke", notes, warnings, findings
}

// inertReuseWarning reports a reuse declaration that nothing will ever act on.
//
// A pakke that ships payloads and no layout takes the pin route in install and
// sync, and neither of those reads the source's own declaration, so the file is
// committed, reviewed, and inert (#870). It is not a violation: the declaration
// harms nothing, and refusing it would break a pakke that already ships one.
// But validate is where an author asks whether the manifest does what she
// thinks, so this is the one place the silence has to end.
func inertReuseWarning(m *agentpakke.Manifest, root string) string {
	if !m.PayloadOnly() {
		return ""
	}
	d, err := agentpakke.LoadDeclaration(root)
	if err != nil || d.Source == "" {
		return ""
	}
	return fmt.Sprintf(
		"%s declares reuse of %q, and nothing reads it: this agentpakke ships payloads and no layout, "+
			"so install and sync pin a revision of its payloads rather than compose a base into it. "+
			"A Tier 2 pakke composes at build time, with its own tooling, and ships the result in the payload; "+
			"record where that content came from in the manifest's provenance.base and provenance.overlays, "+
			"which nav-pilot parses but never checks against the payload. "+
			"The file is not an error, and it still names the pakke this repo installs for its own developers, "+
			"but it adds nothing to what this pakke publishes",
		agentpakke.DeclarationPath, d.Source)
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
			{KindHook, m.Hooks},
		} {
			for _, item := range entry.names {
				if _, ok := resolver.Get(entry.kind, item); !ok {
					findings = append(findings, fmt.Errorf(
						"collections/%s lists %s %q, which does not exist in %s/",
						name, entry.kind.Name, item, entry.kind.Dir))
				}
			}
		}
	}
	return findings
}
