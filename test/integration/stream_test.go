//go:build integration && linux

package integration

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"github.com/MAWINA-ant/System_monitor/internal/app"
	"github.com/MAWINA-ant/System_monitor/internal/client"
	"github.com/MAWINA-ant/System_monitor/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	testTimeout = 30 * time.Second
	protoTCP    = "tcp"
	stateEstab  = "ESTAB"
)

// startServer runs the real gRPC server with the real collectors on a free local port.
func startServer(t *testing.T, cfg config.Config) pb.StatsServiceClient {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(context.Background(), protoTCP, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := app.NewGRPCServer(cfg)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return pb.NewStatsServiceClient(conn)
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	return ctx
}

type snapshotStream struct {
	t      *testing.T
	stream pb.StatsService_GetStatsClient
	start  time.Time
}

// openStream requests a snapshot every second averaged over the last m seconds.
func openStream(ctx context.Context, t *testing.T, c pb.StatsServiceClient, m int64) *snapshotStream {
	t.Helper()

	start := time.Now()

	stream, err := c.GetStats(ctx, &pb.StatsRequest{NSeconds: 1, MSeconds: m})
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}

	return &snapshotStream{t: t, stream: stream, start: start}
}

// next waits for the next snapshot and returns it with the time elapsed since the request.
func (s *snapshotStream) next() (*pb.Snapshot, time.Duration) {
	s.t.Helper()

	snap, err := s.stream.Recv()
	if err != nil {
		s.t.Fatalf("Recv: %v", err)
	}

	return snap, time.Since(s.start)
}

func checkSnapshot(t *testing.T, s *pb.Snapshot) {
	t.Helper()

	if s.GetTimestamp() <= 0 {
		t.Errorf("timestamp = %d, want a positive Unix time", s.GetTimestamp())
	}

	if la := s.GetLoadAverage(); la == nil || la.GetLoad1() < 0 {
		t.Errorf("load average = %v, want a non-negative value", la)
	}

	if cpu := s.GetCpuLoad(); cpu == nil {
		t.Error("cpu load is missing")
	} else if sum := cpu.GetUserPercent() + cpu.GetSystemPercent() + cpu.GetIdlePercent(); sum < 99 || sum > 101 {
		t.Errorf("cpu percents add up to %.2f, want about 100: %v", sum, cpu)
	}

	if len(s.GetDiskUsage()) == 0 {
		t.Error("disk usage is empty, at least the root filesystem is expected")
	}
	for _, d := range s.GetDiskUsage() {
		if d.GetUsedPercent() < 0 || d.GetUsedPercent() > 100 {
			t.Errorf("filesystem %s: used percent = %.1f, want 0..100", d.GetMountPoint(), d.GetUsedPercent())
		}
	}

	if s.GetNetworkStats() == nil {
		t.Error("network stats are missing")
	}
}

// listenLocal opens a TCP listener on a free local port and closes it when the test ends.
func listenLocal(ctx context.Context, t *testing.T) (net.Listener, int) {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(ctx, protoTCP, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = lis.Close() })

	addr, ok := lis.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %v is not a TCP address", lis.Addr())
	}

	return lis, addr.Port
}

func waitFor(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}

	return condition()
}

// serverWork reports which parts of the server work for a stream are alive: the monitor loop and the samplers.
func serverWork() (monitor, samplers bool) {
	buf := make([]byte, 1<<22)
	stacks := string(buf[:runtime.Stack(buf, true)])

	monitor = strings.Contains(stacks, "core.(*Monitor).Run")
	samplers = strings.Contains(stacks, "collectors/cpu.(*Collector).Run") &&
		strings.Contains(stacks, "collectors/diskload.(*Collector).Run")

	return monitor, samplers
}

func serverWorking() bool {
	monitor, samplers := serverWork()

	return monitor && samplers
}

func serverIdle() bool {
	monitor, samplers := serverWork()

	return !monitor && !samplers
}

func findListening(s *pb.Snapshot, port int) *pb.ListeningSocket {
	for _, sock := range s.GetNetworkStats().GetListeningSockets() {
		if sock.GetProtocol() == protoTCP && int(sock.GetPort()) == port {
			return sock
		}
	}

	return nil
}

func establishedCount(s *pb.Snapshot) int32 {
	for _, st := range s.GetNetworkStats().GetConnectionStates() {
		if st.GetState() == stateEstab {
			return st.GetCount()
		}
	}

	return 0
}

func TestStream_DeliversSnapshotsOnSchedule(t *testing.T) {
	t.Parallel()

	const m = 2

	s := openStream(testContext(t), t, startServer(t, config.Default()), m)

	arrivals := make([]time.Duration, 0, 3)
	timestamps := make([]int64, 0, 3)

	for range 3 {
		snap, at := s.next()
		checkSnapshot(t, snap)

		arrivals = append(arrivals, at)
		timestamps = append(timestamps, snap.GetTimestamp())
	}

	if arrivals[0] < m*time.Second {
		t.Errorf("first snapshot arrived after %v, want not earlier than %ds", arrivals[0], m)
	}

	for i := 1; i < len(arrivals); i++ {
		if gap := arrivals[i] - arrivals[i-1]; gap < 500*time.Millisecond || gap > 3*time.Second {
			t.Errorf("gap between snapshots %d and %d is %v, want about 1s", i-1, i, gap)
		}
		if timestamps[i] < timestamps[i-1] {
			t.Errorf("timestamps go backwards: %v", timestamps)
		}
	}
}

func TestStream_DisabledSubsystemsAreAbsent(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Subsystems.CPU = false
	cfg.Subsystems.DiskLoad = false
	cfg.Subsystems.NetworkStats = false

	snap, _ := openStream(testContext(t), t, startServer(t, cfg), 1).next()

	if snap.GetCpuLoad() != nil {
		t.Errorf("cpu is disabled, but present: %v", snap.GetCpuLoad())
	}
	if snap.GetNetworkStats() != nil {
		t.Errorf("network stats are disabled, but present: %v", snap.GetNetworkStats())
	}
	if len(snap.GetDiskLoad()) != 0 {
		t.Errorf("disk load is disabled, but present: %v", snap.GetDiskLoad())
	}
	if snap.GetLoadAverage() == nil || len(snap.GetDiskUsage()) == 0 {
		t.Errorf("enabled subsystems must stay in the snapshot: %v", snap)
	}
}

func TestStream_ReflectsNewListeningSocket(t *testing.T) {
	t.Parallel()

	ctx := testContext(t)
	s := openStream(ctx, t, startServer(t, config.Default()), 1)

	before, _ := s.next()

	_, port := listenLocal(ctx, t)

	if findListening(before, port) != nil {
		t.Fatalf("port %d is listed before it was opened", port)
	}

	comm, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Fatalf("read own command name: %v", err)
	}

	for range 5 {
		snap, _ := s.next()

		sock := findListening(snap, port)
		if sock == nil {
			continue
		}

		if int(sock.GetPid()) != os.Getpid() {
			t.Errorf("socket owner pid = %d, want %d", sock.GetPid(), os.Getpid())
		}
		if want := strings.TrimSpace(string(comm)); sock.GetCommand() != want {
			t.Errorf("socket owner command = %q, want %q", sock.GetCommand(), want)
		}

		return
	}

	t.Fatalf("listening port %d did not appear in the next snapshots", port)
}

func TestStream_CountsEstablishedConnections(t *testing.T) {
	t.Parallel()

	const conns = 5

	ctx := testContext(t)
	s := openStream(ctx, t, startServer(t, config.Default()), 1)

	lis, _ := listenLocal(ctx, t)

	for range conns {
		conn, err := (&net.Dialer{}).DialContext(ctx, protoTCP, lis.Addr().String())
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
	}

	// Both ends of every loopback connection are in the table, so at least 2*conns are established.
	var last int32

	for range 5 {
		snap, _ := s.next()

		if last = establishedCount(snap); last >= 2*conns {
			return
		}
	}

	t.Fatalf("established connections = %d, want at least %d", last, 2*conns)
}

func TestStream_RejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	ctx := testContext(t)
	c := startServer(t, config.Default())

	stream, err := c.GetStats(ctx, &pb.StatsRequest{NSeconds: 0, MSeconds: 5})
	if err == nil {
		_, err = stream.Recv()
	}

	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("code = %v (%v), want %v", got, err, codes.InvalidArgument)
	}
}

// Not parallel: it looks at the goroutines of the whole process, so no other server may run meanwhile.
func TestStream_ClientCancellationStopsServerWork(t *testing.T) {
	ctx, cancel := context.WithCancel(testContext(t))
	// An hour-long window: no snapshot is due while the test runs, the server only waits and samples.
	s := openStream(ctx, t, startServer(t, config.Default()), 3600)

	if !waitFor(3*time.Second, serverWorking) {
		t.Fatal("the monitor and the samplers have not started after the request")
	}

	cancel()

	done := make(chan error, 1)
	go func() {
		_, err := s.stream.Recv()
		done <- err
	}()

	select {
	case err := <-done:
		if got := status.Code(err); got != codes.Canceled {
			t.Errorf("code = %v (%v), want %v", got, err, codes.Canceled)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the stream is still open after the client cancelled it")
	}

	if !waitFor(3*time.Second, serverIdle) {
		monitor, samplers := serverWork()
		t.Errorf("the server keeps working after the client is gone: monitor=%v samplers=%v", monitor, samplers)
	}
}

func TestClient_PrintsTablesFromRealServer(t *testing.T) {
	t.Parallel()

	opts, err := client.ParseFlags(nil, io.Discard)
	if err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	opts.N, opts.M = 1, 1

	c := startServer(t, config.Default())

	ctx, cancel := context.WithCancel(testContext(t))
	defer cancel()

	// An explicit cancellation, not a deadline: a deadline is also enforced by the server.
	time.AfterFunc(3500*time.Millisecond, cancel)

	var out bytes.Buffer
	if err := client.Run(ctx, c, opts, &out); err != nil {
		t.Fatalf("client.Run: %v", err)
	}

	text := out.String()

	if got := strings.Count(text, "=== "); got < 2 {
		t.Errorf("printed %d snapshots in 3.5s, want at least 2:\n%s", got, text)
	}
	wantParts := []string{"Load average:", "CPU: user", "Disk usage", "Listening sockets", "TCP connections by state"}
	for _, want := range wantParts {
		if !strings.Contains(text, want) {
			t.Errorf("output does not contain %q:\n%s", want, text)
		}
	}
}
