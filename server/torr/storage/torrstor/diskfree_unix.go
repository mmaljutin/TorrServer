//go:build !windows

package torrstor

import "golang.org/x/sys/unix"

// FreeSpace returns the number of bytes available on the filesystem that
// contains path. It returns -1 if the value cannot be determined (callers
// should treat -1 as "unknown" and allow the operation).
func FreeSpace(path string) int64 {
	if path == "" {
		return -1
	}
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
