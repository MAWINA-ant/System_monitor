package core

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

type fakeLoadAverageCollector struct {
	result LoadAverage
	err    error
	calls  int
}

func (f *fakeLoadAverageCollector) Collect(_ context.Context) (LoadAverage, error) {
	f.calls++
	return f.result, f.err
}

type fakeCPUCollector struct {
	result CPULoad
	err    error
	calls  int
}

func (f *fakeCPUCollector) Collect(_ context.Context, _ time.Duration) (CPULoad, error) {
	f.calls++
	return f.result, f.err
}

func TestMonitor_Collect_AssemblesEnabledSubsystems(t *testing.T) {
	la := &fakeLoadAverageCollector{result: LoadAverage{Load1: 0.5}}
	cpu := &fakeCPUCollector{result: CPULoad{UserPercent: 10}}

	m := &Monitor{
		LoadAverage: la,
		CPU:         cpu,
		Now:         func() time.Time { return time.Unix(100, 0) },
	}

	snap, err := m.Collect(context.Background(), 15*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if la.calls != 1 {
		t.Errorf("LoadAverage.Collect calls = %d, want 1", la.calls)
	}
	if cpu.calls != 1 {
		t.Errorf("CPU.Collect calls = %d, want 1", cpu.calls)
	}
	if snap.LoadAverage.Load1 != 0.5 {
		t.Errorf("snap.LoadAverage.Load1 = %v, want 0.5", snap.LoadAverage.Load1)
	}
	if snap.CPULoad.UserPercent != 10 {
		t.Errorf("snap.CPULoad.UserPercent = %v, want 10", snap.CPULoad.UserPercent)
	}
	if !snap.Timestamp.Equal(time.Unix(100, 0)) {
		t.Errorf("snap.Timestamp = %v, want %v", snap.Timestamp, time.Unix(100, 0))
	}
}

func TestMonitor_Collect_SkipsDisabledSubsystems(t *testing.T) {
	m := &Monitor{} // все коллекторы nil — все подсистемы "отключены"

	snap, err := m.Collect(context.Background(), 15*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := Snapshot{Timestamp: snap.Timestamp}
	if !reflect.DeepEqual(snap, want) {
		t.Errorf("disabled subsystems must stay nil, got %+v", snap)
	}
}

func TestMonitor_Collect_PropagatesCollectorError(t *testing.T) {
	wantErr := errors.New("proc read failed")
	la := &fakeLoadAverageCollector{err: wantErr}
	cpu := &fakeCPUCollector{}

	m := &Monitor{LoadAverage: la, CPU: cpu}

	_, err := m.Collect(context.Background(), 15*time.Second)
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}
	if cpu.calls != 0 {
		t.Errorf("CPU.Collect should not be called after LoadAverage error, but was called %d times", cpu.calls)
	}
}

type fakeSamplingCPU struct {
	fakeCPUCollector
	started atomic.Bool
	stopped atomic.Bool
}

func (f *fakeSamplingCPU) Run(ctx context.Context) {
	f.started.Store(true)
	<-ctx.Done()
	f.stopped.Store(true)
}

func TestMonitor_Run_EmitsOnScheduleAndStopsSamplers(t *testing.T) {
	const (
		interval = 5 * time.Millisecond
		timespan = 10 * time.Millisecond
		emits    = 3
	)

	cpu := &fakeSamplingCPU{fakeCPUCollector: fakeCPUCollector{result: CPULoad{UserPercent: 7}}}
	m := &Monitor{CPU: cpu}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var snapshots []Snapshot
	start := time.Now()
	firstAt := time.Duration(0)

	err := m.Run(ctx, interval, timespan, func(s Snapshot) error {
		if len(snapshots) == 0 {
			firstAt = time.Since(start)
		}
		snapshots = append(snapshots, s)
		if len(snapshots) == emits {
			cancel()
		}

		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want %v", err, context.Canceled)
	}
	if len(snapshots) != emits {
		t.Fatalf("got %d snapshots, want %d", len(snapshots), emits)
	}
	if firstAt < timespan {
		t.Errorf("first snapshot after %v, want not earlier than %v", firstAt, timespan)
	}
	if snapshots[0].CPULoad.UserPercent != 7 {
		t.Errorf("snapshot does not contain collector data: %+v", snapshots[0])
	}
	if !cpu.started.Load() || !cpu.stopped.Load() {
		t.Errorf("sampler started=%v stopped=%v, want both true", cpu.started.Load(), cpu.stopped.Load())
	}
}

func TestMonitor_Run_CancelledBeforeFirstSnapshot(t *testing.T) {
	m := &Monitor{CPU: &fakeCPUCollector{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	emitted := false
	err := m.Run(ctx, time.Second, time.Hour, func(Snapshot) error {
		emitted = true

		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want %v", err, context.Canceled)
	}
	if emitted {
		t.Error("no snapshot must be emitted before the timespan has elapsed")
	}
}

func TestMonitor_Run_StopsOnEmitError(t *testing.T) {
	wantErr := errors.New("client is gone")
	m := &Monitor{CPU: &fakeCPUCollector{}}

	err := m.Run(context.Background(), time.Millisecond, time.Millisecond, func(Snapshot) error {
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestMonitor_Run_StopsOnCollectError(t *testing.T) {
	wantErr := errors.New("proc read failed")
	m := &Monitor{LoadAverage: &fakeLoadAverageCollector{err: wantErr}}

	err := m.Run(context.Background(), time.Millisecond, time.Millisecond, func(Snapshot) error {
		t.Error("emit must not be called when collection fails")

		return nil
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}
