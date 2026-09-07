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
