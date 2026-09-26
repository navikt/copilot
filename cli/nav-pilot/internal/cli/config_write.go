package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
)

// Every change to config.toml goes through this file: `config set`/`unset`,
// `config setup`, `config init`, the settings page and the commands that
// persist a key for the user (install --source, alpha local).
//
// The edit is line-based so comments and layout survive, but the lines are
// found by the TOML parser rather than by pattern: a statement is the shortest
// run of lines that parses on its own, so a quoted key ("model" = …), a value
// spanning lines, or a "[" inside a multi-line array cannot fool it. The result
// is parsed again and compared with the original before anything is written:
// the key must hold the new value at the top level and every other value must
// be unchanged, or nothing is written.

// errNothingWritten marks a refusal that left the config file untouched.
var errNothingWritten = errors.New("nothing was written")

// configStatement is one top-level `key = value` statement: lines[start..end].
type configStatement struct {
	start, end int
	key        string // the one top-level key it defines
}

// configLayout is what the parser says about each line before the first
// table header.
type configLayout struct {
	statements []configStatement
	commented  map[string]int // key -> first "# key = value" comment line
	regionEnd  int            // index of the first table header, or len(lines)
}

func decodesTo(s string) (map[string]any, bool) {
	m := map[string]any{}
	if _, err := toml.Decode(s, &m); err != nil {
		return nil, false
	}
	return m, true
}

func singleKey(m map[string]any) string {
	if len(m) != 1 {
		return ""
	}
	for k := range m {
		return k
	}
	return ""
}

// layoutConfig walks the top-level region statement by statement. Because it
// advances whole statements at a time it is never inside a multi-line string
// or array, so a line starting with "[" is a table header.
func layoutConfig(lines []string) (configLayout, error) {
	l := configLayout{commented: map[string]int{}, regionEnd: len(lines)}
	for i := 0; i < len(lines); {
		t := strings.TrimSpace(lines[i])
		switch {
		case t == "":
			i++
			continue
		case strings.HasPrefix(t, "#"):
			body := strings.TrimSpace(strings.TrimLeft(t, "#"))
			if m, ok := decodesTo(body); ok {
				if k := singleKey(m); k != "" {
					if _, seen := l.commented[k]; !seen {
						l.commented[k] = i
					}
				}
			}
			i++
			continue
		case strings.HasPrefix(t, "["):
			l.regionEnd = i
			return l, nil
		}
		j := i
		var m map[string]any
		for ; j < len(lines); j++ {
			var ok bool
			if m, ok = decodesTo(strings.Join(lines[i:j+1], "\n")); ok {
				break
			}
		}
		if j == len(lines) {
			return l, fmt.Errorf("line %d does not parse", i+1)
		}
		l.statements = append(l.statements, configStatement{start: i, end: j, key: singleKey(m)})
		i = j + 1
	}
	return l, nil
}

// inlineComment returns the trailing "  # …" of a one-line statement, spacing
// included, or "". The comment starts at the first '#' the value does not need.
func inlineComment(line string) string {
	full, ok := decodesTo(line)
	if !ok {
		return ""
	}
	for p := strings.IndexByte(line, '#'); p >= 0; {
		if m, ok := decodesTo(line[:p]); ok && reflect.DeepEqual(m, full) {
			return line[len(strings.TrimRight(line[:p], " \t")):]
		}
		next := strings.IndexByte(line[p+1:], '#')
		if next < 0 {
			break
		}
		p += 1 + next
	}
	return ""
}

func leadingSpace(s string) string {
	return s[:len(s)-len(strings.TrimLeft(s, " \t"))]
}

// editTopLevelKey returns content with the top-level key set to tomlVal, or
// removed when tomlVal is "". replaces names a retired key the new one takes
// the place of (agent → client), or "". It refuses, and the caller writes
// nothing, when the original does not parse or the result is not exactly the
// original with that one change.
func editTopLevelKey(content, key, tomlVal, replaces string) (string, error) {
	before, ok := decodesTo(content)
	if !ok {
		_, err := toml.Decode(content, &map[string]any{})
		return "", fmt.Errorf("the file does not parse, so nav-pilot will not edit it: %v\n\nFix it by hand; nav-pilot config validate shows where", err)
	}
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if content == "" {
		lines = nil
	}
	layout, err := layoutConfig(lines)
	if err != nil {
		return "", err
	}

	find := func(k string) *configStatement {
		for i := range layout.statements {
			if layout.statements[i].key == k {
				return &layout.statements[i]
			}
		}
		return nil
	}
	splice := func(start, end int, repl ...string) {
		lines = append(lines[:start], append(repl, lines[end+1:]...)...)
	}

	target, retired := find(key), (*configStatement)(nil)
	if replaces != "" {
		retired = find(replaces)
	}
	if target == nil && tomlVal != "" {
		target, retired = retired, nil // the new key takes the retired one's line
	}
	// Splice the later statement first so the earlier one's indices hold.
	if retired != nil && target != nil && retired.start > target.start {
		splice(retired.start, retired.end)
		retired = nil
	}
	switch {
	case tomlVal == "" && target == nil:
	case tomlVal == "":
		splice(target.start, target.end)
	case target != nil:
		comment := ""
		if target.start == target.end {
			comment = inlineComment(lines[target.start])
		}
		splice(target.start, target.end, leadingSpace(lines[target.start])+key+" = "+tomlVal+comment)
	default:
		newLine := key + " = " + tomlVal
		if i, ok := layout.commented[key]; ok {
			lines[i] = newLine
			break
		}
		at := layout.regionEnd
		if n := len(layout.statements); n > 0 {
			at = layout.statements[n-1].end + 1
		}
		insert := []string{newLine}
		if at == layout.regionEnd && at < len(lines) {
			insert = append(insert, "") // keep a blank line above the table header
		}
		lines = append(lines[:at], append(insert, lines[at:]...)...)
	}
	if retired != nil && tomlVal != "" {
		splice(retired.start, retired.end)
	}
	out := strings.Join(lines, "\n") + "\n"

	// The result must be the original with exactly this change.
	want := make(map[string]any, len(before)+1)
	for k, v := range before {
		want[k] = v
	}
	delete(want, key)
	if replaces != "" && tomlVal != "" {
		delete(want, replaces)
	}
	if tomlVal != "" {
		v, ok := decodesTo(key + " = " + tomlVal)
		if !ok {
			return "", fmt.Errorf("%s = %s is not valid TOML", key, tomlVal)
		}
		want[key] = v[key]
	}
	after, ok := decodesTo(out)
	if !ok {
		return "", fmt.Errorf("the edited file would not parse")
	}
	if !reflect.DeepEqual(want, after) {
		return "", fmt.Errorf("%s did not end up as a top-level key with the new value", key)
	}
	return out, nil
}

// updateConfigKey sets (or, with tomlVal "", removes) one top-level key in the
// config file through editTopLevelKey and writeConfigFile. A missing file is
// created with version = 1.
func updateConfigKey(key, tomlVal string) error {
	path := configPath()
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if tomlVal == "" {
			return nil
		}
		if key != "version" {
			data = []byte("version = 1\n")
		}
	case err != nil:
		return fmt.Errorf("reading config: %w", err)
	}
	replaces := ""
	for old, next := range renamedConfigKeys {
		if next == key {
			replaces = old
		}
	}
	out, err := editTopLevelKey(string(data), key, tomlVal, replaces)
	if err != nil {
		return fmt.Errorf("could not change %s in %s, %w: %v", key, path, errNothingWritten, err)
	}
	return writeConfigFile(path, []byte(out))
}

// writeConfigFile replaces the config file with content: it must parse, the
// previous file is kept as config.toml.bak, and the new one lands by rename
// from a temp file in the same directory, so a crash or Ctrl-C leaves either
// the old file or the new one. The file keeps its mode; a new one is 0600.
func writeConfigFile(path string, content []byte) error {
	if _, ok := decodesTo(string(content)); !ok {
		return fmt.Errorf("internal error: the new %s would not parse, %w", path, errNothingWritten)
	}
	// Write through a symlink (a dotfiles checkout) rather than replacing it.
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	mode := os.FileMode(0o600)
	old, err := os.ReadFile(path)
	switch {
	case err == nil:
		if bytes.Equal(old, content) {
			return nil
		}
		if fi, err := os.Stat(path); err == nil {
			mode = fi.Mode().Perm()
		}
		if err := writeFileAtomic(path+".bak", old, mode); err != nil {
			return fmt.Errorf("backing up %s, %w: %w", path, errNothingWritten, err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("reading config: %w", err)
	default:
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
	}
	if err := writeFileAtomic(path, content, mode); err != nil {
		return fmt.Errorf("writing %s, %w: %w", path, errNothingWritten, err)
	}
	return nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Chmod(mode)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// tomlString quotes s as a TOML basic string. Go's %q is not TOML: it writes
// \x01 for a control character, which TOML rejects.
func tomlString(s string) string {
	var b bytes.Buffer
	_ = toml.NewEncoder(&b).Encode(map[string]string{"k": s})
	return strings.TrimSuffix(strings.TrimPrefix(b.String(), "k = "), "\n")
}
