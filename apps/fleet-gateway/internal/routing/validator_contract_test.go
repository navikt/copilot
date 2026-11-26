package routing

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// contractCase is one row of fleet-control's validator_contract.json, the
// rules fleet-control validates with before it publishes a snapshot (#2894).
// Valid is both sides' verdict; Control and Gateway override one side and need
// a Reason.
type contractCase struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	NS      string `json:"ns"`
	Repeat  int    `json:"repeat"`
	Valid   bool   `json:"valid"`
	Control *bool  `json:"control"`
	Gateway *bool  `json:"gateway"`
	Reason  string `json:"reason"`
}

func (c contractCase) want() bool {
	if c.Gateway != nil {
		return *c.Gateway
	}
	return c.Valid
}

func TestValidatorContract(t *testing.T) {
	raw, err := os.ReadFile("../../../fleet-control/internal/model/testdata/validator_contract.json")
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
		if got := ApplyToolPolicy(&Route{Route: "r", Tools: []string{name}}) == nil; got != tc.want() {
			t.Errorf("tool %q: valid=%v, contract says %v", tc.Name, got, tc.want())
		}
	}
	for _, tc := range c.Upstreams {
		if _, err := ValidateUpstreamURL(tc.URL, tc.NS); (err == nil) != tc.want() {
			t.Errorf("upstream %q ns %q: valid=%v, contract says %v", tc.URL, tc.NS, err == nil, tc.want())
		}
	}
}
