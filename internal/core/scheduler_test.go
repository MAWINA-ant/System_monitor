package core

import (
	"testing"
	"time"
)

func TestScheduler_NextEmitAt(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewScheduler(5*time.Second, 15*time.Second, start)

	want := start.Add(15 * time.Second)
	if got := s.NextEmitAt(); !got.Equal(want) {
		t.Fatalf("first NextEmitAt() = %v, want %v", got, want)
	}

	s.Advance()
	want = start.Add(20 * time.Second)
	if got := s.NextEmitAt(); !got.Equal(want) {
		t.Fatalf("second NextEmitAt() = %v, want %v", got, want)
	}

	s.Advance()
	want = start.Add(25 * time.Second)
	if got := s.NextEmitAt(); !got.Equal(want) {
		t.Fatalf("third NextEmitAt() = %v, want %v", got, want)
	}
}

func TestScheduler_DifferentIntervalAndTimespan(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		advanceFirst int
		want         time.Duration
	}{
		{0, 30 * time.Second},
		{1, 40 * time.Second},
		{4, 70 * time.Second},
	}

	for _, tc := range cases {
		s := NewScheduler(10*time.Second, 30*time.Second, start)
		for i := 0; i < tc.advanceFirst; i++ {
			s.Advance()
		}
		if got := s.NextEmitAt(); got != start.Add(tc.want) {
			t.Fatalf("after %d advances: got %v, want start+%v", tc.advanceFirst, got, tc.want)
		}
	}
}
