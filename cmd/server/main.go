package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	pb "github.com/MAWINA-ant/System_monitor/api/statspb"
	"github.com/MAWINA-ant/System_monitor/internal/collectors"
	"github.com/MAWINA-ant/System_monitor/internal/config"
	"github.com/MAWINA-ant/System_monitor/internal/core"
	"github.com/MAWINA-ant/System_monitor/internal/server"
	"google.golang.org/grpc"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags, err := config.ParseFlags(args, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	cfg, err := config.Load(flags.ConfigPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := net.JoinHostPort("", strconv.Itoa(flags.Port))

	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	newMonitor := func(timespan time.Duration) *core.Monitor {
		return collectors.NewMonitor(cfg.Subsystems, timespan)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterStatsServiceServer(grpcServer, server.New(newMonitor))

	go func() {
		<-ctx.Done()
		grpcServer.Stop()
	}()

	slog.Info("server listening", "addr", lis.Addr().String(), "subsystems", cfg.Subsystems)

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}
