//go:build linux

package cpu

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MAWINA-ant/System_monitor/internal/core"
)

const statFixture = `cpu  100 20 50 800 30 5 15 10 0 0
cpu0 50 10 25 400 15 2 7 5 0 0
intr 12345
ctxt 6789
`

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestParseCPUTimes(t *testing.T) {
	got, err := parseCPUTimes(strings.NewReader(statFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := cpuTimes{user: 120, system: 80, idle: 830}
	if got != want {
		t.Errorf("parseCPUTimes() = %+v, want %+v", got, want)
	}
}

func TestParseCPUTimes_OldKernelWithoutSteal(t *testing.T) {
	got, err := parseCPUTimes(strings.NewReader("cpu  10 0 5 80\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := cpuTimes{user: 10, system: 5, idle: 80}
	if got != want {
		t.Errorf("parseCPUTimes() = %+v, want %+v", got, want)
	}
}

func TestParseCPUTimes_Errors(t *testing.T) {
	cases := map[string]string{
		"no aggregate line": "cpu0 1 2 3 4\nintr 1\n",
		"too few fields":    "cpu  1 2\n",
		"not a number":      "cpu  a b c d\n",
		"empty":             "",
	}

	for name, input := range cases {
		if _, err := parseCPUTimes(strings.NewReader(input)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestLoadBetween(t *testing.T) {
	oldest := cpuTimes{user: 100, system: 50, idle: 850}
	newest := cpuTimes{user: 200, system: 100, idle: 1700}

	got, err := loadBetween(oldest, newest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := core.CPULoad{UserPercent: 10, SystemPercent: 5, IdlePercent: 85}
	if got != want {
		t.Errorf("loadBetween() = %+v, want %+v", got, want)
	}
}

func TestLoadBetween_Errors(t *testing.T) {
	same := cpuTimes{user: 1, system: 1, idle: 1}

	if _, err := loadBetween(same, same); !errors.Is(err, errNoTimeElapsed) {
		t.Errorf("identical samples: got %v, want %v", err, errNoTimeElapsed)
	}

	if _, err := loadBetween(cpuTimes{user: 10}, cpuTimes{user: 5}); !errors.Is(err, errCountersReset) {
		t.Errorf("decreasing counters: got %v, want %v", err, errCountersReset)
	}
}

func writeStat(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCollector_Collect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stat")

	var current time.Time
	c := New(time.Second, time.Minute)
	c.statPath = path
	c.now = func() time.Time { return current }

	current = base
	writeStat(t, path, "cpu  100 0 50 850 0 0 0 0\n")
	c.sample()

	current = base.Add(10 * time.Second)
	writeStat(t, path, "cpu  200 0 100 1700 0 0 0 0\n")
	c.sample()

	got, err := c.Collect(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := core.CPULoad{UserPercent: 10, SystemPercent: 5, IdlePercent: 85}
	if got != want {
		t.Errorf("Collect() = %+v, want %+v", got, want)
	}
}

func TestCollector_Collect_UsesFreshSample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stat")

	var current time.Time
	c := New(time.Second, time.Minute)
	c.statPath = path
	c.now = func() time.Time { return current }

	current = base
	writeStat(t, path, "cpu  100 0 50 850 0 0 0 0\n")
	c.sample()

	// The sampler has not ticked again yet: Collect must not depend on it.
	current = base.Add(time.Second)
	writeStat(t, path, "cpu  200 0 100 1700 0 0 0 0\n")

	got, err := c.Collect(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := core.CPULoad{UserPercent: 10, SystemPercent: 5, IdlePercent: 85}
	if got != want {
		t.Errorf("Collect() = %+v, want %+v", got, want)
	}
}

func TestCollector_Collect_NotEnoughSamples(t *testing.T) {
	c := New(time.Second, time.Minute)
	c.statPath = filepath.Join(t.TempDir(), "missing")

	if _, err := c.Collect(context.Background(), 10*time.Second); !errors.Is(err, errNotEnoughSamples) {
		t.Errorf("got %v, want %v", err, errNotEnoughSamples)
	}
}

func TestCollector_SampleSkipsUnreadableFile(t *testing.T) {
	c := New(time.Second, time.Minute)
	c.statPath = filepath.Join(t.TempDir(), "missing")

	c.sample()

	if got := c.history.Len(); got != 0 {
		t.Errorf("history has %d samples after a failed read, want 0", got)
	}
}

func TestCollector_RunSamplesUntilCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stat")
	writeStat(t, path, statFixture)

	c := New(5*time.Millisecond, time.Minute)
	c.statPath = path

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
