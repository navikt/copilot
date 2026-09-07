package agentpakke

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/navikt/copilot/cli/nav-pilot/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// RetiredSchemaID is the $id of the published retired-artifact record schema.
const RetiredSchemaID = "https://raw.githubusercontent.com/navikt/copilot/main/cli/nav-pilot/schemas/agentpakke-retired-v1.json"

// RetiredRecordPath is where an agentpakke publishes the record, relative to
// the repo root.
const RetiredRecordPath = ".nav-pilot/retired-artifacts.json"

// RetiredSchemaJSON returns the published schema bytes, for a caller that wants
// to vendor or serve it. A copy, so a caller cannot mutate the embedded
// contract.
func RetiredSchemaJSON() []byte {
	out := make([]byte, len(schemas.AgentpakkeRetiredV1))
	copy(out, schemas.AgentpakkeRetiredV1)
	return out
}

var (
	retiredCompileOnce sync.Once
	retiredCompiled    *jsonschema.Schema
	retiredCompileErr  error
)

func retiredSchema() (*jsonschema.Schema, error) {
	retiredCompileOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemas.AgentpakkeRetiredV1))
		if err != nil {
			retiredCompileErr = fmt.Errorf("parsing embedded retired-artifact schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource(RetiredSchemaID, doc); err != nil {
			retiredCompileErr = fmt.Errorf("loading embedded retired-artifact schema: %w", err)
			return
		}
		retiredCompiled, err = c.Compile(RetiredSchemaID)
		if err != nil {
			retiredCompileErr = fmt.Errorf("compiling embedded retired-artifact schema: %w", err)
		}
	})
	return retiredCompiled, retiredCompileErr
}

// ValidateRetired checks a retired-artifact record against the published
// schema.
//
// The record drives deletion, so the shape has to be certain before any of it
// is acted on: a hash that is not a blob id, or a path that escapes the repo,
// must not reach the code that removes files.
func ValidateRetired(data []byte) error {
	sch, err := retiredSchema()
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", RetiredRecordPath, err)
	}
	if err := sch.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if !errors.As(err, &verr) {
			return fmt.Errorf("%s failed schema validation: %w", RetiredRecordPath, err)
		}
		return schemaErrorFor(RetiredRecordPath, RetiredSchemaID, schemaViolations(verr))
	}
	return nil
}
