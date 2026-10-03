package core

import "time"

// Scheduler decides when the next snapshot should be emitted, following
// the rule: stay silent for `timespan`, then emit every `interval`.
type Scheduler struct {
	interval time.Duration
	timespan time.Duration
	start    time.Time
	emitted  int
}

func NewScheduler(interval, timespan time.Duration, start time.Time) *Scheduler {
	return &Scheduler{interval: interval, timespan: timespan, start: start}
}

// NextEmitAt returns the absolute time at which the next snapshot is due.
func (s *Scheduler) NextEmitAt() time.Time {
	return s.start.Add(s.timespan + time.Duration(s.emitted)*s.interval)
}

// Advance marks the current snapshot as emitted and moves to the next one.
func (s *Scheduler) Advance() {
	s.emitted++
}
