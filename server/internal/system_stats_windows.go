//go:build windows

package internal

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type ecMemoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

type ecFileTime struct{ LowDateTime, HighDateTime uint32 }

var (
	ecKernel32      = syscall.NewLazyDLL("kernel32.dll")
	ecGlobalMem     = ecKernel32.NewProc("GlobalMemoryStatusEx")
	ecDiskFree      = ecKernel32.NewProc("GetDiskFreeSpaceExW")
	ecSystemTimes   = ecKernel32.NewProc("GetSystemTimes")
	ecCPUStateMu    sync.Mutex
	ecLastIdle      uint64
	ecLastKernel    uint64
	ecLastUser      uint64
	ecHasCPUReading bool
)

func ecFileTimeUint64(v ecFileTime) uint64 { return uint64(v.HighDateTime)<<32 | uint64(v.LowDateTime) }

// linuxCPUPercent is referenced from the shared developer system-stats path.
// The Linux implementation is excluded from Windows builds, so keep a tiny
// Windows stub here and let populateWindowsSystemStats provide the CPU value.
func linuxCPUPercent() (float64, bool) { return 0, false }

func populateWindowsSystemStats(result map[string]any) {
	// Memory is read directly from kernel32 so a PowerShell/CIM failure cannot
	// take the whole System page down.
	mem := ecMemoryStatusEx{Length: uint32(unsafe.Sizeof(ecMemoryStatusEx{}))}
	if ok, _, _ := ecGlobalMem.Call(uintptr(unsafe.Pointer(&mem))); ok != 0 && mem.TotalPhys > 0 {
		total := int64(mem.TotalPhys)
		used := int64(mem.TotalPhys - mem.AvailPhys)
		result["ram_total_bytes"] = total
		result["ram_used_bytes"] = used
		result["ram_total"] = humanBytes(total)
		result["ram_used"] = humanBytes(used)
	}

	// Use the Windows system drive rather than assuming C:.
	drive := strings.TrimSpace(os.Getenv("SystemDrive"))
	if drive == "" {
		drive = "C:"
	}
	root, err := syscall.UTF16PtrFromString(drive + `\`)
	if err == nil {
		var freeAvail, totalBytes, totalFree uint64
		if ok, _, _ := ecDiskFree.Call(
			uintptr(unsafe.Pointer(root)),
			uintptr(unsafe.Pointer(&freeAvail)),
			uintptr(unsafe.Pointer(&totalBytes)),
			uintptr(unsafe.Pointer(&totalFree)),
		); ok != 0 && totalBytes > 0 {
			total := int64(totalBytes)
			used := int64(totalBytes - totalFree)
			result["storage_total_bytes"] = total
			result["storage_used_bytes"] = used
			result["storage_total"] = humanBytes(total)
			result["storage_used"] = humanBytes(used)
		}
	}

	// GetSystemTimes is native and lightweight. On the first sample we take a
	// short second reading so CPU does not remain at a fake 0% until refresh.
	if cpu, ok := windowsCPUPercent(); ok {
		result["cpu_percent"] = cpu
	} else {
		time.Sleep(120 * time.Millisecond)
		if cpu, ok := windowsCPUPercent(); ok {
			result["cpu_percent"] = cpu
		}
	}

	// netstat -e is available on Windows and does not require PowerShell.
	if out, err := exec.Command("netstat", "-e").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Fields(line)
			if len(f) >= 3 && strings.EqualFold(f[0], "Bytes") {
				rx, _ := strconv.ParseInt(strings.ReplaceAll(f[1], ",", ""), 10, 64)
				tx, _ := strconv.ParseInt(strings.ReplaceAll(f[2], ",", ""), 10, 64)
				if rx >= 0 {
					result["network_rx_bytes"] = rx
				}
				if tx >= 0 {
					result["network_tx_bytes"] = tx
				}
				break
			}
		}
	}

	// Link speed is optional. Keep this command isolated so it cannot affect
	// RAM/CPU/Storage if NetworkManager/CIM is unavailable.
	if value := windowsPowerShellValue(`$a=Get-NetAdapter -ErrorAction SilentlyContinue|Where-Object Status -eq 'Up'|Sort-Object LinkSpeed -Descending|Select-Object -First 1;if($a){$a.LinkSpeed}`); value != "" {
		result["network_speed"] = value
		fields := strings.Fields(value)
		if len(fields) > 0 {
			n, _ := strconv.ParseFloat(strings.ReplaceAll(fields[0], ",", ""), 64)
			lower := strings.ToLower(value)
			if strings.Contains(lower, "gbps") {
				n *= 1000
			}
			if strings.Contains(lower, "kbps") {
				n /= 1000
			}
			if n > 0 {
				result["network_link_mbps"] = n
			}
		}
	}

	// Generic GPU name/memory is another independent optional probe. NVIDIA
	// utilization/memory is filled later by populateGPUUtilization when present.
	if result["gpu_name"] == "Unavailable" {
		if value := windowsPowerShellValue(`$g=Get-CimInstance Win32_VideoController -ErrorAction SilentlyContinue|Where-Object {$_.Name}|Select-Object -First 1;if($g){Write-Output ($g.Name+'|'+[string]$g.AdapterRAM)}`); value != "" {
			parts := strings.SplitN(value, "|", 2)
			if strings.TrimSpace(parts[0]) != "" {
				result["gpu_name"] = strings.TrimSpace(parts[0])
			}
			if len(parts) == 2 {
				if bytes, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil && bytes > 0 {
					result["gpu_memory_total_bytes"] = bytes
					result["gpu_memory_total"] = humanBytes(bytes)
				}
			}
		}
	}

	// Temperature is intentionally independent and optional because many
	// Windows systems do not expose a valid ACPI thermal zone.
	if value := windowsPowerShellValue(`$t=Get-CimInstance -Namespace root/wmi -Class MSAcpi_ThermalZoneTemperature -ErrorAction SilentlyContinue|Select-Object -First 1;if($t -and $t.CurrentTemperature){$c=[math]::Round(($t.CurrentTemperature/10)-273.15,1);if($c -gt 0 -and $c -lt 150){$c}}`); value != "" {
		if c, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && c > 0 && c < 150 {
			result["temperature_c"] = c
			result["temperature"] = strings.TrimSpace(value) + " °C"
		}
	}
}

func windowsCPUPercent() (float64, bool) {
	var idle, kernel, user ecFileTime
	if ok, _, _ := ecSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)),
	); ok == 0 {
		return 0, false
	}
	i, k, u := ecFileTimeUint64(idle), ecFileTimeUint64(kernel), ecFileTimeUint64(user)
	ecCPUStateMu.Lock()
	defer ecCPUStateMu.Unlock()
	if !ecHasCPUReading {
		ecLastIdle, ecLastKernel, ecLastUser, ecHasCPUReading = i, k, u, true
		return 0, false
	}
	dk, du, di := k-ecLastKernel, u-ecLastUser, i-ecLastIdle
	ecLastIdle, ecLastKernel, ecLastUser = i, k, u
	total := dk + du
	if total == 0 {
		return 0, false
	}
	busy := total
	if di < busy {
		busy -= di
	} else {
		busy = 0
	}
	pct := float64(busy) / float64(total) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, true
}

func windowsPowerShellValue(script string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='SilentlyContinue';"+script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
