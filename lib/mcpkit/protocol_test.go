package mcpkit

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestCallToolResultWire(t *testing.T) {
	ok := &CallToolResult{Content: []Content{&TextContent{}}, StructuredContent: map[string]any{"a": 1}}
	var bad CallToolResult
	bad.SetError(errors.New("boom"))
	for res, want := range map[*CallToolResult]string{
		ok:   `{"content":[{"text":"","type":"text"}],"structuredContent":{"a":1}}`,
		&bad: `{"content":[{"text":"boom","type":"text"}],"isError":true}`,
	} {
		got, err := json.Marshal(res)
		if err != nil || string(got) != want {
			t.Errorf("got %s, %v; want %s", got, err, want)
		}
	}
	if bad.GetError() == nil {
		t.Error("GetError lost the error")
	}
}
