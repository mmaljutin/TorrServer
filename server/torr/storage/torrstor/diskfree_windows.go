//go:build windows

package torrstor

import "golang.org/x/sys/windows"

// FreeSpace returns the number of bytes available to the caller on the volume
// that contains path. It returns -1 if the value cannot be determined (callers
// should treat -1 as "unknown" and allow the operation).
func FreeSpace(path string) int64 {
	if path == "" {
		return -1
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return -1
	}
	var freeForCaller, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeForCaller, &total, &totalFree); err != nil {
		return -1
	}
	return int64(freeForCaller)
}
