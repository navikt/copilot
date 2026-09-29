package source

import "golang.org/x/sys/unix"

const (
	ioctlGetTermios        = unix.TCGETS
	ioctlSetTermios        = unix.TCSETS
	ioctlSetTermiosFlushIn = unix.TCSETSF // tcsetattr(TCSAFLUSH)
)
