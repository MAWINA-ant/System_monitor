package core

import (
	"context"
	"time"
)

type LoadAverageCollector interface {
	Collect(ctx context.Context) (LoadAverage, error)
}

type CPUCollector interface {
	Collect(ctx context.Context, timespan time.Duration) (CPULoad, error)
}

type DiskLoadCollector interface {
	Collect(ctx context.Context, timespan time.Duration) ([]DiskLoad, error)
}

type DiskUsageCollector interface {
	Collect(ctx context.Context) ([]DiskUsage, error)
}

type NetworkTopTalkersCollector interface {
	Collect(ctx context.Context, timespan time.Duration) (NetworkTopTalkers, error)
}

type NetworkStatsCollector interface {
	Collect(ctx context.Context) (NetworkStats, error)
}
