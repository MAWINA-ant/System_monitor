//go:build linux

package collectors

import (
	"time"

	"github.com/MAWINA-ant/System_monitor/internal/collectors/cpu"
	"github.com/MAWINA-ant/System_monitor/internal/collectors/diskload"
	"github.com/MAWINA-ant/System_monitor/internal/collectors/diskusage"
	"github.com/MAWINA-ant/System_monitor/internal/collectors/loadavg"
	"github.com/MAWINA-ant/System_monitor/internal/collectors/netstats"
	"github.com/MAWINA-ant/System_monitor/internal/config"
	"github.com/MAWINA-ant/System_monitor/internal/core"
)

const (
	sampleInterval  = time.Second
	retentionMargin = 2 * time.Second
)

// NewMonitor builds a Monitor with the subsystems enabled in cfg.
// Samplers keep history for timespan plus a small margin.
func NewMonitor(cfg config.Subsystems, timespan time.Duration) *core.Monitor {
	retention := timespan + retentionMargin

	m := &core.Monitor{}

	if cfg.LoadAverage {
		m.LoadAverage = loadavg.New()
	}
	if cfg.CPU {
		m.CPU = cpu.New(sampleInterval, retention)
	}
	if cfg.DiskLoad {
		m.DiskLoad = diskload.New(sampleInterval, retention)
	}
	if cfg.DiskUsage {
		m.DiskUsage = diskusage.New()
	}
	if cfg.NetworkStats {
		m.NetworkStats = netstats.New()
	}

	return m
}
