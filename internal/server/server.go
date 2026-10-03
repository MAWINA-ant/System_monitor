package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"github.com/MAWINA-ant/System_monitor/internal/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxSeconds limits n_seconds and m_seconds: the sampling history grows with the window.
const maxSeconds = 3600

// MonitorFactory builds a Monitor for a single client stream averaging over timespan.
type MonitorFactory func(timespan time.Duration) *core.Monitor

type Server struct {
	pb.UnimplementedStatsServiceServer

	newMonitor MonitorFactory
	second     time.Duration
}

func New(newMonitor MonitorFactory) *Server {
	return &Server{newMonitor: newMonitor, second: time.Second}
}

// GetStats streams a snapshot every n_seconds averaged over the last m_seconds.
func (s *Server) GetStats(req *pb.StatsRequest, stream pb.StatsService_GetStatsServer) error {
	interval, timespan, err := s.parseRequest(req)
	if err != nil {
		return err
	}

	monitor := s.newMonitor(timespan)

	err = monitor.Run(stream.Context(), interval, timespan, func(snap core.Snapshot) error {
		return stream.Send(toProto(snap))
	})

	return runErrorToStatus(err)
}

func (s *Server) parseRequest(req *pb.StatsRequest) (interval, timespan time.Duration, err error) {
	n, m := req.GetNSeconds(), req.GetMSeconds()
	if n < 1 || n > maxSeconds || m < 1 || m > maxSeconds {
		return 0, 0, status.Errorf(codes.InvalidArgument,
			"n_seconds and m_seconds must be between 1 and %d, got %d and %d", maxSeconds, n, m)
	}

	return time.Duration(n) * s.second, time.Duration(m) * s.second, nil
}

func runErrorToStatus(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}

	slog.Error("stats stream failed", "error", err)

	return status.Error(codes.Internal, err.Error())
}
