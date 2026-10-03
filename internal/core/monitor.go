package core

import (
	"context"
	"sync"
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
		snap.LoadAverage = &la
	}

	if m.CPU != nil {
		cpu, err := m.CPU.Collect(ctx, timespan)
		if err != nil {
			return Snapshot{}, err
		}
		snap.CPULoad = &cpu
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
		snap.NetworkTopTalkers = &tt
	}

	if m.NetworkStats != nil {
		ns, err := m.NetworkStats.Collect(ctx)
		if err != nil {
			return Snapshot{}, err
		}
		snap.NetworkStats = &ns
	}

	return snap, nil
}

// Run emits a snapshot every interval, each averaged over the last timespan, until ctx is cancelled.
// The first snapshot is emitted when timespan has elapsed. Background samplers of the collectors
// are running for the duration of the call.
func (m *Monitor) Run(ctx context.Context, interval, timespan time.Duration, emit func(Snapshot) error) error {
	ctx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	for _, r := range m.runners() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Run(ctx)
		}()
	}

	scheduler := NewScheduler(interval, timespan, time.Now())

	timer := time.NewTimer(time.Until(scheduler.NextEmitAt()))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}

		snap, err := m.Collect(ctx, timespan)
		if err != nil {
			return err
		}

		if err := emit(snap); err != nil {
			return err
		}

		scheduler.Advance()
		timer.Reset(time.Until(scheduler.NextEmitAt()))
	}
}

func (m *Monitor) runners() []Runner {
	collectors := []any{m.LoadAverage, m.CPU, m.DiskLoad, m.DiskUsage, m.NetworkTopTalkers, m.NetworkStats}

	var runners []Runner
	for _, c := range collectors {
		if r, ok := c.(Runner); ok {
			runners = append(runners, r)
		}
	}

	return runners
}
