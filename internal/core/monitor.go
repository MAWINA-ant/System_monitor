package core

import (
	"context"
	"time"
)

// Monitor assembles a Snapshot by calling each configured collector.
// A nil collector field means that subsystem is disabled (via config).
type Monitor struct {
	LoadAverage       LoadAverageCollector
	CPU               CPUCollector
	DiskLoad          DiskLoadCollector
	DiskUsage         DiskUsageCollector
	NetworkTopTalkers NetworkTopTalkersCollector
	NetworkStats      NetworkStatsCollector

	Now func() time.Time
}

func (m *Monitor) Collect(ctx context.Context, timespan time.Duration) (Snapshot, error) {
	now := time.Now
	if m.Now != nil {
		now = m.Now
	}

	snap := Snapshot{Timestamp: now()}

	if m.LoadAverage != nil {
		la, err := m.LoadAverage.Collect(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		snap.LoadAverage = la
	}

	if m.CPU != nil {
		cpu, err := m.CPU.Collect(ctx, timespan)
		if err != nil {
			return Snapshot{}, err
		}
		snap.CPULoad = cpu
	}

	if m.DiskLoad != nil {
		dl, err := m.DiskLoad.Collect(ctx, timespan)
		if err != nil {
			return Snapshot{}, err
		}
		snap.DiskLoad = dl
	}

	if m.DiskUsage != nil {
		du, err := m.DiskUsage.Collect(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		snap.DiskUsage = du
	}

	if m.NetworkTopTalkers != nil {
		tt, err := m.NetworkTopTalkers.Collect(ctx, timespan)
		if err != nil {
			return Snapshot{}, err
		}
		snap.NetworkTopTalkers = tt
	}

	if m.NetworkStats != nil {
		ns, err := m.NetworkStats.Collect(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		snap.NetworkStats = ns
	}

	return snap, nil
}
