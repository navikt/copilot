package agentpakke

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/navikt/copilot/cli/nav-pilot/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ReleaseSchemaID is the $id of the published release metadata schema.
const ReleaseSchemaID = "https://raw.githubusercontent.com/navikt/copilot/main/cli/nav-pilot/schemas/agentpakke-release-v1.json"

// ReleaseAssetName is the release asset a package owner publishes the metadata
// as.
const ReleaseAssetName = "agentpakke-release.json"

var (
	releaseCompileOnce sync.Once
	releaseCompiled    *jsonschema.Schema
	releaseCompileErr  error
)

func releaseSchema() (*jsonschema.Schema, error) {
	releaseCompileOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemas.AgentpakkeReleaseV1))
		if err != nil {
			releaseCompileErr = fmt.Errorf("parsing embedded release metadata schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource(ReleaseSchemaID, doc); err != nil {
			releaseCompileErr = fmt.Errorf("loading embedded release metadata schema: %w", err)
			return
		}
		releaseCompiled, err = c.Compile(ReleaseSchemaID)
		if err != nil {
			releaseCompileErr = fmt.Errorf("compiling embedded release metadata schema: %w", err)
		}
	})
	return releaseCompiled, releaseCompileErr
}

// ValidateRelease checks release metadata against the published schema, the
// same file a package owner lints the asset with.
func ValidateRelease(data []byte) error {
	sch, err := releaseSchema()
	if err != nil {
		return err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", ReleaseAssetName, err)
	}
	if err := sch.Validate(inst); err != nil {
		var verr *jsonschema.ValidationError
		if !errors.As(err, &verr) {
			return fmt.Errorf("%s failed schema validation: %w", ReleaseAssetName, err)
		}
		// Not schemaErrorFor: its remedy is "fix the manifest", and a release is
		// immutable, so the only fix is a new release.
		return fmt.Errorf("%s does not conform to the release metadata contract (schema %s):\n%s\n"+
			"publish a new release with a conforming asset, and lint it against the published schema before publishing",
			ReleaseAssetName, ReleaseSchemaID, strings.Join(schemaViolations(verr), "\n"))
	}
	return nil
}
