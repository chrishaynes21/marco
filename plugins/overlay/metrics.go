package main

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// pollMetrics samples system CPU/RAM usage for the optional crown widget. It runs
// regardless of the config toggle (the view decides whether to show it) — the
// cost is one ~2 s averaged CPU sample per cycle, negligible. Cross-platform via
// gopsutil (Windows/macOS/Linux).
func pollMetrics(m *model) {
	for {
		c, err := cpu.Percent(2*time.Second, false) // blocks ~2s, returns the average
		cpuPct := 0.0
		if err == nil && len(c) > 0 {
			cpuPct = c[0]
		}
		ramPct := 0.0
		if v, err := mem.VirtualMemory(); err == nil {
			ramPct = v.UsedPercent
		}
		m.setMetrics(cpuPct, ramPct)
	}
}
