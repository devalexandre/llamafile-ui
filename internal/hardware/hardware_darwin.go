//go:build darwin

package hardware

import (
	"golang.org/x/sys/unix"
)

func osName() string {
	if v, err := unix.Sysctl("kern.osproductversion"); err == nil {
		return "macOS " + v
	}
	return "macOS"
}

func memory() (total, available float64) {
	if v, err := unix.SysctlUint64("hw.memsize"); err == nil {
		total = float64(v) / GiB
	}
	// macOS compresses memory aggressively; free pages are not a good signal.
	return total, total * 0.6
}

func cpuName() string {
	v, _ := unix.Sysctl("machdep.cpu.brand_string")
	return v
}

func detectAMD(*Info) bool { return false }
