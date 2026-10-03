//go:build linux

package diskload

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MAWINA-ant/System_monitor/internal/core"
	"github.com/MAWINA-ant/System_monitor/internal/history"
)

const (
	defaultStatsPath    = "/proc/diskstats"
	defaultSysBlockPath = "/sys/block"

	minDiskStatsFields = 10 // major minor name and counters up to sectors written.
	kbPerSector        = 0.5
)

var errNotEnoughSamples = errors.New("not enough disk samples yet")

var _ core.DiskLoadCollector = (*Collector)(nil)

// diskCounters holds cumulative per-device counters.
type diskCounters struct {
	ios     uint64
	sectors uint64
}

type Collector struct {
	statsPath    string
	sysBlockPath string
	interval     time.Duration
	now          func() time.Time
	history      *history.History[map[string]diskCounters]
}

// New creates a collector that samples every interval and remembers samples for retention.
// Retention must be not less than the largest timespan passed to Collect.
func New(interval, retention time.Duration) *Collector {
	return &Collector{
		statsPath:    defaultStatsPath,
		sysBlockPath: defaultSysBlockPath,
		interval:     interval,
		now:          time.Now,
		history:      history.New[map[string]diskCounters](retention),
	}
}

// Run samples disk counters in the background until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) {
	c.sample()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.sample()
		}
	}
}

// sample skips the tick on read errors: Collect reports the lack of samples if it persists.
func (c *Collector) sample() {
	counters, err := c.readCounters()
	if err != nil {
		return
	}

	c.history.Add(c.now(), counters)
}

func (c *Collector) Collect(_ context.Context, timespan time.Duration) ([]core.DiskLoad, error) {
	oldest, newest, ok := c.history.Window(timespan)
	if !ok {
		return nil, errNotEnoughSamples
	}

	return loadBetween(oldest.Value, newest.Value, newest.At.Sub(oldest.At)), nil
}

func (c *Collector) readCounters() (map[string]diskCounters, error) {
	f, err := os.Open(c.statsPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", c.statsPath, err)
	}
	defer f.Close()

	all, err := parseDiskStats(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", c.statsPath, err)
	}

	for name := range all {
		if !c.isPhysicalDisk(name) {
			delete(all, name)
		}
	}

	return all, nil
}

// isPhysicalDisk keeps whole block devices and drops partitions and loop/ram devices.
func (c *Collector) isPhysicalDisk(name string) bool {
	if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
		return false
	}

	_, err := os.Stat(filepath.Join(c.sysBlockPath, name))

	return err == nil
}

func loadBetween(oldest, newest map[string]diskCounters, elapsed time.Duration) []core.DiskLoad {
	seconds := elapsed.Seconds()
	if seconds <= 0 {
		return nil
	}

	result := make([]core.DiskLoad, 0, len(newest))

	for name, n := range newest {
		o, ok := oldest[name]
		if !ok || n.ios < o.ios || n.sectors < o.sectors {
			continue
		}

		result = append(result, core.DiskLoad{
			Device:   name,
			TPS:      float64(n.ios-o.ios) / seconds,
			KBPerSec: float64(n.sectors-o.sectors) * kbPerSector / seconds,
		})
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Device < result[j].Device })

	return result
}

func parseDiskStats(r io.Reader) (map[string]diskCounters, error) {
	result := make(map[string]diskCounters)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		name, counters, ok := parseDiskStatsLine(scanner.Text())
		if ok {
			result[name] = counters
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func parseDiskStatsLine(line string) (string, diskCounters, bool) {
	fields := strings.Fields(line)
	if len(fields) < minDiskStatsFields {
		return "", diskCounters{}, false
	}

	var v [4]uint64
	for i, idx := range [...]int{3, 5, 7, 9} { // reads, sectors read, writes, sectors written.
		n, err := strconv.ParseUint(fields[idx], 10, 64)
		if err != nil {
			return "", diskCounters{}, false
		}
		v[i] = n
	}

	return fields[2], diskCounters{ios: v[0] + v[2], sectors: v[1] + v[3]}, true
}
