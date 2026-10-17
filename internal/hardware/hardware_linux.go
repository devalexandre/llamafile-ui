//go:build linux

package hardware

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func osName() string {
	f, err := os.Open("/etc/os-release")
	if err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
				return strings.Trim(v, `"`)
			}
		}
	}
	return "Linux"
}

func memory() (total, available float64) {
	for _, line := range strings.Split(readFile("/proc/meminfo"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kb, _ := strconv.ParseFloat(fields[1], 64)
		switch fields[0] {
		case "MemTotal:":
			total = kb / (1024 * 1024)
		case "MemAvailable:":
			available = kb / (1024 * 1024)
		}
	}
	return
}

func cpuName() string {
	for _, line := range strings.Split(readFile("/proc/cpuinfo"), "\n") {
		if strings.HasPrefix(line, "model name") {
			if _, v, ok := strings.Cut(line, ":"); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func detectAMD(info *Info) bool {
	cards, _ := filepath.Glob("/sys/class/drm/card[0-9]*/device")
	for _, dev := range cards {
		if readFile(filepath.Join(dev, "vendor")) != "0x1002" {
			continue
		}
		bytes, _ := strconv.ParseFloat(readFile(filepath.Join(dev, "mem_info_vram_total")), 64)
		vram := bytes / GiB
		// APUs report a small carve-out; treat them as CPU-only.
		if vram < 2 || vram <= info.VRAMGB {
			continue
		}
		info.VRAMGB = vram
		info.GPUName = readFile(filepath.Join(dev, "product_name"))
		if info.GPUName == "" {
			info.GPUName = fmt.Sprintf("AMD Radeon (%.0f GB)", vram)
		}
	}
	if info.VRAMGB <= 0 {
		return false
	}
	info.GPUVendor = "amd"
	info.GPUBandwidthGBps = 288
	if info.VRAMGB >= 16 {
		info.GPUBandwidthGBps = 576
	}
	return true
}
