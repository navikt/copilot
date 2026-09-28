package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VersionCacheFile, when set, keeps the answers to `<client> --version`
// across runs, so a launch does not ask again: opencode takes a quarter of a
// second to say its version, on every launch. An answer is keyed by the
// binary's resolved path, size, modification time, inode and status-change
// time, so an upgrade, a reinstall or an overwrite in place asks again. The CLI sets it; empty (unit tests), every process
// asks.
var VersionCacheFile string

// binaryKey is the cache key for bin, "" when there is no cache or bin cannot
// be found.
func binaryKey(bin string) string {
	if VersionCacheFile == "" {
		return ""
	}
	p, err := exec.LookPath(bin)
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	return fmt.Sprintf("%s|%d|%d|%s", p, fi.Size(), fi.ModTime().UnixNano(), fileChange(fi))
}

func readVersionCacheFile() map[string]string {
	m := map[string]string{}
	if data, err := os.ReadFile(VersionCacheFile); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

func readVersionCache(key string) (string, bool) {
	if key == "" {
		return "", false
	}
	out, ok := readVersionCacheFile()[key]
	return out, ok
}

// writeVersionCache records an answer, dropping earlier answers for the same
// path: those are versions no longer installed.
func writeVersionCache(key, out string) {
	if key == "" {
		return
	}
	m := readVersionCacheFile()
	path := key[:strings.Index(key, "|")+1]
	for k := range m {
		if strings.HasPrefix(k, path) {
			delete(m, k)
		}
	}
	m[key] = out
	data, _ := json.Marshal(m)
	dir := filepath.Dir(VersionCacheFile)
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".client-versions-")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr != nil || cerr != nil || os.Rename(tmp.Name(), VersionCacheFile) != nil {
		os.Remove(tmp.Name())
	}
}
