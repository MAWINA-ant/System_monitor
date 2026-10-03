package history

import (
	"sync"
	"time"
)

// Sample is a value observed at a specific moment.
type Sample[T any] struct {
	At    time.Time
	Value T
}

// History keeps recent samples so that callers can look at a sliding time window.
type History[T any] struct {
	mu        sync.Mutex
	retention time.Duration
	samples   []Sample[T]
}

func New[T any](retention time.Duration) *History[T] {
	return &History[T]{retention: retention}
}

// Add stores a sample and drops the ones that are no longer needed to cover the retention period.
func (h *History[T]) Add(at time.Time, value T) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.samples = append(h.samples, Sample[T]{At: at, Value: value})

	cutoff := at.Add(-h.retention)
	drop := 0
	for drop+1 < len(h.samples) && !h.samples[drop+1].At.After(cutoff) {
		drop++
	}
	h.samples = h.samples[drop:]
}

// Len returns the number of stored samples.
func (h *History[T]) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.samples)
}

// Window returns the newest sample and the latest sample taken at least timespan before it.
// If the history is shorter than timespan, the earliest sample is returned instead.
// ok is false when there is nothing to compare yet.
func (h *History[T]) Window(timespan time.Duration) (oldest, newest Sample[T], ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.samples) < 2 || timespan <= 0 {
		return Sample[T]{}, Sample[T]{}, false
	}

	newest = h.samples[len(h.samples)-1]
	cutoff := newest.At.Add(-timespan)

	oldest = h.samples[0]
	for _, s := range h.samples {
		if s.At.After(cutoff) {
			break
		}
		oldest = s
	}

	if !oldest.At.Before(newest.At) {
		return Sample[T]{}, Sample[T]{}, false
	}

	return oldest, newest, true
}
