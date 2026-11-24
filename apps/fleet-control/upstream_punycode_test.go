package main

import "testing"

func TestUpstreamURLPunycodeMessage(t *testing.T) {
	if err := validateUpstreamURL("http://xn--nv-fia.copilot/", "copilot"); err == nil || err.Error() != "punycode hosts are not allowed" {
		t.Fatalf("got %v", err)
	}
}
