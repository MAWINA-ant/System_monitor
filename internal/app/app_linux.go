//go:build linux

package app

import (
	"time"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"github.com/MAWINA-ant/System_monitor/internal/collectors"
	"github.com/MAWINA-ant/System_monitor/internal/config"
	"github.com/MAWINA-ant/System_monitor/internal/core"
	"github.com/MAWINA-ant/System_monitor/internal/server"
	"google.golang.org/grpc"
)

// NewGRPCServer builds a gRPC server that serves statistics of the subsystems enabled in cfg.
func NewGRPCServer(cfg config.Config) *grpc.Server {
	newMonitor := func(timespan time.Duration) *core.Monitor {
		return collectors.NewMonitor(cfg.Subsystems, timespan)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterStatsServiceServer(grpcServer, server.New(newMonitor))

	return grpcServer
}
