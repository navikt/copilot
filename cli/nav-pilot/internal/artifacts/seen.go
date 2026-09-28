package artifacts

import (
	"os"
	"path/filepath"
)

// SeenChanged reports whether value differs from what was last recorded under
// name, and records it. A notice about a state (a missing hook, an untested
// client version, the session model) uses it to say so once per state rather
// than on every launch. It shares provider.FirstTime's store: seen-<name> in
// nav-pilot's state directory, here holding the value. When the store cannot
// be written it answers true: an actionable notice may repeat, never vanish.
func SeenChanged(name, value string) bool {
	p := CacheFilePath()
	if p == "" {
		return true
	}
	dir := filepath.Dir(p)
	p = filepath.Join(dir, "seen-"+name)
	if old, err := os.ReadFile(p); err == nil && string(old) == value {
		return false
	}
	_ = os.MkdirAll(dir, 0o700)
	_ = os.WriteFile(p, []byte(value), 0o600)
	return true
}

// LaunchNotices is set on a launch: notices about a state that SeenChanged
// covers are said once per state there. A command the user ran to ask (sync,
// doctor) still says them every time.
var LaunchNotices bool
