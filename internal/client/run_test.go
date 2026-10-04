package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"google.golang.org/grpc"
)

type fakeStream struct {
	grpc.ClientStream

	snapshots []*pb.Snapshot
	err       error
}

func (f *fakeStream) Recv() (*pb.Snapshot, error) {
	if len(f.snapshots) > 0 {
		snap := f.snapshots[0]
		f.snapshots = f.snapshots[1:]

		return snap, nil
	}

	if f.err != nil {
		return nil, f.err
	}

	return nil, io.EOF
}

type fakeClient struct {
	stream   pb.StatsService_GetStatsClient
	startErr error
	request  *pb.StatsRequest
}

func (f *fakeClient) GetStats(
	_ context.Context,
	in *pb.StatsRequest,
	_ ...grpc.CallOption,
) (pb.StatsService_GetStatsClient, error) {
	f.request = in

	return f.stream, f.startErr
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

var cpuOnly = Options{N: 2, M: 10, Sections: Sections{SectionCPU: true}}

func TestRun_PrintsEverySnapshotUntilStreamEnds(t *testing.T) {
	cpu := &pb.CPULoad{UserPercent: 1, SystemPercent: 2, IdlePercent: 97}
	c := &fakeClient{stream: &fakeStream{snapshots: []*pb.Snapshot{
		{Timestamp: testTimestamp, CpuLoad: cpu},
		{Timestamp: testTimestamp + 5, CpuLoad: cpu},
	}}}

	var out bytes.Buffer
	if err := Run(context.Background(), c, cpuOnly, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := strings.Count(out.String(), "CPU: user 1.0%  system 2.0%  idle 97.0%"); got != 2 {
		t.Errorf("printed %d snapshots, want 2:\n%s", got, out.String())
	}
	if c.request.GetNSeconds() != 2 || c.request.GetMSeconds() != 10 {
		t.Errorf("request = %v, want n=2 m=10", c.request)
	}
}

func TestRun_StartError(t *testing.T) {
	c := &fakeClient{startErr: errors.New("unavailable")}

	err := Run(context.Background(), c, cpuOnly, io.Discard)

	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("Run() error = %v, want it to wrap the start error", err)
	}
}

func TestRun_ReceiveError(t *testing.T) {
	c := &fakeClient{stream: &fakeStream{err: errors.New("connection lost")}}

	err := Run(context.Background(), c, cpuOnly, io.Discard)

	if err == nil || !strings.Contains(err.Error(), "connection lost") {
		t.Errorf("Run() error = %v, want it to wrap the receive error", err)
	}
}

func TestRun_CancelledContextIsNotAnError(t *testing.T) {
	c := &fakeClient{stream: &fakeStream{err: errors.New("rpc error: code = Canceled")}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Run(ctx, c, cpuOnly, io.Discard); err != nil {
		t.Errorf("Run() error = %v, want nil after cancellation", err)
	}
}

func TestRun_WriteError(t *testing.T) {
	c := &fakeClient{stream: &fakeStream{snapshots: []*pb.Snapshot{{Timestamp: testTimestamp}}}}

	err := Run(context.Background(), c, cpuOnly, failingWriter{})

	if err == nil || !strings.Contains(err.Error(), "broken pipe") {
		t.Errorf("Run() error = %v, want it to wrap the write error", err)
	}
}
