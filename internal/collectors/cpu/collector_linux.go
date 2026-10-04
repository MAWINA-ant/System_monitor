//go:build linux

package cpu

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MAWINA-ant/System_monitor/internal/core"
	"github.com/MAWINA-ant/System_monitor/internal/history"
)

const (
	defaultStatPath = "/proc/stat"

	minCPUFields = 5 // "cpu" label plus user, nice, system, idle.
	maxCPUValues = 8 // user nice system idle iowait irq softirq steal.
)

var (
	errNotEnoughSamples = errors.New("not enough cpu samples yet")
	errCountersReset    = errors.New("cpu counters went backwards")
	errNoTimeElapsed    = errors.New("no cpu time elapsed between samples")
)

var _ core.CPUCollector = (*Collector)(nil)

// cpuTimes holds cumulative CPU time counters (in clock ticks) grouped into three buckets.
type cpuTimes struct {
	user   uint64
	system uint64
	idle   uint64
}

type Collector struct {
	statPath string
	interval time.Duration
	now      func() time.Time
	history  *history.History[cpuTimes]
}

// New creates a collector that samples every interval and remembers samples for retention.
// Retention must be not less than the largest timespan passed to Collect.
func New(interval, retention time.Duration) *Collector {
	return &Collector{
		statPath: defaultStatPath,
		interval: interval,
		now:      time.Now,
		history:  history.New[cpuTimes](retention),
	}
}

// Run samples CPU counters in the background until ctx is cancelled.
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
	times, err := c.readTimes()
	if err != nil {
		return
	}

	c.history.Add(c.now(), times)
}

// Collect takes a fresh sample first, so the window ends right now and does not depend on the sampler tick.
func (c *Collector) Collect(_ context.Context, timespan time.Duration) (core.CPULoad, error) {
	c.sample()

	oldest, newest, ok := c.history.Window(timespan)
	if !ok {
		return core.CPULoad{}, errNotEnoughSamples
	}

	return loadBetween(oldest.Value, newest.Value)
}

func (c *Collector) readTimes() (cpuTimes, error) {
	f, err := os.Open(c.statPath)
	if err != nil {
		return cpuTimes{}, fmt.Errorf("open %s: %w", c.statPath, err)
	}
	defer f.Close()

	times, err := parseCPUTimes(f)
	if err != nil {
		return cpuTimes{}, fmt.Errorf("parse %s: %w", c.statPath, err)
	}

	return times, nil
}

func loadBetween(oldest, newest cpuTimes) (core.CPULoad, error) {
	if newest.user < oldest.user || newest.system < oldest.system || newest.idle < oldest.idle {
		return core.CPULoad{}, errCountersReset
	}

	user := newest.user - oldest.user
	system := newest.system - oldest.system
	idle := newest.idle - oldest.idle

	total := user + system + idle
	if total == 0 {
		return core.CPULoad{}, errNoTimeElapsed
	}

	return core.CPULoad{
		UserPercent:   float64(user) / float64(total) * 100,
		SystemPercent: float64(system) / float64(total) * 100,
		IdlePercent:   float64(idle) / float64(total) * 100,
	}, nil
}

// parseCPUTimes reads the aggregate "cpu" line of /proc/stat.
// user includes nice, system includes irq, softirq and steal, idle includes iowait.
func parseCPUTimes(r io.Reader) (cpuTimes, error) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || fields[0] != "cpu" {
			continue
		}

		return parseCPULine(fields)
	}

	if err := scanner.Err(); err != nil {
		return cpuTimes{}, err
	}

	return cpuTimes{}, errors.New("aggregate cpu line not found")
}

func parseCPULine(fields []string) (cpuTimes, error) {
	if len(fields) < minCPUFields {
		return cpuTimes{}, fmt.Errorf("unexpected cpu line: %q", strings.Join(fields, " "))
	}

	var v [maxCPUValues]uint64
	for i := 0; i < maxCPUValues && i+1 < len(fields); i++ {
		n, err := strconv.ParseUint(fields[i+1], 10, 64)
		if err != nil {
			return cpuTimes{}, fmt.Errorf("parse cpu counter %q: %w", fields[i+1], err)
		}
		v[i] = n
	}

	return cpuTimes{
		user:   v[0] + v[1],
		system: v[2] + v[5] + v[6] + v[7],
		idle:   v[3] + v[4],
	}, nil
}
