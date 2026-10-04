//go:build linux

package collectors

import (
	"testing"
	"time"

	"github.com/MAWINA-ant/System_monitor/internal/config"
)

func TestNewMonitor_AllEnabled(t *testing.T) {
	m := NewMonitor(config.Default().Subsystems, time.Minute)

	if m.LoadAverage == nil || m.CPU == nil || m.DiskLoad == nil || m.DiskUsage == nil || m.NetworkStats == nil {
		t.Errorf("every enabled subsystem must have a collector: %+v", m)
	}
}

func TestNewMonitor_NothingEnabled(t *testing.T) {
	m := NewMonitor(config.Subsystems{}, time.Minute)

	if m.LoadAverage != nil || m.CPU != nil || m.DiskLoad != nil || m.DiskUsage != nil || m.NetworkStats != nil {
		t.Errorf("disabled subsystems must have no collectors: %+v", m)
	}
}

func TestNewMonitor_OnlyCPUEnabled(t *testing.T) {
	m := NewMonitor(config.Subsystems{CPU: true}, time.Minute)

	if m.CPU == nil {
		t.Error("CPU collector is missing")
	}
	if m.LoadAverage != nil || m.DiskLoad != nil || m.DiskUsage != nil || m.NetworkStats != nil {
		t.Errorf("only CPU must be enabled: %+v", m)
	}
}
