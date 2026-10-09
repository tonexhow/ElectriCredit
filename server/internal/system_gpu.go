package internal

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func populateGPUUtilization(result map[string]any) {
	// NVIDIA provides a lightweight cross-platform query when available.
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name,utilization.gpu,memory.total,memory.used", "--format=csv,noheader,nounits").Output()
		if err == nil {
			line := strings.Split(strings.TrimSpace(string(out)), "\n")[0]
			parts := strings.Split(line, ",")
			if len(parts) >= 4 {
				name := strings.TrimSpace(parts[0])
				percent, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				totalMB, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
				usedMB, _ := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
				if name != "" {
					result["gpu_name"] = name
				}
				result["gpu_percent"] = percent
				if totalMB > 0 {
					total := totalMB * 1024 * 1024
					result["gpu_memory_total_bytes"] = total
					result["gpu_memory_total"] = humanBytes(total)
				}
				if usedMB >= 0 {
					used := usedMB * 1024 * 1024
					result["gpu_memory_used_bytes"] = used
					result["gpu_memory_used"] = humanBytes(used)
				}
				return
			}
		}
	}

	// Raspberry Pi exposes the VideoCore memory split through vcgencmd.
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("vcgencmd"); err == nil {
			if result["gpu_name"] == "Unavailable" {
				result["gpu_name"] = "Broadcom VideoCore"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if out, err := exec.CommandContext(ctx, "vcgencmd", "get_mem", "gpu").Output(); err == nil {
				raw := strings.TrimSpace(string(out))
				raw = strings.TrimPrefix(raw, "gpu=")
				raw = strings.TrimSuffix(raw, "M")
				if mb, err := strconv.ParseInt(raw, 10, 64); err == nil && mb > 0 {
					total := mb * 1024 * 1024
					result["gpu_memory_total_bytes"] = total
					result["gpu_memory_total"] = humanBytes(total)
				}
			}
		}
	}
}
