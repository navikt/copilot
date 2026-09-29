package source

import "golang.org/x/sys/unix"

const (
	ioctlGetTermios        = unix.TIOCGETA
	ioctlSetTermios        = unix.TIOCSETA
	ioctlSetTermiosFlushIn = unix.TIOCSETAF // tcsetattr(TCSAFLUSH)
)
