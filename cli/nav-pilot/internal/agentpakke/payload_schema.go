package agentpakke

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/navikt/copilot/cli/nav-pilot/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// PayloadSchemaID is the $id of the published payload manifest schema.
const PayloadSchemaID = "https://github.com/navikt/copilot/cli/nav-pilot/schemas/agentpakke-payload-v1.json"

// PayloadSchemaJSON returns the published payload schema bytes, for a caller
// that wants to vendor or serve it. A copy, so a caller cannot mutate the
// embedded contract.
func PayloadSchemaJSON() []byte {
	out := make([]byte, len(schemas.AgentpakkePayloadV1))
	copy(out, schemas.AgentpakkePayloadV1)
	return out
}

var (
	payloadCompileOnce sync.Once
	payloadCompiled    *jsonschema.Schema
	payloadCompileErr  error
)

// payloadSchema compiles the embedded payload schema once. A compile failure is
// a defect in this repo, not something a payload author can fix.
func payloadSchema() (*jsonschema.Schema, error) {
	payloadCompileOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemas.AgentpakkePayloadV1))
		if err != nil {
			payloadCompileErr = fmt.Errorf("parsing embedded payload schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource(PayloadSchemaID, doc); err != nil {
			payloadCompileErr = fmt.Errorf("loading embedded payload schema: %w", err)
			return
		}
		payloadCompiled, err = c.Compile(PayloadSchemaID)
		if err != nil {
			payloadCompileErr = fmt.Errorf("compiling embedded payload schema: %w", err)
		}
	})
	return payloadCompiled, payloadCompileErr
}

// validatePayloadSchema checks raw payload-manifest bytes against the published
// schema.
//
// It runs before the hand-written checks in [ParsePayloadManifest], not instead
// of them. The schema is the published shape, so it is what an agentpakke
// author lints against and what must decide whether a manifest conforms; the Go
// checks that follow keep naming the offending path and the remedy, which a
// schema error cannot do as well. The two must therefore agree, and the tests
// pin the places where agreement is easy to lose: unknown keys in a file
// record, and the path grammar.
func validatePayloadSchema(data []byte, manifestPath string) error {
	sch, err := payloadSchema()
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("payload manifest %s is not valid JSON: %w", manifestPath, err)
	}
	if err := sch.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if !errors.As(err, &verr) {
			return fmt.Errorf("payload manifest %s failed schema validation: %w", manifestPath, err)
		}
		return schemaErrorFor(manifestPath, PayloadSchemaID, schemaViolations(verr))
	}
	return nil
}
