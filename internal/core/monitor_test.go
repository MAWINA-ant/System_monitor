package core

import (
	"context"
	"errors"
	"reflect"
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
		t.Errorf("expected zero-value snapshot fields, got %+v", snap)
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
