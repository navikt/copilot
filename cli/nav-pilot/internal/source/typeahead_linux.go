package source

import (
	"os"

	"golang.org/x/sys/unix"
)

// dropTypeahead discards what was typed on a terminal stdin but not yet read,
// as tcflush(TCIFLUSH) does. On anything but a terminal the ioctl fails and
// nothing happens.
func dropTypeahead() {
	_ = unix.IoctlSetInt(int(os.Stdin.Fd()), unix.TCFLSH, unix.TCIFLUSH)
}
