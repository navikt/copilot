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
}

type wirePayload struct {
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
	p := Payload{SessionID: w.SessionID, ToolName: w.ToolName, ToolArgs: w.ToolArgs}
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
