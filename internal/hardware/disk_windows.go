//go:build windows

package hardware

import "golang.org/x/sys/windows"

// DiskFreeGB returns the free space of the volume holding path.
func DiskFreeGB(path string) float64 {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0
	}
	return float64(free) / GiB
}
