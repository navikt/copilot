package agentpakke

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/navikt/copilot/cli/nav-pilot/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// SchemaID is the published identity of the agentpakke manifest schema — the
// $id an agentpakke repo references when linting its manifest in CI.
const SchemaID = "https://github.com/navikt/copilot/cli/nav-pilot/schemas/agentpakke-v1.json"

// SchemaJSON returns the published JSON Schema bytes. They come from the file
// that ships in the repo (cli/nav-pilot/schemas/agentpakke-v1.json), so the
// binary and an agentpakke repo's CI validate against identical bytes.
func SchemaJSON() []byte {
	out := make([]byte, len(schemas.AgentpakkeV1))
	copy(out, schemas.AgentpakkeV1)
	return out
}

var (
	compileOnce sync.Once
	compiled    *jsonschema.Schema
	compileErr  error

	// errPrinter renders the library's localized error kinds. The library keeps
	// its own printer unexported, so validation messages need one here.
	errPrinter = message.NewPrinter(language.English)
)

// schema compiles the embedded schema once. A compile failure is a defect in
// this repo (the schema ships with the binary), not something a manifest author
// can fix.
func schema() (*jsonschema.Schema, error) {
	compileOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemas.AgentpakkeV1))
		if err != nil {
			compileErr = fmt.Errorf("parsing embedded agentpakke schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource(SchemaID, doc); err != nil {
			compileErr = fmt.Errorf("loading embedded agentpakke schema: %w", err)
			return
		}
		compiled, err = c.Compile(SchemaID)
		if err != nil {
			compileErr = fmt.Errorf("compiling embedded agentpakke schema: %w", err)
		}
	})
	return compiled, compileErr
}

// validateSchema checks raw manifest bytes against the published JSON Schema
// and rewrites library errors into actionable ones (A3: what is wrong, where,
// and what the contract expects).
func validateSchema(data []byte) error {
	sch, err := schema()
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", ManifestPath, err)
	}
	if err := sch.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if !errors.As(err, &verr) {
			return fmt.Errorf("%s failed schema validation: %w", ManifestPath, err)
		}
		return schemaError(verr)
	}
	return nil
}

// schemaError turns a jsonschema ValidationError tree into a single actionable
// message: one line per concrete violation, each naming the manifest field by
// path plus a hint drawn from the contract.
func schemaError(verr *jsonschema.ValidationError) error {
	causes := leafCauses(verr, nil)
	lines := make([]string, 0, len(causes))
	seen := make(map[string]bool, len(causes))
	for _, c := range causes {
		line := "  - " + describeCause(c)
		if seen[line] {
			continue
		}
		seen[line] = true
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return fmt.Errorf(
		"%s does not conform to the agentpakke contract (schema %s):\n%s\n"+
			"fix the manifest, or lint it against the published schema in your own CI before pushing",
		ManifestPath, SchemaID, strings.Join(lines, "\n"))
}

// leafCauses flattens a ValidationError tree to its most specific failures;
// intermediate nodes only restate that some subschema failed.
func leafCauses(verr *jsonschema.ValidationError, acc []*jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(verr.Causes) == 0 {
		return append(acc, verr)
	}
	for _, c := range verr.Causes {
		acc = leafCauses(c, acc)
	}
	return acc
}

// describeCause renders one violation as "<field>: <what failed> (<how to fix>)".
func describeCause(c *jsonschema.ValidationError) string {
	loc := instanceLocation(c.InstanceLocation)
	msg := strings.TrimSpace(c.ErrorKind.LocalizedString(errPrinter))
	if hint := hintFor(c.InstanceLocation, msg); hint != "" {
		return fmt.Sprintf("%s: %s (%s)", loc, msg, hint)
	}
	return fmt.Sprintf("%s: %s", loc, msg)
}

// instanceLocation renders a JSON pointer path as a readable field reference.
func instanceLocation(parts []string) string {
	if len(parts) == 0 {
		return "manifest root"
	}
	return strings.Join(parts, ".")
}

// hintFor adds contract guidance for the violations manifest authors hit most,
// so an error says how to fix it and not only what failed.
func hintFor(loc []string, msg string) string {
	if len(loc) == 0 {
		switch {
		case strings.Contains(msg, "contractVersion"):
			return "supported contract versions: " + strings.Join(SupportedContractMajors, ", ")
		case strings.Contains(msg, "clients"):
			return `declare at least one client, e.g. "clients": {"opencode": {"primaryAgents": ["nav-pilot"]}}`
		}
		return ""
	}
	switch loc[0] {
	case "contractVersion":
		return `expected "1" or "1.<minor>"; supported: ` + strings.Join(SupportedContractMajors, ", ")
	case "name":
		return "lowercase identifier: ^[a-z][a-z0-9-]*$"
	case "clients":
		switch {
		case len(loc) == 1:
			return "declare at least one client entry"
		case len(loc) == 2:
			return `a Tier 1 client entry needs primaryAgents; a Tier 2 entry declares payloads instead, and each payload carries its own primaryAgents`
		case loc[2] == "primaryAgents":
			return "list at least one agent name selectable in this client"
		case loc[2] == "payloads":
			// len 4 is the payload object itself (a missing required field),
			// len 5+ is one of its fields.
			if len(loc) >= 5 && loc[4] == "primaryAgents" {
				return "list at least one agent name launchable in this context; the first is the context's default persona"
			}
			return `each context maps to {"path": "<dir>", "primaryAgents": ["<agent>"]}; the first primaryAgent is that context's default persona, and the payload manifest resolves to <path>/manifest.json`
		}
	case "layout":
		return "layout requires agents and skills as repo-relative paths"
	case "minNavPilotVersion":
		return "expected a nav-pilot release version, e.g. 2026.09.01-120000-a1b2c3d"
	}
	return ""
}
