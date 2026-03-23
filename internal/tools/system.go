package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemInfo returns host, CPU, memory, and disk summary.
func SystemInfo() ([]byte, error) {
	h, err := host.Info()
	if err != nil {
		return nil, err
	}
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	c, err := cpu.Counts(true)
	if err != nil {
		c = runtime.NumCPU()
	}
	wd, _ := os.Getwd()
	parts, err := disk.Partitions(false)
	if err != nil {
		parts = nil
	}
	var usage []map[string]any
	for _, p := range parts {
		if u, err := disk.Usage(p.Mountpoint); err == nil {
			usage = append(usage, map[string]any{
				"mountpoint": p.Mountpoint,
				"total_gb":   fmt.Sprintf("%.2f", float64(u.Total)/(1024*1024*1024)),
				"used_pct":   u.UsedPercent,
			})
			if len(usage) >= 8 {
				break
			}
		}
	}
	out := map[string]any{
		"os":           h.OS,
		"platform":     h.Platform,
		"platform_ver": h.PlatformVersion,
		"kernel":       h.KernelVersion,
		"hostname":     h.Hostname,
		"cpu_cores":    c,
		"mem_total_gb": fmt.Sprintf("%.2f", float64(v.Total)/(1024*1024*1024)),
		"mem_used_pct": v.UsedPercent,
		"cwd":          wd,
		"disk":         usage,
	}
	return json.Marshal(out)
}
