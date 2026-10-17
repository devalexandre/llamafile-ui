//go:build !windows

package hardware

import "golang.org/x/sys/unix"

// DiskFreeGB returns the free space of the filesystem holding path.
func DiskFreeGB(path string) float64 {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0
	}
	return float64(st.Bavail) * float64(st.Bsize) / GiB
}
