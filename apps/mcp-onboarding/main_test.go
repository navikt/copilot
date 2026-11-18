package main

import "testing"

func TestSanitizeLogValueStripsCRLF(t *testing.T) {
	if got := sanitizeLogValue("a\r\nb"); got != "ab" {
		t.Fatalf("got %q", got)
	}
}
