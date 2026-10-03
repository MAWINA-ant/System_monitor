package server

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"github.com/MAWINA-ant/System_monitor/internal/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeLoadAverage struct {
	result core.LoadAverage
	err    error
}

func (f fakeLoadAverage) Collect(context.Context) (core.LoadAverage, error) {
	return f.result, f.err
}

type fakeStream struct {
	grpc.ServerStream

	ctx     context.Context
	cancel  context.CancelFunc
	limit   int
	sendErr error
	sent    []*pb.Snapshot
}

func (f *fakeStream) Context() context.Context { return f.ctx }

func (f *fakeStream) Send(s *pb.Snapshot) error {
	if f.sendErr != nil {
		return f.sendErr
	}

	f.sent = append(f.sent, s)
	if len(f.sent) == f.limit {
		f.cancel()
	}

	return nil
}

func newFakeStream(limit int) *fakeStream {
	ctx, cancel := context.WithCancel(context.Background())

	return &fakeStream{ctx: ctx, cancel: cancel, limit: limit}
}

// newTestServer makes one "second" of the protocol last a millisecond.
func newTestServer(factory MonitorFactory) *Server {
	s := New(factory)
	s.second = time.Millisecond

	return s
}

func TestServer_GetStats_StreamsSnapshots(t *testing.T) {
	var gotTimespan time.Duration

	factory := func(timespan time.Duration) *core.Monitor {
		gotTimespan = timespan

		return &core.Monitor{
			LoadAverage: fakeLoadAverage{result: core.LoadAverage{Load1: 0.5}},
			Now:         func() time.Time { return time.Unix(100, 0) },
		}
	}

	stream := newFakeStream(3)
	err := newTestServer(factory).GetStats(&pb.StatsRequest{NSeconds: 2, MSeconds: 4}, stream)

	if got := status.Code(err); got != codes.Canceled {
		t.Fatalf("GetStats() code = %v (%v), want %v", got, err, codes.Canceled)
	}
	if gotTimespan != 4*time.Millisecond {
		t.Errorf("factory got timespan %v, want 4ms", gotTimespan)
	}
	if len(stream.sent) != 3 {
		t.Fatalf("sent %d snapshots, want 3", len(stream.sent))
	}

	first := stream.sent[0]
	if first.GetLoadAverage().GetLoad1() != 0.5 || first.GetTimestamp() != 100 {
		t.Errorf("unexpected snapshot: %v", first)
	}
	if first.GetCpuLoad() != nil {
		t.Errorf("CPU is disabled, but present in the snapshot: %v", first)
	}
}

func TestServer_GetStats_InvalidRequest(t *testing.T) {
	cases := map[string]*pb.StatsRequest{
		"zero n":     {NSeconds: 0, MSeconds: 5},
		"zero m":     {NSeconds: 5, MSeconds: 0},
		"negative n": {NSeconds: -1, MSeconds: 5},
		"too big m":  {NSeconds: 5, MSeconds: maxSeconds + 1},
		"too big n":  {NSeconds: maxSeconds + 1, MSeconds: 5},
	}

	for name, req := range cases {
		factoryCalled := false
		s := newTestServer(func(time.Duration) *core.Monitor {
			factoryCalled = true

			return &core.Monitor{}
		})

		err := s.GetStats(req, newFakeStream(1))

		if got := status.Code(err); got != codes.InvalidArgument {
			t.Errorf("%s: code = %v, want %v", name, got, codes.InvalidArgument)
		}
		if factoryCalled {
			t.Errorf("%s: monitor must not be created for an invalid request", name)
		}
	}
}

func TestServer_GetStats_CollectorFailure(t *testing.T) {
	s := newTestServer(func(time.Duration) *core.Monitor {
		return &core.Monitor{LoadAverage: fakeLoadAverage{err: errors.New("proc read failed")}}
	})

	err := s.GetStats(&pb.StatsRequest{NSeconds: 1, MSeconds: 1}, newFakeStream(1))

	if got := status.Code(err); got != codes.Internal {
		t.Errorf("code = %v (%v), want %v", got, err, codes.Internal)
	}
}

func TestServer_GetStats_SendFailure(t *testing.T) {
	s := newTestServer(func(time.Duration) *core.Monitor {
		return &core.Monitor{LoadAverage: fakeLoadAverage{}}
	})

	stream := newFakeStream(1)
	stream.sendErr = errors.New("connection reset")

	err := s.GetStats(&pb.StatsRequest{NSeconds: 1, MSeconds: 1}, stream)

	if got := status.Code(err); got != codes.Internal {
		t.Errorf("code = %v (%v), want %v", got, err, codes.Internal)
	}
}
