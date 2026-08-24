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
// Its $id is https://github.com/navikt/copilot/cli/nav-pilot/schemas/agentpakke-v1.json.
//
//go:embed agentpakke-v1.json
var AgentpakkeV1 []byte
