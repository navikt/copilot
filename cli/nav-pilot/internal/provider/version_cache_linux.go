package provider

import (
	"fmt"
	"os"
	"syscall"
)

// fileChange is the inode and status-change time: a binary overwritten in
// place with its size and mtime preserved (cp -p, rsync -a) still changes
// them.
func fileChange(fi os.FileInfo) string {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d|%d.%d", st.Ino, st.Ctim.Sec, st.Ctim.Nsec)
}
