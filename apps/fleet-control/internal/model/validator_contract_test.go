package model

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// contractCase is one row of testdata/validator_contract.json, also run by
// fleet-gateway's internal/routing validator_contract_test.go (#2894). Valid is
// both sides' verdict; Control and Gateway override one side and need a Reason.
type contractCase struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	NS      string `json:"ns"`
	App     string `json:"app"`
	Repeat  int    `json:"repeat"`
	Valid   bool   `json:"valid"`
	Control *bool  `json:"control"`
	Gateway *bool  `json:"gateway"`
	Reason  string `json:"reason"`
}

func (c contractCase) want() bool {
	if c.Control != nil {
		return *c.Control
	}
	return c.Valid
}

func TestValidatorContract(t *testing.T) {
	raw, err := os.ReadFile("testdata/validator_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Tools     []contractCase `json:"tools"`
		Upstreams []contractCase `json:"upstreams"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Tools) == 0 || len(c.Upstreams) == 0 {
		t.Fatal("contract has no cases")
	}
	for _, tc := range append(append([]contractCase{}, c.Tools...), c.Upstreams...) {
		if (tc.Control != nil || tc.Gateway != nil) && tc.Reason == "" {
			t.Errorf("%q%q: a side override needs a reason", tc.Name, tc.URL)
		}
	}
	for _, tc := range c.Tools {
		name := tc.Name
		if tc.Repeat > 0 {
			name = strings.Repeat("x", tc.Repeat)
		}
		if got := ValidateTools([]string{name}) == nil; got != tc.want() {
			t.Errorf("tool %q: valid=%v, contract says %v", tc.Name, got, tc.want())
		}
	}
	for _, tc := range c.Upstreams {
		app := tc.App
		if app == "" {
			app = "app"
		}
		if got := validateUpstreamURL(tc.URL, tc.NS, app) == nil; got != tc.want() {
			t.Errorf("upstream %q ns %q app %q: valid=%v, contract says %v", tc.URL, tc.NS, app, got, tc.want())
		}
	}
}
