package survey

import (
	"testing"
)

func TestMergeSmallChannels(t *testing.T) {
	got := MergeSmallChannels(map[string]int{"web": 5, "copilot": 2, "opencode": 1})
	if len(got) != 2 || got["web"] != 5 || got["annet"] != 3 {
		t.Errorf("got %v", got)
	}
}
