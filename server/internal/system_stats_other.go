//go:build !windows

package internal

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	linuxCPUStateMu sync.Mutex
	linuxLastTotal  uint64
	linuxLastIdle   uint64
	linuxHasCPURead bool
)

func populateWindowsSystemStats(result map[string]any) {}

// linuxCPUPercent reads Linux's cumulative scheduler counters directly from
// /proc/stat. This avoids depending on procps/top output formatting and works
// on Raspberry Pi OS as well as normal Debian Linux hosts.
func linuxCPUPercent() (float64, bool) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, false
	}
	line := strings.SplitN(string(raw), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, false
	}

	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, parseErr := strconv.ParseUint(field, 10, 64)
		if parseErr != nil {
			return 0, false
		}
		values = append(values, value)
	}
	if len(values) < 4 {
		return 0, false
	}

	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4] // iowait is idle time for utilization purposes
	}

	linuxCPUStateMu.Lock()
	defer linuxCPUStateMu.Unlock()
	if !linuxHasCPURead {
		linuxLastTotal, linuxLastIdle, linuxHasCPURead = total, idle, true
		return 0, false
	}
	if total < linuxLastTotal || idle < linuxLastIdle {
		linuxLastTotal, linuxLastIdle = total, idle
		return 0, false
	}
	deltaTotal := total - linuxLastTotal
	deltaIdle := idle - linuxLastIdle
	linuxLastTotal, linuxLastIdle = total, idle
	if deltaTotal == 0 {
		return 0, false
	}
	busy := deltaTotal
	if deltaIdle < busy {
		busy -= deltaIdle
	} else {
		busy = 0
	}
	percent := float64(busy) / float64(deltaTotal) * 100
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return percent, true
}
