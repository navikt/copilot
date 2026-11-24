package cli

import (
	"strings"
	"testing"
)

func TestCpltManagedDoctorLine(t *testing.T) {
	if got := cpltManagedDoctorLine("/h/.config/cplt/managed.d/nav-pilot.toml", ""); !strings.Contains(got, "managed file /h/.config/cplt/managed.d/nav-pilot.toml") {
		t.Errorf("in use: %q", got)
	}
	if got := cpltManagedDoctorLine("", "/h/.config is group- or world-writable"); !strings.Contains(got, "per-launch flags: /h/.config is group- or world-writable") {
		t.Errorf("fallback: %q", got)
	}
}
