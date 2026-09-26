//go:build darwin || linux

package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// ptyRunName is the name the test binary answers to when it runs a command on
// a terminal: binary() links it next to nav-pilot, and TestMain checks
// os.Args[0]. testscript's own ttyin puts a terminal on stdin only, and
// nav-pilot asks for stdin AND stdout, so its prompts need this.
const ptyRunName = "pty-run"

// ptyRun is `pty-run [-wait RE -send KEYS]... -- PROG ARGS...`: PROG runs with
// stdin, stdout and stderr on one pseudo-terminal, as its controlling
// terminal. Everything it writes is copied to stdout. Each -wait pattern is
// awaited in turn, and when it has appeared its KEYS are typed (Go escapes,
// so '\x03' is Ctrl-C). It exits with PROG's code, 128+n for a signal, or 124
// after 20 seconds.
func ptyRun(args []string) int {
	type step struct {
		wait *regexp.Regexp
		send string
	}
	var steps []step
	for len(args) >= 4 && args[0] == "-wait" && args[2] == "-send" {
		keys, err := strconv.Unquote(`"` + args[3] + `"`)
		if err != nil {
			fmt.Fprintln(os.Stderr, "pty-run: -send:", err)
			return 2
		}
		steps = append(steps, step{regexp.MustCompile(args[1]), keys})
		args = args[4:]
	}
	if len(args) < 2 || args[0] != "--" {
		fmt.Fprintln(os.Stderr, "usage: pty-run [-wait RE -send KEYS]... -- PROG ARGS...")
		return 2
	}
	args = args[1:]

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pty-run:", err)
		return 2
	}
	defer master.Close()
	name, err := ptsName(master)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pty-run:", err)
		return 2
	}
	tty, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pty-run:", err)
		return 2
	}
	// A terminal of zero rows and columns draws nothing.
	if err := unix.IoctlSetWinsize(int(tty.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 40, Col: 100}); err != nil {
		fmt.Fprintln(os.Stderr, "pty-run:", err)
		return 2
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "pty-run:", err)
		return 2
	}
	tty.Close()

	var seen bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])
				seen.Write(buf[:n])
				// Answer the queries a terminal answers, or the TUI waits for
				// them: background colour, and cursor position.
				if bytes.Contains(buf[:n], []byte("\x1b]11;?")) {
					io.WriteString(master, "\x1b]11;rgb:0000/0000/0000\x1b\\")
				}
				if bytes.Contains(buf[:n], []byte("\x1b[6n")) {
					io.WriteString(master, "\x1b[1;1R")
				}
				for len(steps) > 0 && steps[0].wait.Match(seen.Bytes()) {
					// A moment for the prompt to finish drawing and read raw keys.
					time.Sleep(200 * time.Millisecond)
					io.WriteString(master, steps[0].send)
					seen.Reset()
					steps = steps[1:]
				}
			}
			if err != nil {
				return
			}
		}
	}()

	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case err = <-waited:
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-waited
		fmt.Fprintln(os.Stderr, "pty-run: timed out")
		return 124
	}
	select {
	case <-done:
	case <-time.After(time.Second):
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if st, ok := ee.Sys().(syscall.WaitStatus); ok && st.Signaled() {
			return 128 + int(st.Signal())
		}
		return ee.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pty-run:", err)
		return 2
	}
	return 0
}
