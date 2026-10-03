//go:build linux

package diskload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MAWINA-ant/System_monitor/internal/core"
)

const diskStatsFixture = `   8       0 sda 1000 10 20000 500 2000 20 40000 900 0 1200 1400 0 0 0 0
   8       1 sda1 900 5 18000 450 1900 10 38000 800 0 1100 1250 0 0 0 0
   7       0 loop0 10 0 20 1 0 0 0 0 0 1 1 0 0 0 0
`

const (
	devSDA  = "sda"
	devSDB  = "sdb"
	devNVMe = "nvme0n1"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestParseDiskStats(t *testing.T) {
	got, err := parseDiskStats(strings.NewReader(diskStatsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]diskCounters{
		devSDA:  {ios: 3000, sectors: 60000},
		"sda1":  {ios: 2800, sectors: 56000},
		"loop0": {ios: 10, sectors: 20},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDiskStats() = %+v, want %+v", got, want)
	}
}

func TestParseDiskStats_SkipsMalformedLines(t *testing.T) {
	input := "garbage\n" +
		"   8 0 sda 1 2 3\n" +
		"   8 0 sdb a 0 b 0 c 0 d 0\n" +
		"   8 16 sdc 5 0 10 0 7 0 20 0 0 0 0\n"

	got, err := parseDiskStats(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]diskCounters{"sdc": {ios: 12, sectors: 30}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDiskStats() = %+v, want %+v", got, want)
	}
}

func TestLoadBetween(t *testing.T) {
	oldest := map[string]diskCounters{
		devSDA: {ios: 1000, sectors: 10000},
		devSDB: {ios: 500, sectors: 500},
		"sdc":  {ios: 100, sectors: 100},
	}
	newest := map[string]diskCounters{
		devSDA: {ios: 1500, sectors: 30000},
		devSDB: {ios: 10, sectors: 10}, // counters went backwards: device is skipped.
		"sdd":  {ios: 1, sectors: 1},   // appeared after the window start: device is skipped.
	}

	got := loadBetween(oldest, newest, 10*time.Second)

	want := []core.DiskLoad{{Device: devSDA, TPS: 50, KBPerSec: 1000}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("loadBetween() = %+v, want %+v", got, want)
	}
}

func TestLoadBetween_SortedByDevice(t *testing.T) {
	oldest := map[string]diskCounters{devSDB: {}, devSDA: {}, devNVMe: {}}
	newest := map[string]diskCounters{devSDB: {ios: 1}, devSDA: {ios: 1}, devNVMe: {ios: 1}}

	got := loadBetween(oldest, newest, time.Second)

	names := make([]string, 0, len(got))
	for _, d := range got {
		names = append(names, d.Device)
	}

	if want := []string{devNVMe, devSDA, devSDB}; !reflect.DeepEqual(names, want) {
		t.Errorf("device order = %v, want %v", names, want)
	}
}

func TestLoadBetween_ZeroElapsed(t *testing.T) {
	counters := map[string]diskCounters{"sda": {ios: 1}}

	if got := loadBetween(counters, counters, 0); got != nil {
		t.Errorf("loadBetween() with zero elapsed = %+v, want nil", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newTestCollector(t *testing.T) (*Collector, string) {
	t.Helper()

	dir := t.TempDir()
	sysBlock := filepath.Join(dir, "sys_block")
	if err := os.MkdirAll(filepath.Join(sysBlock, devSDA), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sysBlock, "loop0"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	c := New(time.Second, time.Minute)
	c.statsPath = filepath.Join(dir, "diskstats")
	c.sysBlockPath = sysBlock

	return c, c.statsPath
}

func TestCollector_Collect(t *testing.T) {
	c, statsPath := newTestCollector(t)

	var current time.Time
	c.now = func() time.Time { return current }

	current = base
	writeFile(t, statsPath, diskStatsFixture)
	c.sample()

	current = base.Add(10 * time.Second)
	writeFile(t, statsPath, `   8       0 sda 1250 10 30000 500 2250 20 50000 900 0 1200 1400 0 0 0 0
   8       1 sda1 900 5 18000 450 1900 10 38000 800 0 1100 1250 0 0 0 0
   7       0 loop0 99 0 99 1 0 0 0 0 0 1 1 0 0 0 0
`)
	c.sample()

	got, err := c.Collect(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// sda: +500 operations and +20000 sectors in 10s; sda1 (partition) and loop0 are filtered out.
	want := []core.DiskLoad{{Device: devSDA, TPS: 50, KBPerSec: 1000}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Collect() = %+v, want %+v", got, want)
	}
}

func TestCollector_Collect_NotEnoughSamples(t *testing.T) {
	c := New(time.Second, time.Minute)

	if _, err := c.Collect(context.Background(), 10*time.Second); !errors.Is(err, errNotEnoughSamples) {
		t.Errorf("got %v, want %v", err, errNotEnoughSamples)
	}
}

func TestCollector_SampleSkipsUnreadableFile(t *testing.T) {
	c := New(time.Second, time.Minute)
	c.statsPath = filepath.Join(t.TempDir(), "missing")

	c.sample()

	if got := c.history.Len(); got != 0 {
		t.Errorf("history has %d samples after a failed read, want 0", got)
	}
}

func TestCollector_RunSamplesUntilCancelled(t *testing.T) {
	c, statsPath := newTestCollector(t)
	c.interval = 5 * time.Millisecond
	writeFile(t, statsPath, diskStatsFixture)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for c.history.Len() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}

	if got := c.history.Len(); got < 3 {
		t.Errorf("history has %d samples, want at least 3", got)
	}
}
