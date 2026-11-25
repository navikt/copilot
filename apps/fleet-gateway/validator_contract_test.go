package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// validatorContract is fleet-control's testdata/validator_contract.json, the
// rules fleet-control validates with before it publishes a snapshot (#2894).
type validatorContract struct {
	Tools []struct {
		Name   string `json:"name"`
		Repeat int    `json:"repeat"`
		Valid  bool   `json:"valid"`
	} `json:"tools"`
	Upstreams []struct {
		URL   string `json:"url"`
		NS    string `json:"ns"`
		Valid bool   `json:"valid"`
	} `json:"upstreams"`
}

func TestValidatorContract(t *testing.T) {
	raw, err := os.ReadFile("../fleet-control/testdata/validator_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var c validatorContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	for _, tc := range c.Tools {
		name := tc.Name
		if tc.Repeat > 0 {
			name = strings.Repeat("x", tc.Repeat)
		}
		if got := applyToolPolicy(&PilotRoute{Route: "r", Tools: []string{name}}) == nil; got != tc.Valid {
			t.Errorf("tool %q: valid=%v, contract says %v", tc.Name, got, tc.Valid)
		}
	}
	for _, tc := range c.Upstreams {
		if got := func() bool { _, err := validateInternalUpstreamURL(tc.URL, tc.NS); return err == nil }(); got != tc.Valid {
			t.Errorf("upstream %q ns %q: valid=%v, contract says %v", tc.URL, tc.NS, got, tc.Valid)
		}
	}
}
