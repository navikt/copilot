// Package schemas publishes nav-pilot's JSON Schemas as embeddable bytes.
//
// The .json files in this directory are the source of truth: an agentpakke repo
// lints its manifest against the same file (by URL or vendored copy) that the
// nav-pilot binary validates with, so the two can never disagree. The package
// exists only because go:embed cannot reach outside its own directory — keep it
// free of logic.
package schemas

import _ "embed"

// AgentpakkeV1 is the agentpakke manifest schema, contract version 1.
// Its $id is
// https://raw.githubusercontent.com/navikt/copilot/main/cli/nav-pilot/schemas/agentpakke-v1.json,
// which is a URL that resolves: an agentpakke repo is told to lint against it.
//
//go:embed agentpakke-v1.json
var AgentpakkeV1 []byte

// AgentpakkePayloadV1 is the payload manifest schema, contract version 1. Its
// $id is
// https://raw.githubusercontent.com/navikt/copilot/main/cli/nav-pilot/schemas/agentpakke-payload-v1.json.
//
// The payload manifest is the other half of the trust boundary: the top-level
// manifest says which trees exist, this one says exactly what each tree holds.
// It had no published schema, so an agentpakke author had nothing to lint
// against in their own CI (#704 T4).
//
//go:embed agentpakke-payload-v1.json
var AgentpakkePayloadV1 []byte

// AgentpakkeRetiredV1 is the retired-artifact record schema, contract version 1.
// Its $id is
// https://raw.githubusercontent.com/navikt/copilot/main/cli/nav-pilot/schemas/agentpakke-retired-v1.json
//
// The record itself is optional: an agentpakke that has never deleted an
// artifact needs none. It exists because sync removes only what its state file
// tracks, so an artifact retired upstream stays installed forever on a machine
// whose state predates it (#716). Publishing the hashes the pakke once shipped
// is what lets nav-pilot tell its own bytes from a file the user wrote (#729).
//
//go:embed agentpakke-retired-v1.json
var AgentpakkeRetiredV1 []byte

// AgentpakkeReleaseV1 is the release metadata schema, schemaVersion 1. Its $id
// is
// https://raw.githubusercontent.com/navikt/copilot/main/cli/nav-pilot/schemas/agentpakke-release-v1.json
//
// A package owner publishes the metadata as a release asset, and nav-pilot
// follows stable releases by it (#779).
//
//go:embed agentpakke-release-v1.json
var AgentpakkeReleaseV1 []byte
