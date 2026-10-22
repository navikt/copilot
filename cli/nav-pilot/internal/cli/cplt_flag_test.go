package cli

import (
	"strings"
	"testing"
)

func TestValidateCpltArg(t *testing.T) {
	for _, v := range []string{"--allow-clipboard", "--allow-port=8080", "--preset=permissive"} {
		if err := validateCpltArg(v); err != nil {
			t.Errorf("%q refused: %v", v, err)
		}
	}
	for _, v := range []string{"--", "--agent", "--agent=copilot", "8080", "-d", "--project-dir=/x", "--yes", "--no-audit"} {
		if err := validateCpltArg(v); err == nil {
			t.Errorf("%q accepted", v)
		}
	}
}

// A value as its own word gets the "=" hint, not "unknown command".
func TestCpltValueAsSeparateWord(t *testing.T) {
	err := run([]string{"--cplt", "--pass-env", "FOO"})
	if err == nil || !strings.Contains(err.Error(), "--cplt --pass-env=FOO") {
		t.Errorf("err = %v", err)
	}
}
