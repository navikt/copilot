package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// validatorContract is testdata/validator_contract.json, also run by
// fleet-gateway's validator_contract_test.go (#2894).
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
	raw, err := os.ReadFile("testdata/validator_contract.json")
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
		if got := validateTools([]string{name}) == nil; got != tc.Valid {
			t.Errorf("tool %q: valid=%v, contract says %v", tc.Name, got, tc.Valid)
		}
	}
	for _, tc := range c.Upstreams {
		if got := validateUpstreamURL(tc.URL, tc.NS) == nil; got != tc.Valid {
			t.Errorf("upstream %q ns %q: valid=%v, contract says %v", tc.URL, tc.NS, got, tc.Valid)
		}
	}
}
