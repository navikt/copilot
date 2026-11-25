package provider

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// cpltManagedFloor is the first cplt release that reads CPLT_MANAGED_CONFIG
// (navikt/cplt#1096, release 2026.11.24-020236-4ea5789). Only this feature is
// gated on it: the release also refuses some existing write grants, so the
// global cplt minimum stays where it is.
const cpltManagedFloor = "2026.11.24-020236"

// CpltManagedEnv names the managed layer file for cplt. cplt does not scan
// managed.d; it reads only the file this names.
const CpltManagedEnv = "CPLT_MANAGED_CONFIG"

// passwdHome is the home directory from the user database, which is the only
// home cplt accepts for the managed layer. A var for tests.
var passwdHome = func() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

// cpltManagedSupported mirrors [FleetGatewayActive] with the managed floor:
// only a parsed stamp at or past it, or a "cplt 0.0.0" dev build, says yes.
func cpltManagedSupported() bool {
	out, err := probeCpltVersion()
	if err != nil {
		return false
	}
	if first, _, _ := strings.Cut(out, "\n"); strings.TrimSpace(first) == "cplt 0.0.0" {
		return true
	}
	stamp := cpltStamp(out)
	return stamp != "" && stamp >= cpltManagedFloor
}

// cpltManagedHeader opens every nav-pilot.toml nav-pilot writes, schema 1
// included. A file without it is someone else's and is never overwritten.
const cpltManagedHeader = "managed_by = \"nav-pilot\"\nschema = 1\n"

// cpltManagedContent is nav-pilot.toml: [FleetGatewayHosts], then the
// approved hosts this launch already passes as --allow-private-domain (the
// pakke waiver and MCP registry grants), never more. Nil when a host is not a
// bare lowercase DNS name, so the launch keeps its flags and writes nothing.
func cpltManagedContent(approved []string) []byte {
	var quoted []string
	for _, h := range append(slices.Clone(FleetGatewayHosts), approved...) {
		if !validManagedHost(h) {
			return nil
		}
		if q := strconv.Quote(h); !slices.Contains(quoted, q) {
			quoted = append(quoted, q)
		}
	}
	return []byte(cpltManagedHeader + "\n[proxy]\nallow_private_domains = [" + strings.Join(quoted, ", ") + "]\n")
}

// validManagedHost is a bare lowercase host name with at least two labels:
// no wildcard, port, IP address or localhost.
func validManagedHost(h string) bool {
	if net.ParseIP(h) != nil || !strings.Contains(h, ".") || len(h) > 253 {
		return false
	}
	return validHostLabels(h)
}

// CpltManagedConfig writes ~/.config/cplt/managed.d/nav-pilot.toml when cplt
// supports it and every precondition holds, and returns its path. Otherwise
// it returns "" and why; the launch then keeps the per-launch flags. It
// writes nothing else in managed.d and rewrites only on a content change.
// approved is the private-domain hosts the launch's proposal flags carry.
func CpltManagedConfig(approved []string) (path, reason string) { return cpltManaged(approved, true) }

// CpltManagedCheck is [CpltManagedConfig] without writing, for doctor: the
// path when the file is present and current, else what launch would do or
// why it cannot.
func CpltManagedCheck() (path, reason string) { return cpltManaged(nil, false) }

func cpltManaged(approved []string, write bool) (path, reason string) {
	if !cpltManagedSupported() {
		return "", "cplt is older than " + cpltManagedFloor + " or its version cannot be read"
	}
	want := cpltManagedContent(approved)
	if want == nil {
		return "", "an approved host is not a plain host name"
	}
	path, err := ensureCpltManagedFile(want, write)
	if err != nil {
		return "", err.Error()
	}
	return path, ""
}

func ensureCpltManagedFile(want []byte, write bool) (string, error) {
	home, err := passwdHome()
	if err != nil || home == "" || !filepath.IsAbs(home) {
		return "", errors.New("cannot read your home directory from the user database")
	}
	home = filepath.Clean(home)
	if filepath.Clean(os.Getenv("HOME")) != home {
		return "", fmt.Errorf("$HOME is not %s, so cplt would ignore the managed file", home)
	}
	uid := os.Getuid()
	// Every directory from home down must be owned by you or root, not group-
	// or world-writable and not a symlink. Missing ones are created 0755;
	// existing ones are never chmodded or chowned.
	dir := home
	if err := checkManagedDir(dir, uid, false); err != nil {
		return "", err
	}
	for _, name := range []string{".config", "cplt", "managed.d"} {
		dir = filepath.Join(dir, name)
		if !write {
			if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("%s is missing; the next launch writes it", filepath.Join(home, ".config", "cplt", "managed.d", "nav-pilot.toml"))
			}
		} else if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("cannot create %s: %v", dir, err)
		}
		if err := checkManagedDir(dir, uid, true); err != nil {
			return "", err
		}
	}
	path := filepath.Join(dir, "nav-pilot.toml")
	if fi, err := os.Lstat(path); err == nil {
		st, _ := fi.Sys().(*syscall.Stat_t)
		if !fi.Mode().IsRegular() || st == nil || int(st.Uid) != uid {
			return "", fmt.Errorf("%s is not a regular file you own; nav-pilot leaves it alone", path)
		}
		if !fileStartsWith(path, []byte(cpltManagedHeader)) {
			return "", fmt.Errorf("%s was not written by nav-pilot; nav-pilot leaves it alone", path)
		}
		// Doctor passes no hosts: the approved set varies per launch, so a
		// sound file nav-pilot wrote is current enough to report.
		if fi.Mode().Perm()&0o022 == 0 && st.Nlink == 1 && (!write || fileHolds(path, want)) {
			return path, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if !write {
		return "", fmt.Errorf("%s is missing or stale; the next launch writes it", path)
	}
	if err := writeAtomic(dir, path, want); err != nil {
		return "", fmt.Errorf("cannot write %s: %v", path, err)
	}
	return path, nil
}

func checkManagedDir(dir string, uid int, noSymlink bool) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		if noSymlink {
			return fmt.Errorf("%s is a symlink, which cplt refuses", dir)
		}
		if fi, err = os.Stat(dir); err != nil {
			return err
		}
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !fi.IsDir() || !ok {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if int(st.Uid) != uid && st.Uid != 0 {
		return fmt.Errorf("%s is not owned by you or root", dir)
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s is group- or world-writable", dir)
	}
	return nil
}

// writeAtomic writes via a temp file in the same directory, fsync and rename,
// so concurrent nav-pilot runs never leave a partial file for cplt to read.
func writeAtomic(dir, path string, data []byte) error {
	f, err := os.CreateTemp(dir, ".nav-pilot.toml.*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func fileStartsWith(path string, prefix []byte) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	have := make([]byte, len(prefix))
	_, err = io.ReadFull(f, have)
	return err == nil && bytes.Equal(have, prefix)
}

// fileHolds reports whether path holds exactly want, reading at most 64 KiB:
// anything larger differs and is rewritten.
func fileHolds(path string, want []byte) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	have, err := io.ReadAll(io.LimitReader(f, 64<<10))
	return err == nil && bytes.Equal(have, want)
}
