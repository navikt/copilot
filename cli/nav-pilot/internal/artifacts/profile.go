package artifacts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/navikt/copilot/cli/nav-pilot/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ProfileURL is where the published nav-pilot profile is read from: the raw
// file on the default branch, deliberately a moving target. Editing
// nav-pilot-profile.json in navikt/copilot is how a Nav-wide default changes
// without building and shipping a binary.
const ProfileURL = "https://raw.githubusercontent.com/navikt/copilot/main/nav-pilot-profile.json"

// ProfileSchemaID is the published identity of the profile schema.
const ProfileSchemaID = "https://github.com/navikt/copilot/cli/nav-pilot/schemas/nav-pilot-profile-v1.json"

// ProfileVersion is the only profile contract version this binary understands.
// A profile declaring anything else fails validation and is ignored, which is
// how a future incompatible profile stays harmless to today's binaries.
const ProfileVersion = "1"

// Profile is the centrally published set of Nav defaults. It is advisory: it
// carries preferences, never policy. Everything about the way it is read is
// fail-soft, which is the opposite of the agentpakke manifest's fail-closed
// rule, and the difference is deliberate. A manifest governs what runs; a
// default is only a preference, and the previous default is always a safe
// answer. Nothing here may ever stop a launch.
//
// The kill switch (#485) will want the same transport and must not reuse this
// rule: it has to fail closed. Ship it as its own read.
type Profile struct {
	ProfileVersion string            `json:"profileVersion"`
	DefaultModels  map[string]string `json:"defaultModels"`
}

// FetchProfile fetches the raw profile document. It is injected by package cli
// (which owns HTTP) on the same seam as provider.FetchLatestVersion. A nil
// FetchProfile disables the lookup, which is what every test and every
// non-launch caller gets by default.
var FetchProfile func() ([]byte, error)

var (
	profileSchemaOnce sync.Once
	profileSchema     *jsonschema.Schema
	profileSchemaErr  error
)

func compileProfileSchema() (*jsonschema.Schema, error) {
	profileSchemaOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemas.NavPilotProfileV1))
		if err != nil {
			profileSchemaErr = fmt.Errorf("parsing embedded profile schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource(ProfileSchemaID, doc); err != nil {
			profileSchemaErr = fmt.Errorf("loading embedded profile schema: %w", err)
			return
		}
		profileSchema, err = c.Compile(ProfileSchemaID)
		if err != nil {
			profileSchemaErr = fmt.Errorf("compiling embedded profile schema: %w", err)
		}
	})
	return profileSchema, profileSchemaErr
}

// ParseProfile validates raw profile bytes against the embedded JSON Schema and
// decodes them. Every failure is an ordinary error the caller drops on the
// floor: unlike an agentpakke manifest, nobody downstream needs to be told.
func ParseProfile(data []byte) (*Profile, error) {
	sch, err := compileProfileSchema()
	if err != nil {
		return nil, err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("profile is not valid JSON: %w", err)
	}
	if err := sch.Validate(inst); err != nil {
		return nil, fmt.Errorf("profile does not conform to %s: %w", ProfileSchemaID, err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decoding profile: %w", err)
	}
	return &p, nil
}

// ProfileDefaultModel returns the model the published profile declares for a
// client, or "" when there is no cached profile, no entry for the client, or
// no network has ever been reached. "" means "use the compiled-in default",
// which is the answer on every failure path.
func ProfileDefaultModel(client string) string {
	c := ReadCache()
	if c == nil {
		return ""
	}
	return c.DefaultModels[client]
}

// refreshProfile fetches and validates the profile, returning prev unchanged on
// any failure: no fetch function, network error, malformed JSON, or a document
// that fails the schema. prev is the last known good value, so a bad profile
// costs nothing and a deleted one is not a way to break launches.
func refreshProfile(prev map[string]string) map[string]string {
	if FetchProfile == nil {
		return prev
	}
	data, err := FetchProfile()
	if err != nil {
		return prev
	}
	p, err := ParseProfile(data)
	if err != nil {
		return prev
	}
	return p.DefaultModels
}
