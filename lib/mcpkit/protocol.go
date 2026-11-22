package mcpkit

import (
	"encoding/json"
)

// Protocol types named and shaped after go-sdk v1.9.0-pre.1 (mcp/protocol.go,
// mcp/content.go), so a server can later swap mcpkit.X for mcp.X (#2599).

// CallToolParamsRaw holds tools/call params with arguments left undecoded.
type CallToolParamsRaw struct {
	Meta      map[string]any  `json:"_meta,omitempty"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolRequest is a tools/call request as a server handler sees it.
type CallToolRequest struct {
	Params *CallToolParamsRaw
}

// Content is a content block in a tool result. Only TextContent is supported.
type Content interface {
	MarshalJSON() ([]byte, error)
}

// TextContent is a textual content block.
type TextContent struct {
	Text string
	Meta map[string]any
}

// MarshalJSON always writes "text", even when empty, as go-sdk does, and in
// go-sdk's key order so a later move to go-sdk changes nothing on the wire.
func (c *TextContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string         `json:"type"`
		Text string         `json:"text"`
		Meta map[string]any `json:"_meta,omitempty"`
	}{"text", c.Text, c.Meta})
}

// CallToolResult is the result of tools/call.
type CallToolResult struct {
	Meta              map[string]any `json:"_meta,omitempty"`
	Content           []Content      `json:"content"`
	StructuredContent any            `json:"structuredContent,omitempty"`
	IsError           bool           `json:"isError,omitempty"`

	err error
}

// SetError marks the result as a tool error and, when Content is empty, puts
// the error text in a text block.
func (r *CallToolResult) SetError(err error) {
	if len(r.Content) == 0 {
		r.Content = []Content{&TextContent{Text: err.Error()}}
	}
	r.IsError = true
	r.err = err
}

// GetError returns the error passed to SetError.
func (r *CallToolResult) GetError() error { return r.err }
