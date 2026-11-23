package cli

import (
	"strings"
	"testing"
)

func TestBudgetOpensLink(t *testing.T) {
	isolatedRun(t)
	var opened string
	orig := openBrowserFn
	openBrowserFn = func(u string) error { opened = u; return nil }
	t.Cleanup(func() { openBrowserFn = orig })

	out, _ := captureRun(t, func() {
		if err := run([]string{"budget", "--open"}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, budgetURL) || opened != budgetURL {
		t.Errorf("want link printed and opened, got opened=%q out=%q", opened, out)
	}
}
