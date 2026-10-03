package history

import (
	"testing"
	"time"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func at(sec int) time.Time {
	return base.Add(time.Duration(sec) * time.Second)
}

func TestHistory_WindowNeedsTwoSamples(t *testing.T) {
	h := New[int](time.Minute)

	if _, _, ok := h.Window(10 * time.Second); ok {
		t.Fatal("empty history must not produce a window")
	}

	h.Add(at(0), 1)
	if _, _, ok := h.Window(10 * time.Second); ok {
		t.Fatal("single sample must not produce a window")
	}
}

func TestHistory_WindowPicksSampleAtWindowStart(t *testing.T) {
	h := New[int](time.Minute)
	for sec := 0; sec <= 20; sec++ {
		h.Add(at(sec), sec)
	}

	oldest, newest, ok := h.Window(15 * time.Second)
	if !ok {
		t.Fatal("expected a window")
	}
	if oldest.Value != 5 || newest.Value != 20 {
		t.Errorf("window = [%d..%d], want [5..20]", oldest.Value, newest.Value)
	}
}

func TestHistory_WindowFallsBackToEarliestSample(t *testing.T) {
	h := New[int](time.Minute)
	h.Add(at(0), 0)
	h.Add(at(1), 1)
	h.Add(at(2), 2)

	oldest, newest, ok := h.Window(15 * time.Second)
	if !ok {
		t.Fatal("expected a window")
	}
	if oldest.Value != 0 || newest.Value != 2 {
		t.Errorf("window = [%d..%d], want [0..2]", oldest.Value, newest.Value)
	}
}

func TestHistory_WindowRejectsNonPositiveTimespan(t *testing.T) {
	h := New[int](time.Minute)
	h.Add(at(0), 0)
	h.Add(at(1), 1)

	if _, _, ok := h.Window(0); ok {
		t.Fatal("zero timespan must not produce a window")
	}
}

func TestHistory_AddDropsOldSamplesButKeepsWindowStart(t *testing.T) {
	h := New[int](10 * time.Second)
	for sec := 0; sec <= 30; sec++ {
		h.Add(at(sec), sec)
	}

	// retention 10s at t=30: the sample at t=20 must stay to cover the window start.
	if got := h.Len(); got != 11 {
		t.Errorf("Len() = %d, want 11", got)
	}

	oldest, _, ok := h.Window(10 * time.Second)
	if !ok || oldest.Value != 20 {
		t.Errorf("oldest = %d (ok=%v), want 20", oldest.Value, ok)
	}
}
