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

func TestBudgetPrintsLinkWithoutOpening(t *testing.T) {
	isolatedRun(t)
	opened := false
	orig := openBrowserFn
	openBrowserFn = func(string) error { opened = true; return nil }
	t.Cleanup(func() { openBrowserFn = orig })

	out, _ := captureRun(t, func() {
		if err := run([]string{"budget"}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, budgetURL) || opened {
		t.Errorf("want link printed and no browser, got opened=%v out=%q", opened, out)
	}
}
