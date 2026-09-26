package e2e

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ptsName grants and unlocks the terminal behind master and returns its path.
func ptsName(master *os.File) (string, error) {
	fd := int(master.Fd())
	for _, req := range []uint{unix.TIOCPTYGRANT, unix.TIOCPTYUNLK} {
		if err := unix.IoctlSetInt(fd, req, 0); err != nil {
			return "", err
		}
	}
	var buf [128]byte
	//lint:ignore SA1019 x/sys has no wrapper that hands TIOCPTYGNAME a 128-byte buffer
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&buf[0]))); errno != 0 {
		return "", errno
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i]), nil
		}
	}
	return string(buf[:]), nil
}
