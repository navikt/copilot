package source

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

// quietStdin turns echo off on a terminal stdin while nav-pilot fetches, so a
// key pressed meanwhile neither shows nor moves the cursor off the spinner
// line. The returned func turns echo back on and discards what was typed
// but not read (tcsetattr with TCSAFLUSH), so an Enter does not reach the
// prompt that follows (#1279, #1287). It checks stdin, not stderr: the keys
// arrive on stdin whether or not the spinner is drawn. A Ctrl-C or SIGTERM
// meanwhile restores echo before nav-pilot dies of it. On anything but a
// terminal it does nothing.
func quietStdin() (restore func()) {
	fd := int(os.Stdin.Fd())
	orig, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return func() {}
	}
	quiet := *orig
	quiet.Lflag &^= unix.ECHO | unix.ECHONL // ECHONL echoes Enter even without ECHO
	if unix.IoctlSetTermios(fd, ioctlSetTermios, &quiet) != nil {
		return func() {}
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case s := <-sigs:
			_ = unix.IoctlSetTermios(fd, ioctlSetTermios, orig)
			signal.Stop(sigs)
			_ = syscall.Kill(os.Getpid(), s.(syscall.Signal))
		case <-stop:
		}
	}()
	return func() {
		signal.Stop(sigs)
		close(stop)
		<-stopped
		_ = unix.IoctlSetTermios(fd, ioctlSetTermiosFlushIn, orig)
	}
}
