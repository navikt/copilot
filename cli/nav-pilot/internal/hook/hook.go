// Package hook holds the Copilot CLI hooks nav-pilot ships as part of its own
// binary, as opposed to the Python hook artifacts it installs from a source.
//
// They exist because the Copilot CLI hooks run for every session, whatever the
// model: a cloud session sends its traffic to GitHub and nav-pilot's local
// guard never sees it, but the CLI still runs a postToolUse hook on this
// machine after every tool call, and hands it the result.
//
// The payload contract was measured, not only read off the docs
// (https://docs.github.com/en/copilot/reference/hooks-configuration), against
// Copilot CLI 1.0.89 on a cloud model:
//
//   - The payload arrives as one JSON object on stdin. A hook registered under
//     postToolUse gets camelCase keys (sessionId, toolName, toolArgs,
//     toolResult.resultType, toolResult.textResultForLlm); one registered under
//     PostToolUse gets snake_case (session_id, tool_name, tool_input,
//     tool_result.result_type, tool_result.text_result_for_llm). toolArgs is an
//     object, not a string.
//   - The session id is the one `copilot --resume=<id>` takes, stable across
//     every call in the session.
//   - A failed shell command is still resultType "success": the exit code is
//     in the text ("<shellId: 0 completed with exit code 1>").
//   - Printing {"modifiedResult": {"resultType", "textResultForLlm"}} on stdout
//     replaces what the model sees, in either dialect. Several postToolUse
//     hooks chain: the next one is handed the previous one's rewrite.
//   - A postToolUse hook that exits non-zero or times out is ignored (fail
//     open). Only preToolUse fails closed.
package hook

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Payload is the part of a postToolUse payload the hooks read, from either
// dialect.
type Payload struct {
	SessionID  string
	ToolName   string
	ToolArgs   json.RawMessage
	ResultType string
	Result     string
	HasResult  bool
	// Cwd is the session's working directory, which both dialects send.
	Cwd string
	// MCPServer is the registry name of the MCP server the tool belongs to,
	// when the client says (OpenCode's hooks bridge). Copilot CLI sends no
	// server field; its MCP tools are resolved from ToolName via
	// [CopilotMCPServer].
	MCPServer string
}

type wirePayload struct {
	Cwd        string          `json:"cwd"`
	MCPServer  string          `json:"mcpServer"`
	SessionID  string          `json:"sessionId"`
	ToolName   string          `json:"toolName"`
	ToolArgs   json.RawMessage `json:"toolArgs"`
	ToolResult *struct {
		ResultType string `json:"resultType"`
		Text       string `json:"textResultForLlm"`
	} `json:"toolResult"`

	SessionIDSnake  string          `json:"session_id"`
	ToolNameSnake   string          `json:"tool_name"`
	ToolInput       json.RawMessage `json:"tool_input"`
	ToolResultSnake *struct {
		ResultType string `json:"result_type"`
		Text       string `json:"text_result_for_llm"`
	} `json:"tool_result"`
}

// ParsePayload reads a hook payload in either dialect.
func ParsePayload(b []byte) (Payload, error) {
	var w wirePayload
	if err := json.Unmarshal(b, &w); err != nil {
		return Payload{}, err
	}
	p := Payload{SessionID: w.SessionID, ToolName: w.ToolName, ToolArgs: w.ToolArgs, Cwd: w.Cwd, MCPServer: w.MCPServer}
	if p.SessionID == "" {
		p.SessionID = w.SessionIDSnake
	}
	if p.ToolName == "" {
		p.ToolName = w.ToolNameSnake
	}
	if len(p.ToolArgs) == 0 {
		p.ToolArgs = w.ToolInput
	}
	switch {
	case w.ToolResult != nil:
		p.ResultType, p.Result, p.HasResult = w.ToolResult.ResultType, w.ToolResult.Text, true
	case w.ToolResultSnake != nil:
		p.ResultType, p.Result, p.HasResult = w.ToolResultSnake.ResultType, w.ToolResultSnake.Text, true
	}
	return p, nil
}

// NoChange is what a hook prints when it leaves the result alone.
const NoChange = "{}"

// ModifiedResult is the output that replaces the result the model sees. The
// result type is carried over unchanged: the hooks change what the model reads,
// never whether the call succeeded.
func ModifiedResult(resultType, text string) string {
	if resultType == "" {
		resultType = "success"
	}
	b, err := json.Marshal(map[string]any{
		"modifiedResult": map[string]string{"resultType": resultType, "textResultForLlm": text},
	})
	if err != nil {
		return NoChange
	}
	return string(b)
}

var copilotIDChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func copilotToolPrefix(serverKey string) string {
	return copilotIDChars.ReplaceAllString(serverKey, "-") + "-"
}

// CopilotMCPServer returns the verified registry name for the MCP server that
// toolName belongs to, from the launch's NAV_PILOT_COPILOT_MCP_SOURCES JSON
// map (config key -> registry name, or "" for an unverified server).
//
// Measured against Copilot CLI 1.0.115: MCP tools arrive as
// "<sanitizedServerKey>-<toolName>", replacing each character outside
// [a-zA-Z0-9_-] in the server key with "-". Longest matching prefix wins, and
// colliding prefixes fail closed ("").
func CopilotMCPServer(toolName, rawSources string) string {
	if toolName == "" || rawSources == "" {
		return ""
	}
	var sources map[string]string
	if json.Unmarshal([]byte(rawSources), &sources) != nil || len(sources) == 0 {
		return ""
	}
	byPrefix := make(map[string]string, len(sources))
	ambiguous := map[string]bool{}
	for key, regName := range sources {
		if key == "" {
			continue
		}
		p := copilotToolPrefix(key)
		if prev, seen := byPrefix[p]; seen && prev != regName {
			ambiguous[p] = true
			byPrefix[p] = ""
			continue
		}
		if !ambiguous[p] {
			byPrefix[p] = regName
		}
	}
	bestPrefix, bestName := "", ""
	for p, regName := range byPrefix {
		if len(p) > len(bestPrefix) && len(toolName) > len(p) && strings.HasPrefix(toolName, p) {
			bestPrefix, bestName = p, regName
		}
	}
	return bestName
}
