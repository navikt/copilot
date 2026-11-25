package main

import (
	"encoding/json"
	"testing"
)

func TestDecodeToolRiskLenient(t *testing.T) {
	raw := []byte(`{"routes":[{"name":"a","tool_risk":{"get":"read","put":7}},{"name":"b","tool_risk":"bogus"}]}`)
	var snap routesSnapshot
	_ = json.Unmarshal(raw, &snap) // the bogus tool_risk is not part of the struct
	decodeToolRisk(raw, &snap)
	if got := snap.Routes[0].toolRisk; got["get"] != "read" || len(got) != 1 {
		t.Fatalf("route a tool_risk = %v, want only get=read", got)
	}
	if snap.Routes[1].toolRisk != nil {
		t.Fatalf("route b got tool_risk from a malformed snapshot")
	}
}
